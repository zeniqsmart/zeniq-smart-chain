package ccrpc

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

func TestCCQueue(t *testing.T) {
	tdir := t.TempDir()
	// testdbdata is needed here
	fifo := NewFifo(tdir+"/dbdata", nil)
	fifo.Println(tdir)

	cce0 := &ccrpctypes.CCrpcEpoch{
		FirstHeight:       0,
		LastHeight:        2,
		EpochEndBlockTime: 99,
		EEBTSmartHeight:   0,
	}
	cce0.TransferInfos = make([]*ccrpctypes.CCrpcTransferInfo, 2)
	cce0.TransferInfos[0] = &ccrpctypes.CCrpcTransferInfo{
		Height:            1,
		TxID:              [32]byte{},
		Amount:            decimal.NewFromFloat(1.2),
		SenderPubkey:      [33]byte{},
		EpochEndHeight:    2,
		EpochEndBlockTime: 99,
		ApplicationHeight: 0,
		Receiver:          [20]byte{},
	}
	cce0.TransferInfos[1] = &ccrpctypes.CCrpcTransferInfo{
		Height:            2,
		TxID:              [32]byte{},
		Amount:            decimal.NewFromFloat(2.3),
		SenderPubkey:      [33]byte{},
		EpochEndHeight:    2,
		EpochEndBlockTime: 99,
		ApplicationHeight: 0,
		Receiver:          [20]byte{},
	}
	fifo.Come(cce0)

	cce1 := &ccrpctypes.CCrpcEpoch{
		FirstHeight:       0,
		LastHeight:        4, // changed
		EpochEndBlockTime: 99,
		EEBTSmartHeight:   0,
	}
	cce1.TransferInfos = make([]*ccrpctypes.CCrpcTransferInfo, 2)
	cce1.TransferInfos[0] = &ccrpctypes.CCrpcTransferInfo{
		Height:            3,
		TxID:              [32]byte{},
		Amount:            decimal.NewFromFloat(1.2),
		SenderPubkey:      [33]byte{},
		EpochEndHeight:    4,
		EpochEndBlockTime: 99,
		ApplicationHeight: 0,
		Receiver:          [20]byte{},
	}
	cce1.TransferInfos[1] = &ccrpctypes.CCrpcTransferInfo{
		Height:            3,
		TxID:              [32]byte{},
		Amount:            decimal.NewFromFloat(2.3),
		SenderPubkey:      [33]byte{},
		EpochEndHeight:    4,
		EpochEndBlockTime: 99,
		ApplicationHeight: 0,
		Receiver:          [20]byte{},
	}
	fifo.Come(cce1)

	processOne := func(cce *ccrpctypes.CCrpcEpoch) int64 {
		if cce.LastHeight == 2 {
			cce.EEBTSmartHeight = 3
			cce.TransferInfos[0].Receiver = common.Address{60, 5, 27, 151, 223, 91, 8, 119, 214, 176, 42, 138, 249, 160, 250, 11, 68, 165, 11, 252}
			cce.TransferInfos[0].ApplicationHeight = 3
			return 2
		}
		return 0
	}

	nextDoneMain := fifo.Serve(0, processOne)

	cci := fifo.CrosschainInfo(0, 4)
	require.Equal(t, 4, len(cci))
	assert.Equal(t, int64(3), cci[0].ApplicationHeight)
	assert.Equal(t, int64(2), nextDoneMain)

	assert.Equal(t, int64(4), fifo.LastFetched())

	cci4 := fifo.CrosschainInfo(3, 3)
	require.Equal(t, 2, len(cci4))

	fifo.Close()
}
