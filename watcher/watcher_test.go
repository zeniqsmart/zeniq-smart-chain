package watcher

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"

	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
	cctypes "github.com/zeniqsmart/zeniq-smart-chain/crosschain/types"
	"github.com/zeniqsmart/zeniq-smart-chain/param"
	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
	"github.com/zeniqsmart/zeniq-smart-chain/watcher/types"
)

type MockBCHNode struct {
	height      int64
	blocks      []*types.BCHBlock
	reorgBlocks map[[32]byte]*types.BCHBlock
}

var testValidatorPubkey1 = [32]byte{0x1}

//var testValidatorPubkey2 = [32]byte{0x2}
//var testValidatorPubkey3 = [32]byte{0x3}

func buildMockBCHNodeWithOnlyValidator1() *MockBCHNode {
	m := &MockBCHNode{
		height: 100,
		blocks: make([]*types.BCHBlock, 100),
	}
	for i := range m.blocks {
		m.blocks[i] = &types.BCHBlock{
			Height:      int64(i + 1),
			Timestamp:   int64(i * 10 * 60),
			HashId:      [32]byte{byte(i + 1)},
			ParentBlk:   [32]byte{byte(i)},
			Nominations: make([]stake.Nomination, 1),
		}
		m.blocks[i].Nominations[0] = stake.Nomination{
			Pubkey:         testValidatorPubkey1,
			NominatedCount: 1,
		}
	}
	return m
}

// block at height 99 forked
func buildMockBCHNodeWithReorg() *MockBCHNode {
	m := buildMockBCHNodeWithOnlyValidator1()
	m.blocks[m.height-1] = &types.BCHBlock{
		Height:      100,
		Timestamp:   99 * 10 * 60,
		HashId:      [32]byte{byte(100)},
		ParentBlk:   [32]byte{byte(199)},
		Nominations: make([]stake.Nomination, 1),
	}
	m.reorgBlocks = make(map[[32]byte]*types.BCHBlock)
	m.reorgBlocks[[32]byte{byte(199)}] = &types.BCHBlock{
		Height:      99,
		Timestamp:   98 * 10 * 60,
		HashId:      [32]byte{byte(199)},
		ParentBlk:   [32]byte{byte(98)},
		Nominations: make([]stake.Nomination, 1),
	}
	return m
}

type MockRpcClient struct {
	node *MockBCHNode
}

// nolint
func (m MockRpcClient) start() {
	go func() {
		time.Sleep(1 * time.Second)
		m.node.height++
	}()
}
func (m MockRpcClient) DoWatch() bool             { return true }
func (m MockRpcClient) Dial()                     {}
func (m MockRpcClient) Close()                    {}
func (m MockRpcClient) GetMainnetHeight() int64   { return m.node.height }
func (m MockRpcClient) NetworkSmartHeight() int64 { return 1 }
func (m MockRpcClient) GetBlockByHeight(height int64, retry bool) *types.BCHBlock {
	if height > m.node.height {
		return nil
	}
	return m.node.blocks[height-1]
}
func (m MockRpcClient) GetBlockByHash(hash [32]byte) *types.BCHBlock {
	height := int64(hash[0])
	if height > m.node.height {
		return m.node.reorgBlocks[hash]
	}
	return m.node.blocks[height-1]
}
func (m MockRpcClient) GetEpochs(start, end uint64) []*stake.Epoch {
	fmt.Printf("mock Rpc not support get Epoch")
	return nil
}
func (m MockRpcClient) GetCCEpochs(start, end uint64) []*cctypes.CCEpoch {
	fmt.Printf("mock Rpc not support get cc Epoch")
	return nil
}
func (m MockRpcClient) FetchCC(first, last int64) (cc *ccrpctypes.CCrpcEpoch) { return }
func (m MockRpcClient) IsConnected() bool                                     { return true }

var _ types.RpcClient = MockRpcClient{}
var defaults = param.DefaultConfig()

type MockEpochConsumer struct {
	w         *Watcher
	epochList []*stake.Epoch
}

// nolint
func (m *MockEpochConsumer) consume() {
	for {
		select {
		case e := <-m.w.EpochChan:
			m.epochList = append(m.epochList, e)
		}
	}
}

func TestRun1(t *testing.T) {
	defaults.AppConfig.Speedup = false
	w := NewWatcher(log.NewNopLogger(), 0, 0, 0, defaults,
		MockRpcClient{node: buildMockBCHNodeWithOnlyValidator1()})
	catchupChan := make(chan bool, 1)
	go w.WatcherMain(catchupChan)
	<-catchupChan
	time.Sleep(1 * time.Second)
	require.Equal(t, int(100/param.StakingNumBlocksInEpoch), len(w.epochList))
	require.Equal(t, 91, len(w.heightToFinalizedBlock))
	require.Equal(t, int64(91), w.latestFinalizedHeight)
}

func TestRunWithNewEpoch(t *testing.T) {
	defaults.AppConfig.Speedup = false
	w := NewWatcher(log.NewNopLogger(), 0, 0, 0, defaults,
		MockRpcClient{node: buildMockBCHNodeWithOnlyValidator1()})
	c := MockEpochConsumer{
		w: w,
	}
	numBlocksInEpoch := 10
	w.SetNumBlocksInEpoch(int64(numBlocksInEpoch))
	catchupChan := make(chan bool, 1)
	go w.WatcherMain(catchupChan)
	<-catchupChan
	go c.consume()
	time.Sleep(3 * time.Second)
	//test watcher clear
	//require.Equal(t, 6*int(WatcherNumBlocksInEpoch)-1+10 /*bch finalize block num*/, len(w.hashToBlock))
	require.Equal(t, 6*numBlocksInEpoch, len(w.heightToFinalizedBlock))
	require.Equal(t, 5, len(w.epochList))
	require.Equal(t, int64(91), w.latestFinalizedHeight)
	require.Equal(t, 9, len(c.epochList))
	for i, e := range c.epochList {
		require.Equal(t, int64(i*numBlocksInEpoch)+1, e.StartHeight)
	}
}

func TestRunWithFork(t *testing.T) {
	defaults.AppConfig.Speedup = false
	w := NewWatcher(log.NewNopLogger(), 0, 0, 0, defaults,
		MockRpcClient{node: buildMockBCHNodeWithReorg()})
	w.SetNumBlocksToClearMemory(100)
	w.SetNumBlocksInEpoch(1000)
	catchupChan := make(chan bool, 1)
	go w.WatcherMain(catchupChan)
	<-catchupChan
	time.Sleep(5 * time.Second)
	require.Equal(t, 0, len(w.epochList))
	require.Equal(t, 91, len(w.heightToFinalizedBlock))
	require.Equal(t, int64(91), w.latestFinalizedHeight)
}

func TestEpochSort(t *testing.T) {
	epoch := &stake.Epoch{
		Nominations: make([]*stake.Nomination, 100),
	}
	for i := 0; i < 100; i++ {
		epoch.Nominations[i] = &stake.Nomination{
			Pubkey:         [32]byte{byte(i)},
			NominatedCount: int64(i/5 + 1),
		}
	}
	sortEpochNominations(epoch)
	epoch.Nominations = epoch.Nominations[:30]
	i := 0
	for j := 1; i < 30 && j < 30; j++ {
		require.True(t, epoch.Nominations[i].NominatedCount > epoch.Nominations[j].NominatedCount ||
			(epoch.Nominations[i].NominatedCount == epoch.Nominations[j].NominatedCount &&
				bytes.Compare(epoch.Nominations[i].Pubkey[:], epoch.Nominations[j].Pubkey[:]) < 0))
		i++
	}
}
