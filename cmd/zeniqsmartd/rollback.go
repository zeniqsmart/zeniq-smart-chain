package main

import (
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"

	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/p2p"

	ads "github.com/zeniqsmart/ads-zeniq-smart-chain/store"
	"github.com/zeniqsmart/ads-zeniq-smart-chain/store/rabbit"
	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
)

func datadir() string {
	hd, _ := os.UserHomeDir()
	datadir := hd + "/.zeniqsmartd/data"
	return datadir
}

func rollback() {

	datadir := datadir()

	dbType := dbm.BackendType("goleveldb")

	blockStoreDB, err := dbm.NewDB("blockstore", dbType, datadir)
	defer blockStoreDB.Close()
	if err != nil {
		panic("blockstore error")
	}
	stateDB, err := dbm.NewDB("state", dbType, datadir)
	if err != nil {
		panic("statestore error")
	}
	defer stateDB.Close()

	bs := store.NewBlockStore(blockStoreDB)
	ss := state.NewStore(stateDB)

	invalidState, err := ss.Load()
	if err != nil {
		panic("Cannot load state")
	}
	if invalidState.IsEmpty() {
		panic("State empty")
	}

	ssHeight := invalidState.LastBlockHeight
	rollbackHeight := ssHeight - 1

	if bs.Height() < rollbackHeight {
		panic(fmt.Sprintf("blockstore %v smaller than the wanted height %v",
			bs.Height(), rollbackHeight))
	}

	rollbackBlock := bs.LoadBlockMeta(rollbackHeight)
	if rollbackBlock == nil {
		panic(fmt.Sprintf("rollback block at height %d not found", rollbackHeight))
	}

	latestBlock := bs.LoadBlockMeta(ssHeight)
	if latestBlock == nil {
		panic(fmt.Sprintf("latest block at height %d not found", rollbackHeight))
	}

	previousLastValidatorSet, err := ss.LoadValidators(rollbackHeight)
	if err != nil {
		panic("Validatorset cannot be loaded")
	}

	previousParams, err := ss.LoadConsensusParams(ssHeight)
	if err != nil {
		panic("Consensus params cannot be loaded")
	}

	valChangeHeight := invalidState.LastHeightValidatorsChanged
	if valChangeHeight > rollbackHeight {
		valChangeHeight = ssHeight
	}

	paramsChangeHeight := invalidState.LastHeightConsensusParamsChanged
	if paramsChangeHeight > rollbackHeight {
		paramsChangeHeight = ssHeight
	}

	rolledBackState := state.State{
		Version: tmstate.Version{
			Consensus: tmversion.Consensus{
				Block: version.BlockProtocol,
				App:   previousParams.Version.AppVersion,
			},
			Software: version.TMCoreSemVer,
		},
		ChainID:       invalidState.ChainID,
		InitialHeight: invalidState.InitialHeight,

		LastBlockHeight: rollbackBlock.Header.Height,
		LastBlockID:     rollbackBlock.BlockID,
		LastBlockTime:   rollbackBlock.Header.Time,

		NextValidators:              invalidState.Validators,
		Validators:                  invalidState.LastValidators,
		LastValidators:              previousLastValidatorSet,
		LastHeightValidatorsChanged: valChangeHeight,

		ConsensusParams:                  previousParams,
		LastHeightConsensusParamsChanged: paramsChangeHeight,

		LastResultsHash: latestBlock.Header.LastResultsHash,
		AppHash:         latestBlock.Header.AppHash,
	}

	if err := ss.Save(rolledBackState); err != nil {
		panic(fmt.Errorf("failed to save rolled back state: %w", err))
	}

	fmt.Printf("Rolled back 1 to state %v and block %v with apphash %v before %v\n",
		rollbackHeight, bs.Height(), latestBlock.Header.AppHash, rollbackBlock.Header.AppHash)

}

func ShowNodeID() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-node-id",
		Short: "Show this node's ID",
		RunE:  showNodeID,
	}
	return cmd
}

func showNodeID(cmd *cobra.Command, args []string) error {
	hd, _ := os.UserHomeDir()
	configkeyfile := hd + "/.zeniqsmartd/config/node_key.json"
	nodeKey, err := p2p.LoadNodeKey(configkeyfile)
	if err != nil {
		return err
	}
	fmt.Println(nodeKey.ID())
	return nil
}

func getRunTxContext(trunk *ads.TrunkStore, currHeight int64) *types.Context {
	c := types.NewContext(nil, nil, math.MaxInt64)
	r := rabbit.NewRabbitStore(trunk)
	c = c.WithRbt(&r)
	c.SetShaGateForkBlock(0)
	c.SetXHedgeForkBlock(0)
	c.SetCCRPCForkBlock(math.MaxInt64)
	c.SetCurrentHeight(currHeight)
	return c
}

func buildBasicInfoBlockFromTM(tmBlock *tmtypes.Block) *types.Block {
	block := &types.Block{
		Number: tmBlock.Height,
	}
	copy(block.Hash[:], tmBlock.Hash())
	copy(block.ParentHash[:], tmBlock.LastBlockID.Hash)
	copy(block.Miner[:], tmBlock.ProposerAddress.Bytes())
	block.Number = tmBlock.Height
	block.Timestamp = tmBlock.Time.Unix()
	block.Size = int64(tmBlock.Size())
	return block
}
