package history

import (
	"sync"
)

var (
	IndexParallel int  = 16
	DBG_C         bool // DEBUG_CACHE_GROW_SHRINK
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
	HF_hash      string // = "hashdb/history.dat.hash[0-9a-f]"
	WriterChan   chan *HistoryObject
	IndexChan    chan *HistoryIndex
	IndexChans   [16]chan *HistoryIndex
	BatchLogs    BatchLOGGER
	BatchLocks   map[string]map[string]chan struct{}
	BoltDBsMap   map[string]*BOLTDB_PTR // using a ptr to a struct in the map allows updating the struct values without updating the map
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

type BatchLOGGER struct {
	mux sync.Mutex
	dat []*BatchLOG
}

type BatchLOG struct {
	char   *string
	bucket *string
	ins1   uint64
	took   int64 // microseconds
}
