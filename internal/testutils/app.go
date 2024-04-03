package testutils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	gethcmn "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	dbtypes "github.com/zeniqsmart/db-zeniq-smart-chain/types"
	"github.com/zeniqsmart/evm-zeniq-smart-chain/ebp"
	evmtc "github.com/zeniqsmart/evm-zeniq-smart-chain/evmwrap/testcase"
	evmtypes "github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/app"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/bigutils"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/ethutils"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	"github.com/zeniqsmart/zeniq-smart-chain/staking"

	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	cctypes "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
	wtypes "github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
)

const (
	testAdsDir  = "./testdbdata"
	testDbDir   = "./dbdata"
	testSyncDir = "./syscdb"
)

const (
	DefaultGasLimit    = 1000000
	DefaultGasPrice    = 0
	DefaultInitBalance = uint64(10000000)
	BlockInterval      = 5 * time.Second
	debug              = false
)

var (
	checkAllBalance = GetIntEvn("UT_CHECK_ALL_BALANCE", 0) != 0
	checkAppState   = GetIntEvn("UT_CHECK_APP_STATE", 1) != 0
)

type MockRpcClient struct{}

func (m MockRpcClient) start()                                                     { go func() {}() }
func (m MockRpcClient) Dial()                                                      {}
func (m MockRpcClient) Close()                                                     {}
func (m MockRpcClient) NetworkSmartHeight() int64                                  { return 1 }
func (m MockRpcClient) GetMainnetHeight() (height int64)                           { return 1 }
func (m MockRpcClient) GetBlockByHeight(height int64, retry bool) *wtypes.BCHBlock { return nil }
func (m MockRpcClient) GetBlockByHash(hash [32]byte) *wtypes.BCHBlock              { return nil }
func (m MockRpcClient) GetEpochs(start, end uint64) []*stake.Epoch                 { return nil }
func (m MockRpcClient) GetCCEpochs(start, end uint64) []*cctypes.CCEpoch           { return nil }
func (m MockRpcClient) FetchCC(first, last int64) (cc *ccrpctypes.CCrpcEpoch)      { return nil }
func (m MockRpcClient) IsConnected() bool                                          { return true }

var _ wtypes.RpcClient = MockRpcClient{}

// var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
var nopLogger = log.NewNopLogger()

type TestApp struct {
	*app.App
	TestPubkey     crypto.PubKey
	StateRoot      []byte
	StartTime      time.Time
	initAllBalance *uint256.Int
}

type TestAppInitArgs struct {
	StartHeight *int64
	StartTime   *time.Time
	ValPubKey   *crypto.PubKey
	InitAmt     *uint256.Int
	PrivKeys    []string
	ArchiveMode bool
	WithSyncDB  bool
}

func CreateTestApp(keys ...string) *TestApp {
	return createTestApp0(0, time.Now(), ed25519.GenPrivKey().PubKey(), bigutils.NewU256(DefaultInitBalance),
		keys, false, false, MockRpcClient{})
}
func CreateTestAppInArchiveMode(keys ...string) *TestApp {
	return createTestApp0(0, time.Now(), ed25519.GenPrivKey().PubKey(), bigutils.NewU256(DefaultInitBalance),
		keys, true, false, MockRpcClient{})
}
func CreateTestAppWithSyncDB(keys ...string) *TestApp {
	return createTestApp0(0, time.Now(), ed25519.GenPrivKey().PubKey(), bigutils.NewU256(DefaultInitBalance),
		keys, true, true, MockRpcClient{})
}
func CreateTestAppWithCC(rpccl wtypes.RpcClient, keys ...string) *TestApp {
	return createTestApp0(0, time.Now(), ed25519.GenPrivKey().PubKey(), bigutils.NewU256(DefaultInitBalance),
		keys, false, false, rpccl)
}

func CreateTestAppWithArgs(args TestAppInitArgs, rpccl wtypes.RpcClient) *TestApp {
	startHeight := int64(0)
	if args.StartHeight != nil {
		startHeight = *args.StartHeight
	}

	startTime := time.Now()
	if args.StartTime != nil {
		startTime = *args.StartTime
	}

	pubKey := ed25519.GenPrivKey().PubKey()
	if args.ValPubKey != nil {
		pubKey = *args.ValPubKey
	}

	initAmt := bigutils.NewU256(DefaultInitBalance)
	if args.InitAmt != nil {
		initAmt = args.InitAmt
	}

	if rpccl == nil {
		return createTestApp0(startHeight, startTime, pubKey, initAmt, args.PrivKeys,
			args.ArchiveMode, args.WithSyncDB, MockRpcClient{})
	} else {
		return createTestApp0(startHeight, startTime, pubKey, initAmt, args.PrivKeys,
			args.ArchiveMode, args.WithSyncDB, rpccl)
	}

}

func createTestApp0(startHeight int64, startTime time.Time, valPubKey crypto.PubKey, initAmt *uint256.Int, keys []string,
	archiveMode bool, withSyncDB bool, rpccl wtypes.RpcClient) *TestApp {

	err := os.RemoveAll(testAdsDir)
	if err != nil {
		panic("remove test ads failed " + err.Error())
	}
	err = os.RemoveAll(testDbDir)
	if err != nil {
		panic("remove test db failed " + err.Error())
	}
	p := param.DefaultConfig()

	p.AppConfig.AppDataPath = testAdsDir
	p.AppConfig.DbDataPath = testDbDir
	p.AppConfig.SyncdbDataPath = testSyncDir
	p.AppConfig.ArchiveMode = archiveMode
	p.AppConfig.WithSyncDB = withSyncDB

	_app := app.NewApp(p, bigutils.NewU256(1), 0, nopLogger, rpccl)
	genesisData := app.GenesisData{
		Alloc: KeysToGenesisAlloc(initAmt, keys),
	}

	testValidator := &app.Validator{}
	copy(testValidator.Address[:], valPubKey.Address().Bytes())
	copy(testValidator.Pubkey[:], valPubKey.Bytes())
	copy(testValidator.StakedCoins[:], staking.MinimumStakingAmount.Bytes())
	testValidator.Introduction = "val0"
	testValidator.VotingPower = 1
	genesisData.Validators = append(genesisData.Validators, testValidator)

	appStateBytes, _ := json.Marshal(genesisData)

	//reset config for test
	staking.DefaultMinGasPrice = 0
	staking.MinGasPriceLowerBound = 0
	//setMinGasPrice(_app, 0)

	_app.InitChain(abci.RequestInitChain{AppStateBytes: appStateBytes})
	_app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{
		Height:          startHeight,
		Time:            startTime,
		ProposerAddress: valPubKey.Address(),
	}})
	_app.EndBlock(abci.RequestEndBlock{
		Height: startHeight,
	})
	if startHeight > 1 {
		_app.AddBlockFotTest(&dbtypes.Block{Height: startHeight - 1})
	}
	stateRoot := _app.Commit().Data
	if debug {
		fmt.Println("h: 0 StateRoot:", hex.EncodeToString(stateRoot))
	}

	allBalance := uint256.NewInt(0)
	if checkAllBalance {
		allBalance = _app.SumAllBalance()
	}
	return &TestApp{
		App:            _app,
		TestPubkey:     valPubKey,
		StateRoot:      stateRoot,
		StartTime:      startTime,
		initAllBalance: allBalance,
	}
}

func (_app *TestApp) ReloadApp() *TestApp {
	//_app.Stop()
	p := param.DefaultConfig()
	p.AppConfig.AppDataPath = testAdsDir
	p.AppConfig.DbDataPath = testDbDir
	newApp := app.NewApp(p, bigutils.NewU256(1), 0, nopLogger,
		MockRpcClient{})
	allBalance := uint256.NewInt(0)
	if checkAllBalance {
		allBalance = _app.SumAllBalance()
	}
	return &TestApp{
		App:            newApp,
		initAllBalance: allBalance,
	}
}

func (_app *TestApp) DestroyWithoutCheck() {
	_app.Stop()
	cleanUpTestData()
}

func (_app *TestApp) Destroy() {
	allBalance := uint256.NewInt(0)
	if checkAllBalance {
		allBalance = _app.App.SumAllBalance()
	}
	if checkAllBalance && !allBalance.Eq(_app.initAllBalance) {
		panic(fmt.Sprintf("balance check failed! init balance: %s, final balance: %s",
			_app.initAllBalance.Hex(), allBalance.Hex()))
	}

	preState := _app.GetWordState()
	_app.Stop()
	if checkAppState {
		newApp := _app.ReloadApp()
		postState := newApp.GetWordState()
		isSame, err := evmtc.CompareWorldState(preState, postState)
		if !isSame {
			panic(fmt.Sprintf("world state not same after app reload: %s", err))
		}
		newApp.Stop()
	}
	cleanUpTestData()
}

func cleanUpTestData() {
	_ = os.RemoveAll(testAdsDir)
	_ = os.RemoveAll(testDbDir)
	_ = os.RemoveAll(testSyncDir)
}

func (_app *TestApp) WaitMS(n int64) {
	time.Sleep(time.Duration(n) * time.Millisecond)
}

func (_app *TestApp) SetMinGasPrice(gp uint64) {
	setMinGasPrice(_app.App, gp)
	_app.ExecTxsInBlock()
}
func setMinGasPrice(_app *app.App, gp uint64) {
	ctx := _app.GetRunTxContext()
	staking.SaveMinGasPrice(ctx, gp, true)
	staking.SaveMinGasPrice(ctx, gp, false)
	ctx.Close(true)
}

func (_app *TestApp) GetMinGasPrice(isLast bool) uint64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)

	return staking.LoadMinGasPrice(ctx, isLast)
}

func (_app *TestApp) GetSeq(addr gethcmn.Address) uint64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if acc := ctx.GetAccount(addr); acc != nil {
		return acc.Sequence()
	}
	return 0
}

func (_app *TestApp) GetNonce(addr gethcmn.Address) uint64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if acc := ctx.GetAccount(addr); acc != nil {
		return acc.Nonce()
	}
	return 0
}

func (_app *TestApp) GetBalance(addr gethcmn.Address) *big.Int {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if acc := ctx.GetAccount(addr); acc != nil {
		return acc.Balance().ToBig()
	}
	return nil
}

func (_app *TestApp) GetStorageAt(addr gethcmn.Address, key []byte) []byte {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if acc := ctx.GetAccount(addr); acc != nil {
		return ctx.GetStorageAt(acc.Sequence(), string(key))
	}
	return nil
}

func (_app *TestApp) GetDynamicArray(addr gethcmn.Address, arrSlot string) [][]byte {
	seq := _app.GetSeq(addr)

	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	return ctx.GetDynamicArray(seq, arrSlot)
}

func (_app *TestApp) GetCode(addr gethcmn.Address) []byte {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if codeInfo := ctx.GetCode(addr); codeInfo != nil {
		return codeInfo.BytecodeSlice()
	}
	return nil
}

func (_app *TestApp) GetBlock(h int64) *evmtypes.Block {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	if ctx.GetLatestHeight() != h {
		_app.WaitMS(500)
	}
	b, err := ctx.GetBlockByHeight(uint64(h))
	if err != nil {
		panic(err)
	}
	return b
}

func (_app *TestApp) GetTx(h gethcmn.Hash) (tx *evmtypes.Transaction) {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)

	var err error
	for i := 0; i < 10; i++ { // retry ten times
		tx, _, err = ctx.GetTxByHash(h)
		if err == nil {
			return
		}
		_app.WaitMS(300)
	}
	if err != nil {
		panic(err)
	}
	return nil
}

func (_app *TestApp) GetTxsByAddr(addr gethcmn.Address) []*evmtypes.Transaction {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	txs, _, err := ctx.QueryTxByAddr(addr, 1, uint32(_app.BlockNum())+1, 0)
	if err != nil {
		panic(err)
	}
	return txs
}

func (_app *TestApp) GetToAddressCount(addr gethcmn.Address) int64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	return ctx.GetToAddressCount(addr)
}
func (_app *TestApp) GetSep20FromAddressCount(contract, addr gethcmn.Address) int64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	return ctx.GetSep20FromAddressCount(contract, addr)
}
func (_app *TestApp) GetSep20ToAddressCount(contract, addr gethcmn.Address) int64 {
	ctx := _app.GetRpcContext()
	defer ctx.Close(false)
	return ctx.GetSep20ToAddressCount(contract, addr)
}

func (_app *TestApp) GetTestPubkey() crypto.PubKey {
	return _app.TestPubkey
}

func (_app *TestApp) StoreBlocks(blocks ...*dbtypes.Block) {
	ctx := _app.GetRunTxContext()
	for _, block := range blocks {
		ctx.StoreBlock(block, nil)
	}
	ctx.StoreBlock(nil, nil) // flush previous block
	ctx.Close(true)
}
func (_app *TestApp) AddBlocksToHistory(blocks ...*dbtypes.Block) {
	for _, blk := range blocks {
		_app.HistoryStore().AddBlock(blk, -1, nil)
	}
	_app.HistoryStore().AddBlock(nil, -1, nil)
	_app.WaitMS(10)
}

func (_app *TestApp) MakeAndSignTx(hexPrivKey string,
	toAddr *gethcmn.Address, val int64, data []byte) (*gethtypes.Transaction, gethcmn.Address) {

	return _app.MakeAndSignTxWithGas(hexPrivKey, toAddr, val, data, DefaultGasLimit, DefaultGasPrice)
}

func (_app *TestApp) MakeAndSignTxWithNonce(hexPrivKey string,
	toAddr *gethcmn.Address, val int64, data []byte, nonce int64) (*gethtypes.Transaction, gethcmn.Address) {

	return _app.MakeAndSignTxWithAllArgs(hexPrivKey, toAddr, val, data, DefaultGasLimit, DefaultGasPrice, nonce)
}

func (_app *TestApp) MakeAndSignTxWithGas(hexPrivKey string,
	toAddr *gethcmn.Address, val int64, data []byte, gasLimit uint64, gasPrice int64) (*gethtypes.Transaction, gethcmn.Address) {

	return _app.MakeAndSignTxWithAllArgs(hexPrivKey, toAddr, val, data, gasLimit, gasPrice, -1)
}

func (_app *TestApp) MakeAndSignTxWithAllArgs(hexPrivKey string,
	toAddr *gethcmn.Address, val int64, data []byte, gasLimit uint64, gasPrice int64,
	nonce int64) (*gethtypes.Transaction, gethcmn.Address) {

	privKey, _, err := ethutils.HexToPrivKey(hexPrivKey)
	if err != nil {
		panic(err)
	}

	addr := ethutils.PrivKeyToAddr(privKey)
	nonceU64 := uint64(nonce)
	if nonce < 0 {
		nonceU64 = _app.GetNonce(addr)
	}

	chainID := _app.ChainID().ToBig()
	txData := &gethtypes.LegacyTx{
		Nonce:    nonceU64,
		GasPrice: big.NewInt(gasPrice),
		Gas:      gasLimit,
		To:       toAddr,
		Value:    big.NewInt(val),
		Data:     data,
	}
	tx := gethtypes.NewTx(txData)
	tx, err = ethutils.SignTx(tx, chainID, privKey)
	if err != nil {
		panic(err)
	}

	return tx, addr
}

func (_app *TestApp) CallWithABI(sender, contractAddr gethcmn.Address,
	abi ethutils.ABIWrapper, methodName string, args ...interface{}) []interface{} {

	data := abi.MustPack(methodName, args...)
	statusCode, statusStr, output := _app.Call(sender, contractAddr, data)
	if statusCode != 0 {
		panic(statusStr)
	}

	return abi.MustUnpack(methodName, output)
}

func (_app *TestApp) Call(sender, contractAddr gethcmn.Address, data []byte) (int, string, []byte) {
	return _app.CallAtHeight(sender, contractAddr, data, -1)
}
func (_app *TestApp) CallAtHeight(sender, contractAddr gethcmn.Address, data []byte, height int64) (int, string, []byte) {
	tx := ethutils.NewTx(0, &contractAddr, big.NewInt(0), DefaultGasLimit, big.NewInt(0), data)
	runner, _ := _app.RunTxForRpc(tx, sender, false, height)
	return runner.Status, ebp.StatusToStr(runner.Status), runner.OutData
}
func (_app *TestApp) EstimateGas(sender gethcmn.Address, tx *gethtypes.Transaction) (int, string, int64) {
	runner, estimatedGas := _app.RunTxForRpc(tx, sender, true, -1)
	return runner.Status, ebp.StatusToStr(runner.Status), estimatedGas
}

func (_app *TestApp) DeployContractInBlock(privKey string, data []byte) (*gethtypes.Transaction, int64, gethcmn.Address) {
	tx, addr := _app.MakeAndSignTx(privKey, nil, 0, data)
	h := _app.ExecTxInBlock(tx)
	contractAddr := gethcrypto.CreateAddress(addr, tx.Nonce())
	return tx, h, contractAddr
}

func (_app *TestApp) MakeAndExecTxInBlock(privKey string,
	toAddr gethcmn.Address, val int64, data []byte) (*gethtypes.Transaction, int64) {

	return _app.MakeAndExecTxInBlockWithGas(privKey, toAddr, val, data, DefaultGasLimit, DefaultGasPrice)
}
func (_app *TestApp) MakeAndExecTxInBlockWithGas(privKey string,
	toAddr gethcmn.Address, val int64, data []byte, gasLimit uint64, gasPrice int64) (*gethtypes.Transaction, int64) {

	tx, _ := _app.MakeAndSignTxWithGas(privKey, &toAddr, val, data, gasLimit, gasPrice)
	h := _app.ExecTxInBlock(tx)
	return tx, h
}

func (_app *TestApp) ExecTxInBlock(tx *gethtypes.Transaction) int64 {
	if tx == nil {
		return _app.ExecTxsInBlock()
	}
	return _app.ExecTxsInBlock(tx)
}

func (_app *TestApp) ExecTxsInBlock(txs ...*gethtypes.Transaction) int64 {
	height := _app.BlockNum() + 1
	_app.AddTxsInBlock(height, txs...)
	_app.WaitNextBlock(height)
	return height
}

func (_app *TestApp) AddTxsInBlock(height int64, txs ...*gethtypes.Transaction) int64 {
	_app.BeginBlock(abci.RequestBeginBlock{
		Hash: UintToBytes32(uint64(height)),
		Header: tmproto.Header{
			Height:          height,
			Time:            _app.StartTime.Add(BlockInterval * time.Duration(height)),
			ProposerAddress: _app.TestPubkey.Address(),
		},
	})
	for _, tx := range txs {
		_app.DeliverTx(abci.RequestDeliverTx{
			Tx: MustEncodeTx(tx),
		})
	}
	_app.EndBlock(abci.RequestEndBlock{Height: height})
	_app.StateRoot = _app.Commit().Data
	if debug {
		fmt.Println("h:", height, "StateRoot:", hex.EncodeToString(_app.StateRoot))
	}
	_app.WaitLock()
	return height
}
func (_app *TestApp) WaitNextBlock(currHeight int64) {
	_app.BeginBlock(abci.RequestBeginBlock{
		Hash: UintToBytes32(uint64(currHeight + 1)),
		Header: tmproto.Header{
			Height: currHeight + 1,
			Time:   _app.StartTime.Add(BlockInterval * time.Duration(currHeight+1)),
		},
	})
	_app.DeliverTx(abci.RequestDeliverTx{})
	_app.EndBlock(abci.RequestEndBlock{Height: currHeight + 1})
	_app.StateRoot = _app.Commit().Data
	if debug {
		fmt.Println("h:", currHeight+1, "StateRoot:", hex.EncodeToString(_app.StateRoot))
	}
	_app.WaitLock()
}

func (_app *TestApp) EnsureTxSuccess(hash gethcmn.Hash) {
	tx := _app.GetTx(hash)
	if tx.Status != gethtypes.ReceiptStatusSuccessful || tx.StatusStr != "success" {
		panic("tx is failed, statusStr:" + tx.StatusStr + ", outData:" + string(tx.OutData))
	}
}
func (_app *TestApp) EnsureTxFailed(hash gethcmn.Hash, msg string) {
	tx := _app.GetTx(hash)
	if tx.Status != gethtypes.ReceiptStatusFailed {
		panic("tx is success")
	}
	if tx.StatusStr != msg {
		panic("expected " + msg + ", got " + tx.StatusStr)
	}
}
func (_app *TestApp) EnsureTxFailedWithOutData(hash gethcmn.Hash, statusStr, outData string) {
	tx := _app.GetTx(hash)
	if tx.Status != gethtypes.ReceiptStatusFailed {
		panic("tx is success")
	}
	if tx.StatusStr != statusStr {
		panic("expected statusStr: " + statusStr + ", got: " + tx.StatusStr)
	}
	if string(tx.OutData) != outData {
		panic("expected outData: " + outData + ", got: " + string(tx.OutData))
	}
}

func (_app *TestApp) CheckNewTxABCI(tx *gethtypes.Transaction) uint32 {
	code, _ := _app.CheckTxABCI(tx, true)
	return code
}
func (_app *TestApp) RecheckTxABCI(tx *gethtypes.Transaction) uint32 {
	code, _ := _app.CheckTxABCI(tx, false)
	return code
}
func (_app *TestApp) CheckTxABCI(tx *gethtypes.Transaction, newTx bool) (uint32, string) {
	txCheckType := abci.CheckTxType_New
	if !newTx {
		txCheckType = abci.CheckTxType_Recheck
	}
	res := _app.CheckTx(abci.RequestCheckTx{
		Tx:   MustEncodeTx(tx),
		Type: txCheckType,
	})
	return res.Code, res.Info
}
