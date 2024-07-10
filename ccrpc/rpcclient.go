package ccrpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/libs/log"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"
)

/*
ccrpc/rpcclient.go:166:21: undefined: ccrpctypes.BlockInfo
ccrpc/rpcclient.go:196:40: undefined: ccrpctypes.TxInfo
ccrpc/rpcclient.go:215:32: undefined: ccrpctypes.BlockCountResp
ccrpc/rpcclient.go:235:33: undefined: ccrpctypes.BlockNumber
ccrpc/rpcclient.go:256:31: undefined: ccrpctypes.BlockHashResp
ccrpc/rpcclient.go:274:31: undefined: ccrpctypes.BlockInfoResp
*/

const (
	ReqStrBlockCount = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getblockcount", "params": [] }`
	ReqStrBlockHash  = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "getblockhash", "params": [%d] }`
	//verbose = 2, show all txs rawdata
	ReqStrCC       = `{"jsonrpc": "1.0", "id":"zeniqsmart", "method": "crosschain", "params": ["%d", "%d", "%d"] }`
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

var _ ccrpctypes.RpcClient = (*RpcClientImp)(nil)

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

func (client *RpcClientImp) FetchCrosschain(first, last, minimum int64) *ccrpctypes.CCrpcEpoch {
	var err error
	var cc *ccrpctypes.CCrpcEpoch
	cc, err = client.getCrosschain(first, last, minimum)
	if err != nil {
		client.logger.Debug(fmt.Sprintf("getCrosschain %d %d %v failed", first, last, minimum), err.Error())
		return cc
	}
	for _, cce := range cc.TransferInfos {
		client.logger.Debug(fmt.Sprintf("cc entry at block %d ", cce.Height))
	}
	return cc
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

func (client *RpcClientImp) getCurrHeight() int64 {
	var respData []byte

	respData, client.err = client.sendRequest(ReqStrBlockCount)

	if client.err != nil {
		return -1
	}
	var blockCountResp ccrpctypes.BlockCountResp
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
	var blockNumberResp ccrpctypes.BlockNumber
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

func (client *RpcClientImp) getCrosschain(first, last, minimum int64) (*ccrpctypes.CCrpcEpoch, error) {
	respData, err := client.sendRequest(fmt.Sprintf(ReqStrCC, first, last, minimum))
	if err != nil {
		return nil, err
	}
	return DO_GetCrosschain(first, last, respData)
}

func DO_GetCrosschain(first, last int64, respData []byte) (*ccrpctypes.CCrpcEpoch, error) {
	var ccResponses ccrpctypes.CCrpcResponses
	iserr := json.Unmarshal(respData, &ccResponses.Error)
	if iserr != nil && ccResponses.Error != nil && ccResponses.Error.Code < 0 {
		return nil, fmt.Errorf("crosschain rpc request error, code:%d, msg:%s\n",
			ccResponses.Error.Code, ccResponses.Error.Message)
	}
	err := json.Unmarshal(respData, &ccResponses)
	if err != nil {
		return nil, err
	}
	var ccepoch = ccrpctypes.CCrpcEpoch{
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
			&ccrpctypes.CCrpcTransferInfo{
				Height:            cci.Height,
				TxID:              txid,
				SenderPubkey:      pubkey,
				Amount:            cci.Amount,
				EpochEndHeight:    ccepoch.LastHeight,
				EpochEndBlockTime: ccepoch.EpochEndBlockTime,
			})
	}
	return &ccepoch, nil
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
