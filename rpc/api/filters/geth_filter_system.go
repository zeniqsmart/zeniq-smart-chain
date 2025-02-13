// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package filters implements an ethereum filtering system for block,
// transactions and log events.
package filters

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	eth "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
)

// Type determines the kind of filter and is used to put the filter in to
// the correct bucket when added.
type Type byte

const (
	// UnknownSubscription indicates an unknown subscription type
	UnknownSubscription Type = iota
	// LogsSubscription queries for new or removed (chain reorg) logs
	LogsSubscription
	// PendingLogsSubscription queries for logs in pending blocks
	PendingLogsSubscription
	// MinedAndPendingLogsSubscription queries for logs in mined and pending blocks.
	MinedAndPendingLogsSubscription
	// PendingTransactionsSubscription queries tx hashes for pending
	// transactions entering the pending state
	PendingTransactionsSubscription
	// BlocksSubscription queries hashes for blocks that are imported
	BlocksSubscription
	// LastSubscription keeps track of the last index
	LastIndexSubscription
)

const (
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// rmLogsChanSize is the size of channel listening to RemovedLogsEvent.
	rmLogsChanSize = 10
	// logsChanSize is the size of channel listening to LogsEvent.
	logsChanSize = 10
	// chainEvChanSize is the size of channel listening to ChainEvent.
	chainEvChanSize = 10
)

type subscription struct {
	id        rpc.ID
	typ       Type
	created   time.Time
	logsCrit  ethereum.FilterQuery
	logs      chan []*eth.Log
	hashes    chan []common.Hash
	txs       chan []*eth.Transaction
	headers   chan *types.Header
	installed chan struct{} // closed when the filter is installed
	err       chan error    // closed when the filter is uninstalled
}

// EventSystem creates subscriptions, processes events and broadcasts them to the
// subscription which match the subscription criteria.
type EventSystem struct {
	backend   Backend
	lightMode bool
	// lastHead  *types.Header

	// Subscriptions
	txsSub    event.Subscription // Subscription for new transaction event
	logsSub   event.Subscription // Subscription for new log event
	rmLogsSub event.Subscription // Subscription for removed log event
	//P pendingLogsSub event.Subscription // Subscription for pending log event
	chainSub event.Subscription // Subscription for new chain event

	// Channels
	install       chan *subscription         // install filter for event notification
	uninstall     chan *subscription         // remove filter for event notification
	txsCh         chan core.NewTxsEvent      // Channel to receive new transactions event
	logsCh        chan []*eth.Log            // Channel to receive new log event
	pendingLogsCh chan []*eth.Log            // Channel to receive new log event
	rmLogsCh      chan core.RemovedLogsEvent // Channel to receive removed log event
	chainCh       chan types.ChainEvent      // Channel to receive new chain event
}

// NewEventSystem creates a new manager that listens for event on the given mux,
// parses and filters them. It uses the all map to retrieve filter changes. The
// work loop holds its own index that is used to forward events to filters.
//
// The returned manager has a loop that needs to be stopped with the Stop function
// or by stopping the given mux.
func NewEventSystem(backend Backend, lightMode bool) *EventSystem {
	m := &EventSystem{
		backend:       backend,
		lightMode:     lightMode,
		install:       make(chan *subscription),
		uninstall:     make(chan *subscription),
		txsCh:         make(chan core.NewTxsEvent, txChanSize),
		logsCh:        make(chan []*eth.Log, logsChanSize),
		rmLogsCh:      make(chan core.RemovedLogsEvent, rmLogsChanSize),
		pendingLogsCh: make(chan []*eth.Log, logsChanSize),
		chainCh:       make(chan types.ChainEvent, chainEvChanSize),
	}

	// Subscribe events
	m.txsSub = m.backend.SubscribeNewTxsEvent(m.txsCh)
	m.logsSub = m.backend.SubscribeLogsEvent(m.logsCh)
	m.rmLogsSub = m.backend.SubscribeRemovedLogsEvent(m.rmLogsCh)
	m.chainSub = m.backend.SubscribeChainEvent(m.chainCh)
	//P m.pendingLogsSub = m.backend.SubscribePendingLogsEvent(m.pendingLogsCh)

	// Make sure none of the subscriptions are empty
	//P if m.txsSub == nil || m.logsSub == nil || m.rmLogsSub == nil || m.chainSub == nil || m.pendingLogsSub == nil {
	if m.txsSub == nil || m.logsSub == nil || m.rmLogsSub == nil || m.chainSub == nil /*|| m.pendingLogsSub == nil*/ {
		log.Crit("Subscribe for event system failed")
	}

	go m.eventLoop()
	return m
}

// Subscription is created when the client registers itself for a particular event.
type Subscription struct {
	ID        rpc.ID
	f         *subscription
	es        *EventSystem
	unsubOnce sync.Once
}

// Err returns a channel that is closed when unsubscribed.
func (sub *Subscription) Err() <-chan error {
	return sub.f.err
}

// Unsubscribe uninstalls the subscription from the event broadcast loop.
func (sub *Subscription) Unsubscribe() {
	sub.unsubOnce.Do(func() {
	uninstallLoop:
		for {
			// write uninstall request and consume logs/hashes. This prevents
			// the eventLoop broadcast method to deadlock when writing to the
			// filter event channel while the subscription loop is waiting for
			// this method to return (and thus not reading these events).
			select {
			case sub.es.uninstall <- sub.f:
				break uninstallLoop
			case <-sub.f.logs:
			case <-sub.f.hashes:
			case <-sub.f.headers:
			}
		}

		// wait for filter to be uninstalled in work loop before returning
		// this ensures that the manager won't use the event channel which
		// will probably be closed by the client asap after this method returns.
		<-sub.Err()
	})
}

// subscribe installs the subscription in the event broadcast loop.
func (es *EventSystem) subscribe(sub *subscription) *Subscription {
	es.install <- sub
	<-sub.installed
	return &Subscription{ID: sub.id, f: sub, es: es}
}

// SubscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel. Default value for the from and to
// block is "latest". If the fromBlock > toBlock an error is returned.
func (es *EventSystem) SubscribeLogs(crit ethereum.FilterQuery, logs chan []*eth.Log) (*Subscription, error) {
	var from, to rpc.BlockNumber
	if crit.FromBlock == nil {
		from = rpc.LatestBlockNumber
	} else {
		from = rpc.BlockNumber(crit.FromBlock.Int64())
	}
	if crit.ToBlock == nil {
		to = rpc.LatestBlockNumber
	} else {
		to = rpc.BlockNumber(crit.ToBlock.Int64())
	}

	//P // only interested in pending logs
	//P if from == rpc.PendingBlockNumber && to == rpc.PendingBlockNumber {
	//P 	return es.subscribePendingLogs(crit, logs), nil
	//P }
	// only interested in new mined logs
	if from == rpc.LatestBlockNumber && to == rpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	// only interested in mined logs within a specific block range
	if from >= 0 && to >= 0 && to >= from {
		return es.subscribeLogs(crit, logs), nil
	}
	//P // interested in mined logs from a specific block number, new logs and pending logs
	//P if from >= rpc.LatestBlockNumber && to == rpc.PendingBlockNumber {
	//P 	return es.subscribeMinedPendingLogs(crit, logs), nil
	//P }
	// interested in logs from a specific block number to new mined blocks
	if from >= 0 && to == rpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	return nil, fmt.Errorf("invalid from and to block combination: from > to")
}

//P // subscribeMinedPendingLogs creates a subscription that returned mined and
//P // pending logs that match the given criteria.
//P func (es *EventSystem) subscribeMinedPendingLogs(crit ethereum.FilterQuery, logs chan []*eth.Log) *Subscription {
//P 	sub := &subscription{
//P 		id:        rpc.NewID(),
//P 		typ:       MinedAndPendingLogsSubscription,
//P 		logsCrit:  crit,
//P 		created:   time.Now(),
//P 		logs:      logs,
//P 		hashes:    make(chan []common.Hash),
//P 		headers:   make(chan *types.Header),
//P 		installed: make(chan struct{}),
//P 		err:       make(chan error),
//P 	}
//P 	return es.subscribe(sub)
//P }

// subscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel.
func (es *EventSystem) subscribeLogs(crit ethereum.FilterQuery, logs chan []*eth.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       LogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []common.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// subscribePendingLogs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) subscribePendingLogs(crit ethereum.FilterQuery, logs chan []*eth.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		hashes:    make(chan []common.Hash),
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribeNewHeads creates a subscription that writes the header of a block that is
// imported in the chain.
func (es *EventSystem) SubscribeNewHeads(headers chan *types.Header) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       BlocksSubscription,
		created:   time.Now(),
		logs:      make(chan []*eth.Log),
		hashes:    make(chan []common.Hash),
		headers:   headers,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribePendingTxs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) SubscribePendingTxs(txs chan []*eth.Transaction, logs chan []*eth.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingTransactionsSubscription,
		created:   time.Now(),
		logs:      logs,
		txs:       txs,
		headers:   make(chan *types.Header),
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

type filterIndex map[Type]map[rpc.ID]*subscription

func (es *EventSystem) handleLogs(filters filterIndex, ev []*eth.Log) {
	if len(ev) == 0 {
		return
	}
	for _, f := range filters[LogsSubscription] {
		matchedLogs := filterLogs(ev, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
		if len(matchedLogs) > 0 {
			f.logs <- matchedLogs
		}
	}
}

//P func (es *EventSystem) handlePendingLogs(filters filterIndex, ev []*eth.Log) {
//P 	if len(ev) == 0 {
//P 		return
//P 	}
//P 	for _, f := range filters[PendingLogsSubscription] {
//P 		matchedLogs := filterLogs(ev, nil, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
//P 		if len(matchedLogs) > 0 {
//P 			f.logs <- matchedLogs
//P 		}
//P 	}
//P }

func (es *EventSystem) handleRemovedLogs(filters filterIndex, ev core.RemovedLogsEvent) {
	for _, f := range filters[LogsSubscription] {
		matchedLogs := filterLogs(ev.Logs, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics)
		if len(matchedLogs) > 0 {
			f.logs <- matchedLogs
		}
	}
}

func (es *EventSystem) handleTxsEvent(filters filterIndex, ev core.NewTxsEvent) {
	for _, f := range filters[PendingTransactionsSubscription] {
		f.txs <- ev.Txs
	}
}

func (es *EventSystem) handleChainEvent(filters filterIndex, ev types.ChainEvent) {
	for _, f := range filters[BlocksSubscription] {
		f.headers <- ev.BlockHeader
	}
	//P if es.lightMode && len(filters[LogsSubscription]) > 0 {
	//P 	es.lightFilterNewHead(ev.BlockHeader, func(header *types.Header, remove bool) {
	//P 		for _, f := range filters[LogsSubscription] {
	//P 			if matchedLogs := es.lightFilterLogs(header, f.logsCrit.Addresses, f.logsCrit.Topics, remove); len(matchedLogs) > 0 {
	//P 				f.logs <- matchedLogs
	//P 			}
	//P 		}
	//P 	})
	//P }
}

/*
func (es *EventSystem) lightFilterNewHead(newHeader *types.Header, callBack func(*types.Header, bool)) {
	oldh := es.lastHead
	es.lastHead = newHeader
	if oldh == nil {
		return
	}
	newh := newHeader
	// find common ancestor, create list of rolled back and new block hashes
	var oldHeaders, newHeaders []*types.Header
	for oldh.Hash() != newh.Hash() {
		if oldh.Number >= newh.Number {
			oldHeaders = append(oldHeaders, oldh)
			oldh = rawdb.ReadHeader(es.backend.ChainDb(), oldh.ParentHash, oldh.Number-1)
		}
		if oldh.Number < newh.Number {
			newHeaders = append(newHeaders, newh)
			newh = rawdb.ReadHeader(es.backend.ChainDb(), newh.ParentHash, newh.Number-1)
			if newh == nil {
				// happens when CHT syncing, nothing to do
				newh = oldh
			}
		}
	}
	// roll back old blocks
	for _, h := range oldHeaders {
		callBack(h, true)
	}
	// check new blocks (array is in reverse order)
	for i := len(newHeaders) - 1; i >= 0; i-- {
		callBack(newHeaders[i], false)
	}
}
*/

// //filter logs of a single header in light client mode
// func (es *EventSystem) lightFilterLogs(header *types.Header, addresses []common.Address, topics [][]common.Hash, remove bool) []*eth.Log {
// 	if bloomFilter(header.Bloom, addresses, topics) {
// 		// Get the logs of the block
// 		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
// 		defer cancel()
// 		logsList, err := es.backend.GetLogs(ctx, header.Hash())
// 		if err != nil {
// 			return nil
// 		}
// 		var unfiltered []*eth.Log
// 		for _, logs := range logsList {
// 			for _, log := range logs {
// 				logcopy := *log
// 				logcopy.Removed = remove
// 				unfiltered = append(unfiltered, &logcopy)
// 			}
// 		}
// 		logs := filterLogs(unfiltered, nil, nil, addresses, topics)
// 		if len(logs) > 0 && logs[0].TxHash == (common.Hash{}) {
// 			// We have matching but non-derived logs
// 			receipts, err := es.backend.GetReceipts(ctx, header.Hash())
// 			if err != nil {
// 				return nil
// 			}
// 			unfiltered = unfiltered[:0]
// 			for _, receipt := range receipts {
// 				for _, log := range receipt.Logs {
// 					logcopy := *log
// 					logcopy.Removed = remove
// 					unfiltered = append(unfiltered, &logcopy)
// 				}
// 			}
// 			logs = filterLogs(unfiltered, nil, nil, addresses, topics)
// 		}
// 		return logs
// 	}
// 	return nil
// }

// eventLoop (un)installs filters and processes mux events.
func (es *EventSystem) eventLoop() {
	// Ensure all subscriptions get cleaned up
	defer func() {
		es.txsSub.Unsubscribe()
		es.logsSub.Unsubscribe()
		es.rmLogsSub.Unsubscribe()
		//P es.pendingLogsSub.Unsubscribe()
		es.chainSub.Unsubscribe()
	}()

	index := make(filterIndex)
	for i := UnknownSubscription; i < LastIndexSubscription; i++ {
		index[i] = make(map[rpc.ID]*subscription)
	}

	for {
		select {
		case ev := <-es.txsCh:
			es.handleTxsEvent(index, ev)
		case ev := <-es.logsCh:
			es.handleLogs(index, ev)
		case ev := <-es.rmLogsCh:
			es.handleRemovedLogs(index, ev)
		case <-es.pendingLogsCh:
		//P case ev := <-es.pendingLogsCh:
		//P es.handlePendingLogs(index, ev)
		case ev := <-es.chainCh:
			es.handleChainEvent(index, ev)

		case f := <-es.install:
			//P if f.typ == MinedAndPendingLogsSubscription {
			//P 	// the type are logs and pending logs subscriptions
			//P 	index[LogsSubscription][f.id] = f
			//P 	index[PendingLogsSubscription][f.id] = f
			//P } else {
			index[f.typ][f.id] = f
			//P }
			close(f.installed)

		case f := <-es.uninstall:
			//P if f.typ == MinedAndPendingLogsSubscription {
			//P 	// the type are logs and pending logs subscriptions
			//P 	delete(index[LogsSubscription], f.id)
			//P 	delete(index[PendingLogsSubscription], f.id)
			//P } else {
			delete(index[f.typ], f.id)
			//P }
			close(f.err)

		// System stopped
		case <-es.txsSub.Err():
			return
		case <-es.logsSub.Err():
			return
		case <-es.rmLogsSub.Err():
			return
		case <-es.chainSub.Err():
			return
		}
	}
}
