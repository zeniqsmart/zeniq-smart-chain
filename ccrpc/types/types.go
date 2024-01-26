package types

import ()

type CCrpcTransferInfo struct {
	Height       int64
	TxID         [32]byte
	SenderPubkey [33]byte
	Amount       float64
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
