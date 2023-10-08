package history

import (
	//"bufio"
	//"bytes"
	//"encoding/base64"
	//"encoding/gob"
	"fmt"
	//"github.com/go-while/go-utils"
	//"github.com/edsrzf/mmap-go"
	//"hash/fnv"
	"io"
	"log"
	"os"
	//"strconv"
	"strings"
	//"sync"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"time"
)

const (
	// never change this
	HashShort  int = 11
	HashFNV32  int = 22
	HashFNV32a int = 33
	HashFNV64  int = 44
	HashFNV64a int = 55
	MinKeyLen  int = 4
	BoltDBs    int = 16
)

var (
	NumQueueIndexChan    int    = BoltDBs // Main-indexchan can queue this
	NumQueueIndexChans   int    = 1       // every sub-indexchans for a `char` can queue this
	DefaultKeyLen        int    = 6
	BoltDB_AllocSize     int    = 16 * 1024 * 1024             // default: 16 * 1024 * 1024
	Bolt_SYNC_EVERYs     int64  = 5                            // call db.sync() every seconds (only used with 'boltopts.NoSync: true')
	Bolt_SYNC_EVERYn     uint64 = 100                          // call db.sync() after N inserts (only used with 'boltopts.NoSync = true')
	BoltINITParallel     int    = BoltDBs                      // set this via 'history.BoltINITParallel = 1' before calling History_Boot.
	BoltSYNCParallel     int    = BoltDBs                      // set this via 'history.BoltSYNCParallel = 1' before calling History_Boot.
	BoltHashOpen                = make(chan struct{}, BoltDBs) // dont change this
	HISTORY_INDEX_LOCK          = make(chan struct{}, 1)       // main lock
	HISTORY_INDEX_LOCK16        = make(chan struct{}, BoltDBs) // sub locks
	empty_offsets        []int64
)

type BOLTDB_PTR struct {
	BoltDB *bolt.DB
}

// boltDB_Init initializes the history database (HashDB) and starts the worker goroutines for processing historical data.
// It creates worker channels for each character in HEXCHARS and launches corresponding worker goroutines.
// The provided boltOpts parameter allows configuring the BoltDB database options.
func (his *HISTORY) boltDB_Init(boltOpts *bolt.Options) {
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

	if NumQueueIndexChans <= 0 {
		NumQueueIndexChans = 1
	} else if NumQueueIndexChans > 1024*1024 {
		NumQueueIndexChans = 1024 * 1024
	}

	his.boltInitChan = make(chan struct{}, BoltINITParallel)
	his.boltSyncChan = make(chan struct{}, BoltSYNCParallel)
	his.BatchLocks = make(map[string]map[string]chan struct{})
	for i, char := range HEXCHARS {
		his.charsMap[char] = i
		his.IndexChans[i] = make(chan *HistoryIndex, NumQueueIndexChans)
		his.BatchLocks[char] = make(map[string]chan struct{})
		for _, bucket := range HEXCHARS {
			his.BatchLocks[char][bucket] = make(chan struct{}, 1)
		}
	}
	for i, char := range HEXCHARS { // dont move this up into the first for loop or it drops race conditions for nothing...
		go his.boltDB_Worker(char, i, his.IndexChans[i], boltOpts)
	}
	logf(DEBUG2, "his.boltDB_Init() boltInitChan=%d boltSyncChan=%d", cap(his.boltInitChan), cap(his.boltSyncChan))
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
						logf(DEBUG, "Stopping boltDB_Index IndexChan p=%d/%d received nil pointer", p, IndexParallel)
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
					if his.IndexChans[his.charsMap[C1]] != nil {
						his.IndexChans[his.charsMap[C1]] <- hi // sends object to hash boltDB_Worker char
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
	for _, achan := range his.IndexChans {
		// passing nils to IndexChans will stop boltDB_Worker
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
	dbpath := his.HF_hash + "." + char
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
	db.MaxBatchSize = BoltDB_MaxBatchSize
	db.MaxBatchDelay = BoltDB_MaxBatchDelay
	logf(DEBUG2, "HDBZW: INIT HashDB char=%s db='%#v' db.MaxBatchSize=%d", char, db, db.MaxBatchSize)
	his.BoltDBsMap[char].BoltDB = db
	testkey := "1"
	testoffsets := []int64{1}
	testoffset := int64(1)
	testhash := TESTHASH1
	tocheck := 16
	checked, created := 0, 0
	setempty := false
	initLongTest := false
	his.boltInitChan <- struct{}{} // locks parallel intializing of boltDBs

	for _, bucket := range HEXCHARS {
		retbool, err := boltCreateBucket(db, &char, &bucket)
		if err != nil || !retbool {
			if err == bolt.ErrBucketExists {
				if !initLongTest {
					checked++
					continue
				}
			} else {
				log.Printf("ERROR HDBZW INIT HashDB boltCreateBucket [%s|%s] err='%v' retbool=%t", char, bucket, err, retbool)
				return
			}
		} else if retbool {
			created++ // <= bucket has been created
			if !initLongTest {
				checked++
				continue
			}
			// put1
			if err := his.boltBucketKeyPutOffsets(db, &char, &bucket, &testkey, &testhash, &testoffset, &testoffsets, setempty, nil); err != nil {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets1 [%s|%s] err='%v' retbool=%t", char, bucket, err, retbool)
				os.Exit(1)
			}
			// get1
			offsets1, err := his.boltBucketGetOffsets(db, &char, &bucket, &testkey)
			if err != nil || offsets1 == nil {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 [%s|%s] key=%s err='%v'", char, bucket, testkey, err)
				os.Exit(1)
			}
			if len(*offsets1) != 1 {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 [%s|%s] len(offsets)=%d != 1 ", char, bucket, len(*offsets1))
				os.Exit(1)
			}
			// put2
			*offsets1 = append(*offsets1, 2)
			if err := his.boltBucketKeyPutOffsets(db, &char, &bucket, &testkey, &testhash, &testoffset, offsets1, setempty, nil); err != nil {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets2 [%s|%s] err='%v'", char, bucket, err)
				os.Exit(1)
			}
		}
		if !initLongTest {
			continue
		}
		// get2
		offsets2, err := his.boltBucketGetOffsets(db, &char, &bucket, &testkey)
		if err != nil || offsets2 == nil || len(*offsets2) != 2 {
			log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets2 [%s|%s] key=%s err='%v'", char, bucket, testkey, err)
			os.Exit(1)
		}
		if keys, err := boltGetAllKeys(db, &char, &bucket); err != nil || keys == nil {
			log.Printf("ERROR HDBZW INIT HashDB boltGetAllKeys [%s|%s] err='%v'", char, bucket, err)
			return
		} else if len(*keys) == 1 {
			checked++
		}
		break
		//log.Printf("HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tcheck)
	} // end for c1
	<-his.boltInitChan
	logf(DEBUG0, "HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
	//if checked != 4096 {
	if checked != tocheck /*|| created != tocheck */ {
		log.Printf("ERROR HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
		return
	}
	his.setBoltHashOpen()

	timeout := 90
	for {
		if timeout <= 0 {
			log.Printf("ERROR HDBZW char=%s open his.HF not found?!", char)
			return
		}
		if utils.FileExists(his.HF) {
			break
		}
		time.Sleep(time.Second / 10)
		timeout--
	}

	lastsync := utils.UnixTimeSec()
	var added, passed, processed, dupes, searches, retry uint64
	cutHashlen := 8 // 2:8 = 6 chars
	if his.keyalgo == HashShort {
		cutHashlen = 2 + his.keylen
	}

	// opens history.dat for Fseeking
	historyfile, err := os.OpenFile(his.HF, os.O_RDONLY, 0666)
	if err != nil {
		log.Printf("ERROR HDBZW os.OpenFile his.HF err='%v'", err)
		return
	}

	adaptBatch := AdaptiveBatchSize

	// make the batchQueue
	for _, bucket := range HEXCHARS {
		// The batchQueue, like a ravenous dragon, gorges itself on memory, holding twofold the might of the actual CharBucketBatchSize.
		// A daring gamble that ignites the fires of performance, but beware the voracious appetite!
		batchQcap := CharBucketBatchSize * 2
		batchQueue := make(chan *BatchOffset, batchQcap)
		his.BatchQueues.mux.Lock()
		his.BatchQueues.Maps[char][bucket] = batchQueue
		his.BatchQueues.mux.Unlock()
		// Lo Wang unleashes a legion of batch queues, one for each sacred bucket in this 'char' database.
		// It results in a total of 16 by 16 queues, as the CharBucketBatchSize stands resolute, guarding each [char][bucket] with its mighty power!
		go func(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset) {
			if batchQueue == nil {
				log.Printf("ERROR boltDB_Worker batchQueue=nil")
				return
			}
			// every batchQueue adds an empty struct to count Booted. results in 16*16 queues.
			wCBBS := CharBucketBatchSize // copy value allows each worker to play with it
			if !LOCKfunc(his.BatchQueues.Booted, "his.BatchQueues.Booted") {
				return
			}
			lastflush := utils.UnixTimeMilliSec()
			var forced, closed, cont bool
			var inserted, conti, slept uint64
			var err error
			//var timer, mintimer, maxtimer int64 = 500, 500, 500
			var timer int64 = 500
			var decr, incr int = 4, 4
			Q := 0
		forbatchqueue:
			for {
				//if timer > 0 {
				if !forced {
					time.Sleep(time.Duration(timer) * time.Millisecond)
					slept++
				}
				lft := utils.UnixTimeMilliSec() - lastflush
				src := fmt.Sprintf("gofunc:[%s|%s]:t=%d Q=%d lft=%d", char, bucket, timer, Q, lft)
				inserted, err, closed = his.boltBucketPutBatch(db, char, bucket, batchQueue, forced, src, true, lft, wCBBS)
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

				/*
					//logf(DEBUG, "forbatchqueue A0 [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS)
					if Q == 0 {
						timer += 10
					} else
					if Q > wCBBS { // more than 100% queued, yes because cap is *2
						//logf(DEBUG, "forbatchqueue B0 [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS)
						timer -= 10
					} else
					if Q > wCBBS/2 { // more than 50% queued
						//logf(DEBUG, "forbatchqueue B1 >50%% [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft/1000, inserted, wCBBS)
						timer -= 1
					} else if Q < wCBBS/2 { // less than 50% queued
						//logf(DEBUG, "forbatchqueue B2 <50%% [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft/1000, inserted, wCBBS)
						timer += 1
					}

					if timer < mintimer {
						timer = mintimer
						//if inserted > 0 {
						//	logf(DEBUG, "forbatchqueue C1 [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS)
						//}
					} else if timer > maxtimer {
						timer = maxtimer
						//logf(DEBUG, "forbatchqueue C2 [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS)
					}
				*/

				if inserted > 0 {
					lastflush = utils.UnixTimeMilliSec()
					cont = true
					// something got inserted
					//if forced && adaptBatch {
					if adaptBatch { // trys to adapt wCBBS to `BatchFlushEvery`
						/*
							if int(inserted) == wCBBS {
								// inserted exactly wCBBS
								// pass, do nothing, batchsize looks fine
								logf(DEBUG, "forbatchqueue D0## [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d adaptBatch=%t", char, bucket, timer, Q, forced, lft, inserted, wCBBS, adaptBatch)
							} else */
						if int(inserted) >= wCBBS {
							// inserted exactly or more than wCBBS: increase wCBBS
							if wCBBS < 65536+incr {
								logf(DEBUG, "forbatchqueue D1++ [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d adaptBatch=%t incr=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS, adaptBatch, incr)
								wCBBS += incr // adaptive BatchSize incr
							}
						} else {
							// inserted less than wCBBS: decrease wCBBS
							if wCBBS > 16+decr {
								logf(DEBUG, "forbatchqueue D2-- [%s|%s] timer=%d Q=%d forced=%t lft=%d inserted=%d wCBBS=%d adaptBatch=%t decr=%d", char, bucket, timer, Q, forced, lft, inserted, wCBBS, adaptBatch, decr)
								wCBBS -= decr // adaptive BatchSize decr
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
			UNLOCKfunc(his.BatchQueues.Booted, "his.BatchQueues.Booted")
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
		if len(his.BatchQueues.Booted) == 16*16 {
			break
		}
		BQtimeout--
		if BQtimeout <= 0 {
			log.Printf("ERROR boltDB_Worker char=%s BatchQueues.Boot timeout", char)
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
					his.BatchQueues.Maps[char][bucket] <- nil
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
			isDup, err := his.DupeCheck(db, &char, &bucket, key, hi.Hash, &hi.Offset, false, historyfile, his.BatchQueues.Maps[char][bucket])
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
				if added >= Bolt_SYNC_EVERYn || (added > 0 && lastsync <= utils.UnixTimeSec()-Bolt_SYNC_EVERYs) {
					if err := his.BoltSync(db, char); err != nil {
						break forever
					}
					added, lastsync = 0, utils.UnixTimeSec()
				}
			}
		} // end select
	} // end for
	for _, bucket := range HEXCHARS {
		his.BatchQueues.Maps[char][bucket] <- nil
		close(his.BatchQueues.Maps[char][bucket])
	}
	for _, bucket := range HEXCHARS {
		logf(DEBUG2, "FINAL-BATCH HDBZW [%s|%s]", char, bucket)
		his.boltBucketPutBatch(db, char, bucket, his.BatchQueues.Maps[char][bucket], true, fmt.Sprintf("defer:[%s|%s]", char, bucket), false, -1, -1)
	}
	logf(DEBUG, "Quit HDBZW char=%s added=%d passed=%d dupes=%d processed=%d searches=%d retry=%d", char, added, passed, dupes, processed, searches, retry)
	his.Sync_upcounterN("searches", searches)
	historyfile.Close()
	his.boltSyncClose(db, char)
	time.Sleep(time.Second / 10)
	his.returnBoltHashOpen()
} // end func boltDB_Worker

// DupeCheck checks for duplicate message-ID hashes in a BoltDB bucket.
// It manages offsets associated with message hashes and handles duplicates, ensuring the integrity of the historical data.
// If a hash is a duplicate, it returns 1, otherwise, it returns 0.
// It also handles the creation of new hash entries in the bucket when needed.
func (his *HISTORY) DupeCheck(db *bolt.DB, char *string, bucket *string, key *string, hash *string, offset *int64, setempty bool, file *os.File, batchQueue chan *BatchOffset) (int, error) {
	// return -999 with err only
	if db == nil {
		return -999, fmt.Errorf("ERROR DupeCheck db=nil")
	}
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

	offsets, err := his.boltBucketGetOffsets(db, char, bucket, key)
	if err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets [%s|%s] key=%s hash='%s' err='%v'", *char, *bucket, *key, *hash, err)
		return -999, err
	}
	len_offsets := 0
	if offsets != nil {
		len_offsets = len(*offsets)
	}
	if offsets == nil || (offsets != nil && len_offsets == 0) { // no offsets stored for numhash
		if *offset == -1 { // search only
			//his.L1Cache.Set(hash, *char, -2)
			//his.L3Cache.SetOffsets(*char+*bucket+*key, *char, &empty_offsets)
			return CasePass, nil // pass, not a duplicate
		}
		newoffsets := []int64{*offset}
		// add hash=>key:offset to db
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, &newoffsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW DupeCheck char=%s Add boltBucketKeyPutOffsets bucket=%s err='%v'", *char, *bucket, err)
			return -999, err
		}
		//logf(DEBUG2, "HDBZW char=%s DupeCheck CREATED key=%s hash=%s offset=0x%08x=%d", *char, *key, *hash, *offset, *offset)

		//his.L1Cache.Set(hash, *char, 1) // offset of history entry added to key: hash is a duplicate in cached response now
		//his.L2Cache.SetOffsetHash(offset, hash)

		his.Sync_upcounter("key_add")
		return CaseAdded, nil
	}

	// got offset(s) stored for numhash
	if len_offsets > 0 {
		if *offset > 0 { // is not a search
			logf(DEBUG2, "INFO HDBZW char=%s key=%s tryhash='%s' GOT multiple offsets=%d=%#v +offset=%d", *char, *key, *hash, len_offsets, *offsets, *offset)
			his.Sync_upcounter("addmultioffsets")
		} else {
			his.Sync_upcounter("trymultioffsets")
		}
	}
	for _, check_offset := range *offsets {
		// check history for duplicate hash / evades collissions
		logf(DEBUG2, "HDBZW char=%s CHECK DUP key=%s lo=%d offset=%d", *char, *key, len_offsets, check_offset)
		historyHash, err := his.FseekHistoryMessageHash(file, &check_offset)
		if historyHash == nil && err == nil {
			log.Printf("ERROR HDBZW char=%s CHECK DUP bucket=%s historyHash=nil err=nil hash=%s", *char, *bucket, err, *hash)
			return -999, fmt.Errorf("ERROR historyHash=nil err=nil @offset=%d +offset=%d", *historyHash, check_offset, *offset)
		}
		if err != nil {
			log.Printf("ERROR HDBZW char=%s FseekHistoryMessageHash bucket=%s err='%v' offset=%d", *char, *bucket, err, check_offset)
			return -999, err
		}
		if historyHash != nil {

			if len(*historyHash) == 3 && *historyHash == eofhash {
				log.Printf("EOF history.dat offset=%d", check_offset)
				// The history file reached EOF for check_offset, which means the entry was not flushed. Retry later.
				his.L1Cache.Set(hash, *char, CaseRetry)
				return CaseRetry, nil

			} else if string(*historyHash) == string(*hash) {
				// hash is a duplicate in history
				if *offset > 0 { // not a search
					logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d +offset='%d'", *historyHash, check_offset, *offset)
				} else { // is a search
					logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d", *historyHash, check_offset)
				}
				his.L1Cache.Set(hash, *char, CaseDupes)
				return CaseDupes, nil
			}
		} else {
			return -999, fmt.Errorf("ERROR DupeCheck historyHash=nil check_offset=%d", check_offset)
		}
	}
	if *offset == -1 {
		// search did not find a match in check_offset over range *offsets
		return CasePass, nil // pass, not a duplicate
	}

	if *offset > 0 {
		logf(DEBUG1, "HDBZW [%s|%s] APPEND key=%s hash=%s +offset=%d offsets=%d='%#v'", *char, *bucket, *key, *hash, *offset, len(*offsets), *offsets)
		if err := AppendOffset(offsets, offset); err != nil {
			return -999, err
		}
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, offsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketKeyPutOffsets [%s|%s] err='%v'", *char, *bucket, err)
			return -999, err
		}
		logf(DEBUG1, "HDBZW [%s|%s] APPENDED key=%s hash=%s offset=%d offsets=%d='%#v'", *char, *bucket, *key, *hash, *offset, len(*offsets), *offsets)
		his.L1Cache.Set(hash, *char, CaseDupes) // offset of history entry added to key: hash is a duplicate in cached response now
		if len(*offsets) > 1 {
			his.Sync_upcounter("key_app")
		}
		return CaseAdded, nil
	}

	return -999, fmt.Errorf("ERROR DupeCheck returned unknown error. [%s|%s] key=%s hash=%s offset=%d offsets=%#v", *char, *bucket, *key, *hash, *offset, *offsets)
} // end func DupeCheck

func boltCreateBucket(db *bolt.DB, char *string, bucket *string) (retbool bool, err error) {
	if char == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=nil")
	}
	if db == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s db=nil", *char)
	}
	if bucket == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s bucket=nil", *char)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		//_, err := tx.CreateBucketIfNotExists([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		if err != nil {
			return err
		}
		logf(DEBUG2, "OK boltCreateBucket char=%s bucket='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s bucket='%s' err='%v'", *char, *bucket, err)
		}
		return false, err
	}
	return
} // end func boltCreateBucket

/*
func boltBucketPutString(db *bolt.DB, char *string, bucket *string, key *string, val *string) (err error) {
	if char == nil {
		return fmt.Errorf("ERROR boltBucketPut char=nil")
	}
	if db == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s db=nil", *char)
	}
	if bucket == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=nil", *char)
	}
	if key == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] key=nil", *char, *bucket)
	}
	if val == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] val=nil", *char, *bucket)
	}
	if err:= db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), []byte(*val))
		return err
	}); err != nil {
		log.Printf("ERROR boltBucketPut [%s|%s] key=%s val=%s err='%v'", *char, *bucket, *key, *val, err)
		return err
	}
	return
} // end func boltBucketPutString
*/

func AppendOffset(offsets *[]int64, offset *int64) error {
	if offsets == nil || offset == nil || *offset <= 0 {
		return fmt.Errorf("ERROR AppendOffset i/o=nil")
	}
	*offsets = append(*offsets, *offset)
	return nil
} // end func AppendOffset

func (his *HISTORY) boltBucketKeyPutOffsets(db *bolt.DB, char *string, bucket *string, key *string, hash *string, offset *int64, offsets *[]int64, setempty bool, batchQueue chan *BatchOffset) (err error) {
	if char == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=nil")
	}
	if db == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=nil", char)
	}
	if key == nil || *key == "" {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] key=nil", *char, *bucket)
	}
	if offsets == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsets=nil", *char, *bucket)
	}
	if !setempty && len(*offsets) == 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=0 setempty=%t", *char, *bucket, setempty)
	}
	if setempty && len(*offsets) != 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=%d setempty=%t", *char, *bucket, len(*offsets), setempty)
	}
	gobEncodedOffsets, err := gobEncodeOffsets(*offsets, "boltBucketKeyPutOffsets")
	if err != nil {
		log.Printf("ERROR boltBucketKeyPutOffsets gobEncodedOffsets err='%v'", err)
		return err
	}

	//his.mux.Lock()
	his.L1Cache.Set(hash, *char, CaseDupes) // offset of history entry added to key: hash is a duplicate in cached response now
	his.L2Cache.SetOffsetHash(offset, hash)
	his.L3Cache.SetOffsets(*char+*bucket+*key, *char, offsets)
	//his.mux.Unlock()

	if batchQueue != nil {
		batchQueue <- &BatchOffset{bucket: *bucket, key: *key, gobEncodedOffsets: gobEncodedOffsets}
		return nil
	}
	if batchQueue == nil {
		if err := db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(*bucket))
			err := b.Put([]byte(*key), gobEncodedOffsets)
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketKeyPutOffsets [%s|%s] key=%s offsetsN=%d err='%v'", *char, *bucket, *key, len(*offsets), err)
			return err
		}
	}
	return
} // end func boltBucketKeyPutOffsets

func boltBucketGetBytes(db *bolt.DB, char *string, bucket *string, key *string) (retval *[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s bucket=nil", *char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet [%s|%s] key=nil", *char, *bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet [%s|%s] key=%s", *char, *bucket, *key)
			return nil
		}
		//log.Printf("GOT boltBucketGet [%s|%s] key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = &v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGet [%s|%s] key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	return
} // end func boltBucketGetBytes

func (his *HISTORY) boltBucketGetOffsets(db *bolt.DB, char *string, bucket *string, key *string) (offsets *[]int64, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets char=%s bucket=nil", *char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets [%s|%s] key=nil", *char, *bucket)
	}

	offsets = his.L3Cache.GetOffsets(*char+*bucket+*key, *char)
	if offsets != nil && len(*offsets) >= 0 {
		his.Sync_upcounter("L3CACHE_Get")
		//logf(DEBUG1,"boltBucketGetOffsets: get CACHED [%s|%s] key=%s offsets='%#v'", *char, *bucket, *key, *offsets)
		return offsets, nil
	}

	var gobEncodedOffsets []byte
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			logf(DEBUG2, "NOTFOUND boltBucketGetOffsets [%s|%s] key=%s", *char, *bucket, *key)
			his.L3Cache.SetOffsets(*char+*bucket+*key, *char, &empty_offsets)
			return nil
		}
		//logf(DEBUG2, "GOT boltBucketGetOffsets [%s|%s] key=%s bytes=%d", *char, *bucket, *key, len(v))
		gobEncodedOffsets = v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGetOffsets [%s|%s] key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	if len(gobEncodedOffsets) > 0 {
		decodedOffsets, err := gobDecodeOffsets(gobEncodedOffsets, "boltBucketGetOffsets:boltDB")
		if err != nil || decodedOffsets == nil {
			log.Printf("ERROR boltBucketGetOffsets gobDecodeOffsets [%s|%s] key=%s err='%v'", *char, *bucket, *key, err)
			return nil, err
		}
		his.Sync_upcounter("BoltDB_decodedOffsets")
		offsets = decodedOffsets

		if offsets != nil {
			his.L3Cache.SetOffsets(*char+*bucket+*key, *char, offsets)
		}
	}
	//if offsets != nil {
	//logf(DEBUG2, "boltBucketGetOffsets returns [%s|%s] key=%s err='%v' offsetsN=%d", *char, *bucket, *key, err, len(*offsets))
	//}
	return
} // end func boltBucketGetOffsets

func boltGetAllKeysVals(db *bolt.DB, char *string, bucket *string) (keyvals map[*string]*[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s bucket=nil", char)
	}
	keyvals = make(map[*string]*[]byte)
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			key := string(k)
			keyvals[&key] = &v
			//*keys = append(*keys, &key)
			//*vals = append(*vals, &v)
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return
} // end func boltGetAllKeysVals

func boltGetAllKeys(db *bolt.DB, char *string, bucket *string) (retkeys *[]*string, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s bucket=nil", char)
	}
	var keys []*string
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			//fmt.Printf("key=%s", k)
			key := string(k)
			keys = append(keys, &key)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	retkeys = &keys
	return
} // end func boltGetAllKeys

func (his *HISTORY) returnBoltHashOpen() {
	<-BoltHashOpen
} /// end func returnBoltHashOpen

func (his *HISTORY) setBoltHashOpen() {
	BoltHashOpen <- struct{}{}
} // end func setBoltHashOpen

func (his *HISTORY) GetBoltHashOpen() int {
	return len(BoltHashOpen)
} // end func GetBoltHashOpen

// BoltSync is a Public Function. for every DB call function with: db=nil and char=[0-9a-f]
func (his *HISTORY) BoltSync(db *bolt.DB, char string) error {
	if db == nil && char == "" {
		return fmt.Errorf("ERROR BoltSync db=nil char=nil")
	}
	if char == "" {
		return fmt.Errorf("ERROR BoltSync char=nil")
	}
	if db == nil && char != "" {
		if his.BoltDBsMap[char].BoltDB != nil {
			db = his.BoltDBsMap[char].BoltDB
		}
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
	err := db.Close()
	if err != nil {
		log.Printf("ERROR boltSyncClose char=%s err='%v'", char, err)
		return err
	}
	his.BoltDBsMap[char].BoltDB = nil
	logf(DEBUG2, "BoltDB boltSyncClose char=%s", char)
	return nil
} // end func boltSyncClose

func (his *HISTORY) returnBoltSync() {
	<-his.boltSyncChan
} // end func returnBoltSync

func (his *HISTORY) lockBoltSync() {
	his.boltSyncChan <- struct{}{}
} // end func lockBoltSync

func (his *HISTORY) PrintGetBoltStatsEvery(char string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		his.GetBoltStats("")
	}
} // end func PrintGetBoltStatsEvery

func (his *HISTORY) GetBoltStats(char string) (int, int) {
	startedTx, committTx := 0, 0
	if char == "" {
		for _, char := range HEXCHARS {
			if his.BoltDBsMap[char].BoltDB != nil {
				dbstats, _ := his.getDBStats(his.BoltDBsMap[char].BoltDB)
				startedTx += dbstats.OpenTxN
				committTx += dbstats.TxN
				//log.Printf("BoltStats [%s] dbstats:'%+v' err='%#v' startedTx=%d committTx=%d", char, dbstats, err)
				//log.Printf("BoltStats [%s] startedTx=%d committTx=%d err='%v'", char, dbstats.OpenTxN, dbstats.TxN, err)
			}
		}
	}
	/*
		if char != "" {
			for _, bucket := range HEXCHARS {
				if his.BoltDBsMap[char].BoltDB != nil {
					stats, err := his.getBucketStats(his.BoltDBsMap[char].BoltDB, bucket)
					//startedTx += stats.TxStats.Started
					//committTx += stats.TxStats.Committed
					log.Printf("BoltStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
			return startedTx, committTx
		}

		for _, char := range HEXCHARS {
			for _, bucket := range HEXCHARS {
				if his.BoltDBsMap[char].BoltDB != nil {
					stats, err := his.getBucketStats(his.BoltDBsMap[char].BoltDB, bucket)
					//startedTx += stats.TxStats.Started
					//committTx += stats.TxStats.Committed
					log.Printf("BoltStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
		}
	*/
	return startedTx, committTx
} // end func GetBoltStats

func (his *HISTORY) getDBStats(db *bolt.DB) (bolt.Stats, error) {
	var stats bolt.Stats

	if db == nil {
		return stats, fmt.Errorf("ERROR getDBStats db=nil")
	}

	err := db.View(func(tx *bolt.Tx) error {
		// Get statistics
		stats = db.Stats()
		return nil
	})

	return stats, err
} // end func getDBStats

func (his *HISTORY) getBucketStats(db *bolt.DB, bucketName string) (bolt.BucketStats, error) {
	var stats bolt.BucketStats

	if db == nil {
		return stats, fmt.Errorf("ERROR getBucketStats db=nil")
	}

	err := db.View(func(tx *bolt.Tx) error {
		// Access the specified bucket
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("Bucket not found: %s", bucketName)
		}

		// Get statistics
		stats = bucket.Stats()
		return nil
	})

	return stats, err
} // end func getBucketStats

func (his *HISTORY) IndexQuery(hash *string, IndexRetChan chan int) (int, error) {
	if hash == nil {
		return -999, fmt.Errorf("ERROR IndexQuery hash=nil")
	}
	if his.useHashDB && his.IndexChan != nil {
		if IndexRetChan == nil {
			IndexRetChan = make(chan int, 1)
		}
		his.IndexChan <- &HistoryIndex{Hash: hash, Offset: -1, IndexRetChan: IndexRetChan}
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

func (his *HISTORY) Sync_upcounter(counter string) {
	go func(counter string) {
		his.cmux.Lock()
		his.Counter[counter] += 1
		his.cmux.Unlock()
	}(counter)
} // end func sync_upcounter

func (his *HISTORY) Sync_upcounterN(counter string, value uint64) {
	go func(counter string, value uint64) {
		his.cmux.Lock()
		his.Counter[counter] += value
		his.cmux.Unlock()
	}(counter, value)
} // end func Sync_upcounterN

func (his *HISTORY) GetCounter(counter string) uint64 {
	his.cmux.Lock()
	retval := his.Counter[counter]
	his.cmux.Unlock()
	return retval
} // end func GetCounter
