package app

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"

	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	gethcore "github.com/ethereum/go-ethereum/core"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/holiman/uint256"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"

	// tmstate "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/zeniqsmart/ads-zeniq-smart-chain/ads"
	"github.com/zeniqsmart/ads-zeniq-smart-chain/store"
	"github.com/zeniqsmart/ads-zeniq-smart-chain/store/rabbit"
	"github.com/zeniqsmart/db-zeniq-smart-chain/db"

	dbtypes "github.com/zeniqsmart/db-zeniq-smart-chain/types"
	"github.com/zeniqsmart/evm-zeniq-smart-chain/ebp"
	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"

	"github.com/zeniqsmart/zeniq-smart-chain/internal/ethutils"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	"github.com/zeniqsmart/zeniq-smart-chain/staking"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"

	"github.com/zeniqsmart/zeniq-smart-chain/ccrpc"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

var (
	_ abcitypes.Application = (*App)(nil)
	_ IApp                  = (*App)(nil)
)

const (
	CannotDecodeTx       uint32 = 101
	CannotRecoverSender  uint32 = 102
	SenderNotFound       uint32 = 103
	AccountNonceMismatch uint32 = 104
	CannotPayGasFee      uint32 = 105
	GasLimitInvalid      uint32 = 106
	InvalidMinGasPrice   uint32 = 107
	HasPendingTx         uint32 = 108
	MempoolBusy          uint32 = 109
	GasLimitTooSmall     uint32 = 110
	// HEIGHT_PUBLISH_AFTER_BLOCK cannot be activate
	// because it would lose compliance with existing scripts and ethereum
	HEIGHT_PUBLISH_AFTER_BLOCK int64 = 0x7FFFFFFFFFFFFFFF
)

var (

/*
	EIP1559?

defaultBaseFeePerGas = uint256.NewInt(0).Bytes32()
defaultBaseFeeBlob   = uint256.NewInt(0).Bytes32()
*/
)

var (
	mainStartHeights = [...]int64{
		121901, 123917, 125933, 127949, 129965, 131981, 133997, 136013, 138029,
		140045, 142061, 144077, 146093, 148109, 150125, 152141, 154157, 156173,
		158189, 160205, 162221, 164237, 166253, 168269, 170285, 172301, 174317,
		176333, 178349, 180365, 182381, 184397, 186413, 188429, 190445, 192461,
		194477, 196493, 198509, 200525, 202541, 204557, 206573, 208589, 210605,
		212621, 214637, 216653}
	mainEndTimes = [...]int64{
		1655493388, 1656708178, 1657913318, 1659117037, 1660333525, 1661538992,
		1662751048, 1663963628, 1665171804, 1666387339, 1667596930, 1668808103,
		1670016286, 1671230243, 1672439080, 1673646974, 1674846192, 1676062145,
		1677264872, 1678493252, 1679693750, 1680899064, 1682104404, 1683312425,
		1684523135, 1685735117, 1686945653, 1688155407, 1689363327, 1690564098,
		1691776545, 1692988845, 1694204584, 1695410336, 1696616840, 1697824405,
		1699035453, 1700246287, 1701451583, 1702662933, 1703871725, 1705083467,
		1706290414, 1707500717, 1708713749, 1709922444, 1711128053, 1712338342}
)

type IApp interface {
	ChainID() *uint256.Int
	GetRunTxContext() *types.Context
	GetRpcContext() *types.Context
	GetRpcContextAtHeight(height int64) *types.Context
	GetHistoryOnlyContext() *types.Context
	RunTxForRpc(gethTx *gethtypes.Transaction, sender gethcmn.Address, estimateGas bool, height int64) (*ebp.TxRunner, int64)
	RunTxForSbchRpc(gethTx *gethtypes.Transaction, sender gethcmn.Address, height int64) (*ebp.TxRunner, int64)
	GetCurrEpoch() *stake.Epoch
	GetLatestBlockNum() int64
	SubscribeChainEvent(ch chan<- types.ChainEvent) event.Subscription
	SubscribeLogsEvent(ch chan<- []*gethtypes.Log) event.Subscription
	SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription
	LoadBlockInfo() *types.BlockInfo
	GetValidatorsInfo() ValidatorsInfo
	IsArchiveMode() bool
	GetBlockForSync(height int64) (syncBytes []byte, err error)
	GetCCRPCForkBlock() int64
	CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo
}

type App struct {
	mtx sync.Mutex

	//config
	config  *param.ChainConfig
	chainId *uint256.Int

	//store
	mads         *ads.ADS
	root         *store.RootStore
	historyStore dbtypes.DB

	currHeight int64
	trunk      *store.TrunkStore
	checkTrunk *store.TrunkStore
	// 'block' contains some meta information of a block. It is collected during BeginBlock&DeliverTx,
	// and save to world state in Commit.
	block *types.Block
	// 'blockInfo', filled via syncBlockInfo in Commit, will be later used by RpcContext
	// Thus, eth_call can view the new block's height a little earlier than eth_blockNumber
	blockInfo       atomic.Value // to store *types.BlockInfo
	slashValidators [][20]byte   // updated in BeginBlock, used in Commit
	lastVoters      [][]byte     // updated in BeginBlock, used in Commit
	lastProposer    [20]byte     // updated in refresh of last block, used in updateValidatorsAndStakingInfo
	// of current block. It needs to be reloaded in NewApp
	lastGasUsed     uint64      // updated in last block's postCommit, used in current block's refresh
	lastGasRefund   uint256.Int // updated in last block's postCommit, used in current block's refresh
	lastGasFee      uint256.Int // updated in last block's postCommit, used in current block's refresh
	lastMinGasPrice uint64      // updated in refresh, used in next block's CheckTx and Commit. It needs
	// to be reloaded in NewApp
	txid2sigMap map[[32]byte][65]byte //updated in DeliverTx, flushed in refresh

	// feeds
	chainFeed event.Feed // For pub&sub new blocks
	logsFeed  event.Feed // For pub&sub new logs
	txsFeed   event.Feed // For NewPendingTransactionFilter
	scope     event.SubscriptionScope

	//engine
	txEngine    ebp.TxExecutor
	reorderSeed int64        // recorded in BeginBlock, used in Commit
	frontier    ebp.Frontier // recorded in Commit, used in next block's CheckTx

	//util
	signer gethtypes.Signer
	logger log.Logger

	// tendermint wants to know validators whose voting power change
	// it is loaded from ctx in Commit and used in EndBlock
	validatorUpdate []*stake.Validator

	//signature cache, cache ecrecovery's resulting sender addresses, to speed up checktx
	sigCache map[gethcmn.Hash]SenderAndHeight

	// it shows how many tx remains in the mempool after committing a new block
	recheckCounter int

	CCRPC *ccrpc.CCrpcImp

	appHash []byte
}

// The value entry of signature cache. The Height helps in evicting old entries.
type SenderAndHeight struct {
	Sender gethcmn.Address
	Height int64
}

func (app *App) fromHeightRevision(height int64) uint32 {
	var rev uint32
	rev = 7 // EVMC_ISTANBUL is default since height 0
	for _, h_r := range app.config.AppConfig.HeightRevision {
		if height < h_r[0] {
			break
		}
		rev = uint32(h_r[1])
	}
	// app.logger.Info(fmt.Sprintf("Revision %v for height %v\n", rev, height))
	return rev
}

func validHeightRevision(e [][2]int64) {
	if e == nil || len(e) <= 1 {
		panic(fmt.Sprintf("height-revision %v must have [0,7] and at least one more entries\n", e))
	}
	var lastee0 int64 = 0
	for ie, ee := range e {
		if ie == 0 && ee[0] != 0 && ee[1] != 7 {
			panic(fmt.Sprintf("height-revision %v must start with [0,7]\n", e))
		}
		if ie > 0 && ee[0] < 14444444 {
			panic(fmt.Sprintf("i>0 height-revision %v: block cannot be smaller than 14444444\n", e))
		}
		if ee[1] < 7 {
			panic(fmt.Sprintf("height-revision %v: revision cannot be smaller than 7 (EVMC_ISTANBUL)\n", e))
		}
		if ee[0] < lastee0 {
			panic(fmt.Sprintf("height-revision %v smaller than previous entry\n", e))
		}
		lastee0 = ee[0]
	}
}

func validCCRPCEpochs(e [][]int64) {
	if e == nil || len(e) == 0 {
		panic("cc-rpc-epochs not set\n")
	}
	var lastee0 int64 = 0
	for _, ee := range e {
		if len(ee) < 3 {
			panic("cc-rpc-epochs mast have at least 3 entries\n")
		}
		if ee[0] < lastee0 {
			panic("cc-rpc-epochs block smaller than previous entry\n")
		}
		if ee[0] == 0 || ee[1] == 0 {
			panic("Zero in cc-rpc-epochs entry\n")
		}
		if ee[1] < 6 || ee[1] > 2500 {
			panic(fmt.Sprintf("cc-rpc-epochs epoch count NOT smaller than 6 and NOT larger than 2500\n"))
		}
		if ee[2] < ee[1]*1000 || ee[2]%2 != 0 {
			panic(fmt.Errorf("ccrpc: error: (%d,%d) second smart not even and >= 1000 times first main", ee[1], ee[2]))
		}
		lastee0 = ee[0]
	}
}

func NewApp(config *param.ChainConfig, chainId *uint256.Int,
	genesisWatcherHeight int64, logger log.Logger, rpccl ccrpctypes.RpcClient) (app *App) {

	app = &App{}

	/*------set config------*/
	app.config = config
	app.chainId = chainId

	app.logger = logger.With("module", "app")
	app.logger.Info(fmt.Sprintf("Version %v \n", GitTag))
	app.logger.Info(fmt.Sprintf("Revision %v \n", app.config.AppConfig.HeightRevision))

	/*------ CCRPC check ------*/
	app.logger.Info(fmt.Sprintf("NewApp CCRPCEpochs %v\n", config.AppConfig.CCRPCEpochs))
	if !config.AppConfig.Testing {
		validCCRPCEpochs(config.AppConfig.CCRPCEpochs)
	}

	/*------ height-revision check ------*/
	if !config.AppConfig.Testing {
		validHeightRevision(config.AppConfig.HeightRevision)
	}

	/*------signature cache------*/
	app.sigCache = make(map[gethcmn.Hash]SenderAndHeight, config.AppConfig.SigCacheSize)

	/*------set util------*/
	app.signer = gethtypes.NewEIP155Signer(app.chainId.ToBig())

	/*------set store------*/
	app.root, app.mads = CreateRootStore(config.AppConfig.AppDataPath, config.AppConfig.ArchiveMode)
	app.historyStore = CreateHistoryStore(config.AppConfig.DbDataPath, config.AppConfig.UseLiteDB, config.AppConfig.RpcEthGetLogsMaxResults,
		app.logger.With("module", "db"))
	app.trunk = app.root.GetTrunkStore(config.AppConfig.TrunkCacheSize).(*store.TrunkStore)
	app.checkTrunk = app.root.GetReadOnlyTrunkStore(config.AppConfig.TrunkCacheSize).(*store.TrunkStore)

	app.appHash = app.root.GetRootHash()

	CCRPCForkBlock := app.GetCCRPCForkBlock()
	/*------set engine------*/
	app.txEngine = ebp.NewEbpTxExec(
		param.EbpExeRoundCount,
		param.EbpRunnerNumber,
		param.EbpParallelNum,
		5000, /*not consensus relevant*/
		app.signer,
		app.logger.With("module", "engine"),
		CCRPCForkBlock,
	)
	//ebp.AdjustGasUsed = false

	/*------set system contract------*/
	ctx := app.GetRunTxContext()
	defer func() {
		app.lastMinGasPrice = staking.LoadMinGasPrice(ctx, true)
		ctx.Close(true)
	}()

	ebp.RegisterPredefinedContract(ctx, staking.StakingContractAddress, staking.NewStakingContractExecutor(app.logger.With("module", "staking")))

	// We assign empty maps to them just to avoid accessing nil-maps.
	// Commit will assign meaningful contents to them
	app.txid2sigMap = make(map[[32]byte][65]byte)
	app.frontier = ebp.GetEmptyFrontier()

	/*------set refresh field------*/
	prevBlock := ctx.GetCurrBlockBasicInfo()
	if prevBlock != nil {
		app.block = prevBlock //will be overwritten in BeginBlock soon
		app.currHeight = app.block.Number
		app.lastProposer = app.block.Miner
	} else {
		app.block = &types.Block{}
	}

	app.root.SetHeight(app.currHeight)
	app.txEngine.SetContext(app.GetRunTxContext())
	if app.currHeight > 0 && app.currHeight < HEIGHT_PUBLISH_AFTER_BLOCK { // for (=>) avoid a repeat after restart or crash
		app.mtx.Lock()
		app.postCommit(app.syncBlockInfo(), false)
	}

	/*------set stakingInfo------*/
	stakingInfo := staking.LoadStakingInfo(ctx)
	currValidators := stake.GetActiveValidators(stakingInfo.Validators, staking.MinimumStakingAmount)
	app.validatorUpdate = stakingInfo.ValidatorsUpdate
	for _, val := range currValidators {
		app.logger.Debug(fmt.Sprintf("Load validator in NewApp: address(%s), pubkey(%s), votingPower(%d)",
			gethcmn.Address(val.Address).String(), ed25519.PubKey(val.Pubkey[:]).String(), val.VotingPower))
	}
	if stakingInfo.CurrEpochNum == 0 && stakingInfo.GenesisMainnetBlockHeight == 0 {
		stakingInfo.GenesisMainnetBlockHeight = genesisWatcherHeight
		staking.SaveStakingInfo(ctx, stakingInfo) // only executed at genesis
	}

	r := time.Duration(rand.Int() % 1000)
	time.Sleep(r * time.Millisecond)

	/*------CCRPC------*/
	app.CCRPC = ccrpc.Newccrpc(app.logger.With("module", "ccrpc"), app.config, rpccl)

	return
}

func CreateRootStore(dataPath string, isArchiveMode bool) (*store.RootStore, *ads.ADS) {
	first := [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
	last := [8]byte{255, 255, 255, 255, 255, 255, 255, 255}
	mads, err := ads.NewADS(dataPath, isArchiveMode,
		[][]byte{first[:], last[:]})
	if err != nil {
		panic(err)
	}
	root := store.NewRootStore(mads, func(k []byte) bool {
		return len(k) >= 1 && k[0] > (128+64) //only cache the standby queue
	})
	return root, mads
}

func CreateHistoryStore(dataPath string, useLiteDB bool, maxLogResults int, logger log.Logger) (historyStore dbtypes.DB) {
	dbDir := dataPath
	if useLiteDB {
		historyStore = db.NewLiteDB(dbDir)
	} else {
		if _, err := os.Stat(dbDir); os.IsNotExist(err) {
			_ = os.MkdirAll(path.Join(dbDir, "data"), 0700)
			var seed [8]byte // use current time as db's hash seed
			binary.LittleEndian.PutUint64(seed[:], uint64(time.Now().UnixNano()))
			historyStore = db.CreateEmptyDB(dbDir, seed, logger)
		} else {
			historyStore = db.NewDB(dbDir, logger)
		}
		historyStore.SetMaxEntryCount(maxLogResults)
	}
	return
}

func (app *App) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{
		LastBlockHeight:  app.block.Number,
		LastBlockAppHash: app.appHash,
	}
}

func (app *App) SetOption(option abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{} // take it as a nop
}

func (app *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: abcitypes.CodeTypeOK} // take it as a nop
}

func (app *App) sigCacheAdd(txid gethcmn.Hash, value SenderAndHeight) {
	if len(app.sigCache) > app.config.AppConfig.SigCacheSize { //select one old entry to evict
		delKey, minHeight, count := gethcmn.Hash{}, int64(math.MaxInt64), 6 /*iterate 6 steps*/
		for key, value := range app.sigCache {                              //pseudo-random iterate
			if minHeight > value.Height { //select the oldest entry within a short iteration
				minHeight, delKey = value.Height, key
			}
			if count--; count == 0 {
				break
			}
		}
		delete(app.sigCache, delKey)
	}
	app.sigCache[txid] = value
}

func (app *App) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	app.logger.Debug("enter check tx!")
	if req.Type == abcitypes.CheckTxType_Recheck {
		app.recheckCounter++ // calculate how many TXs remain in the mempool after a new block
	} else if app.recheckCounter > app.config.AppConfig.RecheckThreshold {
		// Refuse to accept new TXs on P2P to drain the remain TXs in mempool
		return abcitypes.ResponseCheckTx{Code: MempoolBusy, Info: "mempool is too busy"}
	}
	tx := &gethtypes.Transaction{}
	err := tx.DecodeRLP(rlp.NewStream(bytes.NewReader(req.Tx), 0))
	if err != nil {
		return abcitypes.ResponseCheckTx{Code: CannotDecodeTx}
	}
	txid := tx.Hash()
	var sender gethcmn.Address
	senderAndHeight, ok := app.sigCache[txid]
	if ok { // cache hit
		sender = senderAndHeight.Sender
	} else { // cache miss
		sender, err = app.signer.Sender(tx)
		if err != nil {
			return abcitypes.ResponseCheckTx{Code: CannotRecoverSender, Info: "invalid sender: " + err.Error()}
		}
		app.sigCacheAdd(txid, SenderAndHeight{sender, app.currHeight})
	}
	return app.checkTxWithContext(tx, sender, req.Type)
}

func (app *App) checkTxWithContext(tx *gethtypes.Transaction, sender gethcmn.Address, txType abcitypes.CheckTxType) abcitypes.ResponseCheckTx {
	ctx := app.GetCheckTxContext()
	defer ctx.Close(false)
	if ok, res := checkGasLimit(tx); !ok {
		return res
	}
	acc := ctx.GetAccount(sender)
	if acc == nil {
		return abcitypes.ResponseCheckTx{Code: SenderNotFound, Info: types.ErrAccountNotExist.Error()}
	}
	targetNonce, exist := app.frontier.GetLatestNonce(sender)
	if !exist {
		app.frontier.SetLatestBalance(sender, acc.Balance().Clone())
		targetNonce = acc.Nonce()
	}

	app.logger.Debug("checkTxWithContext",
		"isReCheckTx", txType == abcitypes.CheckTxType_Recheck,
		"tx.nonce", tx.Nonce(),
		"account.nonce", acc.Nonce(),
		"targetNonce", targetNonce)

	if tx.Nonce() > targetNonce {
		return abcitypes.ResponseCheckTx{Code: AccountNonceMismatch, Info: "bad nonce: " + types.ErrNonceTooLarge.Error()}
	} else if tx.Nonce() < targetNonce {
		return abcitypes.ResponseCheckTx{Code: AccountNonceMismatch, Info: "bad nonce: " + types.ErrNonceTooSmall.Error()}
	}
	gasPrice, _ := uint256.FromBig(tx.GasPrice())

	if gasPrice.GtUint64(ebp.MaxGasPrice) {
		gasPrice = uint256.NewInt(ebp.MaxGasPrice)
	}

	gasFee := uint256.NewInt(0).Mul(gasPrice, uint256.NewInt(tx.Gas()))
	if gasPrice.Cmp(uint256.NewInt(app.lastMinGasPrice)) < 0 {
		return abcitypes.ResponseCheckTx{Code: InvalidMinGasPrice, Info: "gas price too small"}
	}

	balance, ok := app.frontier.GetLatestBalance(sender)
	if !ok || balance.Cmp(gasFee) < 0 {
		return abcitypes.ResponseCheckTx{Code: CannotPayGasFee, Info: "failed to deduct tx fee"}
	}
	totalGasLimit, _ := app.frontier.GetLatestTotalGas(sender)
	if exist { // We do not count in the gas of the first tx found during CheckTx
		totalGasLimit += tx.Gas()
	}
	if totalGasLimit > app.config.AppConfig.FrontierGasLimit {
		return abcitypes.ResponseCheckTx{Code: GasLimitInvalid, Info: "send transaction too frequent"}
	}
	app.frontier.SetLatestTotalGas(sender, totalGasLimit)

	//update frontier
	app.frontier.SetLatestNonce(sender, tx.Nonce()+1)
	balance.Sub(balance, gasFee)
	value, _ := uint256.FromBig(tx.Value())
	if balance.Cmp(value) < 0 {
		app.frontier.SetLatestBalance(sender, uint256.NewInt(0))
	} else {
		balance = balance.Sub(balance, value)
		app.frontier.SetLatestBalance(sender, balance)
	}
	app.logger.Debug("checkTxWithContext:", "value", value.String(), "balance", balance.String())
	app.logger.Debug("leave check tx!")

	return abcitypes.ResponseCheckTx{
		Code:      abcitypes.CodeTypeOK,
		GasWanted: int64(tx.Gas()),
	}
}

func checkGasLimit(tx *gethtypes.Transaction) (ok bool, res abcitypes.ResponseCheckTx) {
	intrinsicGas, err2 := gethcore.IntrinsicGas(tx.Data(), nil, tx.To() == nil, true, true)
	if err2 != nil || tx.Gas() < intrinsicGas {
		return false, abcitypes.ResponseCheckTx{Code: GasLimitTooSmall, Info: "gas limit too small"}
	}
	if tx.Gas() > param.MaxTxGasLimit {
		return false, abcitypes.ResponseCheckTx{Code: GasLimitInvalid, Info: "invalid gas limit"}
	}
	return true, abcitypes.ResponseCheckTx{}
}

func (app *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	app.logger.Debug("InitChain, id=", req.ChainId)

	if len(req.AppStateBytes) == 0 {
		genDocJSON, _ := ioutil.ReadFile(app.config.NodeConfig.BaseConfig.Genesis)
		genDoc, _ := tmtypes.GenesisDocFromJSON(genDocJSON)
		if genDoc == nil || len(genDoc.AppState) == 0 {
			panic("no AppState")
		}
		req.AppStateBytes = genDoc.AppState
	}

	genesisData := GenesisData{}
	err := json.Unmarshal(req.AppStateBytes, &genesisData)
	if err != nil {
		panic(err)
	}

	app.createGenesisAccounts(genesisData.Alloc)
	genesisValidators := genesisData.StakingValidators()

	if len(genesisValidators) == 0 {
		panic("no genesis validator in genesis.json")
	}

	//store all genesis validators even if it is inactive
	ctx := app.GetRunTxContext()
	staking.AddGenesisValidatorsIntoStakingInfo(ctx, genesisValidators)
	ctx.Close(true)

	currValidators := stake.GetActiveValidators(genesisValidators, staking.MinimumStakingAmount)
	valSet := make([]abcitypes.ValidatorUpdate, len(currValidators))
	for i, v := range currValidators {
		p, _ := cryptoenc.PubKeyToProto(ed25519.PubKey(v.Pubkey[:]))
		valSet[i] = abcitypes.ValidatorUpdate{
			PubKey: p,
			Power:  v.VotingPower,
		}
		app.logger.Debug(fmt.Sprintf("Active genesis validator: address(%s), pubkey(%s), votingPower(%d)\n",
			gethcmn.Address(v.Address).String(), p.String(), v.VotingPower))
	}

	params := &abcitypes.ConsensusParams{
		Block: &abcitypes.BlockParams{
			MaxBytes: param.BlockMaxBytes,
			MaxGas:   param.BlockMaxGas,
		},
	}
	return abcitypes.ResponseInitChain{
		ConsensusParams: params,
		Validators:      valSet,
	}
}

func (app *App) createGenesisAccounts(alloc gethcore.GenesisAlloc) {
	if len(alloc) == 0 {
		return
	}

	rbt := rabbit.NewRabbitStore(app.trunk)

	app.logger.Info("air drop", "accounts", len(alloc))
	for addr, acc := range alloc {
		amt, _ := uint256.FromBig(acc.Balance)
		k := types.GetAccountKey(addr)
		v := types.ZeroAccountInfo()
		v.UpdateBalance(amt)
		rbt.Set(k, v.Bytes())
		//app.logger.Info("Air drop " + amt.String() + " to " + addr.Hex())
	}

	rbt.Close()
	rbt.WriteBack()
}

func (app *App) MainTime() int64 {
	h := app.CCRPC.MainRPC().GetMainnetHeight()
	fcc := app.CCRPC.MainRPC().FetchCrosschain(h, h, 0)
	if fcc != nil {
		return fcc.EpochEndBlockTime
	} else {
		return app.block.Timestamp
	}
}

func (app *App) mainnetNetworkOK() bool {
	if app.CCRPC.MainRPC().GetMainnetActivePeersCount() < int64(4) {
		return false
	}
	return true
}

func (app *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	var stat unix.Statfs_t
	wd, _ := os.Getwd()
	unix.Statfs(wd, &stat)
	available_space_in_bytes := stat.Bavail * uint64(stat.Bsize)
	if available_space_in_bytes < 1_000_000 {
		panic("disk space below 1MB: panic!")
	}

	h := req.Header.Height
	nBehind := app.config.AppConfig.BlocksBehind
	if nBehind > 0 {
		smartRpcClient := ccrpc.NewRpcClient(app.config.AppConfig.ZeniqsmartRPCUrl, "", "", "application/json", app.logger)
		for nBehind+h > smartRpcClient.NetworkSmartHeight() {
			time.Sleep(time.Duration(3 * time.Second))
		}
	}

	var maxLagMain int64 = 3600
	if h > 24000000 {
		maxLagMain = 24 * 3600
	}
	for {
		require_mainnet_time := app.block.Timestamp - maxLagMain
		mainnet_time := app.MainTime()
		if require_mainnet_time <= mainnet_time {
			break
		}
		if h > 24000000 && require_mainnet_time > mainnet_time && app.mainnetNetworkOK() {
			break
		}
		app.logger.Info(fmt.Sprintf("waiting till mainnet reaches %v (currently %v)", require_mainnet_time, mainnet_time))
		time.Sleep(time.Duration(30 * time.Second))
	}

	app.block = &types.Block{
		Number:    req.Header.Height,
		Timestamp: req.Header.Time.Unix(),
		Size:      int64(req.Size()),
	}
	copy(app.block.Miner[:], req.Header.ProposerAddress)
	app.logger.Debug(fmt.Sprintf("current proposer %s, last proposer: %s",
		gethcmn.Address(app.block.Miner).String(), gethcmn.Address(app.lastProposer).String()))
	for _, v := range req.LastCommitInfo.GetVotes() {
		if v.SignedLastBlock {
			app.lastVoters = append(app.lastVoters, v.Validator.Address) //this is validator consensus address
		}
	}
	copy(app.block.ParentHash[:], req.Header.LastBlockId.Hash)
	copy(app.block.TransactionsRoot[:], req.Header.DataHash)
	app.reorderSeed = 0
	if len(req.Header.DataHash) >= 8 {
		app.reorderSeed = int64(binary.LittleEndian.Uint64(req.Header.DataHash[0:8]))
	}
	copy(app.block.Hash[:], req.Hash) // Just use tendermint's block hash
	copy(app.block.StateRoot[:], req.Header.AppHash)
	app.currHeight = req.Header.Height
	// collect slash info, currently only double signing is slashed
	var addr [20]byte
	for _, val := range req.ByzantineValidators {
		//we always slash, without checking the time of bad behavior
		if val.Type == abcitypes.EvidenceType_DUPLICATE_VOTE {
			copy(addr[:], val.Validator.Address)
			app.slashValidators = append(app.slashValidators, addr)
		}
	}
	return abcitypes.ResponseBeginBlock{}
}

func (app *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	app.block.Size += int64(req.Size())
	tx, err := ethutils.DecodeTx(req.Tx)
	if err == nil {
		app.txEngine.CollectTx(tx)
		app.txid2sigMap[tx.Hash()] = ethutils.EncodeVRS(tx)
	}
	return abcitypes.ResponseDeliverTx{Code: abcitypes.CodeTypeOK}
}

func (app *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	valSet := make([]abcitypes.ValidatorUpdate, len(app.validatorUpdate))
	for i, v := range app.validatorUpdate {
		p, _ := cryptoenc.PubKeyToProto(ed25519.PubKey(v.Pubkey[:]))
		valSet[i] = abcitypes.ValidatorUpdate{
			PubKey: p,
			Power:  v.VotingPower,
		}
		app.logger.Debug(fmt.Sprintf("Validator updated in EndBlock: pubkey(%s) votingPower(%d)",
			hex.EncodeToString(v.Pubkey[:]), v.VotingPower))
	}
	txs := app.txEngine.CollectedTx()
	if len(txs) > 0 {
		app.txsFeed.Send(gethcore.NewTxsEvent{Txs: txs})
	}

	return abcitypes.ResponseEndBlock{
		ValidatorUpdates: valSet,
	}
}

func (app *App) GetCCRPCForkBlock() int64 {
	return int64(app.config.AppConfig.CCRPCForkBlock)
}

func (app *App) CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo {
	return app.CCRPC.CrosschainInfo(start, end)
}

func (app *App) Commit() abcitypes.ResponseCommit {
	app.mtx.Lock()
	ctx := app.GetRunTxContext()
	CCRPCForkBlock := app.GetCCRPCForkBlock()
	if app.currHeight >= CCRPCForkBlock {
		app.CCRPC.ProcessCCRPC(ctx, app.currHeight, app.block.Timestamp)
	}
	app.updateValidatorsAndStakingInfo(ctx)
	ctx.Close(true) // context must be written back such that txEngine can read it in 'Prepare'

	app.frontier = app.txEngine.Prepare(app.reorderSeed, 0, param.MaxTxGasLimit)
	app.refresh()
	publishAfterCommit := app.currHeight >= HEIGHT_PUBLISH_AFTER_BLOCK
	go app.postCommit(app.syncBlockInfo(), publishAfterCommit)
	return app.buildCommitResponse()
}

func (app *App) getBlockRewardAndUpdateSysAcc(ctx *types.Context) *uint256.Int {
	if !app.lastGasRefund.IsZero() {
		err := ebp.SubSystemAccBalance(ctx, &app.lastGasRefund)
		if err != nil {
			panic(err)
		}
	}
	//invariant check for fund safe
	sysB := ebp.GetSystemBalance(ctx)
	if sysB.Cmp(&app.lastGasFee) < 0 {
		panic("system balance not enough!")
	}
	//if there has tx in standbyQ, it means there has some gasFee prepay to system account in app.prepare should not distribute in this block.
	//if standbyQ is empty, we distribute all system account balance as reward in that block,
	//which have all coin sent to system account through external transfer in blocks standbyQ not empty, this is not fair but simple, and this is rarely the case now.
	var blockReward = app.lastGasFee //distribute previous block gas fee
	if app.txEngine.StandbyQLen() == 0 {
		// distribute extra balance to validators
		blockReward = *sysB
	}
	if !blockReward.IsZero() {
		//block reward is subtracted from systemAcc here, and will be added to stakingAcc in SlashAndReward.
		err := ebp.SubSystemAccBalance(ctx, &blockReward)
		if err != nil {
			panic(err)
		}
	}
	return &blockReward
}

func (app *App) buildCommitResponse() abcitypes.ResponseCommit {
	res := abcitypes.ResponseCommit{
		Data: app.appHash,
	}
	// prune tendermint history block and state every ChangeRetainEveryN blocks
	if app.config.AppConfig.RetainBlocks > 0 && app.currHeight >= app.config.AppConfig.RetainBlocks && (app.currHeight%app.config.AppConfig.ChangeRetainEveryN == 0) {
		res.RetainHeight = app.currHeight - app.config.AppConfig.RetainBlocks + 1
	}
	return res
}

func (app *App) updateValidatorsAndStakingInfo(ctx *types.Context) {
	currValidators, newValidators, currEpochNum := staking.SlashAndReward(ctx, app.slashValidators, app.block.Miner,
		app.lastProposer, app.lastVoters, app.getBlockRewardAndUpdateSysAcc(ctx))
	app.slashValidators = app.slashValidators[:0]

	//epoch switch delay time should bigger than 10 mainnet block interval as of block finalization need
	epochSwitchDelay := param.StakingEpochSwitchDelay
	// this 20 is hardcode to fix the 20220520 node not upgrade error. don't modify it ever.
	if currEpochNum == 20 {
		epochSwitchDelay = param.StakingEpochSwitchDelay * 10
	}

	if app.GetCCRPCForkBlock() > 0 { // 0 for test net starting with ccrpc from 0
		cen := currEpochNum
		lentimes := int64(len(mainEndTimes)) // no staking epochs afterwards
		if cen < lentimes && app.block.Timestamp > mainEndTimes[cen]+epochSwitchDelay {
			app.logger.Debug(fmt.Sprintf("Switch epoch at block(%d), eppchNum(%d)",
				app.block.Number, mainStartHeights[cen]))
			var e *stake.Epoch
			e = &stake.Epoch{
				Number:      cen + 1,
				StartHeight: mainStartHeights[cen],
				EndTime:     mainEndTimes[cen],
			}
			var xHedgeSequence = param.XHedgeContractSequence
			var posVotes map[[32]byte]int64
			if ctx.IsXHedgeFork() {
				//deploy xHedge contract before fork
				posVotes = staking.GetAndClearPosVotes(ctx, xHedgeSequence)
			}
			newValidators = staking.SwitchEpoch(ctx, e, posVotes, app.logger)
			if ctx.IsXHedgeFork() {
				staking.CreateInitVotes(ctx, xHedgeSequence, newValidators)
			}
		}
	}

	app.validatorUpdate = stake.GetUpdateValidatorSet(currValidators, newValidators)
	for _, v := range app.validatorUpdate {
		app.logger.Debug(fmt.Sprintf("Updated validator in commit: address(%s), pubkey(%s), voting power: %d",
			gethcmn.Address(v.Address).String(), ed25519.PubKey(v.Pubkey[:]), v.VotingPower))
	}
	newInfo := staking.LoadStakingInfo(ctx)
	newInfo.ValidatorsUpdate = app.validatorUpdate
	staking.SaveStakingInfo(ctx, newInfo)
	//log all validators info when validator set update
	if len(app.validatorUpdate) != 0 {
		validatorsInfo := app.getValidatorsInfoFromCtx(ctx)
		bz, _ := json.Marshal(validatorsInfo)
		app.logger.Debug(fmt.Sprintf("ValidatorsInfo:%s", string(bz)))
	}
}

func (app *App) syncBlockInfo() *types.BlockInfo {
	blockInfo := &types.BlockInfo{
		Coinbase:  app.block.Miner,
		Number:    app.block.Number,
		Timestamp: app.block.Timestamp,
		ChainId:   app.chainId.Bytes32(),
		Hash:      app.block.Hash,
		/* EIP1559?
		BaseFeePerGas: defaultBaseFeePerGas,
		BaseFeeBlob:   defaultBaseFeeBlob,
		*/
		Revision: app.fromHeightRevision(app.block.Number),
	}
	app.blockInfo.Store(blockInfo)
	app.logger.Debug(fmt.Sprintf("blockInfo: [height:%d, hash:%s]", blockInfo.Number, gethcmn.Hash(blockInfo.Hash).Hex()))
	return blockInfo
}

func (app *App) LoadBlockInfo() *types.BlockInfo {
	return app.blockInfo.Load().(*types.BlockInfo)
}

func (app *App) postCommit(bi *types.BlockInfo, publishAfterCommit bool) {
	defer app.mtx.Unlock()
	if bi != nil {
		if bi.Number > 1 {
			hash := app.historyStore.GetBlockHashByHeight(bi.Number - 1)
			if hash == [32]byte{} {
				app.logger.Debug(fmt.Sprintf("postcommit blockInfo: height:%d, blockHash:%s", bi.Number-1, gethcmn.Hash(hash).Hex()))
			}
		}
	}
	app.txEngine.Execute(bi)
	app.lastGasUsed, app.lastGasRefund, app.lastGasFee = app.txEngine.GasUsedInfo()
	if publishAfterCommit {
		app.publish(app.block)
	}
}

func (app *App) publish(block *types.Block) {
	copy(block.StateRoot[:], app.appHash)
	block.GasUsed = app.lastGasUsed
	blk4DB := dbtypes.Block{
		Height: block.Number,
	}
	block.Transactions = app.txEngine.CommittedTxIds()
	blkInfo, err := block.MarshalMsg(nil)
	if err != nil {
		panic(err)
	}
	copy(blk4DB.BlockHash[:], block.Hash[:])
	blk4DB.BlockInfo = blkInfo
	blk4DB.TxList = app.txEngine.CommittedTxsForDB()
	if app.config.AppConfig.NumKeptBlocksInDB > 0 && app.currHeight > app.config.AppConfig.NumKeptBlocksInDB {
		app.historyStore.AddBlock(&blk4DB, app.currHeight-app.config.AppConfig.NumKeptBlocksInDB, app.txid2sigMap)
	} else {
		app.historyStore.AddBlock(&blk4DB, -1, app.txid2sigMap) // do not prune db
	}
	app.txid2sigMap = make(map[[32]byte][65]byte) // clear its content after flushing into historyStore
	app.publishNewBlock(&blk4DB)
}

func (app *App) refresh() {
	//close old
	app.checkTrunk.Close(false)

	ctx := app.GetRunTxContext()

	prevBlock := ctx.GetCurrBlockBasicInfo()
	ctx.SetCurrBlockBasicInfo(app.block)

	//refresh lastMinGasPrice
	mGP := staking.LoadMinGasPrice(ctx, false) // load current block's gas price
	staking.SaveMinGasPrice(ctx, mGP, true)    // save it as last block's gas price
	app.lastMinGasPrice = mGP
	ctx.Close(true)

	lastCacheSize := app.trunk.CacheSize() // predict the next truck's cache size with the last one
	app.trunk.Close(true)                  //write cached KVs back to app.root

	if !app.config.AppConfig.ArchiveMode && prevBlock != nil &&
		prevBlock.Number%app.config.AppConfig.PruneEveryN == 0 &&
		prevBlock.Number > app.config.AppConfig.NumKeptBlocks {
		app.mads.PruneBeforeHeight(prevBlock.Number - app.config.AppConfig.NumKeptBlocks)
	}

	app.appHash = append([]byte{}, app.root.GetRootHash()...)

	if app.currHeight <= HEIGHT_PUBLISH_AFTER_BLOCK && prevBlock != nil {
		// (<= instead of <) to have 2 publish at this height needed to catch up
		app.publish(prevBlock)
	}
	//make new
	app.recheckCounter = 0 // reset counter before counting the remained TXs which need rechecking
	app.lastProposer = app.block.Miner
	app.lastVoters = app.lastVoters[:0]
	app.root.SetHeight(app.currHeight)
	app.trunk = app.root.GetTrunkStore(lastCacheSize).(*store.TrunkStore)
	app.checkTrunk = app.root.GetReadOnlyTrunkStore(app.config.AppConfig.TrunkCacheSize).(*store.TrunkStore)
	app.txEngine.SetContext(app.GetRunTxContext())
	return
}

func (app *App) publishNewBlock(mdbBlock *dbtypes.Block) {
	if mdbBlock == nil {
		return
	}
	chainEvent := types.BlockToChainEvent(mdbBlock)
	app.chainFeed.Send(chainEvent)
	if len(chainEvent.Logs) > 0 {
		app.logsFeed.Send(chainEvent.Logs)
	}
}

func (app *App) ListSnapshots(snapshots abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	return abcitypes.ResponseListSnapshots{}
}

func (app *App) OfferSnapshot(snapshot abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	return abcitypes.ResponseOfferSnapshot{}
}

func (app *App) LoadSnapshotChunk(chunk abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	return abcitypes.ResponseLoadSnapshotChunk{}
}

func (app *App) ApplySnapshotChunk(chunk abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	return abcitypes.ResponseApplySnapshotChunk{}
}

func (app *App) Stop() {
	app.historyStore.Close()
	app.root.Close()
	app.scope.Close()
	app.CCRPC.Close()
}

func (app *App) GetRpcContext() *types.Context {
	c := types.NewContext(nil, nil, app.config.AppConfig.CCRPCForkBlock)
	r := rabbit.NewReadOnlyRabbitStore(app.root)
	c = c.WithRbt(&r)
	c = c.WithDb(app.historyStore)
	c.SetShaGateForkBlock(param.ShaGateForkBlock)
	c.SetXHedgeForkBlock(param.XHedgeForkBlock)
	c.SetCCRPCForkBlock(app.config.AppConfig.CCRPCForkBlock)
	c.SetCurrentHeight(app.currHeight)
	return c
}
func (app *App) GetRpcContextAtHeight(height int64) *types.Context {
	if !app.config.AppConfig.ArchiveMode || height < 0 {
		return app.GetRpcContext()
	}
	c := types.NewContext(nil, nil, app.config.AppConfig.CCRPCForkBlock)
	r := rabbit.NewReadOnlyRabbitStoreAtHeight(app.root, uint64(height))
	c = c.WithRbt(&r)
	c = c.WithDb(app.historyStore)
	c.SetShaGateForkBlock(param.ShaGateForkBlock)
	c.SetXHedgeForkBlock(param.XHedgeForkBlock)
	c.SetCCRPCForkBlock(app.config.AppConfig.CCRPCForkBlock)
	c.SetCurrentHeight(height)
	return c
}
func (app *App) GetRunTxContext() *types.Context {
	c := types.NewContext(nil, nil, app.config.AppConfig.CCRPCForkBlock)
	r := rabbit.NewRabbitStore(app.trunk)
	c = c.WithRbt(&r)
	c = c.WithDb(app.historyStore)
	c.SetShaGateForkBlock(param.ShaGateForkBlock)
	c.SetXHedgeForkBlock(param.XHedgeForkBlock)
	c.SetCCRPCForkBlock(app.config.AppConfig.CCRPCForkBlock)
	c.SetCurrentHeight(app.currHeight)
	return c
}
func (app *App) GetHistoryOnlyContext() *types.Context {
	c := types.NewContext(nil, nil, app.config.AppConfig.CCRPCForkBlock)
	c = c.WithDb(app.historyStore)
	c.SetShaGateForkBlock(param.ShaGateForkBlock)
	c.SetXHedgeForkBlock(param.XHedgeForkBlock)
	c.SetCCRPCForkBlock(app.config.AppConfig.CCRPCForkBlock)
	c.SetCurrentHeight(app.currHeight)
	return c
}
func (app *App) GetCheckTxContext() *types.Context {
	c := types.NewContext(nil, nil, app.config.AppConfig.CCRPCForkBlock)
	r := rabbit.NewRabbitStore(app.checkTrunk)
	c = c.WithRbt(&r)
	c.SetShaGateForkBlock(param.ShaGateForkBlock)
	c.SetXHedgeForkBlock(param.XHedgeForkBlock)
	c.SetCCRPCForkBlock(app.config.AppConfig.CCRPCForkBlock)
	c.SetCurrentHeight(app.currHeight)
	return c
}

func (app *App) RunTxForRpc(gethTx *gethtypes.Transaction, sender gethcmn.Address, estimateGas bool, height int64) (*ebp.TxRunner, int64) {
	txToRun := &types.TxToRun{}
	txToRun.FromGethTx(gethTx, sender, uint64(app.currHeight))
	ctx := app.GetRpcContextAtHeight(height)
	defer ctx.Close(false)
	runner := ebp.NewTxRunner(ctx, txToRun)
	bi := app.LoadBlockInfo()
	if height > 0 { // height==0 means latest bi
		blk, err := ctx.GetBlockByHeight(uint64(height))
		if err != nil {
			return nil, 0
		}
		bi = &types.BlockInfo{
			Coinbase:  blk.Miner,
			Number:    blk.Number,
			Timestamp: blk.Timestamp,
			ChainId:   app.chainId.Bytes32(),
			Hash:      blk.Hash,
			/* EIP1559?
			BaseFeePerGas: defaultBaseFeePerGas,
			BaseFeeBlob:   defaultBaseFeeBlob,
			*/
			Revision: app.fromHeightRevision(blk.Number),
		}
	}
	estimateResult := ebp.RunTxForRpc(bi, estimateGas, runner)
	return runner, estimateResult
}

// RunTxForSbchRpc is like RunTxForRpc, with two differences:
// 1. estimateGas is always false
// 2. run under context of block#height-1
func (app *App) RunTxForSbchRpc(gethTx *gethtypes.Transaction, sender gethcmn.Address, height int64) (*ebp.TxRunner, int64) {
	if height < 1 {
		return app.RunTxForRpc(gethTx, sender, false, height)
	}

	txToRun := &types.TxToRun{}
	txToRun.FromGethTx(gethTx, sender, uint64(app.currHeight))
	ctx := app.GetRpcContextAtHeight(height - 1)
	defer ctx.Close(false)
	runner := ebp.NewTxRunner(ctx, txToRun)
	blk, err := ctx.GetBlockByHeight(uint64(height))
	if err != nil {
		return nil, 0
	}
	bi := &types.BlockInfo{
		Coinbase:  blk.Miner,
		Number:    blk.Number,
		Timestamp: blk.Timestamp,
		ChainId:   app.chainId.Bytes32(),
		Hash:      blk.Hash,
		/* EIP1559?
		BaseFeePerGas: defaultBaseFeePerGas,
		BaseFeeBlob:   defaultBaseFeeBlob,
		*/
		Revision: app.fromHeightRevision(blk.Number),
	}
	estimateResult := ebp.RunTxForRpc(bi, false, runner)
	return runner, estimateResult
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (app *App) SubscribeChainEvent(ch chan<- types.ChainEvent) event.Subscription {
	return app.scope.Track(app.chainFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (app *App) SubscribeLogsEvent(ch chan<- []*gethtypes.Log) event.Subscription {
	return app.scope.Track(app.logsFeed.Subscribe(ch))
}

func (app *App) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return app.scope.Track(app.txsFeed.Subscribe(ch))
}

func (app *App) GetLastGasUsed() uint64 {
	return app.lastGasUsed
}

func (app *App) GetLatestBlockNum() int64 {
	return app.currHeight
}

func (app *App) ChainID() *uint256.Int {
	return app.chainId
}

func (app *App) GetValidatorsInfo() ValidatorsInfo {
	ctx := app.GetRpcContext()
	defer ctx.Close(false)
	return app.getValidatorsInfoFromCtx(ctx)
}

func (app *App) getValidatorsInfoFromCtx(ctx *types.Context) ValidatorsInfo {
	stakingInfo := staking.LoadStakingInfo(ctx)
	currValidators := stake.GetActiveValidators(stakingInfo.Validators, staking.MinimumStakingAmount)
	minGasPrice := staking.LoadMinGasPrice(ctx, false)
	lastMinGasPrice := staking.LoadMinGasPrice(ctx, true)
	return NewValidatorsInfo(currValidators, stakingInfo, minGasPrice, lastMinGasPrice)
}

func (app *App) IsArchiveMode() bool {
	return app.config.AppConfig.ArchiveMode
}

func (app *App) GetCurrEpoch() *stake.Epoch {
	ctx := app.GetRpcContext()
	stakingInfo := staking.LoadStakingInfo(ctx)
	lastEpochEndHeight := stakingInfo.GenesisMainnetBlockHeight + param.StakingNumBlocksInEpoch*stakingInfo.CurrEpochNum
	epoch := &stake.Epoch{
		StartHeight: lastEpochEndHeight + 1,
		Nominations: make([]*stake.Nomination, 0, 10),
	}
	return epoch
}

func (app *App) GetBlockForSync(height int64) (syncBytes []byte, err error) {
	return nil, errors.New("syncdb is deprecated")
}

// nolint
// for ((i=10; i<80000; i+=50)); do RANDPANICHEIGHT=$i ./zeniqsmartd start; done | tee a.log
func (app *App) randomPanic(baseNumber, primeNumber int64) { // breaks normal function, only used in test
	heightStr := os.Getenv("RANDPANICHEIGHT")
	if len(heightStr) == 0 {
		return
	}
	h, err := strconv.Atoi(heightStr)
	if err != nil {
		panic(err)
	}
	if app.currHeight < int64(h) {
		return
	}
	go func(sleepMilliseconds int64) {
		time.Sleep(time.Duration(sleepMilliseconds * int64(time.Millisecond)))
		s := fmt.Sprintf("random panic after %d millisecond", sleepMilliseconds)
		panic(s)
	}(baseNumber + time.Now().UnixNano()%primeNumber)
}
