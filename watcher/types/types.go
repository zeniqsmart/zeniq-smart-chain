package types

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	cctypes "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"

	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

const (
	Identifier = "73424348" // ascii code for 'sBCH'
	Version    = "00"

	ShaGateAddress = "14f8c7e99fd4e867c34cbd5968e35575fd5919a4"
)

// These functions must be provided by a client connecting to a Bitcoin Cash's fullnode
type RpcClient interface {
	// smart
	NetworkSmartHeight() int64
	GetEpochs(start, end uint64) []*stake.Epoch
	GetCCEpochs(start, end uint64) []*cctypes.CCEpoch
	// mainnet
	IsConnected() bool
	GetMainnetHeight() int64
	FetchCC(first, last int64) *ccrpctypes.CCrpcEpoch
	GetBlockByHeight(height int64, retry bool) *BCHBlock
}
type RpcClientMock interface {
	DoWatch() bool
}

// This struct contains the useful information of a BCH block
type BCHBlock struct {
	Height          int64
	Timestamp       int64
	HashId          [32]byte
	ParentBlk       [32]byte
	Nominations     []stake.Nomination
	CCTransferInfos []*cctypes.CCTransferInfo
}

// not check Nominations
func (b *BCHBlock) Equal(o *BCHBlock) bool {
	return b.Height == o.Height && b.Timestamp == o.Timestamp &&
		b.HashId == o.HashId && b.ParentBlk == o.ParentBlk
}

/***mainnet data structure*/
type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type BlockCountResp struct {
	Result int64         `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}

type BlockHashResp struct {
	Result string        `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}

type BlockNumber struct {
	Result string        `json:"result"`
	Id     int64         `json:"id"`
	Error  *JsonRpcError `json:"error"`
}

type BlockInfo struct {
	Hash              string   `json:"hash"`
	Confirmations     int      `json:"confirmations"`
	Size              int      `json:"size"`
	Height            int64    `json:"height"`
	Version           int      `json:"version"`
	VersionHex        string   `json:"versionHex"`
	Merkleroot        string   `json:"merkleroot"`
	Tx                []TxInfo `json:"tx"`
	RawTx             []TxInfo `json:"rawtx"` // BCHD
	Time              int64    `json:"time"`
	MedianTime        int64    `json:"mediantime"`
	Nonce             int      `json:"nonce"`
	Bits              string   `json:"bits"`
	Difficulty        float64  `json:"difficulty"`
	Chainwork         string   `json:"chainwork"`
	NumTx             int      `json:"nTx"`
	PreviousBlockhash string   `json:"previousblockhash"`
}

type BlockInfoResp struct {
	Result BlockInfo     `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}

type CoinbaseVin struct {
	Coinbase string `json:"coinbase"`
	Sequence int    `json:"sequence"`
}

type Vout struct {
	Value        float64                `json:"value"`
	N            int                    `json:"n"`
	ScriptPubKey map[string]interface{} `json:"scriptPubKey"`
}

type TxInfo struct {
	TxID          string                   `json:"txid"`
	Hash          string                   `json:"hash"`
	Version       int                      `json:"version"`
	Size          int                      `json:"size"`
	Locktime      int                      `json:"locktime"`
	VinList       []map[string]interface{} `json:"vin"`
	VoutList      []Vout                   `json:"vout"`
	Hex           string                   `json:"hex"`
	Blockhash     string                   `json:"blockhash"`
	Confirmations int                      `json:"confirmations"`
	Time          int64                    `json:"time"`
	BlockTime     int64                    `json:"blocktime"`
}

func (ti TxInfo) GetValidatorPubKey() (pubKey [32]byte, success bool) {
	for _, vout := range ti.VoutList {
		asm, ok := vout.ScriptPubKey["asm"]
		if !ok || asm == nil {
			continue
		}
		script, ok := asm.(string)
		if !ok {
			continue
		}
		prefix := "OP_RETURN " + Identifier + Version
		if !strings.HasPrefix(script, prefix) {
			continue
		}
		script = script[len(prefix):]
		if len(script) != 64 {
			continue
		}
		bz, err := hex.DecodeString(script)
		if err != nil {
			continue
		}
		copy(pubKey[:], bz)
		success = true
		break
	}
	return
}

func (ti TxInfo) GetCCTransferInfos() (infos []*cctypes.CCTransferInfo) {
	for n, vOut := range ti.VoutList {
		asm, ok := vOut.ScriptPubKey["asm"]
		if !ok || asm == nil {
			continue
		}
		script, ok := asm.(string)
		if !ok {
			continue
		}
		target := "OP_HASH160 " + ShaGateAddress + " OP_EQUAL"
		if script != target {
			continue
		}
		var info cctypes.CCTransferInfo
		info.Amount = uint64(vOut.Value * (10e8))
		copy(info.UTXO[:32], ti.Hash)
		var vOutIndex [4]byte
		binary.BigEndian.PutUint32(vOutIndex[:], uint32(n))
		copy(info.UTXO[32:], vOutIndex[:])
		infos = append(infos, &info)
	}
	if len(infos) != 0 {
		vIn := ti.VinList[0]
		//todo: modify this to match real rules, for test now
		value, exist := vIn["test"]
		if !exist || value == nil {
			return nil
		}
		pubkeyString, ok := value.(string)
		if !ok {
			return nil
		}
		pubkeyBytes, err := hex.DecodeString(pubkeyString)
		if err != nil {
			return nil
		}
		var pubkey [33]byte
		copy(pubkey[:], pubkeyBytes)
		fmt.Printf("get cc infos:\n")
		for _, info := range infos {
			info.SenderPubkey = pubkey
			fmt.Printf("info.pubkey:%v, info.amount:%d, info.utxo:%v\n", info.SenderPubkey, info.Amount, info.UTXO)
		}
	}
	return
}

type TxInfoResp struct {
	Result TxInfo        `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}
