package app

import (
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"

	dbtypes "github.com/zeniqsmart/db-zeniq-smart-chain/types"
	evmtc "github.com/zeniqsmart/evm-zeniq-smart-chain/evmwrap/testcase"
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

func (app *App) SumAllBalance() *uint256.Int {
	return evmtc.GetWorldStateFromMads(app.mads).SumAllBalance()
}

func (app *App) GetWordState() *evmtc.WorldState {
	return evmtc.GetWorldStateFromMads(app.mads)
}
