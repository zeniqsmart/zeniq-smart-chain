package watcher

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/tendermint/tendermint/libs/log"

	ccrpc "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	crosschain "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	stakingtypes "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
	"github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
)

const (
	ReqStrBlockCount = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getblockcount", "params": [] }`
	ReqStrBlockHash  = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getblockhash", "params": [%d] }`
	//verbose = 2, show all txs rawdata
	ReqStrBlock    = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getblock", "params": ["%s",2] }`
	ReqStrTx       = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getrawtransaction", "params": ["%s", true, "%s"] }`
	ReqStrCC       = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "crosschain", "params": ["%d", "%d"] }`
	ReqStrEpochs   = `{"jsonrpc": "2.0", "method": "sbch_getEpochs", "params": ["%s","%s"], "id":1}`
	ReqStrCCEpochs = `{"jsonrpc": "2.0", "method": "sbch_getCCEpochs", "params": ["%s","%s"], "id":1}`
	ReqSmartHeight = `{"jsonrpc": "2.0", "method": "eth_blockNumber", "params": [], "id":1}`
)

type RpcClientImp struct {
	url         string
	user        string
	password    string
	err         error
	contentType string
	logger      log.Logger
}

var _ types.RpcClient = (*RpcClientImp)(nil)

func NewRpcClient(url, user, password, contentType string, logger log.Logger) *RpcClientImp {
	if url == "" {
		return nil
	}
	client := &RpcClientImp{
		url:         url,
		user:        user,
		password:    password,
		contentType: contentType,
		logger:      logger,
	}
	if client == nil {
		panic(fmt.Errorf("client nil: check app.toml: current (url,user) = (%s, %s)\n", client.url, client.user))
	}
	return client
}

func (client *RpcClientImp) IsConnected() bool {
	return client != nil
}

func (client *RpcClientImp) GetMainnetHeight() (height int64) {
	height = -1
	for height == -1 {
		height = client.getCurrHeight()
		if client.err != nil {
			client.logger.Debug("GetMainnetHeight failed", client.err.Error())
			time.Sleep(10 * time.Second)
		}
	}
	return
}

func (client *RpcClientImp) NetworkSmartHeight() (height int64) {
	height = -1
	for height == -1 {
		height = client.netSmartHeight()
		if client.err != nil {
			client.logger.Debug("NetworkSmartHeight failed", client.err.Error())
			time.Sleep(time.Second)
		}
	}
	return
}

func (client *RpcClientImp) FetchCC(first, last int64) *ccrpc.CCrpcEpoch {
	var err error
	var cc *ccrpc.CCrpcEpoch
	cc, err = client.getCC(first, last)
	if err != nil {
		client.logger.Debug(fmt.Sprintf("getCC %d %d failed", first, last), err.Error())
		return cc
	}
	for _, cce := range cc.TransferInfos {
		client.logger.Debug(fmt.Sprintf("cc entry at block %d ", cce.Height))
	}
	return cc
}

func (client *RpcClientImp) GetBlockByHeight(height int64, retry bool) *types.BCHBlock {
	var hash string
	var err error
	var blk *types.BCHBlock
	for hash == "" {
		hash, err = client.getBlockHashOfHeight(height)
		if err != nil {
			if !retry {
				return nil
			}
			client.logger.Debug(fmt.Sprintf("getBlockHashOfHeight %d failed", height), err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		//fmt.Printf("get bch block hash\n")
	}
	for blk == nil {
		blk, err = client.getBCHBlock(hash)
		if !retry {
			return blk
		}
		if err != nil {
			client.logger.Debug(fmt.Sprintf("getBCHBlock %d failed", height), err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		//fmt.Printf("get bch block: %d\n", height)
	}
	return blk
}

func (client *RpcClientImp) GetEpochs(start, end uint64) []*stakingtypes.Epoch {
	var epochs []*stakingtypes.Epoch
	if client == nil {
		return epochs
	}
	epochs = client.getEpochs(start, end)
	if client.err != nil {
		client.logger.Debug("GetEpochs failed, turning off speedup", client.err.Error())
	}
	return epochs
}

func (client *RpcClientImp) sendRequest(reqStr string) ([]byte, error) {
	body := strings.NewReader(reqStr)
	req, err := http.NewRequest("POST", client.url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(client.user, client.password)
	req.Header.Set("Content-Type", client.contentType)
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return respData, nil
}

func (client *RpcClientImp) getBCHBlock(hash string) (*types.BCHBlock, error) {
	var bi *types.BlockInfo
	var err error
	bi, err = client.getBlock(hash)
	if err != nil {
		return nil, err
	}
	bchBlock := &types.BCHBlock{
		Height:    bi.Height,
		Timestamp: bi.Time,
	}
	var bz []byte
	bz, err = hex.DecodeString(bi.Hash)
	copy(bchBlock.HashId[:], bz)
	if err != nil {
		return nil, err
	}
	bz, err = hex.DecodeString(bi.PreviousBlockhash)
	copy(bchBlock.ParentBlk[:], bz)
	if err != nil {
		return nil, err
	}
	if bi.Height > 0 {
		nomination := getNomination(bi.Tx[0])
		if nomination != nil {
			bchBlock.Nominations = append(bchBlock.Nominations, *nomination)
		}
		//bchBlock.CCTransferInfos = append(bchBlock.CCTransferInfos, client.getCCTransferInfos(bi)...)
	}
	return bchBlock, nil
}

func getNomination(coinbase types.TxInfo) *stakingtypes.Nomination {
	pubKey, ok := coinbase.GetValidatorPubKey()
	if ok {
		return &stakingtypes.Nomination{
			Pubkey:         pubKey,
			NominatedCount: 1,
		}
	}
	return nil
}

//func (client *RpcClientImp) getCCTransferInfos(bi *types.BlockInfo) []*crosschain.CCTransferInfo {
//	var ccInfos []*crosschain.CCTransferInfo
//	for _, info := range bi.Tx {
//		ccInfos = append(ccInfos, info.GetCCTransferInfos()...)
//	}
//	return ccInfos
//}

func (client *RpcClientImp) getCurrHeight() int64 {
	var respData []byte

	respData, client.err = client.sendRequest(ReqStrBlockCount)

	if client.err != nil {
		return -1
	}
	var blockCountResp types.BlockCountResp
	client.err = json.Unmarshal(respData, &blockCountResp)
	if client.err != nil {
		return -1
	}
	if blockCountResp.Error != nil && blockCountResp.Error.Code < 0 {
		client.err = fmt.Errorf("getCurrHeight error, code:%d, msg:%s\n", blockCountResp.Error.Code, blockCountResp.Error.Message)
		return blockCountResp.Result
	}
	return blockCountResp.Result
}

func (client *RpcClientImp) netSmartHeight() int64 {
	var respData []byte

	respData, client.err = client.sendRequest(ReqSmartHeight)

	if client.err != nil {
		return -1
	}
	var blockNumberResp types.BlockNumber
	client.err = json.Unmarshal(respData, &blockNumberResp)
	if client.err != nil {
		return -1
	}
	if blockNumberResp.Error != nil && blockNumberResp.Error.Code < 0 {
		client.err = fmt.Errorf("netSmartHeight error, code:%d, msg:%s\n", blockNumberResp.Error.Code, blockNumberResp.Error.Message)
		return -1
	}
	Result := new(big.Int)
	Result.SetString(blockNumberResp.Result, 0)
	return Result.Int64()
}

func (client *RpcClientImp) getBlockHashOfHeight(height int64) (string, error) {
	var respData []byte
	var err error
	respData, err = client.sendRequest(fmt.Sprintf(ReqStrBlockHash, height))
	if err != nil {
		return "", err
	}
	var blockHashResp types.BlockHashResp
	err = json.Unmarshal(respData, &blockHashResp)
	if err != nil {
		return "", err
	}
	if blockHashResp.Error != nil && blockHashResp.Error.Code < 0 {
		err = fmt.Errorf("getBlockHashOfHeight error, height:%d, code:%d, msg:%s\n",
			height, blockHashResp.Error.Code, blockHashResp.Error.Message)
		return "", err
	}
	return blockHashResp.Result, nil
}

func (client *RpcClientImp) getBlock(hash string) (*types.BlockInfo, error) {
	respData, err := client.sendRequest(fmt.Sprintf(ReqStrBlock, hash))
	if err != nil {
		return nil, err
	}
	var blockInfoResp types.BlockInfoResp
	err = json.Unmarshal(respData, &blockInfoResp)
	if err != nil {
		return nil, err
	}
	if blockInfoResp.Error != nil && blockInfoResp.Error.Code < 0 {
		return &blockInfoResp.Result, fmt.Errorf("getBlock error, code:%d, msg:%s\n",
			blockInfoResp.Error.Code, blockInfoResp.Error.Message)
	}

	// try to adapt BCHD
	if len(blockInfoResp.Result.Tx) == 0 &&
		len(blockInfoResp.Result.RawTx) > 0 {

		blockInfoResp.Result.Tx = blockInfoResp.Result.RawTx
		for i := 0; i < len(blockInfoResp.Result.Tx); i++ {
			blockInfoResp.Result.Tx[i].Hash = blockInfoResp.Result.Tx[i].TxID
		}
	}

	return &blockInfoResp.Result, nil
}

func (client *RpcClientImp) getCC(first, last int64) (*ccrpc.CCrpcEpoch, error) {
	respData, err := client.sendRequest(fmt.Sprintf(ReqStrCC, first, last))
	if err != nil {
		return nil, err
	}
	return DO_getCC(first, last, respData)
}

func DO_getCC(first, last int64, respData []byte) (*ccrpc.CCrpcEpoch, error) {
	var ccResponses ccrpc.CCrpcResponses
	iserr := json.Unmarshal(respData, &ccResponses.Error)
	if iserr != nil && ccResponses.Error != nil && ccResponses.Error.Code < 0 {
		return nil, fmt.Errorf("crosschain rpc request error, code:%d, msg:%s\n",
			ccResponses.Error.Code, ccResponses.Error.Message)
	}
	err := json.Unmarshal(respData, &ccResponses)
	if err != nil {
		return nil, err
	}
	var ccepoch = ccrpc.CCrpcEpoch{
		FirstHeight:       first,
		LastHeight:        last,
		EpochEndBlockTime: ccResponses.Result.EpochEndBlockTime,
		EEBTSmartHeight:   0,
	}
	for i := 0; i < len(ccResponses.Result.CC); i++ {
		cci := ccResponses.Result.CC[i]
		var ptx []byte
		ptx, err = hex.DecodeString(cci.TxID)
		if err != nil {
			return nil, err
		}
		var txid [32]byte
		copy(txid[:], ptx)
		var pk []byte
		pk, err = hex.DecodeString(cci.Hexpubkey)
		if err != nil {
			return nil, err
		}
		var pubkey [33]byte
		copy(pubkey[:], pk)
		ccepoch.TransferInfos = append(ccepoch.TransferInfos,
			&ccrpc.CCrpcTransferInfo{
				Height:       cci.Height,
				TxID:         txid,
				SenderPubkey: pubkey,
				Amount:       cci.Amount,
			})
	}
	return &ccepoch, nil
}

func (client *RpcClientImp) getTx(hash string, blockhash string) (*types.TxInfo, error) {
	respData, err := client.sendRequest(fmt.Sprintf(ReqStrTx, hash, blockhash))
	if err != nil {
		return nil, err
	}
	var txInfoResp types.TxInfoResp
	err = json.Unmarshal(respData, &txInfoResp)
	if err != nil {
		return nil, err
	}
	if txInfoResp.Error != nil && txInfoResp.Error.Code < 0 {
		return &txInfoResp.Result, fmt.Errorf("getTx error, code:%d, msg:%s\n",
			txInfoResp.Error.Code, txInfoResp.Error.Message)
	}
	return &txInfoResp.Result, nil
}

type zeniqsmartJsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type zeniqsmartJsonrpcMessage struct {
	Version string                  `json:"jsonrpc,omitempty"`
	ID      json.RawMessage         `json:"id,omitempty"`
	Method  string                  `json:"method,omitempty"`
	Params  json.RawMessage         `json:"params,omitempty"`
	Error   *zeniqsmartJsonrpcError `json:"error,omitempty"`
	Result  json.RawMessage         `json:"result,omitempty"`
}

func (client *RpcClientImp) getEpochs(start, end uint64) []*stakingtypes.Epoch {
	var respData []byte
	respData, client.err = client.sendRequest(fmt.Sprintf(ReqStrEpochs, hexutil.Uint64(start).String(), hexutil.Uint64(end).String()))
	if client.err != nil {
		return nil
	}
	var m zeniqsmartJsonrpcMessage
	client.err = json.Unmarshal(respData, &m)
	if client.err != nil {
		return nil
	}
	var epochsResp []*stakingtypes.Epoch
	client.err = json.Unmarshal(m.Result, &epochsResp)
	if client.err != nil {
		return nil
	}
	return epochsResp
}

func (client *RpcClientImp) GetCCEpochs(start, end uint64) []*crosschain.CCEpoch {
	var respData []byte
	respData, client.err = client.sendRequest(fmt.Sprintf(ReqStrCCEpochs, hexutil.Uint64(start).String(), hexutil.Uint64(end).String()))
	if client.err != nil {
		return nil
	}
	var m zeniqsmartJsonrpcMessage
	client.err = json.Unmarshal(respData, &m)
	if client.err != nil {
		return nil
	}
	var epochsResp []*crosschain.CCEpoch
	client.err = json.Unmarshal(m.Result, &epochsResp)
	if client.err != nil {
		return nil
	}
	return epochsResp
}

/***for tool*/
func (client *RpcClientImp) GetBlockHash(height int64) (string, error) {
	return client.getBlockHashOfHeight(height)
}
func (client *RpcClientImp) GetBlockInfo(hash string) (*types.BlockInfo, error) {
	return client.getBlock(hash)
}
func (client *RpcClientImp) GetTxInfo(hash string, blockhash string) (*types.TxInfo, error) {
	return client.getTx(hash, blockhash)
}
