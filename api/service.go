package api

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/bloombits"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/app"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

type CallDetail struct {
	Status                 int
	GasUsed                uint64
	OutData                []byte
	Logs                   []types.EvmLog
	CreatedContractAddress common.Address
	InternalTxCalls        []types.InternalTxCall
	InternalTxReturns      []types.InternalTxReturn
	RwLists                *types.ReadWriteLists
}

type FilterService interface {
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	HeaderByHash(ctx context.Context, blockHash common.Hash) (*types.Header, error)
	GetReceipts(ctx context.Context, blockNum uint64) (gethtypes.Receipts, error)
	GetLogs(ctx context.Context, blockHash common.Hash) ([][]*gethtypes.Log, error)

	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
	SubscribeChainEvent(ch chan<- types.ChainEvent) event.Subscription
	SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription
	SubscribeLogsEvent(ch chan<- []*gethtypes.Log) event.Subscription
	//SubscribePendingLogsEvent(ch chan<- []*stake.Log) event.Subscription

	BloomStatus() (uint64, uint64)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
}

type BackendService interface {
	FilterService

	// General Ethereum API
	//Downloader() *downloader.Downloader
	ProtocolVersion() int
	//SuggestPrice(ctx context.Context) (*big.Int, error)
	//ChainDb() Database
	//AccountManager() *accounts.Manager
	//ExtRPCEnabled() bool
	//RPCGasCap() uint64    // global gas cap for eth_call over rpc: DoS protection
	//RPCTxFeeCap() float64 // global tx fee cap for all transaction related APIs

	// Blockchain API
	ChainId() *big.Int
	//SetHead(number uint64)
	//HeaderByNumber(ctx context.Context, number int64) (*stake.Header, error)
	//HeaderByHash(ctx context.Context, hash common.Hash) (*stake.Header, error)
	//HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*stake.Header, error)
	//CurrentHeader() *stake.Header
	LatestHeight() int64
	CurrentBlock() (*types.Block, error)
	BlockByNumber(number int64) (*types.Block, error)
	BlockByHash(hash common.Hash) (*types.Block, error)
	//BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*stake.Block, error)
	//StateAndHeaderByNumber(ctx context.Context, number int64) (*state.StateDB, error)
	//StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *stake.Header, error)
	//GetReceipts(ctx context.Context, hash common.Hash) (stake.Receipts, error) /*All receipt fields is in stake.Transaction, use getTransaction() instead*/
	//GetTd(ctx context.Context, hash common.Hash) *big.Int
	//GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *stake.Header) (*vm.EVM, func() error, error)

	// Transaction pool API
	SendRawTx(signedTx []byte) (common.Hash, error)
	GetTransaction(txHash common.Hash) (tx *types.Transaction, sig [65]byte, err error)
	//GetPoolTransactions() (stake.Transactions, error)
	//GetPoolTransaction(txHash common.Hash) *stake.Transaction
	//GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	//Stats() (pending int, queued int)
	//TxPoolContent() (map[common.Address]stake.Transactions, map[common.Address]stake.Transactions)

	// Filter API
	//BloomStatus() (uint64, uint64)
	//GetLogs(blockHash common.Hash) ([][]stake.Log, error)
	//ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
	//SubscribeLogsEvent(ch chan<- []*stake.Log) event.Subscription
	//SubscribePendingLogsEvent(ch chan<- []*stake.Log) event.Subscription
	//SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription
	//SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
	//SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	//SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	//SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

	//Engine() consensus.Engine

	//Below is added in zeniq chain only
	GetNonce(address common.Address, height int64) (uint64, error)
	GetBalance(address common.Address, height int64) (*big.Int, error)
	GetCode(contract common.Address, height int64) (bytecode []byte, codeHash []byte)
	GetStorageAt(address common.Address, key string, height int64) []byte
	Call(tx *gethtypes.Transaction, from common.Address, height int64) (statusCode int, retData []byte)
	CallForSbch(tx *gethtypes.Transaction, sender common.Address, height int64) *CallDetail
	EstimateGas(tx *gethtypes.Transaction, from common.Address, height int64) (statusCode int, retData []byte, gas int64)
	QueryLogs(addresses []common.Address, topics [][]common.Hash, startHeight, endHeight uint32, filter types.FilterFunc) ([]types.Log, error)
	QueryTxBySrc(address common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error)
	QueryTxByDst(address common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error)
	QueryTxByAddr(address common.Address, startHeight, endHeight, limit uint32) (tx []*types.Transaction, sigs [][65]byte, err error)
	SbchQueryLogs(addr common.Address, topics []common.Hash, startHeight, endHeight, limit uint32) ([]types.Log, error)
	GetTxListByHeight(height uint32) (tx []*types.Transaction, sigs [][65]byte, err error)
	GetTxListByHeightWithRange(height uint32, start, end int) (tx []*types.Transaction, sigs [][65]byte, err error)
	GetFromAddressCount(addr common.Address) int64
	GetToAddressCount(addr common.Address) int64
	GetSep20ToAddressCount(contract common.Address, addr common.Address) int64
	GetSep20FromAddressCount(contract common.Address, addr common.Address) int64
	GetSeq(address common.Address) uint64
	GetPosVotes() map[[32]byte]*big.Int
	GetSyncBlock(height int64) (blk []byte, err error)
	CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo

	//tendermint info
	NodeInfo() Info
	ValidatorsInfo() app.ValidatorsInfo

	IsArchiveMode() bool
}
