/*
unit test for ccrpc call usage

~/smartzeniq/zeniq-smart-chain
. ./build.sh
~/smartzeniq/zeniq-smart-chain/ccrpc
go test -run TestDATA
go test -run TestCCRPC
go test -gcflags '-N -l' -c
dlv exec ccrpc.test
b testBlockAfterTime
b TestCCRPC
b CCRPCProcessed
b TestDATA

*/

package ccrpc_test

import (
	//"encoding/json"
	"encoding/hex"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/testutils"

	"github.com/zeniqsmart/zeniq-smart-chain/ccrpc"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	cctypes "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
	"github.com/zeniqsmart/zeniq-smart-chain/watcher"
	watchertypes "github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
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
func (m MockRpcClient) GetBlockByHeight(height int64, retry bool) *watchertypes.BCHBlock {
	return &watchertypes.BCHBlock{ // must be not nil, else this data is used nowhere
		Height:          m.GetMainnetHeight(),
		Timestamp:       1661959077,
		HashId:          make([][32]byte, 1)[0],
		ParentBlk:       make([][32]byte, 1)[0],
		Nominations:     make([]stake.Nomination, 0),
		CCTransferInfos: make([]*cctypes.CCTransferInfo, 0),
	}
}
func (m MockRpcClient) GetBlockByHash(hash [32]byte) *watchertypes.BCHBlock {
	return nil
}
func (m MockRpcClient) GetEpochs(start, end uint64) []*stake.Epoch {
	return nil
}
func (m MockRpcClient) GetCCEpochs(start, end uint64) []*cctypes.CCEpoch {
	return nil
}
func (m MockRpcClient) IsConnected() bool {
	return true
}
func (m MockRpcClient) GetMainnetHeight() (height int64) {
	n := int64(6)
	return 184464 + 2*n - n/2 // -1 would_block
}
func (m MockRpcClient) FetchCC(first, last int64) (cc *ccrpctypes.CCrpcEpoch) {
	cc, _ = watcher.DO_getCC(first, last, m.respData)
	return
}

var _ watchertypes.RpcClient = MockRpcClient{}

func TestDATA(t *testing.T) {
	m := mockRpcClientForCCRPC1()
	_, err := watcher.DO_getCC(184464, m.GetMainnetHeight(), m.respData)
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

func testBlockAfterTime(searchBack, epochEndBlockTime int64) int64 {
	var blks = make([]int64, 3600*2)
	for i := 0; i < 2*3600; i++ {
		blks[i] = 1661959077 + 3*int64(i-3600)
	}
	blockNumber := 5999
	after := func(fromEnd int) bool {
		return blks[blockNumber-fromEnd] < epochEndBlockTime
	}
	return int64(blockNumber - int(sort.Search(int(searchBack), after)))
}

func TestCCRPC(t *testing.T) {
	key, addr := testutils.GenKeyAndAddr()
	blockNumber := 4799 // 3600+6*10*60/3-1
	_app := testutils.CreateTestAppWithCC(mockRpcClientForCCRPC(), key)

	// CCRPCMain should loop once and then wait
	ctx := _app.GetRunTxContext()
	defer ctx.Close(false)

	ti := &ccrpctypes.CCrpcTransferInfo{
		Height: 184464,
		TxID:   [32]byte{},
		Amount: float64(12.34567),
	}
	pubkey, _ := hex.DecodeString("03a2413b237bf2201f4df87fc0de1305a5871137b15eed15f40a22cd61f3eb92fc")
	copy(ti.SenderPubkey[:], pubkey)
	ccrpc.AccountCcrpc(ctx, ti)
	receiver := common.Address{60, 5, 27, 151, 223, 91, 8, 119, 214, 176, 42, 138, 249, 160, 250, 11, 68, 165, 11, 252}
	receiverAcc := ctx.GetAccount(receiver)
	require.NotNil(t, receiverAcc)
	receiverAccBalance := receiverAcc.Balance()
	amount := uint256.NewInt(0).Mul(uint256.NewInt(1234567000), uint256.NewInt(1e10))
	require.Equal(t, receiverAccBalance, amount)

	for {
		if _app.CCRPC.CCRPCProcessed(ctx, int64(blockNumber), 1661959077, testBlockAfterTime) {
			break
		}
	}
	require.Equal(t, testutils.DefaultInitBalance, _app.GetBalance(addr).Uint64())
}
