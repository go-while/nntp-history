package history

import (
	//"encoding/gob"
	"sync"
)

var (
	IndexParallel int = 16
)

type HISTORY struct {
	mux          sync.Mutex
	cmux         sync.Mutex // sync counter
	boltmux      sync.Mutex // locks boltdb to protect BoltDBsMap
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
	//GobDecoder   map[string]GOBDEC
	//GobEncoder   map[string]GOBENC
	charsMap    map[string]int
	useHashDB   bool
	keyalgo     int
	keylen      int
	win         bool
	Counter     map[string]uint64
	BatchQueues *BQ
	CacheEvicts map[string]chan *ClearCache
}

/* builds the history.dat header */
type HistorySettings struct {
	KeyAlgo int
	KeyLen  int
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

/* used to query the index */
type HistoryIndex struct {
	Hash         *string
	Char         *string
	Offset       int64
	IndexRetChan chan int
}

/* BatchQueue */
type BQ struct {
	mux    sync.Mutex
	Maps   map[string]map[string]chan *BatchOffset
	Booted chan struct{}
}

/* used to batch write items to boltDB */
type BatchOffset struct {
	bucket            *string
	key               *string
	gobEncodedOffsets *[]byte
	hash              *string // for cache eviction
	char              *string // for cache eviction
	offsets           *[]int64
}

/*
type GOBOFFSETS struct {
	Offsets []int64
}
*/

type BatchLOGGER struct {
	mux sync.Mutex
	dat []*BatchLOG
	did uint64 // counter
}

type BatchLOG struct {
	c *string // char
	b *string // bucket
	i uint64  // inserted
	t int64   // took microseconds
	w int     // workerCharBucketBatchSize
}

type ClearCache struct {
	char   *string // db
	hash   *string // l1 key
	offset *int64  // l2 key
	key    *string // l3 key
}
