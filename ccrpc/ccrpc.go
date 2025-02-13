package ccrpc

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/holiman/uint256"
	"github.com/shopspring/decimal"
	"github.com/tendermint/tendermint/libs/log"

	mevmtypes "github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	"github.com/zeniqsmart/zeniq-smart-chain/param"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/ebp"
)

const (
	//The final x value in CCRPCEpochs[x][1] should be
	//large enough to preclude a block reorg on mainnet
	//as that would make a sync impossible later on.
	ccrpcSequence uint64 = math.MaxUint64 - 5 /*uint64(-6)*/
)

var (
	Slotccrpc string = strings.Repeat(string([]byte{0}), 32)
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

type CCrpcImp struct {
	logger        log.Logger
	rpcMainnet    ccrpctypes.RpcClient
	ccrpcEpochs   [][]int64
	ccrpcInfos    []*ccrpctypes.CCrpcTransferInfo
	ccrpcSearchTo int64
	running       bool
	fifo          FifoCcrpc
}

// Create ccrpc handler.
func Newccrpc(logger log.Logger, chainConfig *param.ChainConfig, rpcclient ccrpctypes.RpcClient) *CCrpcImp {
	var rpc = rpcclient
	if rpc == nil {
		rpc = NewRpcClient(
			chainConfig.AppConfig.MainnetRPCUrl,
			chainConfig.AppConfig.MainnetRPCUsername,
			chainConfig.AppConfig.MainnetRPCPassword,
			"text/plain;", logger)
	}
	if !rpc.IsConnected() {
		panic(fmt.Errorf("ccrpc: error: mainnet zeniqd not connected. smartzeniqd cannot run without"))
	}
	return &CCrpcImp{
		logger:        logger,
		rpcMainnet:    rpc,
		ccrpcEpochs:   chainConfig.AppConfig.CCRPCEpochs,
		ccrpcInfos:    make([]*ccrpctypes.CCrpcTransferInfo, 0),
		ccrpcSearchTo: chainConfig.AppConfig.CCRPCForkBlock,
		running:       false,
		fifo:          NewFifo(chainConfig.AppConfig.DbDataPath, logger),
	}
}

func (cc *CCrpcImp) Close() {
	cc.fifo.Close()
}

func (cc *CCrpcImp) getEpoch(nextFirst int64) (n, nn, minimum int64) {
	n = 0
	nn = 0
	for _, v := range cc.ccrpcEpochs {
		if nextFirst < v[0] {
			break
		}
		n = v[1]
		nn = v[2]
		minimum = 0
		if len(v) > 3 {
			minimum = v[3]
		}
	}
	if n == 0 {
		panic(fmt.Errorf("ccrpc: error: NOT OK cc-rpc-epochs at height %v", nextFirst))
	}
	return
}

func (cc *CCrpcImp) Println(s string) {
	cc.fifo.Println(s)
}
func (cc *CCrpcImp) fetcher(ctx *mevmtypes.Context, wg *sync.WaitGroup) {
	var mainHeight int64 = 0
	var doneFetch int64 = cc.ccrpcEpochs[0][0] - 1 // from beginning to build ccrpcInfos
	lastFetched := cc.fifo.LastFetched()
	if lastFetched > 0 {
		doneFetch = lastFetched
	}
	wg.Done()
	for {
		nextFirst := doneFetch + 1
		n, _, minimum := cc.getEpoch(nextFirst)

		nextLast := nextFirst + n - 1

		startFetching := nextLast + n
		for mainHeight < startFetching {
			mainHeight = cc.MainRPC().GetMainnetHeight()
			if mainHeight >= startFetching {
				break
			}
			cc.suspended(time.Duration(60) * time.Second)
		}

		var ccrpcEpoch = cc.rpcMainnet.FetchCrosschain(nextFirst, nextLast, minimum)

		if ccrpcEpoch != nil {
			doneFetch = ccrpcEpoch.LastHeight
			cc.logger.Info(fmt.Sprintf("ccrpc: fetcher: %d txs from (%d-%d) fetched at %d",
				len(ccrpcEpoch.TransferInfos), nextFirst, nextLast, startFetching))
			cc.fifo.Come(ccrpcEpoch) // contains EEBT=EpochEndBlockTime as given by mainnet
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

func (cc *CCrpcImp) MainRPC() ccrpctypes.RpcClient {
	return cc.rpcMainnet
}

func parse_zeniq_amount(amount decimal.Decimal) *uint256.Int {
	d := amount.Shift(18)
	ds := d.String()
	u, err := uint256.FromDecimal(ds)
	if err != nil {
		panic(err)
	}
	return u
}

func (cc *CCrpcImp) ProcessCCRPC(ctx *mevmtypes.Context, currHeight, currTime int64) (doneApply bool) {
	doneApply = false

	if cc.ccrpcSearchTo > currHeight {
		return
	}
	if cc.ccrpcSearchTo <= currHeight {
		if !cc.running {
			cc.running = true
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go cc.fetcher(ctx, wg)
			wg.Wait()
		}
	}

	blOK := func(bn int64) bool {
		block, _ := ctx.GetBlockByHeight(uint64(bn))
		return block != nil
	}
	var nextCcrpcSearchTo int64 = cc.ccrpcSearchTo
	var lastDoneMain int64 = loadDoneMain(ctx) // expecting 0 the first time

	processOne := func(cce *ccrpctypes.CCrpcEpoch) int64 {
		n, nn, minimum := cc.getEpoch(cce.FirstHeight)
		EEBT := cce.EpochEndBlockTime
		cceLog := fmt.Sprintf("ccrpc: (%d-%d), n %d, nn %d, minimum %d, %d txs, EEBT %d",
			cce.FirstHeight, cce.LastHeight, n, nn, minimum, len(cce.TransferInfos), EEBT)
		if currTime <= cce.EpochEndBlockTime+10 || currHeight <= 1 {
			return 0
		}
		// cce.EEBTSmartHeight also for cce.LastHeight < lastDoneMain to fill cc.ccrpcInfos
		if cce.EEBTSmartHeight == 0 {
			backStart := currHeight - 1
			backTo := cc.ccrpcSearchTo
			for ; backStart > backTo && !blOK(backStart); backStart-- {
			}
			var above int64 = 0
			searchBack := backStart - backTo
			cce.EEBTSmartHeight, above = blockAfterTime(ctx, backStart, searchBack, EEBT, cc.logger)
			cc.logger.Info(fmt.Sprintf("%s search from %d back %d to %d => %d or above %d", cceLog,
				backStart, searchBack, backTo, cce.EEBTSmartHeight, above))
			if above > 0 {
				return 0
			}
			if cce.EEBTSmartHeight > nextCcrpcSearchTo {
				nextCcrpcSearchTo = cce.EEBTSmartHeight
			}
			for _, ti := range cce.TransferInfos {
				ti.ApplicationHeight = cce.EEBTSmartHeight + nn
				ti.Receiver = ZeniqPubkeyToReceiverAddress(ti.SenderPubkey)
				if cce.EEBTSmartHeight+nn <= currHeight {
					amount := parse_zeniq_amount(ti.Amount)
					ebp.AddCrosschain(amount)
				}
			}
			if cce.EEBTSmartHeight == 0 && /*for test*/ searchBack > 0 {
				for _, ti := range cce.TransferInfos {
					cc.logger.Info(fmt.Sprintf("%s ignored TransferInfo %v", cceLog, ti))
				}
				bFirst, _ := ctx.GetBlockByHeight(uint64(backTo))
				bLast, _ := ctx.GetBlockByHeight(uint64(backStart))
				var t1, t2 int64 = 0, 0
				if bFirst != nil {
					t1 = bFirst.Timestamp
				}
				if bLast != nil {
					t2 = bLast.Timestamp
				}
				panic(fmt.Errorf("%s not mapped within [%d,%d]. fix cc-rpc-epochs in app.toml", cceLog, t1, t2))
			}
		}
		if cce.LastHeight > lastDoneMain {
			var nn_new int64 = nn
			for cce.EEBTSmartHeight+nn_new < currHeight {
				nn_new += 1200 // 2*3*10*60/3
			}
			if nn_new > nn {
				// panic else resync would fail since we came here none-deterministically
				panic(fmt.Errorf("%s missed epoch. app.toml: add to cc-rpc-epochs [%d,%d,%d]",
					cceLog, cce.FirstHeight, n, nn_new))
			}
		}
		if cce.EEBTSmartHeight+nn == currHeight {
			cc.logger.Info(fmt.Sprintf("%s mapped smart %d, applying now %d, delaying %d",
				cceLog, cce.EEBTSmartHeight, currHeight, nn))
			for _, ti := range cce.TransferInfos {
				AccountCcrpc(ctx, ti)
			}
			return cce.LastHeight // cce done
		}
		return 0 // cce for next time
	}

	nextDoneMain := cc.fifo.Serve(lastDoneMain, processOne)
	cc.ccrpcSearchTo = nextCcrpcSearchTo

	if nextDoneMain > lastDoneMain {
		doneApply = true
		saveDoneMain(ctx, nextDoneMain) // next lastDoneMain
		cc.logger.Info(fmt.Sprintf("ccrpc: done main epoch %d", nextDoneMain))
	}
	return
}

func (cc *CCrpcImp) CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo {
	ccinfo := cc.fifo.CrosschainInfo(start, end)
	return ccinfo
}

func AccountCcrpc(ctx *mevmtypes.Context, ti *ccrpctypes.CCrpcTransferInfo) {
	amount := parse_zeniq_amount(ti.Amount)
	receiverAcc := ctx.GetAccount(ti.Receiver)
	if receiverAcc == nil {
		receiverAcc = mevmtypes.ZeroAccountInfo()
	}
	receiverAccBalance := receiverAcc.Balance()
	receiverAccBalance.Add(receiverAccBalance, amount)
	receiverAcc.UpdateBalance(receiverAccBalance)
	ctx.SetAccount(ti.Receiver, receiverAcc)
}

func (cc *CCrpcImp) suspended(delayDuration time.Duration) {
	time.Sleep(delayDuration)
}
