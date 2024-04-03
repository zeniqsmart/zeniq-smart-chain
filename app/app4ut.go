package app

import (
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"

	dbtypes "github.com/zeniqsmart/db-zeniq-smart-chain/types"
	evmtc "github.com/zeniqsmart/evm-zeniq-smart-chain/evmwrap/testcase"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

func (app *App) Logger() log.Logger {
	return app.logger
}

func (app *App) HistoryStore() dbtypes.DB {
	return app.historyStore
}

func (app *App) BlockNum() int64 {
	return app.block.Number
}

// nolint
func (app *App) WaitLock() { // wait for postCommit to finish
	app.mtx.Lock()
	app.mtx.Unlock()
}

func (app *App) CloseTrunk() {
	app.trunk.Close(true)
}
func (app *App) CloseTxEngineContext() {
	app.txEngine.Context().Close(false)
}

func (app *App) AddEpochForTest(e *stake.Epoch) { // breaks normal function, only used in test
	app.watcher.EpochChan <- e
}

func (app *App) AddBlockFotTest(dbBlock *dbtypes.Block) { // breaks normal function, only used in test
	app.historyStore.AddBlock(dbBlock, -1, nil)
	app.historyStore.AddBlock(nil, -1, nil) // To Flush
	app.publishNewBlock(dbBlock)
}

func (app *App) SumAllBalance() *uint256.Int {
	return evmtc.GetWorldStateFromMads(app.mads).SumAllBalance()
}

func (app *App) GetWordState() *evmtc.WorldState {
	return evmtc.GetWorldStateFromMads(app.mads)
}
