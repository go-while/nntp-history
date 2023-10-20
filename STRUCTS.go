package history

import (
	bolt "go.etcd.io/bbolt"
	"os"
	"sync"
)

const (
	FlagSearch = -1
)

var (
	IndexParallel     int = intBoltDBs
	NumQueueWriteChan int = intBoltDBs
	HisDatWriteBuffer int = 4 * 1024
)

type HISTORY struct {
	/*   it is mostly NOT safe to change any values or read maps from outside! *
	 *   set, change, update values only inside (his *HISTORY) functions and
	 *   don't forget mutex where needed or run into race conditions.
	 */
	mux            sync.Mutex // global history mutex used to boot
	cmux           sync.Mutex // sync counter mutex
	boltmux        sync.Mutex // locks boltdb to protect BoltDBsMap
	L1Cache        L1CACHE
	L2Cache        L2CACHE
	L3Cache        L3CACHE
	boltInitChan   chan struct{}          // used to lock bolt booting to N in parallel
	boltSyncChan   chan struct{}          // used to lock bolt syncing to N in parallel
	Offset         int64                  // the actual offset for history.dat
	hisDat         string                 // = "history/history.dat"
	hisDatDB       string                 // = "hashdb/history.dat.hash[0-9a-f]"
	WriterChan     chan *HistoryObject    // history.dat writer channel
	IndexChan      chan *HistoryIndex     // main index query channel
	indexChans     [16]chan *HistoryIndex // sub-index channels
	BatchLogs      BatchLOGGER
	BatchLocks     map[string]*BATCHLOCKS // used to lock char:bucket in BoltSync and boltBucketPutBatch
	BoltDBsMap     *BoltDBs               // using a ptr to a struct in the map allows updating the struct values without updating the map
	charsMap       map[string]int
	useHashDB      bool
	keyalgo        int
	keylen         int
	win            bool
	Counter        map[string]uint64
	batchQueues    *BQ
	cacheEvicts    map[string]chan *ClearCache
	adaptBatch     bool // AdaptiveBatchSize
	WBR            bool // WatchBoltRunning
	cEvCap         int  // cacheEvictsCapacity
	wCBBS          int  // CharBucketBatchSize
	indexPar       int  // IndexParallel
	cutFirst       int  // used to set startindex for cutHashlen
	reopenDBeveryN int  // reopens boltDB every N added key:vals (not batchins)
	rootBUCKETS    int
	keyIndex       int
	CPUfile        *os.File // ptr to file for cpu profiling
	MEMfile        *os.File // ptr to file for mem profiling
	// TCPchan: used to send hobj via handleRConn to a remote historyServer
	TCPchan chan *HistoryObject
}

/* builds the history.dat header */
type HistorySettings struct {
	Ka int // keyalgo
	Kl int // keylen
	Ki int // keyindex
	Bp int // bucketsperdb
}

type HistoryObject struct {
	MessageIDHash string
	StorageToken  string // "F" = flatstorage | "M" = mongodb | "X" = deleted
	Char          string
	Arrival       int64
	Expires       int64
	Date          int64
	ResponseChan  chan int // receives a 0,1,2 :: pass|duplicate|retrylater
}

type BoltDBs struct {
	mux   sync.Mutex
	dbptr map[string]*BOLTDB_PTR
}

type BOLTDB_PTR struct {
	BoltDB *bolt.DB
	mux    sync.Mutex
}

/* used to query the index */
type HistoryIndex struct {
	Hash         string
	Char         string
	Offset       int64
	IndexRetChan chan int
}

/* BatchQueue */
type BQ struct {
	mux    sync.Mutex
	lock   chan struct{}
	Maps   map[string]map[string]chan *BatchOffset
	BootCh chan struct{}
}

/* used to batch write items to boltDB */
type BatchOffset struct {
	bucket         string
	key            string
	encodedOffsets []byte  // gob encoded offsets for this key
	hash           string  // for cache eviction
	char           string  // for cache eviction
	offsets        []int64 // stored for this key
}

type BatchLOGGER struct {
	mux sync.Mutex
	dat []*BatchLOG
	did uint64 // counter
}

type BatchLOG struct {
	c string // char
	b string // bucket
	i uint64 // inserted
	t int64  // took microseconds
	w int    // workerCharBucketBatchSize
}

type BATCHLOCKS struct {
	bl map[string]*BLCH
}

// BATCHLOCKCHAN
type BLCH struct {
	ch chan struct{}
}

type ClearCache struct {
	char   string // db
	hash   string // l1 key
	offset int64  // l2 key
	key    string // l3 key
}
