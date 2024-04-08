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

func loadLastEEBTSmartHeight(ctx *mevmtypes.Context) (lastEEBTSmartHeight int64) {
	var bz []byte
	bz = ctx.GetStorageAt(ccrpcSequence, Slotccrpc)
	if bz == nil {
		return 0
	}
	lastEEBTSmartHeight = int64(binary.BigEndian.Uint64(bz))
	return
}

func saveLastEEBTSmartHeight(ctx *mevmtypes.Context, lastEEBTSmartHeight int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(lastEEBTSmartHeight))
	ctx.SetStorageAt(ccrpcSequence, Slotccrpc, b[:])
}

type IContextGetter interface {
	GetRpcContext() *mevmtypes.Context
}

type CCrpcImp struct {
	logger         log.Logger
	rpcMainnet     watchertypes.RpcClient
	doneMainHeight int64
	ccrpcEpochList []*ccrpctypes.CCrpcEpoch
	ccrpcEpochChan chan *ccrpctypes.CCrpcEpoch
	CCRPCEpochs    [][3]int64
}

// Create ccrpc handler.
func Newccrpc(logger log.Logger, chainConfig *param.ChainConfig,
	rpcclient watchertypes.RpcClient) *CCrpcImp {
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
		doneMainHeight: chainConfig.AppConfig.CCRPCEpochs[0][0] - 1,
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

// forward epochs to CCRPCProcessed
func (cc *CCrpcImp) CCRPCMain() {
	if !cc.rpcMainnet.IsConnected() {
		panic(fmt.Errorf("ccrpc: error: CCRPCMain mainnet zeniqd not connected. smartzeniqd cannot run without"))
	}
	var mainHeight int64 = 0
	for {
		nextFirst := cc.doneMainHeight + 1
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
			cc.doneMainHeight = ccrpcEpoch.LastHeight
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
	ctx *mevmtypes.Context, blockStart int64, searchBack, eebt int64, log log.Logger) int64 {
	after := func(fromEnd int) bool {
		findidx := uint64(blockStart - int64(fromEnd))
		block, err := ctx.GetBlockByHeight(findidx)
		if err != nil {
			panic(fmt.Errorf("ccrpc: error: blockAfterTime error (%v) at %v?", err, findidx))
		}
		log.Debug(fmt.Sprintf("blockAfterTime trying index %v with query %v < %v", findidx, block.Timestamp, eebt))
		return block.Timestamp < eebt
	}
	var n_entries = int(searchBack)
	log.Debug(fmt.Sprintf("blockAfterTime searching %v entries backwards starting at block %v", n_entries, blockStart))
	if n_entries > int(blockStart) {
		n_entries = int(blockStart)
	}
	found := blockStart - int64(sort.Search(n_entries, after))
	if found == 0 {
		return found
	}
	// round up for error correction
	found = ((found >> 4) << 4) + 16
	return found
}

func (cc *CCrpcImp) CCRPCProcessed(ctx *mevmtypes.Context, blockNumber, timeStamp int64, bat func(int64, int64) int64) (doneApply bool) {
	var lastEEBTSmartHeight int64 = loadLastEEBTSmartHeight(ctx) // expecting 0 the first time
	var nextEEBTSmartHeight int64 = lastEEBTSmartHeight

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

	ccrpcEpochChan := make(chan *ccrpctypes.CCrpcEpoch, thisLen)
	processOne := func(cce *ccrpctypes.CCrpcEpoch) int64 {
		blOK := func(blockNumber int64) bool {
			block, _ := ctx.GetBlockByHeight(uint64(blockNumber))
			return block != nil
		}
		n, nn := cc.getEpochMainDelaySmart(cce.FirstHeight)
		if cce.EEBTSmartHeight == 0 {
			EEBT := cce.EpochEndBlockTime
			if bat != nil { // for testing
				cce.EEBTSmartHeight = bat(nn+1, EEBT)
			} else {
				blockNumber_1 := blockNumber - 1
				for ; blockNumber_1 > lastEEBTSmartHeight && !blOK(blockNumber_1); blockNumber_1-- {
				}
				cce.EEBTSmartHeight = blockAfterTime(ctx, blockNumber_1, nn+1, EEBT, cc.logger)
			}
			if cce.EEBTSmartHeight == 0 {
				cc.logger.Info(fmt.Sprintf("ccrpc: (%d-%d) EEBT %v not mapped to smart height",
					cce.FirstHeight, cce.LastHeight, EEBT))
				for _, ti := range cce.TransferInfos {
					cc.logger.Info(fmt.Sprintf("ccrpc: (%d-%d) EEBT %v ignored TransferInfo %v",
						cce.FirstHeight, cce.LastHeight, EEBT, ti))
				}
				return 0 // ignore cce
			}
			if cce.EEBTSmartHeight < lastEEBTSmartHeight {
				cc.logger.Info(fmt.Sprintf(
					"ccrpc: (%d-%d) new end of epoch smart height %d smaller than already processed one %d",
					cce.FirstHeight, cce.LastHeight, cce.EEBTSmartHeight, lastEEBTSmartHeight))
				return 0 // ignore cce
			}
		}
		var nn_new int64 = nn
		for cce.EEBTSmartHeight+nn_new < blockNumber {
			nn_new += 200 // 10*60/3
		}
		if nn_new > nn {
			// panic else resync would fail since we came here none-deterministically
			panic(fmt.Errorf("ccrpc: missed epoch. app.toml: add to cc-rpc-epochs [%d,%d,%d] and restart", cce.FirstHeight, n, nn_new))
		}
		var thisApply = cce.EEBTSmartHeight + nn
		if thisApply == blockNumber {
			cc.logger.Info(fmt.Sprintf("ccrpc: (%d-%d) with %d txs mapped to smart height %d, applying now at height=%d, delaying %d",
				cce.FirstHeight, cce.LastHeight, len(cce.TransferInfos), cce.EEBTSmartHeight, blockNumber, (thisApply - cce.EEBTSmartHeight)))
			for _, ti := range cce.TransferInfos {
				AccountCcrpc(ctx, ti)
			}
			return cce.EEBTSmartHeight // cce done
		}
		ccrpcEpochChan <- cce
		return 0 // cce for next time
	}

	var wg sync.WaitGroup
	for _, cce := range cc.ccrpcEpochList {
		if timeStamp > cce.EpochEndBlockTime {
			wg.Add(1)
			go func(pcce *ccrpctypes.CCrpcEpoch) {
				if nxt := processOne(pcce); nxt > atomic.LoadInt64(&nextEEBTSmartHeight) {
					atomic.StoreInt64(&nextEEBTSmartHeight, nxt)
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

	saveLastEEBTSmartHeight(ctx, nextEEBTSmartHeight) // next lastEEBTSmartHeight
	return len(cc.ccrpcEpochList) < thisLen
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
