package ccrpc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/tendermint/tendermint/libs/log"

	mevmtypes "github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	"github.com/zeniqsmart/zeniq-smart-chain/watcher"
	watchertypes "github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
)

const (
	//The final x value in CCRPCEpochs[x][1] should be 1008.
	//Large enough to preclude a block reorg on mainnet
	//as that would make a sync impossible later on.
	ccrpcSequence uint64 = math.MaxUint64 - 5 /*uint64(-6)*/
	queue                = 111
)

var (
	ErrBalanceNotEnough        = errors.New("balance is not enough")
	Slotccrpc           string = strings.Repeat(string([]byte{0}), 32)
)

func loadDoneMain(ctx *mevmtypes.Context) (lastHeight int64) {
	var bz []byte
	bz = ctx.GetStorageAt(ccrpcSequence, Slotccrpc)
	if bz == nil {
		return 0
	}
	lastHeight = int64(binary.BigEndian.Uint64(bz))
	return
}

func saveDoneMain(ctx *mevmtypes.Context, lastHeight int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(lastHeight))
	ctx.SetStorageAt(ccrpcSequence, Slotccrpc, b[:])
}

type IContextGetter interface {
	GetRpcContext() *mevmtypes.Context
}

type CCrpcImp struct {
	logger         log.Logger
	rpcMainnet     watchertypes.RpcClient
	doneMainFetch  int64
	ccrpcEpochList []*ccrpctypes.CCrpcEpoch
	ccrpcEpochChan chan *ccrpctypes.CCrpcEpoch
	CCRPCEpochs    [][3]int64
}

// Create ccrpc handler.
func Newccrpc(logger log.Logger, chainConfig *param.ChainConfig, rpcclient watchertypes.RpcClient, ctx *mevmtypes.Context) *CCrpcImp {
	var doneMH int64
	doneMH = loadDoneMain(ctx)
	if doneMH == 0 {
		doneMH = chainConfig.AppConfig.CCRPCEpochs[0][0] - 1
	}
	var rpc = rpcclient
	if rpc == nil {
		rpc = watcher.NewRpcClient(
			chainConfig.AppConfig.MainnetRPCUrl,
			chainConfig.AppConfig.MainnetRPCUsername,
			chainConfig.AppConfig.MainnetRPCPassword,
			"text/plain;", logger)
	}
	return &CCrpcImp{
		logger:         logger,
		rpcMainnet:     rpc,
		doneMainFetch:  doneMH,
		ccrpcEpochList: make([]*ccrpctypes.CCrpcEpoch, 0),
		ccrpcEpochChan: make(chan *ccrpctypes.CCrpcEpoch, queue),
		CCRPCEpochs:    chainConfig.AppConfig.CCRPCEpochs,
	}
}

func (cc *CCrpcImp) getEpochMainDelaySmart(nextFirst int64) (n int64, nn int64) {
	n = 0
	nn = 0
	for _, v := range cc.CCRPCEpochs {
		if nextFirst < v[0] {
			break
		}
		n = v[1]
		nn = v[2]
	}
	if n == 0 {
		panic(fmt.Errorf("ccrpc: error: NOT OK cc-rpc-epochs at height %v", nextFirst))
	}
	return
}

// forward epochs to ProcessCCRPC
func (cc *CCrpcImp) CCRPCMain() {
	if !cc.rpcMainnet.IsConnected() {
		panic(fmt.Errorf("ccrpc: error: CCRPCMain mainnet zeniqd not connected. smartzeniqd cannot run without"))
	}
	var mainHeight int64 = 0
	for {
		nextFirst := cc.doneMainFetch + 1
		n, _ := cc.getEpochMainDelaySmart(nextFirst)

		nextLast := nextFirst + n - 1

		startFetching := nextLast + n
		for mainHeight < startFetching {
			mainHeight = cc.rpcMainnet.GetMainnetHeight()
			if mainHeight >= startFetching {
				break
			}
			cc.suspended(time.Duration(60) * time.Second)
		}

		var ccrpcEpoch = cc.rpcMainnet.FetchCC(nextFirst, nextLast)
		if ccrpcEpoch != nil {
			cc.doneMainFetch = ccrpcEpoch.LastHeight
			cc.logger.Info(fmt.Sprintf("ccrpc: CCRPCMain: %d txs from (%d-%d) fetched at %d",
				len(ccrpcEpoch.TransferInfos), nextFirst, nextLast, startFetching))
			cc.ccrpcEpochChan <- ccrpcEpoch // contains EEBT=EpochEndBlockTime as given by mainnet
		} else {
			panic(fmt.Errorf("ccrpc: error: fetching from mainnet failed"))
		}
	}
}

type IAppCC interface {
	GetHistoryOnlyContext() *mevmtypes.Context
	GetLatestBlockNum() int64
}

func blockAfterTime(
	ctx *mevmtypes.Context, blockStart int64, searchBack, eebt int64, log log.Logger) (ok, above int64) {
	notFound := fmt.Sprintf("ccrpc: not found eebt %d from %d back %d:", eebt, blockStart, searchBack)
	smallerThanEEBT := func(findidx int64) bool {
		block, err := ctx.GetBlockByHeight(uint64(findidx))
		if err != nil {
			panic(fmt.Errorf("%s error: blockAfterTime error (%v) at %v?", notFound, err, findidx))
		}
		return block.Timestamp < eebt
	}
	after := func(fromEnd int) bool {
		findidx := blockStart - int64(fromEnd)
		return smallerThanEEBT(findidx)
	}
	var n_entries = int(searchBack)
	totalLen := int(blockStart) // + 1 index to length, but no since we don't want to include 0
	if n_entries > totalLen {
		n_entries = totalLen
	}
	founde := int64(sort.Search(n_entries, after))
	found := blockStart - founde
	if founde == int64(n_entries) { // not found
		log.Info(fmt.Sprintf("%s checked %d", notFound, n_entries))
		return 0, 0
	}
	if found == 0 || found == 1 { // 1 genesis block might have a smaller time: ignore that
		log.Info(fmt.Sprintf("%s reached 0 or 1", notFound))
		return 0, 0
	}
	// do a linear search check
	for found > 1 && !smallerThanEEBT(found) {
		found -= 1
	}
	if found == 1 {
		log.Info(fmt.Sprintf("%s reached 1", notFound))
		return 0, 0
	}
	for smallerThanEEBT(found) {
		if found == blockStart {
			// log.Info(fmt.Sprintf("%s reached tip", notFound))
			return 0, found // tip but still smaller
		}
		found += 1
	}
	return found, 0
}

func (cc *CCrpcImp) ProcessCCRPC(ctx *mevmtypes.Context, blockNumber, timeStamp int64) (doneApply bool) {
	doneApply = false
	// cc.logger.Info(fmt.Sprintf("ccrpc: blockNumber %d, timeStamp %d", blockNumber, timeStamp))

	var lastDoneMain int64 = loadDoneMain(ctx) // expecting 0 the first time
	var nextDoneMain int64 = lastDoneMain

	var loop = true
	for loop {
		select {
		case cce := <-cc.ccrpcEpochChan:
			cce.EEBTSmartHeight = 0
			cc.ccrpcEpochList = append(cc.ccrpcEpochList, cce)
			loop = len(cc.ccrpcEpochList) < queue
		default:
			loop = false
		}
	}
	thisLen := len(cc.ccrpcEpochList)

	blOK := func(bn int64) bool {
		block, _ := ctx.GetBlockByHeight(uint64(bn))
		return block != nil
	}
	ccrpcEpochChan := make(chan *ccrpctypes.CCrpcEpoch, thisLen)
	processOne := func(cce *ccrpctypes.CCrpcEpoch) int64 {
		n, nn := cc.getEpochMainDelaySmart(cce.FirstHeight)
		EEBT := cce.EpochEndBlockTime
		cceLog := fmt.Sprintf("ccrpc: (%d-%d) %d txs EEBT %d",
			cce.FirstHeight, cce.LastHeight, len(cce.TransferInfos), EEBT)
		if cce.EEBTSmartHeight == 0 {
			blockNumber_1 := blockNumber - 1
			for ; blockNumber_1 > blockNumber-nn && !blOK(blockNumber_1); blockNumber_1-- {
			}
			var above int64 = 0
			cce.EEBTSmartHeight, above = blockAfterTime(ctx, blockNumber_1, nn, EEBT, cc.logger)
			cc.logger.Info(fmt.Sprintf("%s search %d back %d => (%d + %d) or above %d", cceLog,
				blockNumber_1, nn, cce.EEBTSmartHeight, nn, above))
			if above > 0 {
				ccrpcEpochChan <- cce
				return 0
			}
			if cce.EEBTSmartHeight == 0 {
				for _, ti := range cce.TransferInfos {
					cc.logger.Info(fmt.Sprintf("%s ignored TransferInfo %v", cceLog, ti))
				}
				bFirst, _ := ctx.GetBlockByHeight(uint64(blockNumber_1 - nn))
				bLast, _ := ctx.GetBlockByHeight(uint64(blockNumber_1))
				var b1, b2 int64 = 0, 0
				if bFirst != nil {
					b1 = bFirst.Timestamp
				}
				if bLast != nil {
					b2 = bLast.Timestamp
				}
				panic(fmt.Errorf("%s not mapped within %d [%d,%d]", cceLog, nn, b1, b2))
			}
		}
		var nn_new int64 = nn
		for cce.EEBTSmartHeight+nn_new < blockNumber {
			nn_new += 1200 // 2*3*10*60/3
		}
		if nn_new > nn {
			// panic else resync would fail since we came here none-deterministically
			panic(fmt.Errorf("%s missed epoch. app.toml: add to cc-rpc-epochs [%d,%d,%d]",
				cceLog, cce.FirstHeight, n, nn_new))
		}
		var thisApply = cce.EEBTSmartHeight + nn
		if thisApply == blockNumber {
			cc.logger.Info(fmt.Sprintf("%s mapped smart %d, applying now %d, delaying %d",
				cceLog, cce.EEBTSmartHeight, blockNumber, (thisApply - cce.EEBTSmartHeight)))
			for _, ti := range cce.TransferInfos {
				AccountCcrpc(ctx, ti)
			}
			return cce.LastHeight // cce done
		}
		ccrpcEpochChan <- cce
		return 0 // cce for next time
	}

	var wg sync.WaitGroup
	for _, cce := range cc.ccrpcEpochList {
		//+10 because (blockNumber-blockNumber_1) is normally 2, i.e. 2*3.3 plus a bit
		if timeStamp > cce.EpochEndBlockTime+10 && blockNumber > 1 && cce.LastHeight > lastDoneMain {
			wg.Add(1)
			go func(pcce *ccrpctypes.CCrpcEpoch) {
				if nxt := processOne(pcce); nxt > atomic.LoadInt64(&nextDoneMain) {
					atomic.StoreInt64(&nextDoneMain, nxt)
				}
				wg.Done()
			}(cce)
		} else {
			ccrpcEpochChan <- cce
		}
	}
	wg.Wait()
	cc.ccrpcEpochList = nil
	loop = true
	for loop {
		select {
		case cce := <-ccrpcEpochChan:
			cc.ccrpcEpochList = append(cc.ccrpcEpochList, cce)
		default:
			loop = false
		}
	}

	if nextDoneMain > lastDoneMain {
		doneApply = true
		saveDoneMain(ctx, nextDoneMain) // next lastDoneMain
		cc.logger.Info(fmt.Sprintf("ccrpc: done main epoch %d", nextDoneMain))
	}
	return
}

func AccountCcrpc(ctx *mevmtypes.Context, ti *ccrpctypes.CCrpcTransferInfo) {
	// height := ti.Height // not used
	// txid := ti.TxID // not used
	receiver := ZeniqPubkeyToReceiverAddress(ti.SenderPubkey)
	amount := uint256.NewInt(0).Mul(uint256.NewInt(uint64(math.Round(ti.Amount*1e8))), uint256.NewInt(1e10))
	// account amount to receiver
	receiverAcc := ctx.GetAccount(receiver)
	if receiverAcc == nil {
		receiverAcc = mevmtypes.ZeroAccountInfo() // which will get the balance and be saved/created
	}
	receiverAccBalance := receiverAcc.Balance()
	receiverAccBalance.Add(receiverAccBalance, amount)
	receiverAcc.UpdateBalance(receiverAccBalance)
	ctx.SetAccount(receiver, receiverAcc)
}

func ZeniqPubkeyToReceiverAddress(pubkeyBytes [33]byte) common.Address {
	publicKeyECDSA, _ := crypto.DecompressPubkey(pubkeyBytes[:])
	return crypto.PubkeyToAddress(*publicKeyECDSA)
}

func (cc *CCrpcImp) suspended(delayDuration time.Duration) {
	time.Sleep(delayDuration)
}
