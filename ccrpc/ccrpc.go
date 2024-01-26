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

	"github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

const (
	MIN_CCRPCEpochs = 6
	//The final x value in CCRPCEpochs[x][1] should be 1008.
	//Large enough to preclude a block reorg on mainnet
	//as that would make a sync impossible later on.
	ccrpcSequence uint64 = math.MaxUint64 - 5 /*uint64(-6)*/
)

var (
	ErrBalanceNotEnough        = errors.New("balance is not enough")
	Slotccrpc           string = strings.Repeat(string([]byte{0}), 32)
)

func loadEpochEndBlockNumber(ctx *mevmtypes.Context) int64 {
	var bz []byte
	bz = ctx.GetStorageAt(ccrpcSequence, Slotccrpc)
	if bz == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bz))
}

func saveEpochEndBlockNumber(ctx *mevmtypes.Context, ept int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(ept))
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
		ccrpcEpochList: make([]*ccrpctypes.CCrpcEpoch, 0, 11),
		ccrpcEpochChan: make(chan *ccrpctypes.CCrpcEpoch, 11),
		CCRPCEpochs:    chainConfig.AppConfig.CCRPCEpochs,
	}
}

func (cc *CCrpcImp) get_n(nextFirst int64) (nn int64, smartsPerMain int64) {
	nn = 0
	for _, v := range cc.CCRPCEpochs {
		if nextFirst < v[0] {
			break
		}
		nn = v[1]
	}
	// assume a smart block takes about 3 seconds
	smartsPerMain = 10 * 60 / 3
	return
}

// call as go routine after watcher.WatcherMain() finishes,
// i.e. hands over to this go routine after CCRPCMAINNET
// this adds X cc.ccrpcEpochChan and then blocks
// until 1 is removed by CCRPCProcessed()
// calls to CCRPCProcessed() will start only after reaching smartzeniq's CCRPCForkBlock
// which must be after CCRPCMAINNET's block time
func (cc *CCrpcImp) CCRPCMain() {
	if !cc.rpcMainnet.IsConnected() {
		panic("ccrpc CCRPCMain mainnet zeniqd not connected. smartzeniqd cannot run without")
	}
	for {
		nextFirst := cc.doneMainHeight + 1
		n, _ := cc.get_n(nextFirst)

		nextLast := nextFirst + n - 1
		if n == 0 {
			panic(fmt.Sprintf("ccrpc cc-rpc-epochs (app.toml): epoch 0 blocks at height %v", nextFirst))
		}
		cc.logger.Info(fmt.Sprintf("ccrpc first %d, last %d", nextFirst, nextLast))

		//we must consume more than half a standard main block of time
		//we must avoid that height is beyond +n*smartsPerMain and application is missed,
		//we must not be too close to the last epoch, because the mainnet epoch must mature.
		//this will overlap with the next fetch.

		// n/2 mainnet blocks earlier than 2*n to start fetching
		var n_2 int64 = n >> 1
		if n_2 == 0 {
			n_2 = 1
		}
		startFetching := nextLast + n - n_2
		mainnetHeight := cc.rpcMainnet.GetMainnetHeight()
		if mainnetHeight < startFetching { // only after some cycles we have reached the present
			cc.logger.Info(fmt.Sprintf("ccrpc mainnet height %d not yet %d", mainnetHeight, startFetching))
			cc.suspended(time.Duration(60) * time.Second)
			continue
		}
		cc.suspended(time.Duration(1) * time.Second)
		var ccrpcEpoch = cc.rpcMainnet.FetchCC(nextFirst, nextLast)
		if ccrpcEpoch != nil {
			cc.doneMainHeight = ccrpcEpoch.LastHeight
			cc.logger.Info(fmt.Sprintf("ccrpc (%d-%d) fetching at block %d", nextFirst, nextLast, startFetching))
			cc.ccrpcEpochChan <- ccrpcEpoch
		}
	}
}

type IAppCC interface {
	GetHistoryOnlyContext() *mevmtypes.Context
	GetLatestBlockNum() int64
}

func blockAfterTime(
	ctx *mevmtypes.Context, blockStart int64, searchBack, epochEndBlockTime int64, log log.Logger) int64 {
	after := func(fromEnd int) bool {
		findidx := uint64(blockStart - int64(fromEnd))
		block, err := ctx.GetBlockByHeight(findidx)
		if err != nil {
			panic(fmt.Errorf(
				"blockAfterTime error (%v) at %v: cc-rpc-epochs in app.toml not OK?", err, findidx))
		}
		log.Debug(fmt.Sprintf("blockAfterTime trying index %v with query %v < %v", findidx, block.Timestamp, epochEndBlockTime))
		return block.Timestamp < epochEndBlockTime
	}
	var n_entries = int(searchBack)
	log.Debug(fmt.Sprintf("blockAfterTime searching %v entries backwards starting at block %v", n_entries, blockStart))
	if n_entries > int(blockStart) {
		n_entries = int(blockStart)
	}
	return blockStart - int64(sort.Search(n_entries, after))
}

// called in Commit() to change accounts according the last entry in ccrpcEpochChan
// before the smartzeniq block whose blocktime overtakes an end of epoch time in zeniq mainnet
func (cc *CCrpcImp) CCRPCProcessed(ctx *mevmtypes.Context, blockNumber int64, bat func(int64, int64) int64) (smartHeight int64) {
	smartHeight = -1
	var lastBlockNumber int64 = loadEpochEndBlockNumber(ctx) // expecting 0 the first time
	select {
	case cce := <-cc.ccrpcEpochChan:
		cc.ccrpcEpochList = append(cc.ccrpcEpochList, cce)

		// while syncing up, blockTime will count through the time of cc0.EpochEndBlockTime in steps of about 1 second
		// if already synced up then blockTime is about the time of fetching
		// but both cases need to process at the same smart block / smart blocktime for the sync to work.
		// cc0.EpochEndBlockTime is mapped to EEBTSmartHeight
		// and n*10*60 smart blocks later booked to the smart accounts.
		// n is the epoch length in number of mainnet blocks.
		// smartsPerMain*n determines the delay in smart blocks before accounting the crosschain transactions.
		// NOTE these are different times:
		// - cc epoch is n mainnet blocks
		// - n*smartsPerMain in smart blocks as delay before actually accounting the cc epoch
		n, smartsPerMain := cc.get_n(cce.FirstHeight)

		blOK := func(blockNumber int64) bool {
			var blockTimestamp int64 = 0
			block, _ := ctx.GetBlockByHeight(uint64(blockNumber))
			if block != nil {
				blockTimestamp = block.Timestamp
			}
			cc.logger.Debug(fmt.Sprintf(
				"ccrpc got (%d-%d) with end of epoch time %d searching backwards from %d block %d",
				cce.FirstHeight, cce.LastHeight,
				cce.EpochEndBlockTime,
				blockTimestamp, blockNumber,
			))
			return block != nil
		}

		// EEBT = EpochEndBlockTime
		searchBack := 2 * n * smartsPerMain
		if bat != nil { // for testing
			cce.EEBTSmartHeight = bat(searchBack, cce.EpochEndBlockTime)
		} else {
			blockNumber_1 := blockNumber - 1
			for ; blockNumber_1 > lastBlockNumber && !blOK(blockNumber_1); blockNumber_1-- {
			}
			cce.EEBTSmartHeight = blockAfterTime(ctx, blockNumber_1, searchBack, cce.EpochEndBlockTime, cc.logger)
		}
		cc.logger.Info(fmt.Sprintf(
			"... found ccrpc smart height %d to apply at %d.",
			cce.EEBTSmartHeight,
			cce.EEBTSmartHeight+n*smartsPerMain,
		))

	default:
	}

	if len(cc.ccrpcEpochList) > 0 {
		cce := cc.ccrpcEpochList[0]
		n, smartsPerMain := cc.get_n(cce.FirstHeight)
		if n == 0 {
			panic(fmt.Sprintf("ccrpc cc-rpc-epochs (app.toml): processing: epoch 0 blocks at height %v", cce.FirstHeight))
		}
		if cce.EEBTSmartHeight == 0 {
			panic(fmt.Sprintf("ccrpc: processing: end of block smart height not yet mapped"))
		}
		applyNumber := cce.EEBTSmartHeight + n*smartsPerMain
		if applyNumber < blockNumber {
			panic(fmt.Sprintf(
				"ccrpc: processing: missed ccrpc application due at %v but already %v. Add a safer entry to cc-rpc-epochs.",
				applyNumber, blockNumber,
			))
		}
		if cce.EEBTSmartHeight+n*smartsPerMain == blockNumber {
			if cce.EEBTSmartHeight <= lastBlockNumber {
				panic(fmt.Errorf(
					"ccrpc %d mainnet epoch length, this smart block number for EndOfEpoch %d, and already processed one: %d",
					n, cce.EEBTSmartHeight, lastBlockNumber))
			}
			cc.logger.Info(fmt.Sprintf("ccrpc applying %d mainnet (%d-%d) from before smart %d now at %d",
				n, cce.FirstHeight, cce.LastHeight, cce.EEBTSmartHeight, blockNumber))
			for _, ti := range cce.TransferInfos {
				cc.logger.Debug(fmt.Sprintf("ccrpc applying TransferInfo %v", ti))
				AccountCcrpc(ctx, ti)
			}
			smartHeight = cce.EEBTSmartHeight
			saveEpochEndBlockNumber(ctx, cce.EEBTSmartHeight) // also do this if len(cce.TransferInfos)==0
			cc.ccrpcEpochList = cc.ccrpcEpochList[1:]
		}
	}
	return
}

func AccountCcrpc(ctx *mevmtypes.Context, ti *ccrpctypes.CCrpcTransferInfo) {
	// height := ti.Height // not used
	// txid := ti.TxID // not used
	receiver := ZeniqPubkeyToReceiverAddress(ti.SenderPubkey)
	amount := uint256.NewInt(0).Mul(uint256.NewInt(uint64(math.Round(ti.Amount*1e8))), uint256.NewInt(1e10))
	// book amount to receiver
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
	pubkey, err := bchutil.NewAddressPubKey(pubkeyBytes[:], &MainNetParams) //only using LegacyPubKeyHashAddrID
	if err != nil {
		panic(fmt.Errorf("ZeniqPubkeyToReceiverAddress %v", err))
	}
	return crypto.PubkeyToAddress(*pubkey.PubKey().ToECDSA())
}

var MainNetParams = chaincfg.Params{
	LegacyPubKeyHashAddrID: 110,
}

func transferBch(ctx *mevmtypes.Context, sender, receiver common.Address, value *uint256.Int) error {
	senderAcc := ctx.GetAccount(sender)
	balance := senderAcc.Balance()
	if balance.Lt(value) {
		return ErrBalanceNotEnough
	}
	if !value.IsZero() {
		balance.Sub(balance, value)
		senderAcc.UpdateBalance(balance)
		ctx.SetAccount(sender, senderAcc)

		receiverAcc := ctx.GetAccount(receiver)
		if receiverAcc == nil {
			receiverAcc = mevmtypes.ZeroAccountInfo()
		}
		receiverAccBalance := receiverAcc.Balance()
		receiverAccBalance.Add(receiverAccBalance, value)
		receiverAcc.UpdateBalance(receiverAccBalance)
		ctx.SetAccount(receiver, receiverAcc)
	}
	return nil
}

func (cc *CCrpcImp) suspended(delayDuration time.Duration) {
	time.Sleep(delayDuration)
}
