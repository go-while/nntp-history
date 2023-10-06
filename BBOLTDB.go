package history

import (
	//"bufio"
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	//"github.com/go-while/go-utils"
	//"github.com/edsrzf/mmap-go"
	"hash/fnv"
	"io"
	"log"
	"os"
	//"github.com/nutsdb/nutsdb"
	//"strconv"
	//"strings"
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
	BATCHSIZE            int    = 100
	QueueIndexChan       int    = BoltDBs // Main-indexchan can queue this
	QueueIndexChans      int    = BoltDBs // every sub-indexchans for a `char` can queue this
	DefaultKeyLen        int    = 6
	Bolt_SYNC_EVERYs     int64  = 5                            // call db.sync() every seconds (only used with 'boltopts.NoSync: true')
	Bolt_SYNC_EVERYn     uint64 = 100                          // call db.sync() after N inserts (only used with 'boltopts.NoSync = true')
	BoltINITParallel     int    = BoltDBs                      // set this via 'history.BoltINITParallel = 1' before calling History_Boot.
	BoltSYNCParallel     int    = BoltDBs                      // set this via 'history.BoltSYNCParallel = 1' before calling History_Boot.
	BoltHashOpen                = make(chan struct{}, BoltDBs) // dont change this
	HISTORY_INDEX_LOCK          = make(chan struct{}, 1)
	HISTORY_INDEX_LOCK16        = make(chan struct{}, BoltDBs)
	empty_offsets        []int64
)

// History_DBZinit initializes the history database (HashDB) and starts the worker goroutines for processing historical data.
// It creates worker channels for each character in HEXCHARS and launches corresponding worker goroutines.
// The provided boltOpts parameter allows configuring the BoltDB database options.
func (his *HISTORY) History_DBZinit(boltOpts *bolt.Options) {
	if his.boltInitChan != nil {
		log.Printf("ERROR History_DBZinit already loaded")
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

	if QueueIndexChan <= 0 {
		QueueIndexChan = 1
	} else if QueueIndexChan > 1000000 {
		QueueIndexChan = 1000000
	}

	if QueueIndexChans <= 0 {
		QueueIndexChans = 1
	} else if QueueIndexChans > 1000000 {
		QueueIndexChans = 1000000
	}

	his.boltInitChan = make(chan struct{}, BoltINITParallel)
	his.boltSyncChan = make(chan struct{}, BoltSYNCParallel)
	his.BatchLocks = make(map[string]map[string]chan struct{}, BoltDBs)
	for i, char := range HEXCHARS {
		his.charsMap[char] = i
		his.IndexChans[i] = make(chan *HistoryIndex, QueueIndexChans)
		his.BatchLocks[char] = make(map[string]chan struct{}, BoltDBs)
		for _, bucket := range HEXCHARS {
			his.BatchLocks[char][bucket] = make(chan struct{}, 1)
		}
	}
	for i, char := range HEXCHARS { // dont move this up into the first for loop or it drops race conditions for nothing...
		go his.History_DBZ_Worker(char, i, his.IndexChans[i], boltOpts)
	}
	logf(DEBUG2, "his.History_DBZinit() boltInitChan=%d boltSyncChan=%d", cap(his.boltInitChan), cap(his.boltSyncChan))
	go his.History_DBZ()
} // end func History_DBZinit

// History_DBZ is the main routine for managing the historical data processing.
// It listens to incoming HistoryIndex structs on the IndexChan channel and distributes them to corresponding worker goroutines.
func (his *HISTORY) History_DBZ() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ")
	his.wait4HashDB()
	logf(DEBUG2, "Boot History_DBZ")
	defer logf(DEBUG2, "Quit History_DBZ")
	parallel := BoltDBs
	waitchan := make(chan struct{}, parallel)
	for p := 1; p <= parallel; p++ {
		waitchan <- struct{}{}
		go func(p int, waitchan chan struct{}) {
		forever:
			for {
				select {
				case hi, ok := <-his.IndexChan: // recevies a HistoryIndex struct and passes it down to '0-9a-f' workers
					if !ok {
						logf(DEBUG2, "Stopping History_DBZ IndexChan closed")
						break forever
					}
					if hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // allow at least md5
						logf(DEBUG2, "Stopping History_DBZ IndexChan received nil pointer")
						close(his.IndexChan)
						break forever
					}
					if hi.Offset == 0 {
						log.Printf("ERROR History_DBZ offset=0") // must: Offset -1 to checkonly OR Offset > 0 adds to hashDB
						break forever
					}
					C1 := ""
					if hi.Char != nil && len(*hi.Char) == 1 {
						C1 = *hi.Char
					} else {
						// gets first char of hash: hash must be lowercase!
						// hex.EncodeToString returns a lowercased string of a hashsum
						C1 = string(string(*hi.Hash)[0])
					}
					if his.IndexChans[his.charsMap[C1]] != nil {
						his.IndexChans[his.charsMap[C1]] <- hi // sends object to hash History_DBZ_Worker char
					} else {
						log.Printf("ERROR History_DBZ IndexChan C1=%s=nil", C1)
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
		// passing nils to IndexChans will stop History_DBZ_Worker
		achan <- nil
	}
} // end func History_DBZ

// History_DBZ_Worker is a worker function responsible for processing historical data.
// It manages BoltDB operations, including storing and retrieving offsets, and handles duplicate checks
// to ensure message-ID hashes are correctly tracked in the history file.
func (his *HISTORY) History_DBZ_Worker(char string, i int, indexchan chan *HistoryIndex, boltOpts *bolt.Options) {
	if !LOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char) {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char)
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
	testkey := "1"
	testoffsets := []int64{1}
	testoffset := int64(1)
	testhash := TESTHASH1
	tocheck := 16
	checked, created := 0, 0
	setempty := false
	initLongTest := false
	his.boltInitChan <- struct{}{} // locks parallel intializing of boltDBs
	logf(DEBUG0, "HDBZW: INIT HashDB char=%s", char)
	for _, bucket := range HEXCHARS {
		retbool, err := boltCreateBucket(db, &char, &bucket)
		if err != nil || !retbool {
			if err == bolt.ErrBucketExists {
				if !initLongTest {
					checked++
					continue
				}
			} else {
				log.Printf("ERROR HDBZW INIT HashDB boltCreateBucket char=%s bucket=%s err='%v' retbool=%t", char, bucket, err, retbool)
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
				log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets1 char=%s bucket=%s err='%v' retbool=%t", char, bucket, err, retbool)
				os.Exit(1)
			}
			// get1
			offsets1, err := his.boltBucketGetOffsets(db, &char, &bucket, &testkey)
			if err != nil || offsets1 == nil {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 char=%s bucket=%s key=%s err='%v'", char, bucket, testkey, err)
				os.Exit(1)
			}
			if len(*offsets1) != 1 {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 char=%s bucket=%s len(offsets)=%d != 1 ", char, bucket, len(*offsets1))
				os.Exit(1)
			}
			// put2
			*offsets1 = append(*offsets1, 2)
			if err := his.boltBucketKeyPutOffsets(db, &char, &bucket, &testkey, &testhash, &testoffset, offsets1, setempty, nil); err != nil {
				log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets2 char=%s bucket=%s err='%v'", char, bucket, err)
				os.Exit(1)
			}
		}
		if !initLongTest {
			continue
		}
		// get2
		offsets2, err := his.boltBucketGetOffsets(db, &char, &bucket, &testkey)
		if err != nil || offsets2 == nil || len(*offsets2) != 2 {
			log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets2 char=%s bucket=%s key=%s err='%v'", char, bucket, testkey, err)
			os.Exit(1)
		}
		if keys, err := boltGetAllKeys(db, &char, &bucket); err != nil || keys == nil {
			log.Printf("ERROR HDBZW INIT HashDB boltGetAllKeys char=%s bucket=%s err='%v'", char, bucket, err)
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

	// make the batchQueue
	for _, bucket := range HEXCHARS {
		// batchQueue can hold 4x more than actual batchsize. eats some memory but bursts performance.
		batchQueue := make(chan *BatchOffset, BATCHSIZE*4)
		his.BatchQueues.mux.Lock()
		his.BatchQueues.Maps[char][bucket] = batchQueue
		his.BatchQueues.mux.Unlock()
		// launches a batchQueue for every bucket in this `char` db.
		go func(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset) {
			timer, mintimer, maxtimer := 100, 1, 1000
			lastflush := utils.UnixTimeMilliSec()
			var retbool, forced, closed bool
			var err error
			// every batchQueue adds an empty struct to count Booted. results in 16*16 queues.
			if !LOCKfunc(his.BatchQueues.Booted, "his.BatchQueues.Booted") {
				return
			}
		forbatchqueue:
			for {
				if timer > 0 {
					time.Sleep(time.Duration(timer) * time.Millisecond)
				}
				retbool, err, closed = his.boltBucketPutBatch(db, char, bucket, batchQueue, forced, fmt.Sprintf("gofunc:%s%s:s=%d", char, bucket, timer), true)
				if closed { // received nil pointer
					logf(DEBUG2, "Closed gofunc char=%s bucket=%s", char, bucket)

					break forbatchqueue // this go func
				}
				if err != nil {
					log.Printf("gofunc char=%s boltBucketPutBatch err='%v'", char, err)
					break forbatchqueue // this go func
				}
				switch retbool {
				case true:
					lastflush = utils.UnixTimeMilliSec()
					forced = false
				case false:
					if len(batchQueue) == 0 {
						lastflush = utils.UnixTimeMilliSec()
					}
					if lastflush < utils.UnixTimeMilliSec()-BATCHFLUSH && len(batchQueue) > 0 {
						forced = true
						timer = 1
					}
				}
				Q := len(batchQueue)
				if Q > 10000 {
					logf(DEBUG, "forbatchqueue char=%s bucket=%s sleep=%d Q=%d", char, bucket, timer, Q)
				}
				if Q >= 1000 {
					timer -= 2
				} else if Q < 1000 {
					timer += 1
				}
				if timer < mintimer {
					timer = mintimer
				} else if timer > maxtimer {
					timer = maxtimer
				}
				if Q >= BATCHSIZE {
					timer = 0
				}
				continue forbatchqueue
			} // end forbatchqueue
			UNLOCKfunc(his.BatchQueues.Booted, "his.BatchQueues.Booted")
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
			log.Printf("ERROR History_DBZ_Worker char=%s BatchQueues.Boot timeout", char)
			return
		}
	}
	//log.Printf("BOOTED History_DBZ_Worker char=%s (took %d ms)", char, 1000-BQtimeout)

forever:
	for {
		select {
		case hi, ok := <-indexchan: // receives a HistoryIndex struct
			if !ok || hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // at least md5
				// receiving a nil object stops history_dbz_worker
				logf(DEBUG9, "Stopping History_DBZ_Worker indexchan[%s] received nil pointer", char)
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
				logf(DEBUG2, "WORKER HDBZW char=%s bucket=%s key=%s hash=%s @0x%010x|%d|%x", char, bucket, *key, *hi.Hash, hi.Offset, hi.Offset, hi.Offset)
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
			case CasePass:
				log.Printf("ERROR HDBZW DupeCheck invalid response isDup=CasePass char=%s bucket=%s key=%s hash=%s @0x%010x|%d|%x", char, bucket, *key, *hi.Hash, hi.Offset)
				passed++
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
		logf(DEBUG2, "FINAL-BATCH HDBZW char=%s bucket=%s", char, bucket)
		his.boltBucketPutBatch(db, char, bucket, his.BatchQueues.Maps[char][bucket], true, fmt.Sprintf("defer:%s%s", char, bucket), false)
	}
	logf(DEBUG, "Quit HDBZW char=%s added=%d passed=%d dupes=%d processed=%d searches=%d retry=%d", char, added, passed, dupes, processed, searches, retry)
	his.Sync_upcounterN("searches", searches)
	historyfile.Close()
	his.boltSyncClose(db, char)
	time.Sleep(time.Second / 10)
	his.returnBoltHashOpen()
} // end func History_DBZ_Worker

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
		return -999, fmt.Errorf("ERROR DupeCheck char=%s bucket=%s key=nil", *char, *bucket)
	}
	if hash == nil {
		return -999, fmt.Errorf("ERROR DupeCheck char=%s bucket=%s key=%s hash=nil", *char, *bucket, *key)
	}
	if offset == nil {
		return -999, fmt.Errorf("ERROR DupeCheck char=%s bucket=%s key=%s hash=%s offset=nil", *char, *bucket, *key, *hash)
	}

	offsets, err := his.boltBucketGetOffsets(db, char, bucket, key)
	if err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets char=%s bucket=%s key=%s hash='%s' err='%v'", *char, *bucket, *key, *hash, err)
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
	if len_offsets > 1 {
		if *offset > 0 { // is not a search
			logf(DEBUG2, "INFO HDBZW char=%s key=%s tryhash='%s' GOT multiple offsets=%d=%#v +offset=%d", *char, *key, *hash, len_offsets, *offsets, *offset)
			his.Sync_upcounter("gotmultioffsets")
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
		logf(DEBUG1, "HDBZW char=%s bucket=%s APPEND key=%s hash=%s +offset=%d offsets=%d='%#v'", *char, *bucket, *key, *hash, *offset, len(*offsets), *offsets)
		if err := AppendOffset(offsets, offset); err != nil {
			return -999, err
		}
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, offsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketKeyPutOffsets char=%s bucket=%s err='%v'", *char, *bucket, err)
			return -999, err
		}
		logf(DEBUG1, "HDBZW char=%s bucket=%s APPENDED key=%s hash=%s offset=%d offsets=%d='%#v'", *char, *bucket, *key, *hash, *offset, len(*offsets), *offsets)
		his.L1Cache.Set(hash, *char, CaseDupes) // offset of history entry added to key: hash is a duplicate in cached response now
		if len(*offsets) > 1 {
			his.Sync_upcounter("key_app")
		}
		return CaseAdded, nil
	}

	return -999, fmt.Errorf("ERROR DupeCheck returned unknown error. char=%s buk=%s key=%s hash=%s offset=%d offsets=%#v", *char, *bucket, *key, *hash, *offset, *offsets)
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
		logf(DEBUG2, "OK boltCreateBucket char=%s buk='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s buk='%s' err='%v'", *char, *bucket, err)
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
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=%s key=nil", *char, *bucket)
	}
	if val == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=%s val=nil", *char, *bucket)
	}
	if err:= db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), []byte(*val))
		return err
	}); err != nil {
		log.Printf("ERROR boltBucketPut char=%s buk=%s key=%s val=%s err='%v'", *char, *bucket, *key, *val, err)
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
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=%s key=nil", *char, *bucket)
	}
	if offsets == nil {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=%s offsets=nil", *char, *bucket)
	}
	if !setempty && len(*offsets) == 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=%s offsetsN=0 setempty=%t", *char, *bucket, setempty)
	}
	if setempty && len(*offsets) != 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=%s offsetsN=%d setempty=%t", *char, *bucket, len(*offsets), setempty)
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
			log.Printf("ERROR boltBucketKeyPutOffsets char=%s buk=%s key=%s offsetsN=%d err='%v'", *char, *bucket, *key, len(*offsets), err)
			return err
		}
	}
	return
} // end func boltBucketKeyPutOffsets

func (his *HISTORY) boltBucketPutBatch(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset, forced bool, src string, looped bool) (retbool bool, err error, closed bool) {
	if batchQueue == nil {
		log.Printf("ERROR boltBucketPutBatch batchQueue=nil")
		return
	}
	logf(DEBUG9, "LOCKING boltBucketPutBatch char=%s bucket=%s", char, bucket)
	// locks this char db:bucket for batch writing
	his.BatchLocks[char][bucket] <- struct{}{}
	defer his.returnBatchLock(char, bucket)
	logf(DEBUG9, "iLOCKED boltBucketPutBatch char=%s bucket=%s", char, bucket)

	if len(batchQueue) < BATCHSIZE && !forced {
		return
	}
	batch := []*BatchOffset{}
	batchmap := make(map[string][]*BatchOffset)
	var queued uint64
	start := utils.UnixTimeMilliSec()
	//closed := false
	if batchQueue != nil {
	fetchbatch:
		for {
			select {
			case bo, ok := <-batchQueue:
				if !ok || bo == nil {
					closed = true
					logf(DEBUG2, "boltBucketPutBatch received nil pointer char=%s bucket=%s", char, bucket)
					break fetchbatch
				}
				if bo.bucket != "" {
					if bo.bucket != bucket {
						err = fmt.Errorf("ERROR boltBucketPutBatch bo.bucket=%s != bucket=%s", bo.bucket, bucket)
						return
					}
					batchmap[bo.bucket] = append(batchmap[bo.bucket], bo)
					queued++
					if !forced && len(batchmap[bucket]) >= BATCHSIZE {
						break fetchbatch
					}
				} else {
					// batch to inserted2. maybe remove soon?
					batch = append(batch, bo)
					if !forced && len(batch) >= BATCHSIZE {
						break fetchbatch
					}
				}
			default:
				break fetchbatch
			}
		}
	}

	var inserted1 uint64
	if queued > 0 {
		for buk, v := range batchmap {
			if len(batchmap[buk]) == 0 {
				continue
			}
			if err := db.Update(func(tx *bolt.Tx) error {
				var err error
				b := tx.Bucket([]byte(buk))
			batchmapinsert:
				for _, bo := range v {
					puterr := b.Put([]byte(bo.key), bo.gobEncodedOffsets)
					if puterr != nil {
						err = puterr
						break batchmapinsert
					}
					inserted1++
				}
				return err
			}); err != nil {
				log.Printf("ERROR boltBucketPutBatch char=%s buk=%s err='%v'", char, bucket, err)
				return false, err, closed
			}
		}
	}

	var inserted2 uint64
	if len(batch) > 0 { // if no batchqueue is set (almost impossible since it is hardcoded...?)
		if err := db.Update(func(tx *bolt.Tx) error {
			var err error
			b := tx.Bucket([]byte(bucket))
		batchinsert:
			for _, bo := range batch {
				puterr := b.Put([]byte(bo.key), bo.gobEncodedOffsets)
				if puterr != nil {
					err = puterr
					break batchinsert
				}
				inserted2++
			}
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketPutBatch char=%s buk=%s err='%v'", char, bucket, err)
			return false, err, closed
		}
	}

	//queued := len(batchQueue)
	/*
		if forced && !looped {
			for {
				time.Sleep(time.Second / 100)
				Q := len(batchQueue)
				if Q == 0 {
					logf(DEBUG9, "DONE FORCED SYNCING boltBucketPutBatch char=%s buk=%s", char, bucket)
					return true, nil
				}
				logf(DEBUG, "FORCED SYNCING boltBucketPutBatch char=%s buk=%s Q=%d", char, bucket, Q)
				//his.boltBucketPutBatch(db, char, bucket, batchQueue, forced, src+"+", true)
			}
		}*/

	inserted := inserted1 + inserted2
	if inserted > 0 {
		retbool = true
	}
	his.Sync_upcounterN("inserted1", inserted1)
	his.Sync_upcounterN("inserted2", inserted2)
	logf(DEBUG9, "BATCHED boltBucketPutBatch char=%s buk=%s i1=%d i2=%d Q=%d forced=%t (took %d ms) src=%s", char, bucket, inserted1, inserted2, len(batchQueue), forced, utils.UnixTimeMilliSec()-start, src)
	return
} // end func boltBucketPutBatch

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
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s bucket=%s key=nil", *char, *bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet char=%s buk=%s key=%s", *char, *bucket, *key)
			return nil
		}
		//log.Printf("GOT boltBucketGet char=%s buk=%s key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = &v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGet char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
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
		return nil, fmt.Errorf("ERROR boltBucketGetOffsets char=%s bucket=%s key=nil", *char, *bucket)
	}

	offsets = his.L3Cache.GetOffsets(*char+*bucket+*key, *char)
	if offsets != nil && len(*offsets) >= 0 {
		his.Sync_upcounter("L3CACHE_Get")
		//logf(DEBUG1,"boltBucketGetOffsets: get CACHED char=%s bucket=%s key=%s offsets='%#v'", *char, *bucket, *key, *offsets)
		return offsets, nil
	}

	var gobEncodedOffsets []byte
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			logf(DEBUG2, "NOTFOUND boltBucketGetOffsets char=%s buk=%s key=%s", *char, *bucket, *key)
			his.L3Cache.SetOffsets(*char+*bucket+*key, *char, &empty_offsets)
			return nil
		}
		//logf(DEBUG2, "GOT boltBucketGetOffsets char=%s buk=%s key=%s bytes=%d", *char, *bucket, *key, len(v))
		gobEncodedOffsets = v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGetOffsets char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	if len(gobEncodedOffsets) > 0 {
		decodedOffsets, err := gobDecodeOffsets(gobEncodedOffsets, "boltBucketGetOffsets:boltDB")
		if err != nil || decodedOffsets == nil {
			log.Printf("ERROR boltBucketGetOffsets gobDecodeOffsets char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
			return nil, err
		}
		his.Sync_upcounter("BoltDB_decodedOffsets")
		offsets = decodedOffsets

		if offsets != nil {
			his.L3Cache.SetOffsets(*char+*bucket+*key, *char, offsets)
		}
	}
	//if offsets != nil {
	//logf(DEBUG2, "boltBucketGetOffsets returns char=%s buk=%s key=%s err='%v' offsetsN=%d", *char, *bucket, *key, err, len(*offsets))
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

func FNV32(data *string) (*string, *uint32) {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV32

func FNV64(data *string) (*string, *uint64) {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV64

func FNV32S(data *string) *string {
	hash := fnv.New32()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32S

func FNV32aS(data *string) *string {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32aS

func FNV64S(data *string) *string {
	hash := fnv.New64()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64S

func FNV64aS(data *string) *string {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64aS

func gobEncodeHeader(settings *HistorySettings) (*[]byte, error) {
	gob.Register(HistorySettings{})
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		log.Printf("ERROR gobEncodeHeader Encode err='%v'", err)
		return nil, err
	}
	encodedData := buf.Bytes()
	//return &encodedData, nil
	b64 := []byte(base64.StdEncoding.EncodeToString(encodedData))
	return &b64, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData []byte) (*HistorySettings, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		log.Printf("ERROR gobDecodeHeader base64decode err='%v'", err)
		return nil, err
	}
	buf := bytes.NewBuffer([]byte(decoded))
	//buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	settings := &HistorySettings{}
	err = decoder.Decode(settings)
	if err != nil {
		log.Printf("ERROR gobDecodeHeader Decode err='%v'", err)
		return nil, err
	}
	return settings, nil
} // end func gobDecodeHeader

func gobEncodeOffsets(offsets []int64, src string) ([]byte, error) {
	gob.Register(GOBOFFSETS{})
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	gobOffsets := &GOBOFFSETS{Offsets: offsets}
	err := encoder.Encode(*gobOffsets)
	if err != nil {
		log.Printf("ERROR gobEncodeOffsets offsets='%#v' err='%v'", offsets, err)
		return nil, err
	}
	encodedData := buf.Bytes()
	// costly check try decode encodedData
	//if _, err := gobDecodeOffsets(encodedData, src+":test:gobEncodeOffsets"); err != nil {
	//	return nil, err
	//}
	return encodedData, nil
} // end func gobEncodeOffsets

func gobDecodeOffsets(encodedData []byte, src string) (*[]int64, error) {
	buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	//var decodedOffsets []int64
	gobOffsets := &GOBOFFSETS{}
	err := decoder.Decode(gobOffsets)
	if err != nil {
		log.Printf("ERROR gobDecodeOffsets encodedData='%#v' len=%d gobOffsets='%#v' err='%v' src=%s", encodedData, len(encodedData), gobOffsets, err, src)
		return nil, err
	}
	return &gobOffsets.Offsets, nil
	//return &decodedOffsets, nil
} // end func gobDecodeOffsets

func (his *HISTORY) returnBoltHashOpen() {
	<-BoltHashOpen
} /// end func returnBoltHashOpen

func (his *HISTORY) setBoltHashOpen() {
	BoltHashOpen <- struct{}{}
} // end func setBoltHashOpen

func (his *HISTORY) GetBoltHashOpen() int {
	return len(BoltHashOpen)
} // end func GetBoltHashOpen

func (his *HISTORY) BoltSync(db *bolt.DB, char string) error {
	if db == nil {
		return fmt.Errorf("ERROR BoltSync db=nil")
	}
	if char == "" {
		return fmt.Errorf("ERROR BoltSync char=nil")
	}
	his.lockBoltSync()
	//logf(DEBUG9, "LOCKING BoltSync BatchLocks")
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

func (his *HISTORY) returnBatchLocks(char string, buckets []string) {
	for _, bucket := range buckets {
		<-his.BatchLocks[char][bucket]
	}
} // end func returnBatchLocks

func (his *HISTORY) returnBatchLock(char string, bucket string) {
	select {
	case _ = <-his.BatchLocks[char][bucket]:
		// pass
	default:
		log.Printf("ERROR returnBatchLock %s%s BatchLocks empty!?", char, bucket)
		os.Exit(1)
	}
}

func (his *HISTORY) boltSyncClose(db *bolt.DB, char string) error {
	if db == nil {
		return fmt.Errorf("ERROR boltSyncClose db=nil")
	}
	if err := his.BoltSync(db, char); err != nil {
		return err
	}
	logf(DEBUG2, "BoltDB boltSyncClose char=%s", char)
	return db.Close()
} // end func boltSyncClose

func (his *HISTORY) returnBoltSync() {
	<-his.boltSyncChan
} // end func returnBoltSync

func (his *HISTORY) lockBoltSync() {
	his.boltSyncChan <- struct{}{}
} // end func lockBoltSync

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
				return -999, fmt.Errorf("ERROR IndexQuery IndexRetChan closed! error in History_DBZ_Worker")
			}
			/* the possible return values of IndexQuery(..)
			switch isDup {
			case 0:
				// pass, not a duplicate
				return 0
			case 1:
				dupes++
				return 1
			case 2:
				retry++
				return 2
			}
			*/
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
