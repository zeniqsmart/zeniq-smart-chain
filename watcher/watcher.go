package watcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/libs/log"

	crosschain "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
	"github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
)

const (
	NumBlocksToClearMemory = 1000
	WaitingBlockDelayTime  = 2
)

// A watcher watches the new blocks generated on bitcoin cash's mainnet, and
// outputs epoch information through a channel
type Watcher struct {
	logger log.Logger

	rpcClient           types.RpcClient
	zeniqsmartRpcClient types.RpcClient

	latestFinalizedHeight int64

	heightToFinalizedBlock map[int64]*types.BCHBlock

	EpochChan          chan *stake.Epoch
	epochList          []*stake.Epoch
	numBlocksInEpoch   int64
	lastEpochEndHeight int64
	lastKnownEpochNum  int64

	CCEpochChan          chan *crosschain.CCEpoch
	lastCCEpochEndHeight int64
	numBlocksInCCEpoch   int64
	ccEpochList          []*crosschain.CCEpoch
	lastKnownCCEpochNum  int64

	numBlocksToClearMemory int
	waitingBlockDelayTime  int

	chainConfig *param.ChainConfig

	currentMainnetBlockTimestamp atomic.Int64
}

func NewWatcher(logger log.Logger, lastHeight, lastCCEpochEndHeight int64,
	lastKnownEpochNum int64, chainConfig *param.ChainConfig, rpcclient types.RpcClient) *Watcher {
	if rpcclient == nil {
		rpcclient = NewRpcClient(chainConfig.AppConfig.MainnetRPCUrl, chainConfig.AppConfig.MainnetRPCUsername, chainConfig.AppConfig.MainnetRPCPassword, "text/plain;", logger)
	}
	w := &Watcher{
		logger: logger,

		rpcClient:           rpcclient,
		zeniqsmartRpcClient: NewRpcClient(chainConfig.AppConfig.ZeniqsmartRPCUrl, "", "", "application/json", logger),

		lastEpochEndHeight:    lastHeight,
		latestFinalizedHeight: lastHeight,
		lastKnownEpochNum:     lastKnownEpochNum,

		heightToFinalizedBlock: make(map[int64]*types.BCHBlock),
		epochList:              make([]*stake.Epoch, 0, 10),

		EpochChan: make(chan *stake.Epoch, 10000),

		numBlocksInEpoch:       param.StakingNumBlocksInEpoch,
		numBlocksToClearMemory: NumBlocksToClearMemory,
		waitingBlockDelayTime:  WaitingBlockDelayTime,

		CCEpochChan:          make(chan *crosschain.CCEpoch, 96*10000),
		ccEpochList:          make([]*crosschain.CCEpoch, 0, 40),
		lastCCEpochEndHeight: lastCCEpochEndHeight,
		numBlocksInCCEpoch:   param.BlocksInCCEpoch,

		chainConfig: chainConfig,
	}

	// set big enough for single node startup when no BCH node connected. it will be updated when mainnet block finalize.
	w.currentMainnetBlockTimestamp.Store(math.MaxInt64 - 14*24*3600)

	return w
}

func (watcher *Watcher) NetworkSmartHeight() int64 {
	return watcher.zeniqsmartRpcClient.NetworkSmartHeight()
}

func (watcher *Watcher) SetNumBlocksInEpoch(n int64) {
	watcher.numBlocksInEpoch = n
}

func (watcher *Watcher) SetNumBlocksToClearMemory(n int) {
	watcher.numBlocksToClearMemory = n
}

func (watcher *Watcher) SetWaitingBlockDelayTime(n int) {
	watcher.waitingBlockDelayTime = n
}

// The main function to do a watcher's job. It must be run as a goroutine
func (watcher *Watcher) WatcherMain(catchupChan chan bool) {
	// for testing
	if watcher.rpcClient == (*RpcClientImp)(nil) ||
		0 == watcher.chainConfig.AppConfig.CCRPCForkBlock {
		catchupChan <- true
		return
	}
	switch watcher.rpcClient.(type) {
	case *RpcClientImp:
		break
	default:
		if cm, ok := watcher.rpcClient.(types.RpcClientMock); ok && cm.DoWatch() {
			break
		} else {
			catchupChan <- true
			return
		}
	}
	// else
	latestFinalizedHeight := watcher.latestFinalizedHeight
	latestMainnetHeight := watcher.rpcClient.GetMainnetHeight()
	latestFinalizedHeight = watcher.epochSpeedup(latestFinalizedHeight, latestMainnetHeight)
	watcher.fetchBlocks(catchupChan, latestFinalizedHeight, latestMainnetHeight)
}

func (watcher *Watcher) fetchBlocks(catchupChan chan bool, latestFinalizedHeight, latestMainnetHeight int64) {
	catchup := false
	for {
		if !catchup && latestMainnetHeight <= latestFinalizedHeight+9 {
			latestMainnetHeight = watcher.rpcClient.GetMainnetHeight()
			if latestMainnetHeight <= latestFinalizedHeight+9 {
				watcher.logger.Debug("Catchup")
				catchup = true
				catchupChan <- true
				close(catchupChan)
			}
		}
		latestFinalizedHeight++
		latestMainnetHeight = watcher.rpcClient.GetMainnetHeight()
		//10 confirms
		if latestMainnetHeight < latestFinalizedHeight+9 {
			watcher.logger.Debug("waiting mainnet", "height now is", latestMainnetHeight)
			watcher.suspended(time.Duration(watcher.waitingBlockDelayTime) * time.Second) //delay half of bch mainnet block intervals
			latestFinalizedHeight--
			continue
		}
		for latestFinalizedHeight+9 <= latestMainnetHeight {
			watcher.logger.Debug(fmt.Sprintf("latestFinalizedHeight:%d,latestMainnetHeight:%d\n", latestFinalizedHeight, latestMainnetHeight))
			blk := watcher.rpcClient.GetBlockByHeight(latestFinalizedHeight, true)
			if blk == nil {
				catchupChan <- true // for test
				// panic(fmt.Errorf("get block:%d failed\n", latestFinalizedHeight))
			}
			CCRPCMAINNET := watcher.chainConfig.AppConfig.CCRPCEpochs[0][0]
			if latestFinalizedHeight >= CCRPCMAINNET { // CCRPCMAINNET is already included in ccrpc
				watcher.logger.Info(fmt.Sprintf("leaving WatcherMain at %d time %v", latestFinalizedHeight, blk.Timestamp))
				if !catchup {
					catchup = true
					catchupChan <- true
					close(catchupChan)
				}
				return
			}
			watcher.addFinalizedBlock(blk)
			latestFinalizedHeight++
		}
		latestFinalizedHeight--
	}
}

func (watcher *Watcher) epochSpeedup(latestFinalizedHeight, latestMainnetHeight int64) int64 {
	if watcher.chainConfig.AppConfig.Speedup {
		start := uint64(watcher.lastKnownEpochNum) + 1
		for {
			if latestMainnetHeight < latestFinalizedHeight+watcher.numBlocksInEpoch {
				watcher.ccEpochSpeedup()
				break
			}
			epochs := watcher.zeniqsmartRpcClient.GetEpochs(start, start+100)
			if epochs == nil {
				return latestMainnetHeight
			}
			var le = len(epochs)
			if le == 0 {
				watcher.ccEpochSpeedup()
				break
			}
			for _, e := range epochs {
				out, _ := json.Marshal(e)
				var so = string(out)
				fmt.Println(so)
			}
			watcher.epochList = append(watcher.epochList, epochs...)
			for _, e := range epochs {
				if e.EndTime != 0 {
					watcher.EpochChan <- e
				}
			}
			latestFinalizedHeight += int64(len(epochs)) * watcher.numBlocksInEpoch
			start = start + uint64(len(epochs))
		}
		watcher.latestFinalizedHeight = latestFinalizedHeight
		watcher.lastEpochEndHeight = latestFinalizedHeight
		watcher.logger.Debug("After speedup", "latestFinalizedHeight", watcher.latestFinalizedHeight)
	}
	return latestFinalizedHeight
}

func (watcher *Watcher) ccEpochSpeedup() {
	if !param.ShaGateSwitch {
		return
	}
	start := uint64(watcher.lastKnownCCEpochNum) + 1
	for {
		epochs := watcher.zeniqsmartRpcClient.GetCCEpochs(start, start+100)
		if epochs == nil {
			break
		}
		if len(epochs) == 0 {
			break
		}
		for _, e := range epochs {
			out, _ := json.Marshal(e)
			fmt.Println(string(out))
		}
		watcher.ccEpochList = append(watcher.ccEpochList, epochs...)
		for _, e := range epochs {
			watcher.CCEpochChan <- e
		}
		start = start + uint64(len(epochs))
	}
}

func (watcher *Watcher) suspended(delayDuration time.Duration) {
	time.Sleep(delayDuration)
}

// Record new block and if the blocks for a new epoch is all ready, output the new epoch
func (watcher *Watcher) addFinalizedBlock(blk *types.BCHBlock) {
	watcher.heightToFinalizedBlock[blk.Height] = blk
	watcher.latestFinalizedHeight++
	watcher.currentMainnetBlockTimestamp.Store(blk.Timestamp)

	if watcher.latestFinalizedHeight-watcher.lastEpochEndHeight == watcher.numBlocksInEpoch {
		watcher.generateNewEpoch()
	}
	//if watcher.latestFinalizedHeight-watcher.lastCCEpochEndHeight == watcher.numBlocksInCCEpoch {
	//	watcher.generateNewCCEpoch()
	//}
}

// Generate a new block's information
func (watcher *Watcher) generateNewEpoch() {
	epoch := watcher.buildNewEpoch()
	watcher.epochList = append(watcher.epochList, epoch)
	watcher.logger.Debug("Generate new epoch", "epochNumber", epoch.Number, "startHeight", epoch.StartHeight)
	watcher.EpochChan <- epoch
	watcher.lastEpochEndHeight = watcher.latestFinalizedHeight
	watcher.ClearOldData()
}

func (watcher *Watcher) buildNewEpoch() *stake.Epoch {
	epoch := &stake.Epoch{
		StartHeight: watcher.lastEpochEndHeight + 1,
		Nominations: make([]*stake.Nomination, 0, 10),
	}
	var valMapByPubkey = make(map[[32]byte]*stake.Nomination)
	for i := epoch.StartHeight; i <= watcher.latestFinalizedHeight; i++ {
		blk, ok := watcher.heightToFinalizedBlock[i]
		if !ok {
			panic("Missing Block")
		}
		//Please note that BCH's timestamp is not always linearly increasing
		if epoch.EndTime < blk.Timestamp {
			epoch.EndTime = blk.Timestamp
		}
		for _, nomination := range blk.Nominations {
			if _, ok := valMapByPubkey[nomination.Pubkey]; !ok {
				valMapByPubkey[nomination.Pubkey] = &nomination
			}
			valMapByPubkey[nomination.Pubkey].NominatedCount += nomination.NominatedCount
		}
	}
	for _, v := range valMapByPubkey {
		epoch.Nominations = append(epoch.Nominations, v)
	}
	sortEpochNominations(epoch)
	return epoch
}

func (watcher *Watcher) GetCurrEpoch() *stake.Epoch {
	return watcher.buildNewEpoch()
}
func (watcher *Watcher) GetEpochList() []*stake.Epoch {
	list := stake.CopyEpochs(watcher.epochList)
	currEpoch := watcher.buildNewEpoch()
	return append(list, currEpoch)
}

func (watcher *Watcher) GetCurrMainnetBlockTimestamp() int64 {
	return watcher.currentMainnetBlockTimestamp.Load()
}

//func (watcher *Watcher) generateNewCCEpoch() {
//	if !watcher.chainConfig.ShaGateSwitch {
//		return
//	}
//	epoch := watcher.buildNewCCEpoch()
//	watcher.ccEpochList = append(watcher.ccEpochList, epoch)
//	watcher.logger.Debug("Generate new cc epoch", "epochNumber", epoch.Number, "startHeight", epoch.StartHeight)
//	watcher.CCEpochChan <- epoch
//	watcher.lastCCEpochEndHeight = watcher.latestFinalizedHeight
//}

//func (watcher *Watcher) buildNewCCEpoch() *crosschain.CCEpoch {
//	epoch := &crosschain.CCEpoch{
//		StartHeight:   watcher.lastCCEpochEndHeight + 1,
//		TransferInfos: make([]*crosschain.CCTransferInfo, 0, 10),
//	}
//	for i := epoch.StartHeight; i <= watcher.latestFinalizedHeight; i++ {
//		blk, ok := watcher.heightToFinalizedBlock[i]
//		if !ok {
//			panic("Missing Block")
//		}
//		if epoch.EndTime < blk.Timestamp {
//			epoch.EndTime = blk.Timestamp
//		}
//		//epoch.TransferInfos = append(epoch.TransferInfos, blk.CCTransferInfos...)
//	}
//	return epoch
//}

func (watcher *Watcher) CheckMainnet() {
	if !watcher.rpcClient.IsConnected() {
		panic("Watcher mainnet zeniqd not connected. smartzeniqd cannot run without.")
	}
	watcher.logger.Info("Checking zeniq deamon GetMainnetHeight()")
	latestHeight := watcher.rpcClient.GetMainnetHeight()
	if latestHeight < 0 {
		panic("Watcher GetMainnetHeight failed.")
	}
}

// sort by pubkey (small to big) first; then sort by nominationCount;
// so nominations sort by NominationCount, if count is equal, smaller pubkey stand front
func sortEpochNominations(epoch *stake.Epoch) {
	sort.Slice(epoch.Nominations, func(i, j int) bool {
		return bytes.Compare(epoch.Nominations[i].Pubkey[:], epoch.Nominations[j].Pubkey[:]) < 0
	})
	sort.SliceStable(epoch.Nominations, func(i, j int) bool {
		return epoch.Nominations[i].NominatedCount > epoch.Nominations[j].NominatedCount
	})
}

func (watcher *Watcher) ClearOldData() {
	elLen := len(watcher.epochList)
	if elLen == 0 {
		return
	}
	height := watcher.epochList[elLen-1].StartHeight
	height -= 5 * watcher.numBlocksInEpoch
	for {
		_, ok := watcher.heightToFinalizedBlock[height]
		if !ok {
			break
		}
		delete(watcher.heightToFinalizedBlock, height)
		height--
	}
	if elLen > 5 /*param it*/ {
		watcher.epochList = watcher.epochList[elLen-5:]
	}
	ccEpochLen := len(watcher.ccEpochList)
	if ccEpochLen > 5*int(param.StakingNumBlocksInEpoch/param.BlocksInCCEpoch) {
		watcher.epochList = watcher.epochList[ccEpochLen-5:]
	}
}
