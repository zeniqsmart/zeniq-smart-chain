package main

import (
	"fmt"
	"math"
	"os"

	/*NP
	"bufio"
	"encoding/hex"
	"github.com/zeniqsmart/zeniq-smart-chain/app"
	"strconv"
	"strings"
	adstypes "github.com/zeniqsmart/ads-zeniq-smart-chain/store/types"
	*/ //NP

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

func AddRollbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "rollback stateBlockHeight, appBlockHeight and storeBlockHeight by 1 (see ReplayBlocks() in tendermint)",

		Long: `This rolls back
	~/.zeniqsmartd/data/state.db,
	~/.zeniqsmartd/data/blockstore.db
	Not
	- historyStore (~/.zeniqsmartd/data/app + ~/.zeniqsmartd/data/db)
	- ~/.zeniqsmartd/data/syncdb`,

		Run: func(cmd *cobra.Command, args []string) {
			rollback()
		},
	}
	return cmd
}

// first roll back the state struct containing the appHash
// roll back blockstore if needed (truncate by 1)
// finally roll back app state choosing the appropriate ~/.zeniqsmartd/data/updateOfADSX.txt)
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

	/*NP
	for bb := bs.Height(); bb > ssHeight; bb-- {
		rollback_blockstore(bs)
	}
	*/ //NP

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

	/*NP rolling back app state turned out to be not possible
	rollback_app(datadir, bs, ssHeight)
	// without rollback_app()
	// rm -rf ~/.zeniqsmartd/data/{app,db}
	// # will lead to a replay of blocks if rollback_blockstore keeps stateBlockHeight+1=storeBlockHeight
	// # but not much faster than normal sync, i.e. after
	// rm -rf ~/.zeniqsmartd/data/{app,db,blockstore.db,state.db,evidence.db}
	*/ //NP

	fmt.Printf("Rolled back 1 to state %v and block %v with apphash %v before %v\n",
		rollbackHeight, bs.Height(), latestBlock.Header.AppHash, rollbackBlock.Header.AppHash)

}

/* NP

// rollback_blockstore needs change in
// tendermint/store/store.go::
//
//	func (bs *BlockStore) DeleteHead() error {
//		bs.mtx.RLock()
//		height := bs.height
//		bs.mtx.RUnlock()
//		batch := bs.db.NewBatch()
//		defer batch.Close()
//		flush := func(batch dbm.Batch) error {
//			bs.mtx.Lock()
//			bs.height = height-1
//			bs.mtx.Unlock()
//			bs.saveState()
//			err := batch.WriteSync()
//			if err != nil {
//				return fmt.Errorf("failed to delete last height %v: %w", height, err)
//			}
//			batch.Close()
//			return nil
//		}
//		h := height
//		meta := bs.LoadBlockMeta(h)
//		if meta == nil { // assume already deleted
//			return fmt.Errorf("failed to delete last height %v because block not there", height)
//		}
//		if err := batch.Delete(calcBlockMetaKey(h)); err != nil { return err }
//		if err := batch.Delete(calcBlockHashKey(meta.BlockID.Hash)); err != nil { return err }
//		if err := batch.Delete(calcBlockCommitKey(h)); err != nil { return err }
//		if err := batch.Delete(calcSeenCommitKey(h)); err != nil { return err }
//		for p := 0; p < int(meta.BlockID.PartSetHeader.Total); p++ {
//			if err := batch.Delete(calcBlockPartKey(h, p)); err != nil { return err }
//		}
//		return flush(batch)
//	}
func rollback_blockstore(bs *store.BlockStore) {
	e := bs.DeleteHead()
	if e != nil {
		panic("unable to delete last block")
	}
	fmt.Printf("block store height is now %v\n", bs.Height())
}

func decode(v string) []byte {
	vv, err := hex.DecodeString(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func rollback_app(datadir string, bs *store.BlockStore, ssHeight int64) {
	appDir := datadir + "/app"
	rs, _ := app.CreateRootStore(appDir, false)
	rs.SetHeight(ssHeight)
	trunk := rs.GetTrunkStore(200).(*ads.TrunkStore)
	ctx := getRunTxContext(trunk, ssHeight)
	fn := fmt.Sprintf(appDir+"/updateOfADS%d.txt", ssHeight)
	f, e := os.Open(fn)
	if e != nil {
		panic(fmt.Errorf("NO ~/.zeniqsmartd/data/app/updateOfADS%v.txt to undo", ssHeight))
	}
	lines := bufio.NewScanner(f)
	lines.Split(bufio.ScanLines)
	for lines.Scan() {
		t := lines.Text()
		ss := strings.Split(t, "=")
		if len(ss) < 2 {
			id_height := strings.Split(t, " ")
			i, e := strconv.Atoi(id_height[1])
			if e != nil {
				panic(fmt.Errorf("NO first line of ~/.zeniqsmartd/data/app/updateOfADS%v.txt", ssHeight))
			}
			if int64(i) != ssHeight {
				f.Close()
				panic(fmt.Errorf("WRONG first line of ~/.zeniqsmartd/data/app/updateOfADS%v.txt", ssHeight))
			} else {
				continue
			}
		}
		k, vv := ss[0], strings.Split(ss[1], "/")
		if len(vv[1]) == 0 {
			cv := rabbit.BytesToCachedValue(decode(vv[0]))
			if cv == nil {
				trunk.Update(func(db adstypes.SetDeleter) {
					db.Delete(decode(k))
				})
			} else {
				ctx.Rbt.Delete(cv.GetKey())
			}
		} else {
			cv := rabbit.BytesToCachedValue(decode(vv[1]))
			if cv == nil {
				trunk.Update(func(db adstypes.SetDeleter) {
					db.Set(decode(k), decode(vv[1]))
				})
			} else {
				ctx.Rbt.Set(cv.GetKey(), cv.GetValue())
			}
		}
	}
	f.Close()
	tmBlock := bs.LoadBlock(ssHeight)         //?
	blk := buildBasicInfoBlockFromTM(tmBlock) //?
	ctx.SetCurrBlockBasicInfo(blk)            //?
	ctx.Close(true)
	trunk.Close(true)
	fmt.Printf("appHash now %v\n", strings.ToUpper(hex.EncodeToString(rs.GetRootHash())))
	rs.Close()
}

//    // creation of updateADSX.txt files in app/app.go
//    func logUpdateOfADS(dataPath string, height int64, updateOfADS map[string]string) string {
//    	fm := path.Join(dataPath, fmt.Sprintf("updateOfADS%d.txt", height))
//    	f, _ := os.Create(fm)
//    	f.Write([]byte(fmt.Sprintf("updated_in_height %v\n", height)))
//    	// just new values could be written via
//    	// for k,v := range updateOfADS { f.Write([]byte(fmt.Sprintf("%x=%x\n", k, v))) }
//    	// ... but to have the old value NewOverOldFile
//    	ads.NewOverOldFile = f
//    	return fm
//    }
//    func logUpdateOfAdsDone() {
//    	if ads.NewOverOldFile != nil {
//    		ads.NewOverOldFile.Close()
//    	}
//    	ads.NewOverOldFile = nil
//    }
//    //...
//
//    if app.config.AppConfig.UpdateOfADSLog {
//    	rh := hex.EncodeToString(app.root.GetRootHash())
//    	update_file := logUpdateOfADS(app.config.AppConfig.AppDataPath, app.currHeight, updateOfADS)
//    	app.logger.Info(fmt.Sprintf("%d _apphash_ %v updateOfADS in %s\n",
//    		app.currHeight,
//    		rh,
//    		update_file,
//    	))
//    }
//    app.trunk.Close(true) //write cached KVs back to app.root
//    if app.config.AppConfig.UpdateOfADSLog {
//    	logUpdateOfAdsDone()
//    }

*/ //NP

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
