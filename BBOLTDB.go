package history

import (
	//"bytes"
	"encoding/gob"
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

const (
	// never change this!
	BoltDBs                 int = 16
	DefaultBoltINITParallel     = BoltDBs
	DefaultBoltSYNCParallel     = BoltDBs

	HashShort  = 0x0B // 11
	HashFNV32  = 0x16 // 22
	HashFNV32a = 0x21 // 33
	HashFNV64  = 0x2c // 44
	HashFNV64a = 0x37 // 55
	// KeyLen is used with HashShort
	//  1st char of hash selects boltDB
	//  2nd char of hash selects bucket in boltDB
	//  remaining chars [3:$] are used as Key in BoltDB to store offset(s)
	//  offsets lead into history.dat and point to start of a line containing the full hash
	MinKeyLen = 0x04
)

var (
	NumQueueIndexChan    int     = 16                           // Main-indexchan can queue this
	NumQueueindexChans   int     = 16                           // every sub-indexchans for a `char` can queue this
	BoltDB_AllocSize     int                                    // if not set defaults: 16 * 1024 * 1024 (min: 1024*1024)
	BoltSyncEveryS       int64   = 5                            // call db.sync() every seconds (only used with 'boltopts.NoSync: true')
	BoltSyncEveryN       uint64  = 100                          // call db.sync() after N inserts (only used with 'boltopts.NoSync = true')
	BoltINITParallel     int     = DefaultBoltINITParallel      // set this via 'history.BoltINITParallel = 1' before calling History_Boot.
	BoltSYNCParallel     int     = DefaultBoltSYNCParallel      // set this via 'history.BoltSYNCParallel = 1' before calling History_Boot.
	BoltHashOpen                 = make(chan struct{}, BoltDBs) // dont change this
	HISTORY_INDEX_LOCK           = make(chan struct{}, 1)       // main lock
	HISTORY_INDEX_LOCK16         = make(chan struct{}, BoltDBs) // sub locks
	empty_offsets        []int64                                // just nothing
)

type BOLTDB_PTR struct {
	BoltDB *bolt.DB
}

// boltDB_Init initializes the history database (HashDB) and starts the worker goroutines for processing historical data.
// It creates worker channels for each character in HEXCHARS and launches corresponding worker goroutines.
// The provided boltOpts parameter allows configuring the BoltDB database options.
func (his *HISTORY) boltDB_Init(boltOpts *bolt.Options) {
	//gob.Register(GOBOFFSETS{})
	gob.Register(HistorySettings{})

	his.L2Cache.L2CACHE_Boot(his)
	his.L3Cache.L3CACHE_Boot(his)

	his.batchQueues = &BQ{}
	his.batchQueues.Booted = make(chan struct{}, 16*16)                  // char [0-9a-f] * bucket [0-9a-f]
	his.batchQueues.Maps = make(map[string]map[string]chan *BatchOffset) // maps char : bucket => chan
	his.BoltDBsMap = make(map[string]*BOLTDB_PTR)                        // maps char => boltDB pointer
	//his.GobDecoder = make(map[string]GOBDEC)
	//his.GobEncoder = make(map[string]GOBENC)
	for _, char := range HEXCHARS {
		his.batchQueues.Maps[char] = make(map[string]chan *BatchOffset) // maps bucket => chan
		his.BoltDBsMap[char] = &BOLTDB_PTR{BoltDB: nil}                 // pointer to boltDB
		// TODO try reuse gob de/en-coder
		//his.GobDecoder[char] = GOBDEC{GobDecoder: gob.NewDecoder(bytes.NewBuffer(nil))}
		//his.GobEncoder[char] = GOBENC{GobEncoder: gob.NewEncoder(bytes.NewBuffer(nil))}
	}
	his.IndexChan = make(chan *HistoryIndex, NumQueueIndexChan) // main index chan to query the index
	his.charsMap = make(map[string]int, BoltDBs)                // maps char from HEXCHARS to i

	//log.Printf("HEX %x %x %x %x %x %x", HashShort, HashFNV32, HashFNV32a, HashFNV64, HashFNV64a, MinKeyLen)
	//time.Sleep(time.Second * 10)
	if his.boltInitChan != nil {
		log.Printf("ERROR boltDB_Init already loaded")
		return
	}

	if BoltINITParallel == 0 {
		BoltINITParallel = 1
	} else if BoltINITParallel < 0 || BoltINITParallel > BoltDBs {
		BoltINITParallel = BoltDBs
	}

	if BoltSYNCParallel == 0 {
		BoltSYNCParallel = 1
	} else if BoltSYNCParallel < 0 || BoltSYNCParallel > BoltDBs {
		BoltSYNCParallel = BoltDBs
	}

	if NumQueueIndexChan <= 0 {
		NumQueueIndexChan = 1
	} else if NumQueueIndexChan > 1024*1024 {
		NumQueueIndexChan = 1024 * 1024
	}

	if NumQueueindexChans <= 0 {
		NumQueueindexChans = 1
	} else if NumQueueindexChans > 1024*1024 {
		NumQueueindexChans = 1024 * 1024
	}
	if DefaultEvictsCapacity < 4 {
		DefaultEvictsCapacity = 4
	}
	his.cEvCap = DefaultEvictsCapacity
	his.adaptBatch = AdaptiveBatchSizeON
	his.boltInitChan = make(chan struct{}, BoltINITParallel)
	his.boltSyncChan = make(chan struct{}, BoltSYNCParallel)
	his.BatchLocks = make(map[string]map[string]chan struct{})
	for i, char := range HEXCHARS {
		his.charsMap[char] = i
		his.indexChans[i] = make(chan *HistoryIndex, NumQueueindexChans)
		his.BatchLocks[char] = make(map[string]chan struct{})
		for _, bucket := range HEXCHARS {
			his.BatchLocks[char][bucket] = make(chan struct{}, 1)
		}
	}
	for i, char := range HEXCHARS { // dont move this up into the first for loop or it drops race conditions for nothing...
		go his.boltDB_Worker(char, i, his.indexChans[i], boltOpts)
	}
	logf(DEBUG, "his.boltDB_Init() AdaptiveBatchSizeON=%t", AdaptiveBatchSizeON)
	if DEBUG {
		// run manually: go history.History.WatchBolt()
		go his.WatchBolt()
	}
	go his.boltDB_Index()
} // end func boltDB_Init

// boltDB_Index listens to incoming HistoryIndex structs on the IndexChan channel and distributes them to corresponding worker goroutines.
func (his *HISTORY) boltDB_Index() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "boltDB_Index") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "boltDB_Index")
	his.wait4HashDB()
	logf(DEBUG2, "Boot boltDB_Index")
	defer logf(DEBUG2, "Quit boltDB_Index")
	waitchan := make(chan struct{}, IndexParallel)
	for p := 1; p <= IndexParallel; p++ {
		waitchan <- struct{}{}
		go func(p int, waitchan chan struct{}) {
		forever:
			for {
				select {
				case hi, ok := <-his.IndexChan: // recevies a HistoryIndex struct and passes it down to '0-9a-f' workers
					if !ok {
						logf(DEBUG2, "Stopping boltDB_Index IndexChan closed")
						break forever
					}
					if hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // allow at least md5
						switch IndexParallel {
						case 1:
							close(his.IndexChan)
						default:
							his.IndexChan <- nil
						}
						logf(DEBUG2, "Stopping boltDB_Index IndexChan p=%d/%d received nil pointer", p, IndexParallel)
						break forever
					}
					if hi.Offset == 0 {
						log.Printf("ERROR boltDB_Index offset=0") // must: Offset -1 to checkonly OR Offset > 0 adds to hashDB
						break forever
					}
					C1 := ""
					if hi.Char != nil && len(*hi.Char) == 1 {
						C1 = *hi.Char
					} else {
						// gets first char of hash: hash must be lowercase!
						// hex.EncodeToString returns a lowercased string of a hashsum
						C1 = strings.ToLower(string(string(*hi.Hash)[0]))
					}
					if his.indexChans[his.charsMap[C1]] != nil {
						his.indexChans[his.charsMap[C1]] <- hi // sends object to hash boltDB_Worker char
					} else {
						log.Printf("ERROR boltDB_Index IndexChan C1=%s=nil", C1)
						break forever
					}
				} // end select
			} // end for
			<-waitchan
		}(p, waitchan) // end go func
	} // end for p
	for {
		time.Sleep(time.Second)
		if len(waitchan) == 0 {
			break
		}
	}
	for _, achan := range his.indexChans {
		// passing nils to indexChans will stop boltDB_Worker
		achan <- nil
	}
} // end func boltDB_Index

// boltDB_Worker is a worker function responsible for processing historical data.
// It manages BoltDB operations, including storing and retrieving offsets, and handles duplicate checks
// to ensure message-ID hashes are correctly tracked in the history file.
func (his *HISTORY) boltDB_Worker(char string, i int, indexchan chan *HistoryIndex, boltOpts *bolt.Options) {
	if !LOCKfunc(HISTORY_INDEX_LOCK16, "boltDB_Worker "+char) {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK16, "boltDB_Worker "+char)
	dbpath := his.hisDatDB + "." + char
	if boltOpts == nil {
		defboltOpts := bolt.Options{
			Timeout:         9 * time.Second,
			InitialMmapSize: 1024 * 1024 * 1024,
			PageSize:        64 * 1024,
		}
		boltOpts = &defboltOpts
	}
	db, err := bolt.Open(dbpath, 0600, boltOpts)
	if err != nil {
		log.Printf("ERROR HashDB dbpath='%s' err='%v'", dbpath, err)
		return
	}
	if BoltDB_MaxBatchSize > 0 {
		db.MaxBatchSize = BoltDB_MaxBatchSize
	}
	if BoltDB_MaxBatchDelay > 0 {
		db.MaxBatchDelay = BoltDB_MaxBatchDelay
	}
	if BoltDB_AllocSize >= 1024*1024 {
		db.AllocSize = BoltDB_AllocSize
	}
	logf(DEBUG2, "HDBZW: INIT HashDB [%s] db='%#v' db.MaxBatchSize=%d db.MaxBatchDelay=%d db.AllocSize=%d", char, db, db.MaxBatchSize, db.MaxBatchDelay, db.AllocSize)
	his.boltmux.Lock()
	his.BoltDBsMap[char].BoltDB = db
	his.boltmux.Unlock()
	tocheck, checked, created := 16, 0, 0

	his.boltInitChan <- struct{}{} // locks parallel intializing of boltDBs
	for _, bucket := range HEXCHARS {
		retbool, err := boltCreateBucket(db, &char, &bucket)
		if err != nil || !retbool {
			if err == bolt.ErrBucketExists {
				checked++
			} else {
				log.Printf("ERROR HDBZW INIT HashDB boltCreateBucket [%s|%s] err='%v' retbool=%t", char, bucket, err, retbool)
				return
			}
		} else if retbool {
			created++ // <= bucket has been created
		}
		//log.Printf("HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tcheck)
	} // end for c1
	<-his.boltInitChan

	logf(DEBUG0, "HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
	//if checked != 4096 {
	if checked != tocheck && created != tocheck {
		log.Printf("ERROR HDBZW INIT [%s] checked %d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
		return
	}
	his.setBoltHashOpen()

	timeout := 90
	for {
		if timeout <= 0 {
			log.Printf("ERROR HDBZW [%s] open his.hisDat not found?!", char)
			return
		}
		if utils.FileExists(his.hisDat) {
			break
		}
		time.Sleep(time.Second / 10)
		timeout--
	}

	lastsync := utils.UnixTimeSec()
	var added, passed, processed, dupes, searches, retry, countsearches uint64
	cutHashlen := 8 // 2:8 = 6 chars
	if his.keyalgo == HashShort {
		cutHashlen = 2 + his.keylen
	}

	// char worker opens history.dat for Fseeking
	// allows this char worker to search for offsets in history.dat
	historyfile, err := os.OpenFile(his.hisDat, os.O_RDONLY, 0666)
	if err != nil {
		log.Printf("ERROR HDBZW [%s] os.OpenFile his.hisDat err='%v'", err, char)
		return
	}

	// make the batchQueue
	for _, bucket := range HEXCHARS {
		// The batchQueue, like a ravenous dragon, gorges itself on memory, holding twofold the might of the actual CharBucketBatchSize.
		// A daring gamble that ignites the fires of performance, but beware the voracious appetite!
		batchQcap := CharBucketBatchSize * 2
		batchQueue := make(chan *BatchOffset, batchQcap)
		his.batchQueues.mux.Lock()
		his.batchQueues.Maps[char][bucket] = batchQueue
		his.batchQueues.mux.Unlock()
		// Lo Wang unleashes a legion of batch queues, one for each sacred bucket in this 'char' database.
		// It results in a total of 16 by 16 queues, as the CharBucketBatchSize stands resolute, guarding each [char][bucket] with its mighty power!
		go func(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset) {
			if batchQueue == nil {
				log.Printf("ERROR boltDB_Worker batchQueue=nil")
				return
			}
			// every batchQueue adds an empty struct to count Booted. results in 16*16 queues.
			wCBBS := CharBucketBatchSize // copy value allows each worker to play with it
			if wCBBS < 0 {
				wCBBS = 1
			}
			if !LOCKfunc(his.batchQueues.Booted, "his.batchQueues.Booted") {
				return
			}
			lastflush := utils.UnixTimeMilliSec()
			var forced, closed, cont bool
			var inserted, conti, slept uint64
			var err error
			var timer int64 = 500
			var decr, incr int = 8, 8 // adaptive batchsize
			Q := 0
		forbatchqueue:
			for {
				//if timer > 0 {
				if !forced {
					time.Sleep(time.Duration(timer) * time.Millisecond)
					slept++
				}
				lft := utils.UnixTimeMilliSec() - lastflush
				//src := fmt.Sprintf("gofunc:[%s|%s]:t=%d Q=%d lft=%d", char, bucket, timer, Q, lft)
				//src := fmt.Sprintf("gofunc:[%s|%s] Q=%d lft=%d", char, bucket, timer, Q, lft)
				inserted, err, closed = his.boltBucketPutBatch(db, char, bucket, batchQueue, forced, "gofunc", true, lft, wCBBS)
				//logf(DEBUG2, "INFO forbatchqueue [%s|%s] boltBucketPutBatch inserted=%d err='%v' closed=%t forced=%t Q=%d t=%d", char, bucket, inserted, err, closed, forced, Q, timer)

				if closed { // received nil pointer
					logf(DEBUG2, "Closed gofunc [%s|%s]", char, bucket)
					break forbatchqueue // this go func
				}
				if err != nil {
					log.Printf("gofunc char=%s boltBucketPutBatch err='%v'", char, err)
					break forbatchqueue // this go func
				}
				//time.Sleep(time.Nanosecond)
				Q = len(batchQueue) // get remaining queued items after inserting

				if inserted > 0 {
					// something got inserted
					lastflush = utils.UnixTimeMilliSec()
					cont = true
					if his.adaptBatch {
						// try continuesly to adapt wCBBS to match `BatchFlushEvery`

						if int(inserted) >= wCBBS {
							// inserted exactly or more than wCBBS: increase wCBBS
							if wCBBS < 65536+incr {
								wCBBS += incr // adaptive BatchSize increase
								logf(DBG_FBQ2, "forbatchqueue incr [%s|%s] Queue=%05d Ins=%05d wCBBS=%05d lft=%04d f=%d", char, bucket, Q, inserted, wCBBS, lft, bool2int(forced))
							}
						} else {
							// inserted less than wCBBS: decrease wCBBS
							if wCBBS > 1+decr {
								wCBBS -= decr // adaptive BatchSize decrease
								logf(DBG_FBQ2, "forbatchqueue decr [%s|%s] Queue=%05d Ins=%05d wCBBS=%05d lft=%04d f=%d", char, bucket, Q, inserted, wCBBS, lft, bool2int(forced))
							}
						}

					} // end if forced
				} // end if inserted > 0

				if forced {
					forced = false
				}
				if cont {
					conti++
					cont = false
					continue forbatchqueue
				}

				if Q > 0 && lastflush <= utils.UnixTimeMilliSec()-BatchFlushEvery {
					//logf(DEBUG, "forbatchqueue F1 [%s|%s] timer=%d Q=%d forced=%t=>true lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS)
					forced = true
				}

				continue forbatchqueue
			} // end forbatchqueue
			UNLOCKfunc(his.batchQueues.Booted, "his.batchQueues.Booted")
			his.Sync_upcounterN("wCBBS", uint64(wCBBS))
			his.Sync_upcounterN("wCBBSconti", conti)
			his.Sync_upcounterN("wCBBSslept", slept)
			//log.Printf("forbatchqueue [%s|%s] continued=%d slept=%d", char, bucket, continued, slept)
			// ends this gofunc
		}(db, char, bucket, batchQueue)
	} // end for hexchars

	// wait for batchqueues to boot
	BQtimeout := 1000
	for {
		time.Sleep(time.Millisecond)
		if len(his.batchQueues.Booted) == 16*16 {
			break
		}
		BQtimeout--
		if BQtimeout <= 0 {
			log.Printf("ERROR boltDB_Worker char=%s batchQueues.Boot timeout", char)
			return
		}
	}
	//log.Printf("BOOTED boltDB_Worker char=%s (took %d ms)", char, 1000-BQtimeout)

forever:
	for {
		select {
		case hi, ok := <-indexchan: // sub-indexchan receives a HistoryIndex struct for this char from main IndexChan
			if !ok || hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // at least md5
				// receiving a nil object stops history_dbz_worker
				logf(DEBUG9, "Stopping boltDB_Worker indexchan[%s] received nil pointer", char)
				for _, bucket := range HEXCHARS {
					his.batchQueues.Maps[char][bucket] <- nil
				}
				break forever
			}
			var key *string
			bucket := string(string(*hi.Hash)[1]) // get 1 char for bucket

			switch his.keyalgo {
			case HashShort:
				max := len(*hi.Hash)
				if cutHashlen > max {
					cutHashlen = max
				}
				shorthash := string(string(*hi.Hash)[2:cutHashlen])
				key = &shorthash
			case HashFNV32:
				key = FNV32S(hi.Hash)
			case HashFNV32a:
				key = FNV32aS(hi.Hash)
			case HashFNV64:
				key = FNV64S(hi.Hash)
			case HashFNV64a:
				key = FNV64aS(hi.Hash)
			}
			if hi.Offset > 0 {
				logf(DEBUG2, "WORKER HDBZW [%s|%s] key=%s hash=%s @0x%010x|%d|%x", char, bucket, *key, *hi.Hash, hi.Offset, hi.Offset, hi.Offset)
			}
			isDup, err := his.DupeCheck(db, char, bucket, *key, *hi.Hash, hi.Offset, false, historyfile, his.batchQueues.Maps[char][bucket])
			if err != nil {
				if err != io.EOF {
					if hi.IndexRetChan != nil {
						log.Printf("ERROR HDBZW his.DupeCheck err='%v' close(hi.IndexRetChan)", err)
						close(hi.IndexRetChan)
					} else {
						log.Printf("ERROR HDBZW char=%s DupeCheck err='%v'", char, err)
					}
					break forever
				}
				// dupecheck got EOF from history file
			}

			if hi.IndexRetChan != nil {
				hi.IndexRetChan <- isDup
			}
			if hi.Offset == -1 {
				searches++
				countsearches++
				if countsearches >= 1000 {
					his.Sync_upcounterN("searches", countsearches)
					countsearches = 0
				}
				continue forever
			} else if hi.Offset > 0 {
				processed++
			}

			switch isDup {
			case CaseAdded:
				added++
			//case CasePass:
			//	log.Printf("ERROR HDBZW DupeCheck invalid response isDup=CasePass [%s|%s] key=%s hash=%s @0x%010x|%d|%x", char, bucket, *key, *hi.Hash, hi.Offset)
			//	passed++
			case CaseDupes:
				dupes++
			case CaseRetry:
				retry++
			default:
				log.Printf("ERROR HDBZW DupeCheck returned unknown switch isDup=%d", isDup)
			}
			if boltOpts.NoSync == true {
				if added >= BoltSyncEveryN || (added > 0 && lastsync <= utils.UnixTimeSec()-BoltSyncEveryS) {
					if err := his.BoltSync(db, char); err != nil {
						break forever
					}
					added, lastsync = 0, utils.UnixTimeSec()
				}
			}
		} // end select
	} // end for
	for _, bucket := range HEXCHARS {
		his.batchQueues.Maps[char][bucket] <- nil
		close(his.batchQueues.Maps[char][bucket])
	}
	for _, bucket := range HEXCHARS {
		logf(DEBUG2, "FINAL-BATCH HDBZW [%s|%s]", char, bucket)
		his.boltBucketPutBatch(db, char, bucket, his.batchQueues.Maps[char][bucket], true, fmt.Sprintf("defer:[%s|%s]", char, bucket), false, -1, -1)
	}
	logf(DEBUG2, "Quit HDBZW char=%s added=%d passed=%d dupes=%d processed=%d searches=%d retry=%d", char, added, passed, dupes, processed, searches, retry)
	//his.Sync_upcounterN("searches", searches)
	historyfile.Close()
	his.boltSyncClose(db, char)
	time.Sleep(time.Second / 10)
	his.returnBoltHashOpen()
} // end func boltDB_Worker

// DupeCheck checks for duplicate message-ID hashes in a BoltDB bucket.
// It manages offsets associated with message hashes and handles duplicates, ensuring the integrity of the historical data.
// If a hash is a duplicate, it returns 1, otherwise, it returns 0.
// It also handles the creation of new hash entries in the bucket when needed.
func (his *HISTORY) DupeCheck(db *bolt.DB, char string, bucket string, key string, hash string, offset int64, setempty bool, file *os.File, batchQueue chan *BatchOffset) (int, error) {
	// return -999 with err only
	if db == nil {
		return -999, fmt.Errorf("ERROR DupeCheck db=nil")
	}
	/*
		if char == nil {
			return -999, fmt.Errorf("ERROR DupeCheck char=nil")
		}
		if bucket == nil {
			return -999, fmt.Errorf("ERROR DupeCheck char=%s bucket=nil", *char)
		}
		if key == nil {
			return -999, fmt.Errorf("ERROR DupeCheck [%s|%s] key=nil", *char, *bucket)
		}
		if hash == nil {
			return -999, fmt.Errorf("ERROR DupeCheck [%s|%s] key=%s hash=nil", *char, *bucket, *key)
		}
		if offset == nil {
			return -999, fmt.Errorf("ERROR DupeCheck [%s|%s] key=%s hash=%s offset=nil", *char, *bucket, *key, *hash)
		}
	*/
	offsets, err := his.boltBucketGetOffsets(db, char, bucket, key)
	if err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets [%s|%s] key=%s hash='%s' err='%v'", char, bucket, key, hash, err)
		return -999, err
	}
	len_offsets := 0
	if offsets != nil {
		len_offsets = len(*offsets)
	}
	if offsets == nil || (offsets != nil && len_offsets == 0) { // no offsets stored for numhash
		if offset == -1 { // search only
			return CasePass, nil // pass, not a duplicate
		}
		newoffsets := []int64{offset}
		// add hash=>key:offset to db
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, &newoffsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW DupeCheck char=%s Add boltBucketKeyPutOffsets bucket=%s err='%v'", char, bucket, err)
			return -999, err
		}
		//logf(DEBUG2, "HDBZW char=%s DupeCheck CREATED key=%s hash=%s offset=0x%08x=%d", *char, *key, *hash, *offset, *offset)
		his.Sync_upcounter("key_add")
		return CaseAdded, nil
	}

	// got offset(s) stored for numhash
	if len_offsets > 0 {
		if offset > 0 { // is not a search
			logf(DEBUG2, "INFO HDBZW char=%s key=%s tryhash='%s' GOT multiple offsets=%d=%#v +offset=%d", char, key, hash, len_offsets, offsets, offset)
			if len_offsets > 0 {
				his.Sync_upcounter("appoffset")
			} else {
				his.Sync_upcounter("addoffset")
			}
		} else {
			// is a search
			if len_offsets > 1 {
				his.Sync_upcounter("trymultioffsets")
			} else {
				his.Sync_upcounter("tryoffset")
			}
		}
	}
	for _, check_offset := range *offsets {
		// check history for duplicate hash / evades collissions
		logf(DEBUG2, "HDBZW char=%s CHECK DUP key=%s lo=%d offset=%d", char, key, len_offsets, check_offset)
		historyHash, err := his.FseekHistoryMessageHash(file, check_offset)
		if historyHash == nil && err == nil {
			log.Printf("ERROR HDBZW char=%s CHECK DUP bucket=%s historyHash=nil err=nil hash=%s", char, bucket, err, hash)
			return -999, fmt.Errorf("ERROR historyHash=nil err=nil @offset=%d +offset=%d", *historyHash, check_offset, offset)
		}
		if err != nil {
			log.Printf("ERROR HDBZW char=%s FseekHistoryMessageHash bucket=%s err='%v' offset=%d", char, bucket, err, check_offset)
			return -999, err
		}
		if historyHash != nil {

			if len(*historyHash) == 3 && *historyHash == eofhash {
				log.Printf("EOF history.dat offset=%d", check_offset)
				// The history file reached EOF for check_offset, which means the entry was not flushed. Retry later.
				his.L1Cache.Set(hash, char, CaseRetry, FlagExpires)
				return CaseRetry, nil

			} else if string(*historyHash) == string(hash) {
				// hash is a duplicate in history.dat
				if offset > 0 { // not a search
					logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d +offset='%d'", *historyHash, check_offset, offset)
				} else { // is a search
					logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d", *historyHash, check_offset)
				}
				his.L1Cache.Set(hash, char, CaseDupes, FlagExpires)
				return CaseDupes, nil
			}
		} else {
			return -999, fmt.Errorf("ERROR DupeCheck historyHash=nil check_offset=%d", check_offset)
		}
	}
	if offset == -1 {
		// search did not find a match in check_offset over range *offsets
		return CasePass, nil // pass, not a duplicate
	}

	if offset > 0 {
		logf(DEBUG1, "HDBZW [%s|%s] APPEND key=%s hash=%s +offset=%d offsets=%d='%#v'", char, bucket, key, hash, offset, len(*offsets), offsets)
		if err := AppendOffset(offsets, offset); err != nil {
			return -999, err
		}
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, offsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketKeyPutOffsets [%s|%s] err='%v'", char, bucket, err)
			return -999, err
		}
		logf(DEBUG1, "HDBZW [%s|%s] APPENDED key=%s hash=%s offset=%d offsets=%d='%#v'", char, bucket, key, hash, offset, len(*offsets), *offsets)
		//his.L1Cache.Set(hash, char, CaseWrite, FlagNeverExpires) // was CaseDupes before // offset of history entry added to key: hash is a duplicate in cached response now
		if len(*offsets) > 1 {
			his.Sync_upcounter("key_app")
		}
		return CaseAdded, nil
	}

	return -999, fmt.Errorf("ERROR DupeCheck returned unknown error. [%s|%s] key=%s hash=%s offset=%d offsets=%#v", char, bucket, key, hash, offset, *offsets)
} // end func DupeCheck

func AppendOffset(offsets *[]int64, offset int64) error {
	if offsets == nil || offset <= 0 {
		return fmt.Errorf("ERROR AppendOffset i/o=nil")
	}
	*offsets = append(*offsets, offset)
	return nil
} // end func AppendOffset

func (his *HISTORY) boltBucketKeyPutOffsets(db *bolt.DB, char string, bucket string, key string, hash string, offset int64, offsets *[]int64, setempty bool, batchQueue chan *BatchOffset) (err error) {
	if db == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s db=nil", char)
	}
	/*
		if char == nil {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=nil")
		}
		if bucket == nil || *bucket == "" {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=nil", *char)
		}
		if key == nil || *key == "" {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] key=nil", *char, *bucket)
		}*/
	if offsets == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsets=nil", char, bucket)
	}
	if !setempty && len(*offsets) == 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=0 setempty=%t", char, bucket, setempty)
	}
	if setempty && len(*offsets) != 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=%d setempty=%t", char, bucket, len(*offsets), setempty)
	}
	gobEncodedOffsets, err := gobEncodeOffsets(*offsets, char, bucket, key, "boltBucketKeyPutOffsets", his)
	if err != nil {
		log.Printf("ERROR boltBucketKeyPutOffsets gobEncodedOffsets err='%v'", err)
		return err
	}

	// offset of history entry added to key: hash is a temporary duplicate+retry in cached response now
	his.L1Cache.Set(hash, char, CaseWrite, FlagNeverExpires)                 // was CaseDupes before // boltBucketKeyPutOffsets
	his.L2Cache.SetOffsetHash(offset, hash, FlagNeverExpires)                // boltBucketKeyPutOffsets
	his.L3Cache.SetOffsets(char+bucket+key, char, offsets, FlagNeverExpires) // boltBucketKeyPutOffsets

	// puts offset into batchQ
	batchQueue <- &BatchOffset{bucket: &bucket, key: &key, gobEncodedOffsets: &gobEncodedOffsets, hash: &hash, char: &char, offsets: offsets}
	return
} // end func boltBucketKeyPutOffsets

func (his *HISTORY) boltBucketGetOffsets(db *bolt.DB, char string, bucket string, key string) (offsets *[]int64, err error) {
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets char=%s db=nil", char)
	}
	offsets = his.L3Cache.GetOffsets(char+bucket+key, char)
	if offsets != nil && len(*offsets) >= 0 {
		//logf(DEBUG1,"boltBucketGetOffsets: get CACHED [%s|%s] key=%s offsets='%#v'", *char, *bucket, *key, *offsets)
		return offsets, nil
	}

	var gobEncodedOffsets []byte
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(key))
		if v == nil {
			logf(DEBUG2, "NOTFOUND boltBucketGetOffsets [%s|%s] key=%s", char, bucket, key)
			his.L3Cache.SetOffsets(char+bucket+key, char, &empty_offsets, FlagExpires) // boltBucketGetOffsets
			return nil
		}
		//logf(DEBUG2, "GOT boltBucketGetOffsets [%s|%s] key=%s bytes=%d", *char, *bucket, *key, len(v))
		gobEncodedOffsets = v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGetOffsets [%s|%s] key=%s err='%v'", char, bucket, key, err)
		return nil, err
	}
	if len(gobEncodedOffsets) > 0 {
		decodedOffsets, err := gobDecodeOffsets(gobEncodedOffsets, char, bucket, key, "boltBucketGetOffsets:boltDB", his)
		if err != nil || decodedOffsets == nil {
			log.Printf("ERROR boltBucketGetOffsets gobDecodeOffsets [%s|%s] key=%s err='%v'", char, bucket, key, err)
			return nil, err
		}
		his.Sync_upcounter("BoltDB_decodedOffsets")
		offsets = decodedOffsets

		if offsets != nil {
			his.L3Cache.SetOffsets(char+bucket+key, char, offsets, FlagExpires) // boltBucketGetOffsets
		}
	}
	//if offsets != nil {
	//logf(DEBUG2, "boltBucketGetOffsets returns [%s|%s] key=%s err='%v' offsetsN=%d", *char, *bucket, *key, err, len(*offsets))
	//}
	return
} // end func boltBucketGetOffsets

func (his *HISTORY) returnBoltHashOpen() {
	<-BoltHashOpen
} /// end func returnBoltHashOpen

func (his *HISTORY) setBoltHashOpen() {
	BoltHashOpen <- struct{}{}
} // end func setBoltHashOpen

func (his *HISTORY) GetBoltHashOpen() int {
	return len(BoltHashOpen)
} // end func GetBoltHashOpen

// Public: BoltSyncAll boltDBs
func (his *HISTORY) BoltSyncAll() error {
	for _, char := range HEXCHARS {
		if err := his.BoltSync(nil, char); err != nil {
			return err
		}
	}
	return nil
} // end func BoltSyncAll

// Public: BoltSync - for every DB call function with: db=nil and char=[0-9a-f]
func (his *HISTORY) BoltSync(db *bolt.DB, char string) error {
	if db == nil && char == "" {
		return fmt.Errorf("ERROR BoltSync db=nil char=nil")
	}
	if char == "" {
		return fmt.Errorf("ERROR BoltSync char=nil")
	}
	if db == nil && char != "" {
		his.boltmux.Lock()
		if his.BoltDBsMap[char].BoltDB != nil {
			db = his.BoltDBsMap[char].BoltDB
		}
		his.boltmux.Unlock()
	}
	if db == nil {
		return fmt.Errorf("ERROR BoltSync db=nil")
	}
	his.lockBoltSync()
	logf(DEBUG9, "LOCKING BoltSync BatchLocks")
	for _, bucket := range HEXCHARS {
		// locks every bucket in this char db for syncing
		his.BatchLocks[char][bucket] <- struct{}{}
	}
	logf(DEBUG9, "LOCKED BoltSync BatchLocks")
	start := utils.UnixTimeMilliSec()
	logf(DEBUG2, "BoltDB SYNC char=%s", char)
	// Manually sync the database to flush changes to disk
	if err := db.Sync(); err != nil {
		log.Printf("ERROR BoltSync char=%s db.Sync failed err='%v'", char, err)
		return err
	} else {
		logf(DEBUG2, "BoltDB SYNC char=%s took=%d ms", char, utils.UnixTimeMilliSec()-start)
	}

	for _, bucket := range HEXCHARS {
		// unlocks buckets in this char db
		his.returnBatchLock(char, bucket)
	}
	his.returnBoltSync()
	return nil
} // end func BoltSync

func (his *HISTORY) boltSyncClose(db *bolt.DB, char string) error {
	if db == nil {
		return fmt.Errorf("ERROR boltSyncClose db=nil")
	}
	if err := his.BoltSync(db, char); err != nil {
		return err
	}
	his.GetBoltStats(char, DEBUG2)
	err := db.Close()
	if err != nil {
		log.Printf("ERROR boltSyncClose char=%s err='%v'", char, err)
		return err
	}
	his.boltmux.Lock()
	his.BoltDBsMap[char].BoltDB = nil
	his.boltmux.Unlock()
	logf(DEBUG2, "BoltDB boltSyncClose char=%s", char)
	return nil
} // end func boltSyncClose

func (his *HISTORY) returnBoltSync() {
	<-his.boltSyncChan
} // end func returnBoltSync

func (his *HISTORY) lockBoltSync() {
	his.boltSyncChan <- struct{}{}
} // end func lockBoltSync

func (his *HISTORY) IndexQuery(hash *string, IndexRetChan chan int, offset int64) (int, error) {
	if !his.useHashDB {
		return CasePass, nil
	}
	if hash == nil {
		return -999, fmt.Errorf("ERROR IndexQuery hash=nil")
	}
	if his.useHashDB && his.IndexChan != nil {
		if IndexRetChan == nil {
			IndexRetChan = make(chan int, 1)
		}
		if offset > 0 {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: offset, IndexRetChan: IndexRetChan}
		} else {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: -1, IndexRetChan: IndexRetChan}
		}
		select {
		case isDup, ok := <-IndexRetChan:
			if !ok {
				return -999, fmt.Errorf("ERROR IndexQuery IndexRetChan closed! error in boltDB_Worker")
			}
			return isDup, nil
		} // end select
	}
	return -999, fmt.Errorf("ERROR IndexQuery")
} // end func IndexQuery
