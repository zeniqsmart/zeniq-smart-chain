package ccrpc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
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
	MIN_CCRPCEpochs = 6
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

func loadLastEEBTSmartHeight(ctx *mevmtypes.Context) int64 {
	var bz []byte
	bz = ctx.GetStorageAt(ccrpcSequence, Slotccrpc)
	if bz == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

func saveLastEEBTSmartHeight(ctx *mevmtypes.Context, lastEEBTSmartHeight int64) int64 {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(lastEEBTSmartHeight))
	ctx.SetStorageAt(ccrpcSequence, Slotccrpc, b[:])
	return lastEEBTSmartHeight
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
	CCRPCEpochs    [][2]int64
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
		ccrpcEpochList: make([]*ccrpctypes.CCrpcEpoch, 0, queue),
		ccrpcEpochChan: make(chan *ccrpctypes.CCrpcEpoch, queue),
		CCRPCEpochs:    chainConfig.AppConfig.CCRPCEpochs,
	}
}

func (cc *CCrpcImp) getEpochMainSmart(nextFirst int64) (n int64, nn int64) {
	n = 0
	for _, v := range cc.CCRPCEpochs {
		if nextFirst < v[0] {
			break
		}
		n = v[1]
	}
	if n == 0 {
		panic(fmt.Errorf("ccrpc: error: epoch of 0 blocks at height %v", nextFirst))
	}
	nn = n * 10 * 60 / 3
	return
}

// forward epochs to CCRPCProcessed
func (cc *CCrpcImp) CCRPCMain() {
	if !cc.rpcMainnet.IsConnected() {
		panic(fmt.Errorf("ccrpc: error: CCRPCMain mainnet zeniqd not connected. smartzeniqd cannot run without"))
	}
	for {
		nextFirst := cc.doneMainHeight + 1
		n, _ := cc.getEpochMainSmart(nextFirst)

		nextLast := nextFirst + n - 1
		cc.logger.Debug(fmt.Sprintf("ccrpc: CCRPCMain: next will be (%d-%d)", nextFirst, nextLast))

		// to start fetching an epoch later
		startFetching := nextLast + n
		mainnetHeight := cc.rpcMainnet.GetMainnetHeight()
		if mainnetHeight < startFetching { // only after some cycles we have reached the present
			cc.logger.Debug(fmt.Sprintf(
				"ccrpc: CCRPCMain: delaying (%d-%d), now %d, due %d",
				nextFirst, nextLast, mainnetHeight, startFetching))
			cc.suspended(time.Duration(60) * time.Second)
			continue
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
	return blockStart - int64(sort.Search(n_entries, after))
}

func (cc *CCrpcImp) CCRPCProcessed(ctx *mevmtypes.Context, blockNumber int64, bat func(int64, int64) int64) (doneApply int64) {
	doneApply = -1
	var lastEEBTSmartHeight int64 = loadLastEEBTSmartHeight(ctx) // expecting 0 the first time

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

	blOK := func(blockNumber int64) bool {
		block, _ := ctx.GetBlockByHeight(uint64(blockNumber))
		return block != nil
	}

	for len(cc.ccrpcEpochList) > 0 {
		cce := cc.ccrpcEpochList[0]
		EEBT := cce.EpochEndBlockTime
		_, nn := cc.getEpochMainSmart(cce.FirstHeight)
		if cce.EEBTSmartHeight == 0 { // we maybe did the mapping already
			searchBack := 4 * nn
			if bat != nil { // for testing
				cce.EEBTSmartHeight = bat(searchBack, EEBT)
			} else {
				blockNumber_1 := blockNumber - 1
				for ; blockNumber_1 > lastEEBTSmartHeight && !blOK(blockNumber_1); blockNumber_1-- {
				}
				cce.EEBTSmartHeight = blockAfterTime(ctx, blockNumber_1, searchBack, EEBT, cc.logger)
			}
			if cce.EEBTSmartHeight == 0 {
				cc.logger.Debug(fmt.Sprintf("ccrpc: (%d-%d) EEBT %v not mapped to smart height",
					cce.FirstHeight, cce.LastHeight, EEBT))
				if len(cce.TransferInfos) > 0 {
					for _, ti := range cce.TransferInfos {
						cc.logger.Debug(fmt.Sprintf("ccrpc: (%d-%d) EEBT %v ignored TransferInfo %v",
							cce.FirstHeight, cce.LastHeight, EEBT, ti))
					}
				} else {
					cc.logger.Debug(fmt.Sprintf("ccrpc: (%d-%d) EEBT %v, ignoring as no txs",
						cce.FirstHeight, cce.LastHeight, EEBT))
				}
				cc.ccrpcEpochList = cc.ccrpcEpochList[1:]
				continue
			}
			if cce.EEBTSmartHeight < lastEEBTSmartHeight {
				cc.logger.Info(fmt.Sprintf(
					"ccrpc: (%d-%d) new end of epoch smart height %d smaller than already processed one %d",
					cce.FirstHeight, cce.LastHeight, cce.EEBTSmartHeight, lastEEBTSmartHeight))
				cc.ccrpcEpochList = cc.ccrpcEpochList[1:]
				continue
			}
		}
		var smart_to_main_correction float64 = 1.0
		if lastEEBTSmartHeight != 0 && cce.EEBTSmartHeight > lastEEBTSmartHeight {
			smart_to_main_correction = 1.0 * float64(cce.EEBTSmartHeight-lastEEBTSmartHeight) / float64(nn)
		}
		smartDelay := int64(2.0 * float64(nn) * smart_to_main_correction)
		var thisApply = cce.EEBTSmartHeight + smartDelay
		cc.logger.Debug(fmt.Sprintf("ccrpc: (%d-%d) to apply at %d (delay %d) with %d txs",
			cce.FirstHeight, cce.LastHeight, thisApply, smartDelay, len(cce.TransferInfos)))
		if thisApply <= blockNumber {

			for thisApply < blockNumber && len(cce.TransferInfos) > 0 {
				cc.logger.Debug(fmt.Sprintf(
					"ccrpc: (%d-%d) missed apply due at %v. Adding another delay %d to the due time.",
					cce.FirstHeight, cce.LastHeight, thisApply, smartDelay))
				thisApply = thisApply + smartDelay
			}

			if thisApply == blockNumber {
				cc.logger.Info(fmt.Sprintf("ccrpc: (%d-%d) mapped to smart height=%d, applying now at height=%d",
					cce.FirstHeight, cce.LastHeight, cce.EEBTSmartHeight, blockNumber))
			} else {
				break
			}

			for _, ti := range cce.TransferInfos {
				cc.logger.Debug(fmt.Sprintf("ccrpc: applying TransferInfo %v", ti))
				AccountCcrpc(ctx, ti)
			}
			doneApply = cce.EEBTSmartHeight // return value
			lastEEBTSmartHeight = saveLastEEBTSmartHeight(ctx, cce.EEBTSmartHeight)
			cc.ccrpcEpochList = cc.ccrpcEpochList[1:]
		} else {
			break
		}
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
