/*
unit test for ccrpc call usage

cd ../../zeniq-smart-chain
./build.sh
ll build/zeniqsmartd
. ./build.sh
echo $CGO_LDFLAGS
echo $CGO_CFLAGS

cd ../zeniq-smart-chain/ccrpc
go test -run TestDATA
go test -run TestCCRPC
go test -gcflags '-N -l' -c
dlv exec ccrpc.test
b TestCCRPC
b fetcher
b ProcessCCRPC
b TestDATA

*/

package ccrpc_test

import (
	//"encoding/json"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/testutils"

	"github.com/zeniqsmart/zeniq-smart-chain/ccrpc"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

// 5999 == 3600+2*6*10*60/3-1
func mockRpcClientForCCRPC() *MockRpcClient {
	m := &MockRpcClient{
		height: 5999,
		respData: []byte(`{"result":{"epochEndBlockTime":1661959077, "cc": [{
"height": 184464,
"txid": "91991f205b1f68dc413a0895211f15a9c3eae893e64be8ea79d1e58b6d44423d",
"hexpubkey": "03a2413b237bf2201f4df87fc0de1305a5871137b15eed15f40a22cd61f3eb92fc",
"amount": 0.00456559
}] },"error":null,"id":"zeniqsmart"}`)}
	return m
}
func mockRpcClientForCCRPC1() *MockRpcClient {
	m := &MockRpcClient{
		height:   5999,
		respData: []byte(`{"result":{"epochEndBlockTime":1661959077,"cc":[]},"error":null,"id":"zeniqsmart"}`),
	}
	return m
}

type MockRpcClient struct {
	height   int64
	respData []byte
}

func (m MockRpcClient) Dial()                     {}
func (m MockRpcClient) Close()                    {}
func (m MockRpcClient) NetworkSmartHeight() int64 { return 1 }
func (m MockRpcClient) IsConnected() bool {
	return true
}
func (m MockRpcClient) GetMainnetHeight() (height int64) {
	return 184464 + 2*6 - 1 // to get out of fetcher
}
func (m MockRpcClient) FetchCrosschain(first, last, minimum int64) (cc *ccrpctypes.CCrpcEpoch) {
	cc, _ = ccrpc.DO_GetCrosschain(first, last, m.respData)
	return
}

var _ ccrpctypes.RpcClient = MockRpcClient{}

func TestDATA(t *testing.T) {
	m := mockRpcClientForCCRPC1()
	_, err := ccrpc.DO_GetCrosschain(184464, m.GetMainnetHeight(), m.respData)
	if err != nil {
		t.Errorf("%s", err)
	}
}

func TestZeniqPubkeyToReceiverAddress(t *testing.T) {
	p, _ := hex.DecodeString("03a2413b237bf2201f4df87fc0de1305a5871137b15eed15f40a22cd61f3eb92fc")
	var pubKey [33]byte
	copy(pubKey[:], p)
	smartZeniqAddress := ccrpc.ZeniqPubkeyToReceiverAddress(pubKey)
	wantedReceiverAddress := common.Address{60, 5, 27, 151, 223, 91, 8, 119, 214, 176, 42, 138, 249, 160, 250, 11, 68, 165, 11, 252}
	require.Equal(t, smartZeniqAddress, wantedReceiverAddress)
}

func TestCCRPC(t *testing.T) {
	key, addr := testutils.GenKeyAndAddr()
	blockNumber := 7200 // 2 * 3600
	mockrpc := mockRpcClientForCCRPC()
	_app := testutils.CreateTestAppWithCC(mockrpc, key)

	ctx := _app.GetRunTxContext()
	defer ctx.Close(false)

	ti := &ccrpctypes.CCrpcTransferInfo{
		Height: 184464,
		TxID:   [32]byte{},
		Amount: float64(12.34567),
	}
	pubkey, _ := hex.DecodeString("03a2413b237bf2201f4df87fc0de1305a5871137b15eed15f40a22cd61f3eb92fc")
	copy(ti.SenderPubkey[:], pubkey)
	ccrpc.AccountCcrpc(ctx, ti, true)
	receiver := common.Address{60, 5, 27, 151, 223, 91, 8, 119, 214, 176, 42, 138, 249, 160, 250, 11, 68, 165, 11, 252}
	receiverAcc := ctx.GetAccount(receiver)
	require.NotNil(t, receiverAcc)
	receiverAccBalance := receiverAcc.Balance()
	amount := uint256.NewInt(0).Mul(uint256.NewInt(1234567000), uint256.NewInt(1e10))
	require.Equal(t, receiverAccBalance, amount)

	for {
		if _app.CCRPC.ProcessCCRPC(ctx, int64(blockNumber), 1661959077+10+1) {
			break
		}
	}
	require.Equal(t, testutils.DefaultInitBalance, _app.GetBalance(addr).Uint64())
}
