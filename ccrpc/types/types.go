package types

import (
	"github.com/ethereum/go-ethereum/common"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

type CCrpcTransferInfo struct {
	Height            int64          `json:"height"`
	TxID              [32]byte       `json:"txid"`
	SenderPubkey      [33]byte       `json:"senderpubkey"`
	Amount            float64        `json:"amount"`
	EpochEndHeight    int64          `json:"epochendheight"`
	EpochEndBlockTime int64          `json:"epochendblocktime"` //EEBT
	ApplicationHeight int64          `json:"applicationheight"`
	Receiver          common.Address `json:"receiver"`
}

type CCrpcEpoch struct {
	FirstHeight       int64
	LastHeight        int64
	EpochEndBlockTime int64 // EEBT
	EEBTSmartHeight   int64
	TransferInfos     []*CCrpcTransferInfo
}

type CCrpcResponse struct {
	Height    int64   `json:"height"`
	TxID      string  `json:"txid"`
	Hexpubkey string  `json:"hexpubkey"`
	Amount    float64 `json:"amount"`
}
type CCrpcCC struct {
	EpochEndBlockTime int64           `json:"epochEndBlockTime"`
	CC                []CCrpcResponse `json:"cc"`
}
type CCrpcResponses struct {
	Result CCrpcCC       `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}
type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CCrpcInfoResponse struct {
	Transfers []CCrpcTransferInfo `json:"transfers"`
	Error     *JsonRpcError       `json:"error"`
	Id        string              `json:"id"`
}

type MainBlock struct {
	Height      int64
	Timestamp   int64
	HashId      [32]byte
	ParentBlk   [32]byte
	Nominations []stake.Nomination
}

// not check Nominations
func (b *MainBlock) Equal(o *MainBlock) bool {
	return b.Height == o.Height && b.Timestamp == o.Timestamp &&
		b.HashId == o.HashId && b.ParentBlk == o.ParentBlk
}

type RpcClient interface {
	// smart
	NetworkSmartHeight() int64
	// mainnet
	IsConnected() bool
	GetMainnetHeight() int64
	FetchCrosschain(first, last, minimum int64) *CCrpcEpoch
}

type BlockCountResp struct {
	Result int64         `json:"result"`
	Error  *JsonRpcError `json:"error"`
	Id     string        `json:"id"`
}

type BlockNumber struct {
	Result string        `json:"result"`
	Id     int64         `json:"id"`
	Error  *JsonRpcError `json:"error"`
}
