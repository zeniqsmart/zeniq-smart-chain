package ccrpc

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"os"
	"sync"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	tmlogger "github.com/tendermint/tendermint/libs/log"
	ccrpctypes "github.com/zeniqsmart/zeniq-smart-chain/ccrpc/types"
)

const ccrpcQueueLen = 65534

type FifoCcrpc interface {
	Come(*ccrpctypes.CCrpcEpoch)
	Serve(lastDoneMain int64, processOne func(cce *ccrpctypes.CCrpcEpoch) int64) int64
	LastFetched() int64
	CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo
	Println(s string)
	Close()
}

func (fifo *fifoImp) Come(cce *ccrpctypes.CCrpcEpoch) {
	fifo.mu.Lock()
	defer fifo.mu.Unlock()
	fifo.Println("Come")
	fifo.db.Set(fifo.makeKey(cce.LastHeight), fifo.makeValue(cce))
	fifo.Println("Come done")
}
func (fifo *fifoImp) makeKey(k int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(k))
	return b
}
func (fifo *fifoImp) makeValue(cce *ccrpctypes.CCrpcEpoch) []byte {
	j, err := json.Marshal(cce)
	if err != nil {
		panic("cannot marshal cce")
	}
	return j
}

func (fifo *fifoImp) unmarshal(iter dbm.Iterator) *ccrpctypes.CCrpcEpoch {
	var cce ccrpctypes.CCrpcEpoch = ccrpctypes.CCrpcEpoch{}
	val := iter.Value()
	err := json.Unmarshal(val, &cce)
	if err != nil {
		panic("Unmarshal CCrpcEpoch for crosschain failed")
	}
	return &cce
}

func (fifo *fifoImp) Serve(lastDoneMain int64, processOne func(cce *ccrpctypes.CCrpcEpoch) int64) int64 {
	fifo.mu.RLock()
	defer fifo.mu.RUnlock()
	fifo.Println("Serve")
	var update = make(map[int64][]byte)
	iter, err := fifo.db.Iterator(fifo.makeKey(lastDoneMain+1), nil)
	if err != nil {
		panic("cannot create iterator for crosschain")
	}
	for ; iter.Valid(); iter.Next() {
		cce := fifo.unmarshal(iter)
		lastSmart := cce.EEBTSmartHeight
		var nxt int64 = 0
		if nxt = processOne(cce); nxt > lastDoneMain {
			lastDoneMain = nxt
		}
		if lastSmart != cce.EEBTSmartHeight { // also will store ApplicationHeight and Receiver
			update[cce.LastHeight] = fifo.makeValue(cce)
		}
		if nxt == 0 {
			break
		}
	}
	iter.Close()
	for k, v := range update {
		fifo.db.Set(fifo.makeKey(k), v)
	}
	fifo.Println("Serve done")
	return lastDoneMain
}

func (fifo *fifoImp) LastFetched() (res int64) {
	fifo.mu.RLock()
	defer fifo.mu.RUnlock()
	fifo.Println("LastFetched")
	riter, err := fifo.db.ReverseIterator(nil, nil)
	if err != nil {
		panic("cannot create reverse iterator for crosschain")
	}
	if riter.Valid() {
		res = int64(binary.BigEndian.Uint64(riter.Key()))
	} else {
		res = 0
	}
	fifo.Println("LastFetched done")
	return
}

func (fifo *fifoImp) CrosschainInfo(start, end int64) []*ccrpctypes.CCrpcTransferInfo {
	fifo.mu.RLock()
	defer fifo.mu.RUnlock()
	ccinfo := make([]*ccrpctypes.CCrpcTransferInfo, 0)
	iter, err := fifo.db.Iterator(nil, nil)
	if err != nil {
		panic("cannot create iterator for CrosschainInfo")
	}
	for ; iter.Valid(); iter.Next() {
		cce := fifo.unmarshal(iter)
		for _, info := range cce.TransferInfos {
			if info.Height >= start && info.Height <= end {
				info.Receiver = ZeniqPubkeyToReceiverAddress(info.SenderPubkey)
				ccinfo = append(ccinfo, info)
			}
		}
	}
	iter.Close()
	return ccinfo
}

func (fifo *fifoImp) Println(s string) {
	if fifo.flog != nil {
		fifo.flog.Println(s)
	}
	if fifo.tmlog != nil {
		fifo.tmlog.Debug(s)
	}
}

func newFileLogger() *log.Logger {
	f, err := os.OpenFile("testlogdbdata", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	flog := log.New(f, "", log.LstdFlags|log.Lshortfile)
	flog.SetOutput(f)
	return flog
}

func NewFifo(dir string, tmlog tmlogger.Logger) FifoCcrpc {
	db, err := dbm.NewDB("cc", "goleveldb", dir)
	if err != nil {
		panic("cannot create db for cross chain")
	}

	var flog *log.Logger = nil
	if tmlog == nil {
		flog = newFileLogger()
	}
	fi := &fifoImp{
		ccrpcEpochChan: make(chan *ccrpctypes.CCrpcEpoch, ccrpcQueueLen),
		db:             db,
		mu:             sync.RWMutex{},
		flog:           flog,
		tmlog:          tmlog,
	}

	return fi
}

func (fifo *fifoImp) Close() {
	fifo.db.Close()
}

type fifoImp struct {
	ccrpcEpochChan chan *ccrpctypes.CCrpcEpoch
	db             dbm.DB
	mu             sync.RWMutex
	flog           *log.Logger
	tmlog          tmlogger.Logger
}

func ZeniqPubkeyToReceiverAddress(pubkeyBytes [33]byte) common.Address {
	publicKeyECDSA, _ := crypto.DecompressPubkey(pubkeyBytes[:])
	return crypto.PubkeyToAddress(*publicKeyECDSA)
}
