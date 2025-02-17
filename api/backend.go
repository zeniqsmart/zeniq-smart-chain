package api

import (
	"context"
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/app"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	"github.com/zeniqsmart/zeniq-smart-chain/staking"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

var _ BackendService = &apiBackend{}

const (
	// Ethereum Wire Protocol
	// https://github.com/ethereum/devp2p/blob/master/caps/eth.md
	protocolVersion = 63
)

// SEP206SEP
var SEP206ContractAddress [20]byte = [20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x27, 0x11}
var SEP206AddrAsZeniqOnEthereum [20]byte = [20]byte{0x5b, 0x52, 0xbf, 0xB8, 0x06, 0x2C, 0xe6, 0x64, 0xD7, 0x4b, 0xbC, 0xd4, 0xCd, 0x6D, 0xC7, 0xDf, 0x53, 0xFd, 0x72, 0x33}

type apiBackend struct {
	//extRPCEnabled bool
	node ITmNode
	app  app.IApp
	//gpo *gasprice.Oracle

	//chainSideFeed event.Feed
	//chainHeadFeed event.Feed
	//blockProcFeed event.Feed
	//txsFeed event.Feed
	//logsFeed   event.Feed
	rmLogsFeed event.Feed
	//P pendingLogsFeed event.Feed
}

func NewBackend(node ITmNode, app app.IApp) BackendService {
	return &apiBackend{
		node: node,
		app:  app,
	}
}

func (backend *apiBackend) ChainId() *big.Int {
	return backend.app.ChainID().ToBig()
}

func (backend *apiBackend) GetStorageAt(address common.Address, key string, height int64) []byte {
	ctx := backend.app.GetRpcContextAtHeight(height)
	defer ctx.Close(false)

	CCRPCForkBlock := backend.app.GetCCRPCForkBlock()
	if (height >= CCRPCForkBlock &&
		(address == common.Address(SEP206AddrAsZeniqOnEthereum) || address == common.Address(SEP206ContractAddress))) ||
		(height < CCRPCForkBlock &&
			address == common.Address(SEP206ContractAddress)) {
		SEP206_SEQUENCE := uint64(2000)
		return ctx.GetStorageAt(SEP206_SEQUENCE, key)
	}

	acc := ctx.GetAccount(address)
	if acc == nil {
		return nil
	}
	return ctx.GetStorageAt(acc.Sequence(), key)
}

func (backend *apiBackend) GetCode(contract common.Address, height int64) (bytecode []byte, codeHash []byte) {
	ctx := backend.app.GetRpcContextAtHeight(height)
	defer ctx.Close(false)

	info := ctx.GetCode(contract)
	if info != nil {
		bytecode = info.BytecodeSlice()
		codeHash = info.CodeHashSlice()
	}
	return
}

func (backend *apiBackend) GetBalance(owner common.Address, height int64) (*big.Int, error) {
	ctx := backend.app.GetRpcContextAtHeight(height)
	defer ctx.Close(false)
	b, err := ctx.GetBalance(owner)
	if err != nil {
		return nil, err
	}
	return b.ToBig(), nil
}

func (backend *apiBackend) GetNonce(address common.Address, height int64) (uint64, error) {
	ctx := backend.app.GetRpcContextAtHeight(height)
	defer ctx.Close(false)
	if acc := ctx.GetAccount(address); acc != nil {
		return acc.Nonce(), nil
	}

	return 0, types.ErrAccNotFound
}

func (backend *apiBackend) GetTransaction(txHash common.Hash) (tx *types.Transaction, sig [65]byte, err error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	if tx, sig, err = ctx.GetTxByHash(txHash); err != nil {
		return
	}
	if tx == nil {
		err = errors.New("tx with specific hash not exist")
	}
	return
}

func (backend *apiBackend) BlockByHash(hash common.Hash) (*types.Block, error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)
	block, err := ctx.GetBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (backend *apiBackend) BlockByNumber(number int64) (*types.Block, error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)
	return ctx.GetBlockByHeight(uint64(number))
}

func (backend *apiBackend) ProtocolVersion() int {
	return protocolVersion
}

func (backend *apiBackend) LatestHeight() int64 {
	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)

	return appCtx.GetLatestHeight()
}

func (backend *apiBackend) CurrentBlock() (*types.Block, error) {
	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)

	block, err := appCtx.GetBlockByHeight(uint64(appCtx.GetLatestHeight()))
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (backend *apiBackend) SendRawTx(signedTx []byte) (common.Hash, error) {
	return backend.node.BroadcastTxSync(signedTx)
}

// CallForSbch use app.RunTxForSbchRpc and returns more detailed result info
func (backend *apiBackend) CallForSbch(tx *gethtypes.Transaction, sender common.Address, height int64) *CallDetail {
	runner, _ := backend.app.RunTxForSbchRpc(tx, sender, height)
	return &CallDetail{
		Status:                 runner.Status,
		GasUsed:                runner.GasUsed,
		OutData:                runner.OutData,
		Logs:                   runner.Logs,
		CreatedContractAddress: runner.CreatedContractAddress,
		InternalTxCalls:        runner.InternalTxCalls,
		InternalTxReturns:      runner.InternalTxReturns,
		RwLists:                runner.RwLists,
	}
}

func (backend *apiBackend) Call(tx *gethtypes.Transaction, sender common.Address, height int64) (statusCode int, retData []byte) {
	runner, _ := backend.app.RunTxForRpc(tx, sender, false, height)
	return runner.Status, runner.OutData
}

func (backend *apiBackend) EstimateGas(tx *gethtypes.Transaction, sender common.Address, height int64) (statusCode int, retData []byte, gas int64) {
	runner, gas := backend.app.RunTxForRpc(tx, sender, true, height)
	return runner.Status, runner.OutData, gas
}

func (backend *apiBackend) QueryLogs(addresses []common.Address, topics [][]common.Hash, startHeight, endHeight uint32, filter types.FilterFunc) ([]types.Log, error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.QueryLogs(addresses, topics, startHeight, endHeight, filter)
}

func (backend *apiBackend) QueryTxBySrc(addr common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)
	return ctx.QueryTxBySrc(addr, startHeight, endHeight, limit)
}

func (backend *apiBackend) QueryTxByDst(addr common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)
	return ctx.QueryTxByDst(addr, startHeight, endHeight, limit)
}

func (backend *apiBackend) QueryTxByAddr(addr common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)
	return ctx.QueryTxByAddr(addr, startHeight, endHeight, limit)
}

func (backend *apiBackend) SbchQueryLogs(addr common.Address, topics []common.Hash, startHeight, endHeight, limit uint32) ([]types.Log, error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.BasicQueryLogs(addr, topics, startHeight, endHeight, limit)
}

func (backend *apiBackend) GetTxListByHeight(height uint32) (txs []*types.Transaction, sigs [][65]byte, err error) {
	return backend.GetTxListByHeightWithRange(height, 0, math.MaxInt32)
}
func (backend *apiBackend) GetTxListByHeightWithRange(height uint32, start, end int) (tx []*types.Transaction, sigs [][65]byte, err error) {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.GetTxListByHeightWithRange(height, start, end)
}

func (backend *apiBackend) GetToAddressCount(addr common.Address) int64 {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.GetToAddressCount(addr)
}
func (backend *apiBackend) GetFromAddressCount(addr common.Address) int64 {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.GetFromAddressCount(addr)
}
func (backend *apiBackend) GetSep20ToAddressCount(contract common.Address, addr common.Address) int64 {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.GetSep20ToAddressCount(contract, addr)
}
func (backend *apiBackend) GetSep20FromAddressCount(contract common.Address, addr common.Address) int64 {
	ctx := backend.app.GetHistoryOnlyContext()
	defer ctx.Close(false)

	return ctx.GetSep20FromAddressCount(contract, addr)
}

func (backend *apiBackend) GetCurrEpoch() *stake.Epoch {
	return backend.app.GetCurrEpoch()
}

func (backend *apiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	if blockNr == rpc.LatestBlockNumber {
		blockNr = rpc.BlockNumber(backend.app.GetLatestBlockNum())
	}

	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)
	block, err := appCtx.GetBlockByHeight(uint64(blockNr))
	if err != nil {
		return nil, nil
	}
	return &types.Header{
		Number:    hexutil.Uint64(block.Number),
		BlockHash: block.Hash,
		Bloom:     block.LogsBloom,
	}, nil
}
func (backend *apiBackend) HeaderByHash(ctx context.Context, blockHash common.Hash) (*types.Header, error) {
	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)
	block, err := appCtx.GetBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	return &types.Header{
		Number:    hexutil.Uint64(block.Number),
		BlockHash: block.Hash,
	}, nil
}
func (backend *apiBackend) GetReceipts(ctx context.Context, blockNum uint64) (gethtypes.Receipts, error) {
	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)

	receipts := make([]*gethtypes.Receipt, 0, 8)

	txs, _, err := appCtx.GetTxListByHeight(uint32(blockNum))
	if err == nil {
		for _, tx := range txs {
			receipts = append(receipts, toGethReceipt(tx))
		}
	}

	return receipts, nil
}

func toGethReceipt(tx *types.Transaction) *gethtypes.Receipt {
	return &gethtypes.Receipt{
		Status:            tx.Status,
		CumulativeGasUsed: tx.CumulativeGasUsed,
		Bloom:             tx.LogsBloom,
		Logs:              types.ToGethLogs(tx.Logs),
		TxHash:            tx.Hash,
		ContractAddress:   tx.ContractAddress,
		GasUsed:           tx.GasUsed,
		BlockHash:         tx.BlockHash,
		BlockNumber:       big.NewInt(tx.BlockNumber),
		TransactionIndex:  uint(tx.TransactionIndex),
	}
}

func (backend *apiBackend) GetLogs(ctx context.Context, blockHash common.Hash) ([][]*gethtypes.Log, error) {
	appCtx := backend.app.GetHistoryOnlyContext()
	defer appCtx.Close(false)

	logs := make([][]*gethtypes.Log, 0, 8)

	block, err := appCtx.GetBlockByHash(blockHash)
	if err == nil && block != nil {
		for _, txHash := range block.Transactions {
			tx, _, err := appCtx.GetTxByHash(txHash)
			if err == nil && tx != nil {
				txLogs := types.ToGethLogs(tx.Logs)
				// fix log.TxHash
				for _, txLog := range txLogs {
					txLog.TxHash = tx.Hash
				}
				logs = append(logs, txLogs)
			}
		}
	}

	return logs, nil
}

func (backend *apiBackend) SubscribeChainEvent(ch chan<- types.ChainEvent) event.Subscription {
	return backend.app.SubscribeChainEvent(ch)
}
func (backend *apiBackend) SubscribeLogsEvent(ch chan<- []*gethtypes.Log) event.Subscription {
	return backend.app.SubscribeLogsEvent(ch)
}
func (backend *apiBackend) SubscribeNewTxsEvent(ch chan<- gethcore.NewTxsEvent) event.Subscription {
	return backend.app.SubscribeNewTxsEvent(ch)
}
func (backend *apiBackend) SubscribeRemovedLogsEvent(ch chan<- gethcore.RemovedLogsEvent) event.Subscription {
	return backend.rmLogsFeed.Subscribe(ch)
}

//P func (b2 *apiBackend) SubscribePendingLogsEvent(ch chan<- []*gethtypes.Log) event.Subscription {
//P 	return b2.pendingLogsFeed.Subscribe(ch)
//P }

func (backend *apiBackend) BloomStatus() (uint64, uint64) {
	return 4096, 0 // this is temporary implementation
}
func (backend *apiBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	panic("implement me")
}

func (backend *apiBackend) NodeInfo() Info {
	i := backend.node.GetNodeInfo()
	bi := backend.app.LoadBlockInfo()
	i.NextBlock.Number = bi.Number
	i.NextBlock.Timestamp = bi.Timestamp
	i.NextBlock.Hash = bi.Hash
	return i
}

func (backend *apiBackend) ValidatorsInfo() app.ValidatorsInfo {
	return backend.app.GetValidatorsInfo()
}

func (backend *apiBackend) IsArchiveMode() bool {
	return backend.app.IsArchiveMode()
}

func (backend *apiBackend) GetSeq(address common.Address) uint64 {
	ctx := backend.app.GetRpcContextAtHeight(-1)
	defer ctx.Close(false)
	accInfo := ctx.GetAccount(address)
	if accInfo == nil {
		return 0
	}
	return accInfo.Sequence()
}

func (backend *apiBackend) GetPosVotes() map[[32]byte]*big.Int {
	ctx := backend.app.GetRunTxContext()
	defer ctx.Close(false)

	return staking.GetPosVotes(ctx, param.XHedgeContractSequence)
}

func (backend *apiBackend) GetSyncBlock(height int64) (blk []byte, err error) {
	return backend.app.GetBlockForSync(height)
}

func (backend *apiBackend) CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo {
	return backend.app.CrosschainInfo(start, end)
}
