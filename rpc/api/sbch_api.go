package api

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/zeniqsmart/evm-zeniq-smart-chain/types"
	sbchapi "github.com/zeniqsmart/zeniq-smart-chain/api"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	rpctypes "github.com/zeniqsmart/zeniq-smart-chain/rpc/internal/ethapi"
	"github.com/zeniqsmart/zeniq-smart-chain/staking"
)

var _ SbchAPI = (*sbchAPI)(nil)

type SbchAPI interface {
	GetStandbyTxQueue()
	QueryTxBySrc(addr gethcmn.Address, startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error)
	QueryTxByDst(addr gethcmn.Address, startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error)
	QueryTxByAddr(addr gethcmn.Address, startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error)
	QueryLogs(addr gethcmn.Address, topics []gethcmn.Hash, startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*gethtypes.Log, error)
	GetTxListByHeight(height gethrpc.BlockNumber) ([]map[string]interface{}, error)
	GetTxListByHeightWithRange(height gethrpc.BlockNumber, start, end hexutil.Uint64) ([]map[string]interface{}, error)
	GetAddressCount(kind string, addr gethcmn.Address) hexutil.Uint64
	GetSep20AddressCount(kind string, contract, addr gethcmn.Address) hexutil.Uint64
	HealthCheck(latestBlockTooOldAge hexutil.Uint64) map[string]interface{}
	GetTransactionReceipt(hash gethcmn.Hash) (map[string]interface{}, error)
	Call(args rpctypes.CallArgs, blockNr gethrpc.BlockNumberOrHash) (*CallDetail, error)
	ValidatorsInfo() json.RawMessage
	GetSyncBlock(height hexutil.Uint64) (hexutil.Bytes, error)
	CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo
}

type sbchAPI struct {
	backend sbchapi.BackendService
	logger  log.Logger
}

func newSbchAPI(backend sbchapi.BackendService, logger log.Logger) SbchAPI {
	return sbchAPI{
		backend: backend,
		logger:  logger,
	}
}

func (sbch sbchAPI) GetStandbyTxQueue() {
	sbch.logger.Debug("zeniq_getStandbyTxQueue")
	panic("implement me")
}

func (sbch sbchAPI) GetTxListByHeight(height gethrpc.BlockNumber) ([]map[string]interface{}, error) {
	sbch.logger.Debug("zeniq_getTxListByHeight")
	return sbch.GetTxListByHeightWithRange(height, 0, 0)
}

func (sbch sbchAPI) GetTxListByHeightWithRange(height gethrpc.BlockNumber, start, end hexutil.Uint64) ([]map[string]interface{}, error) {
	sbch.logger.Debug("zeniq_getTxListByHeightWithRange")
	if height == gethrpc.LatestBlockNumber {
		height = gethrpc.BlockNumber(sbch.backend.LatestHeight())
	}

	iStart := int(start)
	iEnd := int(end)
	if iEnd == 0 {
		iEnd = -1
	}
	txs, _, err := sbch.backend.GetTxListByHeightWithRange(uint32(height), iStart, iEnd)
	if err != nil {
		return nil, err
	}
	return txsToReceiptsWithInternalTxs(txs), nil
}

func (sbch sbchAPI) QueryTxBySrc(addr gethcmn.Address,
	startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error) {

	sbch.logger.Debug("zeniq_queryTxBySrc")
	_start, _end := sbch.prepareHeightRange(startHeight, endHeight)
	txs, sigs, err := sbch.backend.QueryTxBySrc(addr, _start, _end, uint32(limit))
	if err != nil {
		return nil, err
	}

	return txsToRpcResp(txs, sigs), nil
}

func (sbch sbchAPI) QueryTxByDst(addr gethcmn.Address,
	startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error) {

	sbch.logger.Debug("zeniq_queryTxByDst")
	_start, _end := sbch.prepareHeightRange(startHeight, endHeight)
	txs, sigs, err := sbch.backend.QueryTxByDst(addr, _start, _end, uint32(limit))
	if err != nil {
		return nil, err
	}

	return txsToRpcResp(txs, sigs), nil
}

func (sbch sbchAPI) QueryTxByAddr(addr gethcmn.Address,
	startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*rpctypes.Transaction, error) {

	sbch.logger.Debug("zeniq_queryTxByAddr")
	_start, _end := sbch.prepareHeightRange(startHeight, endHeight)
	txs, sigs, err := sbch.backend.QueryTxByAddr(addr, _start, _end, uint32(limit))
	if err != nil {
		return nil, err
	}

	return txsToRpcResp(txs, sigs), nil
}

func (sbch sbchAPI) prepareHeightRange(startHeight, endHeight gethrpc.BlockNumber) (uint32, uint32) {
	if startHeight == gethrpc.LatestBlockNumber {
		startHeight = gethrpc.BlockNumber(sbch.backend.LatestHeight())
	}
	if endHeight == gethrpc.LatestBlockNumber {
		endHeight = gethrpc.BlockNumber(sbch.backend.LatestHeight())
	}
	if startHeight > endHeight {
		startHeight++
	} else {
		endHeight++
	}
	return uint32(startHeight), uint32(endHeight)
}

func (sbch sbchAPI) QueryLogs(addr gethcmn.Address, topics []gethcmn.Hash,
	startHeight, endHeight gethrpc.BlockNumber, limit hexutil.Uint64) ([]*gethtypes.Log, error) {

	sbch.logger.Debug("zeniq_queryLogs")
	if startHeight == gethrpc.LatestBlockNumber {
		startHeight = gethrpc.BlockNumber(sbch.backend.LatestHeight())
	}
	if endHeight == gethrpc.LatestBlockNumber {
		endHeight = gethrpc.BlockNumber(sbch.backend.LatestHeight())
	}

	logs, err := sbch.backend.SbchQueryLogs(addr, topics,
		uint32(startHeight), uint32(endHeight), uint32(limit))
	if err != nil {
		return nil, err
	}
	return types.ToGethLogs(logs), nil
}

func (sbch sbchAPI) GetAddressCount(kind string, addr gethcmn.Address) hexutil.Uint64 {
	sbch.logger.Debug("zeniq_getAddressCount")
	fromCount, toCount := int64(0), int64(0)
	if kind == "from" || kind == "both" {
		fromCount = sbch.backend.GetFromAddressCount(addr)
	}
	if kind == "to" || kind == "both" {
		toCount = sbch.backend.GetToAddressCount(addr)
	}
	if kind == "from" {
		return hexutil.Uint64(fromCount)
	} else if kind == "to" {
		return hexutil.Uint64(toCount)
	} else if kind == "both" {
		return hexutil.Uint64(fromCount + toCount)
	}
	return hexutil.Uint64(0)
}

func (sbch sbchAPI) GetSep20AddressCount(kind string, contract, addr gethcmn.Address) hexutil.Uint64 {
	sbch.logger.Debug("zeniq_getSep20AddressCount")
	fromCount, toCount := int64(0), int64(0)
	if kind == "from" || kind == "both" {
		fromCount = sbch.backend.GetSep20FromAddressCount(contract, addr)
	}
	if kind == "to" || kind == "both" {
		toCount = sbch.backend.GetSep20ToAddressCount(contract, addr)
	}
	if kind == "from" {
		return hexutil.Uint64(fromCount)
	} else if kind == "to" {
		return hexutil.Uint64(toCount)
	} else if kind == "both" {
		return hexutil.Uint64(fromCount + toCount)
	}
	return hexutil.Uint64(0)
}

func coinDaysSlotToFloat(coindaysSlot *big.Int) float64 {
	fCoinDays, _ := big.NewFloat(0).Quo(
		big.NewFloat(0).SetInt(coindaysSlot),
		big.NewFloat(0).SetInt(staking.CoindayUnit.ToBig()),
	).Float64()
	return fCoinDays
}

func (sbch sbchAPI) HealthCheck(latestBlockTooOldAge hexutil.Uint64) map[string]interface{} {
	sbch.logger.Debug("zeniq_healthCheck")
	if latestBlockTooOldAge == 0 {
		latestBlockTooOldAge = 30
	}

	var latestBlockHeight hexutil.Uint64
	var latestBlockTimestamp hexutil.Uint64
	var ok bool
	var msg string

	b, err := sbch.backend.CurrentBlock()
	if err == nil {
		latestBlockHeight = hexutil.Uint64(b.Number)
		latestBlockTimestamp = hexutil.Uint64(b.Timestamp)

		latestBlockAge := time.Now().Unix() - b.Timestamp
		ok = latestBlockAge < int64(latestBlockTooOldAge)
		if !ok {
			msg = fmt.Sprintf("latest block is too old: %ds", latestBlockAge)
		}
	} else {
		msg = err.Error()
	}

	return map[string]interface{}{
		"latestBlockHeight":    latestBlockHeight,
		"latestBlockTimestamp": latestBlockTimestamp,
		"ok":                   ok,
		"error":                msg,
	}
}

func (sbch sbchAPI) GetTransactionReceipt(hash gethcmn.Hash) (map[string]interface{}, error) {
	sbch.logger.Debug("zeniq_getTransactionReceipt")
	tx, _, err := sbch.backend.GetTransaction(hash)
	if err != nil {
		// the transaction is not yet available
		return nil, nil
	}
	ret := txToReceiptWithInternalTxs(tx)
	return ret, nil
}

func (sbch sbchAPI) Call(args rpctypes.CallArgs, blockNr gethrpc.BlockNumberOrHash) (*CallDetail, error) {
	sbch.logger.Debug("zeniq_call")

	tx, from := createGethTxFromCallArgs(args)
	height, err := getHeightArg(sbch.backend, blockNr)
	if err != nil {
		return nil, err
	}

	callDetail := sbch.backend.CallForSbch(tx, from, height)
	return toRpcCallDetail(callDetail), nil
}

func (sbch sbchAPI) ValidatorsInfo() json.RawMessage {
	sbch.logger.Debug("zeniq_validatorsInfo")
	info := sbch.backend.ValidatorsInfo()
	bytes, _ := json.Marshal(info)
	return bytes
}

func (sbch sbchAPI) GetSyncBlock(height hexutil.Uint64) (hexutil.Bytes, error) {
	sbch.logger.Debug("zeniq_getSyncBlock")
	return sbch.backend.GetSyncBlock(int64(height))
}

func (sbch sbchAPI) CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo {
	return sbch.backend.CrosschainInfo(start, end)
}
