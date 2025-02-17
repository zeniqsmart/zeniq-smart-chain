package filters

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum/go-ethereum/common"
	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethfilters "github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/zeniqsmart/zeniq-smart-chain/rpc/internal/ethapi"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	mapi "github.com/zeniqsmart/zeniq-smart-chain/api"
)

var _ PublicFilterAPI = (*filterAPI)(nil)

var (
	deadline = 5 * time.Minute // consider a filter inactive if it has not been polled for within deadline
)

type PublicFilterAPI interface {
	GetFilterChanges(id rpc.ID) (interface{}, error)
	GetFilterLogs(id rpc.ID) ([]*gethtypes.Log, error)
	GetLogs(crit gethfilters.FilterCriteria) ([]*gethtypes.Log, error)
	NewBlockFilter() rpc.ID
	NewFilter(crit gethfilters.FilterCriteria) (rpc.ID, error)
	NewPendingTransactionFilter(fullTx *bool) rpc.ID
	UninstallFilter(id rpc.ID) bool
	NewHeads(ctx context.Context) (*rpc.Subscription, error)
	Logs(ctx context.Context, crit gethfilters.FilterCriteria) (*rpc.Subscription, error)
}

type filterAPI struct {
	backend   mapi.BackendService
	events    *EventSystem
	filtersMu sync.Mutex
	filters   map[rpc.ID]*filter
	logger    log.Logger
}

// filter is a helper struct that holds meta information over the filter type
// and associated subscription in the event system.
type filter struct {
	typ      Type
	deadline *time.Timer // filter is inactive when deadline triggers
	hashes   []gethcmn.Hash
	fullTx   bool
	txs      []*gethtypes.Transaction
	crit     gethfilters.FilterCriteria
	logs     []*gethtypes.Log
	s        *Subscription // associated subscription in event system
}

func NewAPI(backend mapi.BackendService, logger log.Logger) PublicFilterAPI {
	_api := &filterAPI{
		backend: backend,
		filters: make(map[rpc.ID]*filter),
		events:  NewEventSystem(backend, false),
		logger:  logger,
	}

	go _api.timeoutLoop()
	return _api
}

// timeoutLoop runs every 5 minutes and deletes filters that have not been recently used.
// Tt is started when the api is created.
func (api *filterAPI) timeoutLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		api.filtersMu.Lock()
		for id, f := range api.filters {
			select {
			case <-f.deadline.C:
				f.s.Unsubscribe()
				delete(api.filters, id)
			default:
				continue
			}
		}
		api.filtersMu.Unlock()
	}
}

// NewFilter creates a new filter and returns the filter id. It can be
// used to retrieve logs when the state changes. This method cannot be
// used to fetch logs that are already stored in the state.
//
// Default criteria for the from and to block are "latest".
// Using "latest" as block number will return logs for mined blocks.
// Using "pending" as block number returns logs for not yet mined (pending) blocks.
// In case logs are removed (chain reorg) previously returned logs are returned
// again but with the removed property set to true.
//
// In case "fromBlock" > "toBlock" an error is returned.
//
// https://eth.wiki/json-rpc/API#eth_newFilter
func (api *filterAPI) NewFilter(crit gethfilters.FilterCriteria) (filterID rpc.ID, err error) {
	api.logger.Debug("eth_newFilter")
	logs := make(chan []*gethtypes.Log)
	logsSub, err := api.events.SubscribeLogs(ethereum.FilterQuery(crit), logs)
	if err != nil {
		return "", err
	}

	api.filtersMu.Lock()
	api.filters[logsSub.ID] = &filter{
		typ:      LogsSubscription,
		crit:     crit,
		deadline: time.NewTimer(deadline),
		logs:     make([]*gethtypes.Log, 0),
		s:        logsSub,
	}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case l := <-logs:
				api.filtersMu.Lock()
				if f, found := api.filters[logsSub.ID]; found {
					f.logs = append(f.logs, l...)
				}
				api.filtersMu.Unlock()
			case <-logsSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, logsSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return logsSub.ID, nil
}

// NewPendingTransactionFilter creates a filter that fetches pending transactions
// as transactions enter the pending state.
//
// It is part of the filter package because this filter can be used through the
// `eth_getFilterChanges` polling method that is also used for log filters.
func (api *filterAPI) NewPendingTransactionFilter(fullTx *bool) rpc.ID {
	api.logger.Debug("eth_newPendingTransactionFilter")
	logs := make(chan []*gethtypes.Log)
	var (
		pendingTxs   = make(chan []*gethtypes.Transaction)
		pendingTxSub = api.events.SubscribePendingTxs(pendingTxs, logs)
	)
	api.filtersMu.Lock()
	api.filters[pendingTxSub.ID] = &filter{typ: PendingTransactionsSubscription, fullTx: fullTx != nil && *fullTx, deadline: time.NewTimer(deadline), txs: make([]*gethtypes.Transaction, 0), s: pendingTxSub}
	api.filtersMu.Unlock()
	go func() {
		for {
			select {
			case pTx := <-pendingTxs:
				api.filtersMu.Lock()
				if f, found := api.filters[pendingTxSub.ID]; found {
					f.txs = append(f.txs, pTx...)
				}
				api.filtersMu.Unlock()
			case <-pendingTxSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, pendingTxSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()
	return pendingTxSub.ID
}

// NewBlockFilter creates a filter that fetches blocks that are imported into the chain.
// It is part of the filter package since polling goes with eth_getFilterChanges.
//
// https://eth.wiki/json-rpc/API#eth_newblockfilter
func (api *filterAPI) NewBlockFilter() rpc.ID {
	api.logger.Debug("eth_newBlockFilter")
	var (
		headers   = make(chan *types.Header)
		headerSub = api.events.SubscribeNewHeads(headers)
	)

	api.filtersMu.Lock()
	api.filters[headerSub.ID] = &filter{
		typ:      BlocksSubscription,
		deadline: time.NewTimer(deadline),
		hashes:   make([]gethcmn.Hash, 0),
		s:        headerSub,
	}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case h := <-headers:
				api.filtersMu.Lock()
				if f, found := api.filters[headerSub.ID]; found {
					f.hashes = append(f.hashes, h.Hash())
				}
				api.filtersMu.Unlock()
			case <-headerSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, headerSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return headerSub.ID
}

// UninstallFilter removes the filter with the given filter id.
//
// https://eth.wiki/json-rpc/API#eth_uninstallfilter
func (api *filterAPI) UninstallFilter(id rpc.ID) bool {
	api.logger.Debug("eth_uninstallFilter")
	api.filtersMu.Lock()
	f, found := api.filters[id]
	if found {
		delete(api.filters, id)
	}
	api.filtersMu.Unlock()
	if found {
		f.s.Unsubscribe()
	}

	return found
}

// GetFilterChanges returns the logs for the filter with the given id since
// last time it was called. This can be used for polling.
//
// For pending transaction and block filters the result is []common.Hash.
// (pending)Log filters return []Log.
//
// https://eth.wiki/json-rpc/API#eth_getfilterchanges
func (api *filterAPI) GetFilterChanges(id rpc.ID) (interface{}, error) {
	api.logger.Debug("eth_uninstallFilter")
	api.filtersMu.Lock()
	defer api.filtersMu.Unlock()

	f, found := api.filters[id]
	if !found {
		return nil, fmt.Errorf("filter %s not found", id)
	}

	if !f.deadline.Stop() {
		// timer expired but filter is not yet removed in timeout loop
		// receive timer value and reset timer
		<-f.deadline.C
	}
	f.deadline.Reset(deadline)

	switch f.typ {
	case PendingTransactionsSubscription:
		if f.fullTx {
			txs := make([]*ethapi.Transaction, 0, len(f.txs))
			for _, tx := range f.txs {
				v, r, s := tx.RawSignatureValues()
				signer := gethtypes.NewEIP155Signer(tx.ChainId())
				from, _ := signer.Sender(tx)
				index := uint64(0)
				txs = append(txs, &ethapi.Transaction{
					BlockHash:        nil,
					BlockNumber:      (*hexutil.Big)(new(big.Int).SetUint64(0)),
					TransactionIndex: (*hexutil.Uint64)(&index),
					From:             from,
					Gas:              hexutil.Uint64(tx.Gas()),
					GasPrice:         (*hexutil.Big)(tx.GasPrice()),
					Hash:             tx.Hash(),
					Input:            hexutil.Bytes(tx.Data()),
					Nonce:            hexutil.Uint64(tx.Nonce()),
					To:               tx.To(),
					Value:            (*hexutil.Big)(tx.Value()),
					V:                (*hexutil.Big)(v),
					R:                (*hexutil.Big)(r),
					S:                (*hexutil.Big)(s),
				})

				//NewRPCPendingTransaction(tx, rpc.LatestBlockNumber, chainConfig))
			}
			f.txs = nil
			return txs, nil
		} else {
			hashes := make([]common.Hash, 0, len(f.txs))
			for _, tx := range f.txs {
				hashes = append(hashes, tx.Hash())
			}
			f.txs = nil
			return hashes, nil
		}
	case BlocksSubscription:
		hashes := f.hashes
		f.hashes = nil
		return returnHashes(hashes), nil
	//P case LogsSubscription, MinedAndPendingLogsSubscription:
	case LogsSubscription /*, MinedAndPendingLogsSubscription*/ :
		logs := make([]*gethtypes.Log, len(f.logs))
		copy(logs, f.logs)
		f.logs = []*gethtypes.Log{}
		return returnLogs(logs), nil
	default:
		return nil, fmt.Errorf("invalid filter %s type %d", id, f.typ)
	}
}

// GetFilterLogs returns the logs for the filter with the given id.
// If the filter could not be found an empty array of logs is returned.
//
// https://eth.wiki/json-rpc/API#eth_getfilterlogs
func (api *filterAPI) GetFilterLogs(id rpc.ID) ([]*gethtypes.Log, error) {
	api.logger.Debug("eth_getFilterLogs")
	api.filtersMu.Lock()
	f, found := api.filters[id]
	api.filtersMu.Unlock()

	if !found || f.typ != LogsSubscription {
		return nil, fmt.Errorf("filter not found")
	}
	return api.GetLogs(f.crit)
}

// GetLogs returns logs matching the given argument that are stored within the state.
//
// https://eth.wiki/json-rpc/API#eth_getLogs
func (api *filterAPI) GetLogs(crit gethfilters.FilterCriteria) ([]*gethtypes.Log, error) {
	api.logger.Debug("eth_getLogs")
	if crit.BlockHash != nil {
		// Block filter requested, construct a single-shot filter
		filter := NewBlockFilter(api.backend, *crit.BlockHash, crit.Addresses, crit.Topics)

		// Run the filter and return all the logs
		logs, err := filter.Logs(context.TODO())
		if err != nil {
			return nil, err
		}
		return returnLogs(logs), nil
	}

	// Convert the RPC block numbers into internal representations
	begin := rpc.LatestBlockNumber.Int64()
	if crit.FromBlock != nil {
		begin = crit.FromBlock.Int64()
	}
	end := rpc.LatestBlockNumber.Int64()
	if crit.ToBlock != nil {
		end = crit.ToBlock.Int64()
	}
	if begin < 0 {
		begin = api.backend.LatestHeight()
	}
	if end < 0 {
		end = api.backend.LatestHeight()
	}

	logs, err := api.backend.QueryLogs(crit.Addresses, crit.Topics, uint32(begin), uint32(end+1), filterFunc)
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Why? begin %d end %d logs %#v\n", begin, end, logs)

	return types.ToGethLogs(logs), nil
}

// NewHeads send a notification each time a new (header) block is appended to the chain.
func (api *filterAPI) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan *types.Header)
		headersSub := api.events.SubscribeNewHeads(headers)

		for {
			select {
			case h := <-headers:
				_ = notifier.Notify(rpcSub.ID, h)
			case <-rpcSub.Err():
				headersSub.Unsubscribe()
				return
			case <-notifier.Closed():
				headersSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (api *filterAPI) Logs(ctx context.Context, crit gethfilters.FilterCriteria) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan []*gethtypes.Log)
	)

	logsSub, err := api.events.SubscribeLogs(ethereum.FilterQuery(crit), matchedLogs)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case logs := <-matchedLogs:
				for _, _log := range logs {
					_ = notifier.Notify(rpcSub.ID, &_log)
				}
			case <-rpcSub.Err(): // client send an unsubscribe request
				logsSub.Unsubscribe()
				return
			case <-notifier.Closed(): // connection dropped
				logsSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// returnHashes is a helper that will return an empty hash array case the given hash array is nil,
// otherwise the given hashes array is returned.
func returnHashes(hashes []gethcmn.Hash) []gethcmn.Hash {
	if hashes == nil {
		return []gethcmn.Hash{}
	}
	return hashes
}

// returnLogs is a helper that will return an empty log array in case the given logs array is nil,
// otherwise the given logs array is returned.
func returnLogs(logs []*gethtypes.Log) []*gethtypes.Log {
	if logs == nil {
		return []*gethtypes.Log{}
	}
	return logs
}
