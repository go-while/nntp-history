package history

import (
	"os"
	"sync"
)

const (
	FlagSearch = -1
)

var (
	IndexParallel     int = NumCacheDBs
	NumQueueWriteChan int = NumCacheDBs
	HisDatWriteBuffer int = 4 * 1024
)

type HISTORY struct {
	/*   it is mostly NOT safe to change any values or read maps from outside! *
	 *   set, change, update values only inside (his *HISTORY) functions and
	 *   don't forget mutex where needed or run into race conditions.
	 */
	DIR            string     // path to folder: history/
	mux            sync.Mutex // global history mutex used to boot
	cmux           sync.Mutex // sync counter mutex
	boltmux        sync.Mutex // locks boltdb to protect BoltDBsMap
	L1Cache        L1CACHE
	L2Cache        L2CACHE
	L3Cache        L3CACHE
	Offset         int64                   // the actual offset for history.dat
	hisDat         string                  // = "history/history.dat"
	cutChar        int
	WriterChan     chan *HistoryObject     // history.dat writer channel
	IndexChan      chan *HistoryIndex      // main index query channel
	indexChans     [NumCacheDBs]chan *HistoryIndex // sub-index channels
	//BatchLogs      BatchLOGGER
	charsMap       map[string]int
	CutCharRO      int
	keyalgo        int
	keylen         int
	//win            bool
	Counter        map[string]uint64
	cacheEvicts    map[string]chan *ClearCache
	//adaptBatch     bool // AdaptiveBatchSize
	WBR            bool // WatchBoltRunning
	cEvCap         int  // cacheEvictsCapacity
	indexPar       int  // IndexParallel
	//cutFirst       int  // used to set startindex for cutHashlen
	reopenDBeveryN int  // reopens hashDB every N added key:vals (not batchins)
	wantReOpen     map[string]chan struct{}
	//rootBUCKETS    int
	CPUfile        *os.File // ptr to file for cpu profiling
	MEMfile        *os.File // ptr to file for mem profiling
	// TCPchan: used to send hobj via handleRConn to a remote historyServer
	TCPchan chan *HistoryObject
	ticker  map[string]chan struct{}
}

/* builds the history.dat header */
type HistorySettings struct {
	// constant values once DBs are initalized
	Ka int // keyalgo
	Kl int // keylen
	//Ki int // keyindex
	//Bp int // bucketsperdb
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

/* used to query the index */
type HistoryIndex struct {
	Hash         string
	Char         string   // first N chars of hash
	Offset       int64    // used to search: -1 or add: > 0 a hash
	IndexRetChan chan int // receives a 0,1,2 :: pass|duplicate|retrylater
}

type ClearCache struct {
	char   string // db
	hash   string // l1 key
	offset int64  // l2 key
	key    string // l3 key
}

type OffsetData struct {
	Shorthash string // first N chars of hash
	Offset int64
}

type SQLiteData struct {
	table string // first N chars of hash
	key string
	offsets []int64
}
