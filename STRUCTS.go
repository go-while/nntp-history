package history

import (
	"sync"
)

var (
	DEBUG_CACHE_GROW_SHRINK bool = true
	DBG_C = DEBUG_CACHE_GROW_SHRINK
)

type HISTORY struct {
	mux          sync.Mutex
	cmux         sync.Mutex // sync counter
	L1Cache      L1CACHE
	L2Cache      L2CACHE
	L3Cache      L3CACHE
	boltInitChan chan struct{}
	boltSyncChan chan struct{}
	Offset       int64
	HF           string // = "history/history.dat"
	HF_hash      string // = "history/history.Hash"
	WriterChan   chan *HistoryObject
	IndexChan    chan *HistoryIndex
	IndexChans   [16]chan *HistoryIndex
	BatchLocks   map[string]map[string]chan struct{}
	charsMap     map[string]int
	useHashDB    bool
	keyalgo      int
	keylen       int
	win          bool
	Counter      map[string]uint64
	BatchQueues  *BQ
}

type BQ struct {
	mux    sync.Mutex
	Maps   map[string]map[string]chan *BatchOffset
	Booted chan struct{}
}

type HistoryObject struct {
	MessageIDHash *string
	StorageToken  *string // "F" = flatstorage | "M" = mongodb | "X" = deleted
	Char          *string
	Arrival       int64
	Expires       int64
	Date          int64
	ResponseChan  chan int // receives a 0,1,2 :: pass|duplicate|retrylater
}

/* builds the history.dat header */
type HistorySettings struct {
	KeyAlgo int
	KeyLen  int
}

type HistoryIndex struct {
	Hash         *string
	Char         *string
	Offset       int64
	IndexRetChan chan int
}

type BatchOffset struct {
	bucket            string
	key               string
	gobEncodedOffsets []byte
}

type GOBOFFSETS struct {
	Offsets []int64
}
