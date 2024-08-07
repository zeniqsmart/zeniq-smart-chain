package api

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/zeniqsmart/evm-zeniq-smart-chain/ebp"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/api"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

// StakingEpoch

type StakingEpoch struct {
	Number      hexutil.Uint64 `json:"number"`
	StartHeight hexutil.Uint64 `json:"startHeight"`
	EndTime     int64          `json:"endTime"`
	Nominations []*Nomination  `json:"nominations"`
	PosVotes    []*PosVote     `json:"posVotes"`
}
type Nomination struct {
	Pubkey         gethcmn.Hash `json:"pubkey"`
	NominatedCount int64        `json:"nominatedCount"`
}
type PosVote struct {
	Pubkey       gethcmn.Hash `json:"pubkey"`
	CoinDaysSlot *hexutil.Big `json:"coinDaysSlot"`
	CoinDays     float64      `json:"coinDays"`
}

func castStakingEpochs(epochs []*stake.Epoch) []*StakingEpoch {
	rpcEpochs := make([]*StakingEpoch, len(epochs))
	for i, epoch := range epochs {
		rpcEpochs[i] = castStakingEpoch(epoch)
	}
	return rpcEpochs
}
func castStakingEpoch(epoch *stake.Epoch) *StakingEpoch {
	return &StakingEpoch{
		Number:      hexutil.Uint64(epoch.Number),
		StartHeight: hexutil.Uint64(epoch.StartHeight),
		EndTime:     epoch.EndTime,
		Nominations: castNominations(epoch.Nominations),
	}
}
func castNominations(nominations []*stake.Nomination) []*Nomination {
	rpcNominations := make([]*Nomination, len(nominations))
	for i, nomination := range nominations {
		rpcNominations[i] = &Nomination{
			Pubkey:         nomination.Pubkey,
			NominatedCount: nomination.NominatedCount,
		}
	}
	return rpcNominations
}

// CallDetail

type CallDetail struct {
	Status                 int             `json:"status"`
	GasUsed                hexutil.Uint64  `json:"gasUsed"`
	OutData                hexutil.Bytes   `json:"returnData"`
	Logs                   []*CallLog      `json:"logs"`
	CreatedContractAddress gethcmn.Address `json:"contractAddress"`
	InternalTxs            []*InternalTx   `json:"internalTransactions"`
	RwLists                *RWLists        `json:"rwLists"`
}
type CallLog struct {
	Address gethcmn.Address `json:"address"`
	Topics  []gethcmn.Hash  `json:"topics"`
	Data    hexutil.Bytes   `json:"data"`
}
type RWLists struct {
	CreationCounterRList []CreationCounterRWOp `json:"creationCounterRList"`
	CreationCounterWList []CreationCounterRWOp `json:"creationCounterWList"`
	AccountRList         []AccountRWOp         `json:"accountRList"`
	AccountWList         []AccountRWOp         `json:"accountWList"`
	BytecodeRList        []BytecodeRWOp        `json:"bytecodeRList"`
	BytecodeWList        []BytecodeRWOp        `json:"bytecodeWList"`
	StorageRList         []StorageRWOp         `json:"storageRList"`
	StorageWList         []StorageRWOp         `json:"storageWList"`
	BlockHashList        []BlockHashOp         `json:"blockHashList"`
}
type CreationCounterRWOp struct {
	Lsb     uint8  `json:"lsb"`
	Counter uint64 `json:"counter"`
}
type AccountRWOp struct {
	Addr    gethcmn.Address `json:"address"`
	Nonce   hexutil.Uint64  `json:"nonce"`
	Balance *uint256.Int    `json:"balance"`
}
type BytecodeRWOp struct {
	Addr     gethcmn.Address `json:"address"`
	Bytecode hexutil.Bytes   `json:"bytecode"`
}
type StorageRWOp struct {
	Seq   hexutil.Uint64 `json:"seq"`
	Key   hexutil.Bytes  `json:"key"`
	Value hexutil.Bytes  `json:"value"`
}
type BlockHashOp struct {
	Height hexutil.Uint64 `json:"height"`
	Hash   gethcmn.Hash   `json:"hash"`
}

func toRpcCallDetail(detail *api.CallDetail) *CallDetail {
	callDetail := &CallDetail{
		Status:                 1, // success
		GasUsed:                hexutil.Uint64(detail.GasUsed),
		OutData:                detail.OutData,
		Logs:                   castEvmLogs(detail.Logs),
		CreatedContractAddress: detail.CreatedContractAddress,
		InternalTxs:            buildInternalCallList(detail.InternalTxCalls, detail.InternalTxReturns),
		RwLists:                castRWLists(detail.RwLists),
	}
	if ebp.StatusIsFailure(detail.Status) {
		callDetail.Status = 0 // failure
	}
	return callDetail
}

func castEvmLogs(evmLogs []types.EvmLog) []*CallLog {
	callLogs := make([]*CallLog, len(evmLogs))
	for i, evmLog := range evmLogs {
		callLogs[i] = &CallLog{
			Address: evmLog.Address,
			Topics:  evmLog.Topics,
			Data:    evmLog.Data,
		}
		if evmLog.Topics == nil {
			callLogs[i].Topics = make([]gethcmn.Hash, 0)
		}
	}
	return callLogs
}

func castRWLists(rwLists *types.ReadWriteLists) *RWLists {
	if rwLists == nil {
		return &RWLists{}
	}
	return &RWLists{
		CreationCounterRList: castCreationCounterOps(rwLists.CreationCounterRList),
		CreationCounterWList: castCreationCounterOps(rwLists.CreationCounterWList),
		AccountRList:         castAccountOps(rwLists.AccountRList),
		AccountWList:         castAccountOps(rwLists.AccountWList),
		BytecodeRList:        castBytecodeOps(rwLists.BytecodeRList),
		BytecodeWList:        castBytecodeOps(rwLists.BytecodeWList),
		StorageRList:         castStorageOps(rwLists.StorageRList),
		StorageWList:         castStorageOps(rwLists.StorageWList),
		BlockHashList:        castBlockHashOps(rwLists.BlockHashList),
	}
}
func castCreationCounterOps(ops []types.CreationCounterRWOp) []CreationCounterRWOp {
	rpcOps := make([]CreationCounterRWOp, len(ops))
	for i, op := range ops {
		rpcOps[i] = CreationCounterRWOp{
			Lsb:     op.Lsb,
			Counter: op.Counter,
		}
	}
	return rpcOps
}
func castAccountOps(ops []types.AccountRWOp) []AccountRWOp {
	rpcOps := make([]AccountRWOp, len(ops))
	for i, op := range ops {
		rpcOp := AccountRWOp{Addr: op.Addr}
		if len(op.Account) > 0 {
			accInfo := types.NewAccountInfo(op.Account)
			rpcOp.Nonce = hexutil.Uint64(accInfo.Nonce())
			rpcOp.Balance = accInfo.Balance()
		}
		rpcOps[i] = rpcOp
	}
	return rpcOps
}
func castBytecodeOps(ops []types.BytecodeRWOp) []BytecodeRWOp {
	rpcOps := make([]BytecodeRWOp, len(ops))
	for i, op := range ops {
		rpcOps[i] = BytecodeRWOp{
			Addr:     op.Addr,
			Bytecode: op.Bytecode,
		}
	}
	return rpcOps
}
func castStorageOps(ops []types.StorageRWOp) []StorageRWOp {
	rpcOps := make([]StorageRWOp, len(ops))
	for i, op := range ops {
		rpcOps[i] = StorageRWOp{
			Seq:   hexutil.Uint64(op.Seq),
			Key:   []byte(op.Key),
			Value: op.Value,
		}
	}
	return rpcOps
}
func castBlockHashOps(ops []types.BlockHashOp) []BlockHashOp {
	rpcOps := make([]BlockHashOp, len(ops))
	for i, op := range ops {
		rpcOps[i] = BlockHashOp{
			Height: hexutil.Uint64(op.Height),
			Hash:   op.Hash,
		}
	}
	return rpcOps
}

func TxToRpcCallDetail(tx *types.Transaction) *CallDetail {
	return &CallDetail{
		Status:                 int(tx.Status),
		GasUsed:                hexutil.Uint64(tx.GasUsed),
		OutData:                tx.OutData,
		Logs:                   castMoLogs(tx.Logs),
		CreatedContractAddress: tx.ContractAddress,
		InternalTxs:            buildInternalCallList(tx.InternalTxCalls, tx.InternalTxReturns),
		RwLists:                castRWLists(tx.RwLists),
	}
}
func castMoLogs(moLogs []types.Log) []*CallLog {
	callLogs := make([]*CallLog, len(moLogs))
	for i, moLog := range moLogs {
		callLogs[i] = &CallLog{
			Address: moLog.Address,
			Topics:  types.ToGethHashes(moLog.Topics),
			Data:    moLog.Data,
		}
	}
	return callLogs
}
