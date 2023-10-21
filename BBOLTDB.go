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
	intBoltDBs                    = 0x10 // do NOT change this! just breaks everything and will not do anything else!
	HashShort                     = 0x0B // 11
	DefaultBoltINITParallel       = intBoltDBs
	DefaultBoltSYNCParallel       = intBoltDBs
	DefaultReplayDistance   int64 = 1024 * 1024
	WCBBS_UL                      = 0xFFFF // adaptive BatchSize => workerCharBucketBatchSize UpperLimit
	WCBBS_LL                      = 0xF    // adaptive BatchSize => workerCharBucketBatchSize LowerLimit
	// KeyLen is used with HashShort
	//  1st char of hash selects boltDB
	//  2nd + 3rd char (+4th char: if 4K his.rootBUCKETS) of hash selects bucket in boltDB
	//  remaining chars [3:$] are used as Key in BoltDB to store offset(s)
	// the key is further divided into 1st+2nd+3rd+... char as sub buckets and remainder used as key in the root.bucket.sub.bucket[3:$]
	//  offsets lead into history.dat and point to start of a line containing the full hash
	MinKeyLen = 8 // goes with HashShort
)

var (
	BoltDBreopenEveryN       = 0x0   // reopens boltDB every N added (not batchins) // very experimental and not working!
	WatchBoltTimer     int64 = 10    // prints bolts stats every N seconds. only with DEBUG
	NoReplayHisDat     bool  = false // can be set before booting to not replay history.dat
	// stop replay HisDat if we got this many OKs with a distance to missing
	// ReplayTestMax depends on bbolt.db.MaxBatchSize. ReplayTestMax should be at least 2x bbolt.db.MaxBatchSize!
	// if the process crashes: do NOT change the MaxBatchSize before starting!
	// ReplayHisDat() needs the same MaxBatchSize!
	ReplayDistance       int64   = DefaultReplayDistance           // defaults to replay at least 128K messages, more if missed ones (not in hashdb) appear.
	QIndexChan           int     = 16                              // Main-indexchan can queue this
	QindexChans          int     = 16                              // every sub-indexchans for a `char` can queue this
	BoltDB_AllocSize     int                                       // if not set defaults: 16 * 1024 * 1024 (min: 1024*1024)
	BoltSyncEveryS       int64   = 60                              // call db.sync() every seconds (only used with 'boltopts.NoSync: true')
	BoltSyncEveryN       uint64  = 500000                          // call db.sync() after N inserts (only used with 'boltopts.NoSync = true')
	BoltINITParallel     int     = DefaultBoltINITParallel         // set this via 'history.BoltINITParallel = 1' before calling History_Boot.
	BoltSYNCParallel     int     = DefaultBoltSYNCParallel         // set this via 'history.BoltSYNCParallel = 1' before calling History_Boot.
	BoltHashOpen                 = make(chan struct{}, intBoltDBs) // dont change this
	HISTORY_INDEX_LOCK           = make(chan struct{}, 1)          // main lock
	HISTORY_INDEX_LOCK16         = make(chan struct{}, intBoltDBs) // sub locks
	empty_offsets        []int64                                   // just nothing

	// adjust root buckets page splitting behavior
	// we mostly do random inserts: lower value should be better?
	RootBucketFillPercent = 1.0

	// adjust sub buckets page splitting behavior
	// unsure if it does anything in sub buckets?
	SubBucketFillPercent = 0.25

	// can be 16 | (default: 256) | 4096 !4K is insane!
	// creates this many batchQueues and more goroutines
	// 16 generates a lot of pagesplits in bbolt
	// 256 is the sweet spot
	// 4096 generates high load as it launches this many and more go routines and queues!
	RootBUCKETSperDB = 16

	// KeyIndex creates 16^N sub buckets in `his.rootBUCKETS
	// KeyIndex cuts (shortens) the KeyLen by this to use as subb.buckets
	// 0 disables sub/nested buckets and uses full Keylen as Key in RootBuckets only.
	KeyIndex = 0

	// ! can't change intBoltDBs !
	//             only rootBUCKETS & KeyIndex
	// intBoltDBs * his.rootBUCKETS * (16^)KeyIndex = n Buckets over all 16 dbs
	//    16      *        16       * (16^)0        =         256
	//    16      *        16       * (16^)1        =        4096
	//    16      *        16       * (16^)2        =       65536
	//    16      *        16       * (16^)3        =     1048576
	//    16      *        16       * (16^)4        =    16777216
	//    16      *        16       * (16^)5        =   268435456
	//    16      *        16       * (16^)6        =  4294967296
	//    16      *        16       * (16^)7        = 68719476736
	//
	//    16      *       256       * (16^)0        =        4096
	//    16      *       256       * (16^)1        =       65536
	//    16      *       256       * (16^)2        =     1048576
	//    16      *       256       * (16^)3        =    16777216
	//    16      *       256       * (16^)4        =   268435456
	//    16      *       256       * (16^)5        =  4294967296
	//    16      *       256       * (16^)6        = 68719476736
	//
	//    16      *      4096       * (16^)0        =       65536
	//    16      *      4096       * (16^)1        =     1048576
	//    16      *      4096       * (16^)2        =    16777216
	//    16      *      4096       * (16^)3        =   268435456
	//    16      *      4096       * (16^)4        =  4294967296
	//    16      *      4096       * (16^)5        = 68719476736
)

func (his *HISTORY) IndexQuery(hash string, indexRetChan chan int, offset int64) (int, error) {
	if !his.useHashDB {
		//log.Printf("INFO return IndexQuery !his.useHashDB")
		return CasePass, nil
	}
	/*
		if hash == nil {
			return -999, fmt.Errorf("ERROR IndexQuery hash=nil")
		}
	*/
	if len(hash) < 32 {
		return -999, fmt.Errorf("ERROR IndexQuery hash=nil")
	}
	if his.useHashDB && his.IndexChan != nil {
		if indexRetChan == nil {
			// for frequent access betters supply a indexRetChan
			indexRetChan = make(chan int, 1)
		}
		//logf(hash == TESTHASH0, "IndexQuery hash='%s' indexRetChan='%#v' offset=%d his.IndexChan=%d/%d", hash, indexRetChan, offset, len(his.IndexChan), cap(his.IndexChan))
		if offset > 0 {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: offset, IndexRetChan: indexRetChan}
		} else {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: -1, IndexRetChan: indexRetChan}
		}
		select {
		case isDup, ok := <-indexRetChan:
			if !ok {
				return -999, fmt.Errorf("ERROR IndexQuery indexRetChan closed! error in boltDB_Worker")
			}
			return isDup, nil
		} // end select
	}
	return -999, fmt.Errorf("ERROR IndexQuery")
} // end func IndexQuery

// boltDB_Init initializes the history database (HashDB) and starts the worker goroutines for processing historical data.
// It creates worker channels for each character in HEXCHARS and launches corresponding worker goroutines.
// The provided boltOpts parameter allows configuring the BoltDB database options.
func (his *HISTORY) boltDB_Init(boltOpts *bolt.Options) {
	if BootHisCli {
		return
	}
	gob.Register(HistorySettings{})

	his.L2Cache.L2CACHE_Boot(his)
	his.L3Cache.L3CACHE_Boot(his)

	his.batchQueues = &BQ{}
	his.batchQueues.BootCh = make(chan struct{}, intBoltDBs*his.rootBUCKETS)         // char [0-9a-f] * bucket [0-9a-f]
	his.batchQueues.Maps = make(map[string]map[string]chan *BatchOffset, intBoltDBs) // maps char : bucket => chan
	//his.BoltDBsMap = make(map[string]*BOLTDB_PTR)                        // maps char => boltDB pointer
	his.BoltDBsMap = &BoltDBs{dbptr: make(map[string]*BOLTDB_PTR, intBoltDBs)}
	his.prioQue = make(map[string]*BBPQ, intBoltDBs)
	his.pqChans = make(map[string]chan struct{}, intBoltDBs)
	his.pqMuxer = make(map[string]*BBPQMUX, intBoltDBs)
	for _, char := range HEXCHARS {
		his.batchQueues.Maps[char] = make(map[string]chan *BatchOffset, intBoltDBs) // maps bucket => chan
		his.BoltDBsMap.dbptr[char] = &BOLTDB_PTR{BoltDB: nil}                       // pointer to boltDB
		his.prioQue[char] = &BBPQ{}
		his.pqChans[char] = make(chan struct{}, 1)
		his.pqMuxer[char] = &BBPQMUX{}
	}
	his.IndexChan = make(chan *HistoryIndex, QIndexChan) // main index chan to query the index
	his.charsMap = make(map[string]int, intBoltDBs)      // maps char from HEXCHARS to i

	if his.boltInitChan != nil {
		log.Printf("ERROR boltDB_Init already loaded")
		return
	}

	if BoltINITParallel == 0 {
		BoltINITParallel = 1
	} else if BoltINITParallel < 0 || BoltINITParallel > intBoltDBs {
		BoltINITParallel = intBoltDBs
	}

	if BoltSYNCParallel == 0 {
		BoltSYNCParallel = 1
	} else if BoltSYNCParallel < 0 || BoltSYNCParallel > intBoltDBs {
		BoltSYNCParallel = intBoltDBs
	}

	if QIndexChan <= 0 {
		QIndexChan = 1
	}

	if QindexChans <= 0 {
		QindexChans = 1
	}

	if DefaultEvictsCapacity < 1024 {
		DefaultEvictsCapacity = 1024
	}

	his.cEvCap = DefaultEvictsCapacity
	his.reopenDBeveryN = BoltDBreopenEveryN
	his.adaptBatch = AdaptBatch
	his.boltInitChan = make(chan struct{}, BoltINITParallel)
	his.boltSyncChan = make(chan struct{}, BoltSYNCParallel)
	//his.BatchLocks = make(map[string]map[string]chan struct{})
	his.BatchLocks = make(map[string]*BATCHLOCKS)
	for i, char := range HEXCHARS {
		his.charsMap[char] = i
		his.indexChans[i] = make(chan *HistoryIndex, QindexChans)
		his.BatchLocks[char] = &BATCHLOCKS{bl: make(map[string]*BLCH)}
		for _, bucket := range ROOTBUCKETS {
			his.BatchLocks[char].bl[bucket] = &BLCH{ch: make(chan struct{}, 1)}
		}
	}
	time.Sleep(time.Millisecond)
	for i, char := range HEXCHARS { // dont move this up into the first for loop or it drops race conditions for nothing...
		go his.boltDB_Worker(char, i, his.indexChans[i], boltOpts)
	}
	go his.boltDB_Index()
	log.Printf("boltDB_Init launched boltDB_Workers & boltDB_Index ... do ReplayHisDat ForcedReplay=%t", ForcedReplay)
	his.ReplayHisDat()
	if ForcedReplay {
		his.mux.Unlock()
		go his.CLOSE_HISTORY()
		CPUBURN() // testing GC and memory arenas after ReplayHisDat
	}

	if DEBUG {
		log.Printf("boltDB_Init launch WatchBolt", ForcedReplay)
		// run manually: go history.History.WatchBolt()
		go his.WatchBolt()
	}
	logf(BootVerbose, "boltDB_Init HashDB='%s.[0-9a-f]'", his.hisDatDB)
	logf(BootVerbose, "  KeyAlgo=%d KeyLen=%d his.indexPar=%d", his.keyalgo, his.keylen, his.indexPar)
	logf(BootVerbose, "  adaptBatch=%t QIndexChan=%d QindexChans=%d", his.adaptBatch, QIndexChan, QindexChans)
	logf(BootVerbose, "  BatchSize=%d  BatchFlushEvery=%d", CharBucketBatchSize, BatchFlushEvery)
} // end func boltDB_Init

// boltDB_Index listens to incoming HistoryIndex structs on the IndexChan channel
// and distributes them to corresponding boltDB_Worker goroutines.
func (his *HISTORY) boltDB_Index() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "boltDB_Index") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "boltDB_Index")
	his.Wait4HashDB()
	//logf(DEBUG2, "Boot boltDB_Index")
	if DEBUG2 {
		defer log.Printf("Quit boltDB_Index")
	}
	waitchan := make(chan struct{}, his.indexPar)
	for p := 1; p <= his.indexPar; p++ {
		waitchan <- struct{}{}
		go func(p int, waitchan chan struct{}) {
			C1 := ""
		forever:
			for {
				select {
				case hi, ok := <-his.IndexChan: // receives a HistoryIndex struct and passes it down to '0-9a-f' workers
					if !ok {
						//logf(DEBUG2, "Stopping boltDB_Index IndexChan closed")
						break forever
					}
					if hi == nil || len(hi.Hash) < 32 { // allow at least md5
						switch his.indexPar {
						case 1:
							close(his.IndexChan)
						default:
							his.IndexChan <- nil
						}
						logf(DEBUG2, "Stopping boltDB_Index IndexChan p=%d/%d received nil pointer", p, his.indexPar)
						break forever
					}
					if hi.Offset == 0 {
						log.Printf("ERROR boltDB_Index offset=0") // must: Offset -1 to checkonly OR Offset > 0 adds to hashDB
						break forever
					}

					if len(hi.Char) == 1 {
						C1 = hi.Char
					} else {
						// gets first char of hash: hash must be lowercase!
						// hex.EncodeToString returns a lowercased string of a hashsum
						C1 = strings.ToLower(string(hi.Hash[0]))
					}
					//logf(hi.Hash == TESTHASH0, "boltDB_Index hash='%s' hi.Offset=%d C1=%s chan=%d/%d",
					//	hi.Hash, hi.Offset, C1, len(his.indexChans[his.charsMap[C1]]), cap(his.indexChans[his.charsMap[C1]]))

					// sends object to hash boltDB_Worker char
					his.indexChans[his.charsMap[C1]] <- hi
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
			InitialMmapSize: 1024 * 1024 * 1024, // yes 128gb! avoids fragmentation on growing?
			PageSize:        64 * 1024,
			// https://github.com/etcd-io/bbolt/issues/401#issuecomment-1424484221
			//NoFreelistSync: true,
			FreelistType: bolt.FreelistMapType,
			//FreelistType:    bolt.FreelistArrayType,
		}
		boltOpts = &defboltOpts
	}
	db, err := bolt.Open(dbpath, 0600, boltOpts)
	if err != nil {
		log.Printf("ERROR HashDB dbpath='%s' err='%v'", dbpath, err)
		return
	}
	if BoltDB_MaxBatchSize > 0xFFFF {
		BoltDB_MaxBatchSize = 0xFFFF
	}
	if BoltDB_MaxBatchSize >= 0 {
		db.MaxBatchSize = BoltDB_MaxBatchSize
	}

	if BoltDB_MaxBatchDelay > 0 && BoltDB_MaxBatchDelay <= 1000 {
		db.MaxBatchDelay = BoltDB_MaxBatchDelay
	}
	if BoltDB_AllocSize > 0xFFFFF {
		db.AllocSize = BoltDB_AllocSize
	}
	//logf(DEBUG2, "HDBZW: INIT HashDB [%s] db='%#v' db.MaxBatchSize=%d db.MaxBatchDelay=%d db.AllocSize=%d", char, db, db.MaxBatchSize, db.MaxBatchDelay, db.AllocSize)
	his.BoltDBsMap.dbptr[char].mux.Lock()
	his.BoltDBsMap.dbptr[char].BoltDB = db
	his.BoltDBsMap.dbptr[char].mux.Unlock()
	tocheck, checked, created := his.rootBUCKETS, 0, 0

	his.boltInitChan <- struct{}{} // locks parallel intializing of boltDBs
	log.Printf("HDBZW [%s] ROOTBUCKETS=%d * SUBBUCKETS=%d", char, len(ROOTBUCKETS), len(SUBBUCKETS))
	for _, bucket := range ROOTBUCKETS {
		retbool, err := his.boltCreateBucket(db, char, bucket)
		if err != nil || !retbool {
			if err == bolt.ErrBucketExists {
				checked++
			} else {
				log.Printf("ERROR HDBZW INIT HashDB boltCreateBucket [%s|%s] err='%v' retbool=%t", char, bucket, err, retbool)
				return
			}
		} else if retbool {
			created++ // <= bucket has been created
			checked++
		}
		//log.Printf("HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tocheck, created, tcheck)
	}
	<-his.boltInitChan
	go his.pqOrchestra(char)

	logf(DEBUG1, "HDBZW [%s] ROOTBUCKETS checked=%d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
	if checked != tocheck || (created > 0 && created != tocheck) {
		log.Printf("ERROR HDBZW INIT [%s] checked %d/%d created=%d/%d", char, checked, tocheck, created, tocheck)
		return
	}
	his.setBoltHashOpen()

	timeout := 90                // waits 90sec for the history.dat file to be created
	time.Sleep(time.Second / 10) // waits a 100ms and breaks, else checks every second
	for {
		if timeout <= 0 {
			log.Printf("ERROR HDBZW [%s] open his.hisDat not found?!", char)
			return
		}
		if utils.FileExists(his.hisDat) {
			break
		}
		time.Sleep(time.Second)
		timeout--
	}

	lastsync := time.Now().Unix()
	var added, processed, dupes, searches, retry, countsearches uint64
	cutHashlen := 9 // 3:9 = 6 chars (1st is db, 2nd+3rd is bucket (bucketsPerDB=256), remaining is used as KeyLen
	if his.keyalgo == HashShort {
		cutHashlen = his.cutFirst + his.keylen
	}

	// char worker opens history.dat for Fseeking
	// allows this char worker to search for offsets in history.dat
	historyfile, err := os.OpenFile(his.hisDat, os.O_RDONLY, 0666)
	if err != nil {
		log.Printf("ERROR HDBZW [%s] os.OpenFile his.hisDat err='%v'", err, char)
		return
	}

	var timer int64 = 128 // milliseconds
	batchQcap := CharBucketBatchSize * 2
	/*
		switch his.rootBUCKETS {
		case 16:
			timer = 64
		case 256:
			timer = 128
		case 4096:
			timer = 512
		}
	*/
	if batchQcap < 1 {
		batchQcap = 1
	}
	closedBuckets := make(chan struct{}, his.rootBUCKETS)
	log.Printf("boltDB_Worker [%s] ROOTBUCKETS=%d booting batchQueues", char, len(ROOTBUCKETS))
	delay, wid := int64(0), 0
	for _, bucket := range ROOTBUCKETS {
		wid++
		//bucket := c1 + c2
		// The batchQueue, like a ravenous dragon, gorges itself on memory, holding twofold the might of the actual CharBucketBatchSize.
		// A daring gamble that ignites the fires of performance, but beware the voracious appetite!
		batchQueue := make(chan *BatchOffset, batchQcap)
		his.batchQueues.mux.Lock()
		his.batchQueues.Maps[char][bucket] = batchQueue
		his.batchQueues.mux.Unlock()

		// delays the start of a worker by batchflushevery divided by len of all root buckets.
		// should somehow result in slowly booting workers to get a better flushing behavior
		// more even distribution on runtime. else all workers boot at the same time and flush at the same time.
		//delay = int64(wid * int(BatchFlushEvery) / len(ROOTBUCKETS))
		// Lo Wang unleashes a legion of batch queues, one for each sacred bucket in this 'char' database.
		// It results in a total of 16 by 16 queues, as the CharBucketBatchSize stands resolute, guarding each [char][bucket] with its mighty power!

		pq := his.prioQue[char]
		pqC := his.pqChans[char]
		pqM := his.pqMuxer[char]
		notifyChan := make(chan struct{}, 1)

		go func(db *bolt.DB, wid int, delay int64, char string, bucket string,
			batchQueue chan *BatchOffset, closedBuckets chan struct{},
			timer int64, pq *BBPQ, pqC chan struct{}, pqM *BBPQMUX, notifyChan chan struct{}) {

			if batchQueue == nil {
				log.Printf("ERROR boltDB_Worker gofunc input batchQueue=nil")
				return
			}
			//logf(DEBUG2, "batchQueue [%s|%s] delayed boot %d ms", char, bucket, delay)
			// every batchQueue adds an empty struct to count Booted.
			wCBBS := CharBucketBatchSize // copy value allows each worker to play with it
			if wCBBS < 0 {
				wCBBS = 1
			}
			batchFlushEvery := BatchFlushEvery
			var lft int64 = -1 // time passed since lastflush in milliseconds
			lastflush := utils.UnixTimeMilliSec()
			var forced, closed bool
			var inserted uint64
			var err error

			//var decr, incr int = 1, 1 // adaptive batchsize
			//var idle int64
			Q, mode := 0, 0
			//wt := 1000

			//his.prioPush(char, pq, pqC, pqM, &BBPQItem{WID: wid, Run: runAT((batchFlushEvery - delay)), NCH: notifyChan, Now: utils.UnixTimeMilliSec()})
			//logf(DEBUG2, "batchQueue [%s] wid=%d boot wait notifyChan", char, wid)
			//pullChan(notifyChan)
			//pushChan(notifyChan)
			//notifyChan <- struct{}{}

			//time.Sleep(time.Duration(delay) * time.Millisecond)

			if !LOCKfunc(his.batchQueues.BootCh, "his.batchQueues.BootCh c="+char) {
				his.CLOSE_HISTORY()
				log.Printf("FATAL ERROR LOCKFUNC batchQueues!!!")
				return
			}
			/*
					waitCH := his.batchQueues.BootCh
				wait: // for other workers
					for {
						if len(waitCH) == len(ROOTBUCKETS)*intBoltDBs {
							break wait
						}
						time.Sleep(time.Millisecond * 250)
					}
			*/
			/*
				wait:
					for {
						if LOCKfunc(his.batchQueues.BootCh, "his.batchQueues.BootCh c="+char) {
							break wait
						}
						if wt <= 0 {
							log.Printf("FATAL ERROR LOCKFUNC batchQueues!!!")
							return // deadfail
						}
						wt--
					}
			*/

			//defaultTimer := timer
			var sleept, sleepn int64
			var minian int64 = 128  // ms min sleeper
			var maxian int64 = 8192 // ms max sleeper
			var median int64 = batchFlushEvery / 10
			var slicelim int = 5 // calculates median lastflush time over N items
			lft_slice := []int64{}
			now := utils.UnixTimeMilliSec()
			bef := now
			logf(DEBUG2, "batchQueue [%s] wid=%d Booted", char, wid)
			lastprintABS2A := now
			lastprintABS2B := now
			lastprintMED := now
			Qcap := cap(batchQueue)
			batchLockChan := his.BatchLocks[char].bl[bucket].ch
		forbatchqueue:
			for {
				if !forced {
					//log.Printf("batchQueue [%s] wid=%d !forced wait4notifyChan", char, wid)
					//select {
					//case <-notifyChan:
					//	//log.Printf("batchQueue [%s] wid=%d received notify wakeup", char, wid)
					//}
					time.Sleep(time.Duration(median) * time.Millisecond)
					//sleept += median
					sleept += median
					sleepn++
				}
				bef = utils.UnixTimeMilliSec()
				Q, inserted, err, closed = his.boltBucketPutBatch(db, char, bucket, batchQueue, Qcap, forced, "gofunc", lft, wCBBS, batchLockChan)
				now = utils.UnixTimeMilliSec()
				if forced { // DEBUG
					logf(wantPrint(DBG_ABS2, &lastprintABS2A, utils.UnixTimeMilliSec(), BatchFlushEvery/4), "DBG_ABS2A forbatchqueue F0 [%s|%s] boltBucketPutBatch F1 Q=%05d inserted=%05d/wCBBS=%05d closed=%t forced=%t timer=%d median=%d lft=%d age=%d err='%v'", char, bucket, Q, inserted, wCBBS, closed, forced, timer, median, lft, now-lastflush, err)
				}
				if closed { // received nil pointer
					logf(his.reopenDBeveryN > 0, "Closed gofunc forbatchqueue [%s|%s]", char, bucket)
					break forbatchqueue // this go func
				}
				if err != nil {
					log.Printf("gofunc char=%s boltBucketPutBatch err='%v'", char, err)
					break forbatchqueue // this go func
				}
				if inserted > 0 {
					lft, lastflush = now-lastflush, now
					logf(wantPrint(DBG_ABS2, &lastprintMED, utils.UnixTimeMilliSec(), 32768),
						"DBG_ABS2 batchQueue [%s|%s] inserted=%d lft=%d forced=%t median=%d sl=[%d] Q=%d/%d (took %d ms) sleept=(%d ms) sleepn=%d",
						char, bucket, inserted, lft, forced, median, len(lft_slice), Q, Qcap, now-bef, sleept, sleepn)
					median = GetMedian(char, bucket, &lft_slice, lft, slicelim, minian, maxian, false)
					sleept, sleepn = 0, 0
				} else {
					//log.Printf("batchQueue [%s|%s] !inserted lft=%d forced=%t median=%d timer=%d len(lft_slice)=%d Q=%d (took %d ms)", char, bucket, now-lastflush, forced, median, timer, len(lft_slice), Q, now-bef)
				}
				//his.prioPush(char, pq, pqC, pqM, &BBPQItem{WID: wid, Run: runAT(median / 2), NCH: notifyChan, Now: now})

				if Q > 0 && lastflush < utils.UnixTimeMilliSec()-batchFlushEvery {
					logf(wantPrint(DBG_ABS2, &lastprintABS2B, utils.UnixTimeMilliSec(), 30000), "DBG_ABS2B forbatchqueue F9 [%s|%s] mode=%d timer=%d Q=%05d forced=%t=>true lft=%d wCBBS=%d", char, bucket, mode, timer, Q, forced, lft, wCBBS)
					forced = true
					time.Sleep(time.Millisecond * 10)
					continue forbatchqueue
				} else {
					// queue has elements: randomly flush early to get some random distribution?
					time.Sleep(time.Millisecond * 10)
					arand, err := generateRandomInt(1, 1000)
					if err == nil && arand == 500 {
						log.Printf("forbatchqueue [%s|%s] arand=%d forced=>true Q=%d median=(%d ms) sleept=%d sleepn=%d", char, bucket, arand, Q, median, sleept, sleepn)
						forced = true
						continue forbatchqueue
					}
				}
				forced = false
				continue forbatchqueue
			} // end forbatchqueue
			UNLOCKfunc(his.batchQueues.BootCh, "his.batchQueues.BootCh")
			// ends this gofunc
			closedBuckets <- struct{}{}
		}(db, wid, delay, char, bucket, batchQueue, closedBuckets, timer, pq, pqC, pqM, notifyChan)
	} // end for ALLBCUKETS

	if CharBucketBatchSize > 0 {
		// wait for batchqueues to boot
		BQtimeout := 399 * 1000
		for {
			time.Sleep(time.Millisecond)
			if len(his.batchQueues.BootCh) == intBoltDBs*his.rootBUCKETS {
				break
			}
			BQtimeout--
			if BQtimeout <= 0 {
				log.Printf("ERROR boltDB_Worker [%s] BQtimeout batchQueues.Boot timeout", char)
				his.CLOSE_HISTORY()
				return
			}
		}
		//log.Printf("BOOTED boltDB_Worker [%s] (took %d ms)", char, 60*1000-BQtimeout)
	} else {
		//log.Printf("BOOTED boltDB_Worker [%s]")
	}
	tmpadded, reopenDBeveryN, reopen := 0, his.reopenDBeveryN, false // experimental
	var key, bucket string
forever:
	for {
		select {
		case hi, ok := <-indexchan: // sub-indexchan receives a HistoryIndex struct for this char from main IndexChan
			if !ok || hi == nil || len(hi.Hash) < 32 { // at least md5
				// receiving a nil object stops history_dbz_worker
				logf(DEBUG9, "Stopping boltDB_Worker indexchan[%s] received nil pointer", char)
				for _, bucket := range ROOTBUCKETS {
					his.batchQueues.Maps[char][bucket] <- nil
				}
				break forever
			}
			bucket = strings.ToLower(string(hi.Hash[1:his.cutFirst])) // get 2 chars (2nd + 3rd) for bucket
			//logf(DEBUG2, "indexchan [%s] bucket=%s char=%s hash=%s offset=%d", char, bucket, hi.Char, hi.Hash, hi.Offset)
			switch his.keyalgo {
			case HashShort:
				if cutHashlen > len(hi.Hash) {
					cutHashlen = len(hi.Hash)
				}
				//shorthash := string(string(*hi.Hash)[his.cutFirst:cutHashlen])
				key = strings.ToLower(string(hi.Hash[his.cutFirst:cutHashlen])) // shorthash
			/*
				case HashFNV32:
					key = FNV32S(hi.Hash)
				case HashFNV32a:
					key = FNV32aS(hi.Hash)
				case HashFNV64:
					key = FNV64S(hi.Hash)
				case HashFNV64a:
					key = FNV64aS(hi.Hash)
			*/
			default:
				log.Printf("ERROR HDBZW unknown switch keyalgo=%x", his.keyalgo)
				his.CLOSE_HISTORY()
				break forever
			}
			//logf(hi.Hash == TESTHASH0, "HDBZW [%s|%s] key='%s' hash='%s' @0x%010x|%d|%x", char, bucket, key, hi.Hash, hi.Offset, hi.Offset, hi.Offset)
			isDup, err := his.DupeCheck(db, char, bucket, key, hi.Hash, hi.Offset, false, historyfile, his.batchQueues.Maps[char][bucket])
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
				//logf(hi.Hash == TESTHASH0, "HDBZW [%s|%s] key='%s' hash='%s' hi.IndexRetChan <- isDup=%d|%x", char, bucket, key, hi.Hash, isDup, isDup)
				hi.IndexRetChan <- isDup
			}
			if hi.Offset == FlagSearch {
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
				tmpadded++
			case CaseDupes:
				dupes++
			case CaseRetry:
				retry++
			default:
				log.Printf("ERROR HDBZW DupeCheck returned unknown switch isDup=%d", isDup)
			}
			if reopenDBeveryN > 0 && tmpadded >= reopenDBeveryN {
				reopen = true
				break forever
			}
			if boltOpts.NoSync == true {
				if added >= BoltSyncEveryN || (added > 0 && lastsync <= time.Now().Unix()-BoltSyncEveryS) {
					err := his.BoltSync(db, char, reopen)
					if err != nil {
						break forever
					}
					added, lastsync = 0, time.Now().Unix()
				}
			}
		} // end select
	} // end forever
	if CharBucketBatchSize > 0 {
		for _, bucket := range ROOTBUCKETS {
			his.batchQueues.Maps[char][bucket] <- nil
			close(his.batchQueues.Maps[char][bucket])
		}
	}
	for _, bucket := range ROOTBUCKETS {
		logf(DEBUG2, "FINAL-BATCH HDBZW [%s|%s]", char, bucket)
		batchLockChan := his.BatchLocks[char].bl[bucket].ch
		batchQueue := his.batchQueues.Maps[char][bucket]
		his.boltBucketPutBatch(db, char, bucket, batchQueue, batchQcap, true, fmt.Sprintf("defer:[%s|%s]", char, bucket), -1, -1, batchLockChan)
	}
	logf(DEBUG2, "Quit HDBZW [%s] ReOpen=%t added=%d dupes=%d processed=%d searches=%d retry=%d", char, reopen, added, dupes, processed, searches, retry)
	//his.Sync_upcounterN("searches", searches)
waiter:
	for {
		if len(closedBuckets) == his.rootBUCKETS {
			break waiter
		}
		time.Sleep(time.Millisecond) // a wait to reopen can take up to 125ms
	}

	if reopen {
		log.Printf("GO ReOpen boltDB_Worker [%s] reopenDBeveryN=%d", char, reopenDBeveryN)
		historyfile.Close()
		his.boltSyncClose(db, char, false, nil)
		his.returnBoltHashOpen()
		go his.boltDB_Worker(char, i, indexchan, boltOpts)
		return
	}

	historyfile.Close()
	his.boltSyncClose(db, char, false, nil)
	//time.Sleep(time.Duration(10 * BoltDB_MaxBatchDelay))
	his.returnBoltHashOpen()
} // end func boltDB_Worker

// DupeCheck checks for duplicate message-ID hashes in a BoltDB bucket.
// It manages offsets associated with message hashes and handles duplicates, ensuring the integrity of the historical data.
// If a hash is a duplicate, it returns 1, otherwise, it returns 0.
// It also handles the creation of new hash entries in the bucket when needed.
func (his *HISTORY) DupeCheck(db *bolt.DB, char string, bucket string, key string, hash string, offset int64, setempty bool, file *os.File, batchQueue chan *BatchOffset) (int, error) {
	// return -999 with err only
	/*
		if db == nil {
			return -999, fmt.Errorf("ERROR DupeCheck db=nil")
		}
	*/
	//logf(hash == TESTHASH0, "DeDup [%s|%s] key='%s' hash='%s'", char, bucket, key, hash)
	var offsets []int64 // retrieves offsets from boltBucketGetOffsets into this
	if _, err := his.boltBucketGetOffsets(db, char, bucket, key, offset, &offsets); err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets [%s|%s] key=%s hash='%s' err='%v'", char, bucket, key, hash, err)
		return -999, err
	}
	len_offsets := len(offsets)
	//logf(hash == TESTHASH0, "DeDup [%s|%s] key='%s' hash='%s' got offsets=%d", char, bucket, key, hash, len_offsets)
	if len_offsets == 0 { // no offsets stored for numhash
		if offset == FlagSearch { // search only
			return CasePass, nil // pass, not a duplicate
		}
		//newoffsets := []int64{offset}
		// add hash=>key:offset to db
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, []int64{offset}, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW DupeCheck char=%s Add boltBucketKeyPutOffsets bucket=%s err='%v'", char, bucket, err)
			return -999, err
		}
		//logf(DEBUG2, "HDBZW char=%s DupeCheck CREATED key=%s hash=%s offset=0x%08x=%d", *char, *key, *hash, *offset, *offset)
		go his.Sync_upcounter("key_add")
		return CaseAdded, nil
	}

	// got offset(s) stored for numhash
	//if len_offsets > 0 {
	if offset > 0 { // is not a FlagSearch
		//logf(DEBUG2, "INFO HDBZW char=%s key=%s tryhash='%s' GOT multiple offsets=%d=%#v +offset=%d", char, key, hash, len_offsets, offsets, offset)
		if len_offsets > 0 {
			go his.Sync_upcounter("appoffset")
		} else {
			go his.Sync_upcounter("addoffset")
		}
	} else {
		// is a search
		if len_offsets > 1 {
			go his.Sync_upcounter("trymultioffsets")
		} else {
			go his.Sync_upcounter("tryoffset")
		}
	}
	//}
	//logf(hash == TESTHASH0, "DeDup [%s|%s] key='%s' hash='%s' check offsets=%d", char, bucket, key, hash, len_offsets)
	for _, check_offset := range offsets {
		// check history for duplicate hash / evades collissions
		//logf(key == TESTKEY || hash == TESTHASH0, "HDBZW [%s|%s] checkFSEEK key='%s' hash='%s' lo=%d check_offset=%d", char, bucket, key, hash, len_offsets, check_offset)
		var historyHash string
		err := his.FseekHistoryMessageHash(file, check_offset, char, bucket, &historyHash)
		if historyHash == "" && err == nil {
			log.Printf("ERROR HDBZW char=%s CHECK DUP bucket=%s historyHash=nil err=nil hash=%s", char, bucket, err, hash)
			return -999, fmt.Errorf("ERROR historyHash=nil err=nil @offset=%d +offset=%d", historyHash, check_offset, offset)
		}
		if err != nil {
			log.Printf("ERROR HDBZW char=%s FseekHistoryMessageHash bucket=%s err='%v' offset=%d", char, bucket, err, check_offset)
			return -999, err
		}
		if historyHash != "" {

			if len(historyHash) == 3 && historyHash == eofhash {
				logf(DEBUG2, "EOF history.dat hash=%s check_offset=%d", hash, check_offset)
				// The history file reached EOF for check_offset, which means the entry was not flushed. Retry later.
				his.L1Cache.Set(hash, char, CaseRetry, FlagExpires)
				return CaseRetry, nil

			} else if historyHash == hash {
				// hash is a duplicate in history.dat
				if offset > 0 { // not a search
					//logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d +offset='%d'", historyHash, check_offset, offset)
				} else { // is a search
					//logf(DEBUG2, "INFO HDBZW DUPLICATE historyHash=%s @offset=%d", historyHash, check_offset)
				}
				his.L1Cache.Set(hash, char, CaseDupes, FlagExpires)
				return CaseDupes, nil
			}
		} else {
			return -999, fmt.Errorf("ERROR DupeCheck historyHash=nil check_offset=%d", check_offset)
		}
	}

	//logf(hash == TESTHASH0, "DeDup [%s|%s] key='%s' hash='%s' checked offsets=%d", char, bucket, key, hash, len_offsets)
	if offset == FlagSearch {
		//logf(hash == TESTHASH0, "DeDup [%s|%s] key='%s' hash='%s' return CasePass", char, bucket, key, hash)
		// search did not find a match in check_offset over range offsets
		return CasePass, nil // pass, not a duplicate
	}

	if offset > 0 {
		//logf(DEBUG, "HDBZW [%s|%s] APPEND key=%s hash=%s +offset=%d offsets=%d='%#v'", char, bucket, key, hash, offset, len(offsets), offsets)
		offsets = append(offsets, offset)
		if err := his.boltBucketKeyPutOffsets(db, char, bucket, key, hash, offset, offsets, setempty, batchQueue); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketKeyPutOffsets [%s|%s] err='%v'", char, bucket, err)
			return -999, err
		}
		//logf(key == TESTKEY||hash == TESTHASH, "HDBZW [%s|%s] APPENDED key=%s hash=%s offset=%d offsets=%d='%#v'", char, bucket, key, hash, offset, len(offsets), offsets)
		//his.L1Cache.Set(hash, char, CaseWrite, FlagNeverExpires) // was CaseDupes before // offset of history entry added to key: hash is a duplicate in cached response now
		if len(offsets) > 1 {
			go his.Sync_upcounter("key_app")
		}
		return CaseAdded, nil
	}

	return -999, fmt.Errorf("ERROR DupeCheck returned unknown error. [%s|%s] key=%s hash=%s offset=%d offsets=%#v", char, bucket, key, hash, offset, offsets)
} // end func DupeCheck

func (his *HISTORY) boltBucketKeyPutOffsets(db *bolt.DB, char string, bucket string, key string, hash string, offset int64, offsets []int64, setempty bool, batchQueue chan *BatchOffset) (err error) {
	/*
		if db == nil {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s db=nil", char)
		}

		if char == "" {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=nil")
		}
		if bucket == "" {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets char=%s bucket=nil", char)
		}
		if key == "" {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] key=nil", char, bucket)
		}
		if offsets == nil {
			return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsets=nil", char, bucket)
		}
	*/
	if !setempty && len(offsets) == 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=0 setempty=%t", char, bucket, setempty)
	}
	if setempty && len(offsets) != 0 {
		return fmt.Errorf("ERROR boltBucketKeyPutOffsets [%s|%s] offsetsN=%d setempty=%t", char, bucket, len(offsets), setempty)
	}
	//encodedOffsets, err := EncodeOffsets(offsets, char, bucket, key, "boltBucketKeyPutOffsets", his)

	// offset of history entry added to key: hash is a temporary duplicate+retry in cached response now
	his.L1Cache.Set(hash, char, CaseWrite, FlagNeverExpires)                                            // was CaseDupes before // boltBucketKeyPutOffsets
	his.L2Cache.SetOffsetHash(offset, hash, FlagNeverExpires)                                           // boltBucketKeyPutOffsets
	his.L3Cache.SetOffsets(char+bucket+key, char, offsets, FlagNeverExpires, "boltBucketKeyPutOffsets") // boltBucketKeyPutOffsets

	var encodedOffsets []byte
	if b, err := concatInt64(offsets, &encodedOffsets); b == 0 || err != nil {
		log.Printf("ERROR boltBucketKeyPutOffsets concatInt64 err='%v' b=0", err)
		return err
	}

	// puts offset into batchQ
	batchQueue <- &BatchOffset{bucket: bucket, key: key, encodedOffsets: encodedOffsets, hash: hash, char: char, offsets: offsets}

	/*
		his.BatchLocks[char][bucket] <- struct{}{}
		defer his.returnBatchLock(char, bucket)

		if err := db.Update(func(tx *bolt.Tx) error {
			var err error
			b := tx.Bucket([]byte(bucket))
			b.FillPercent = 0.3
			puterr := b.Put([]byte(key), encodedOffsets)
			if puterr != nil {
				return puterr
			}
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketKeyPutOffsets [%s|%s] err='%v'", char, bucket, err)
			return err
		}

		his.Sync_upcounter("inserted")
		his.DoCacheEvict(char, hash, 0, char+bucket+key)
		for _, offset := range offsets {
			his.DoCacheEvict(his.L2Cache.OffsetToChar(offset), EmptyStr, offset, EmptyStr)
		}
	*/
	return
} // end func boltBucketKeyPutOffsets

// func (his *HISTORY) boltBucketGetOffsets(db *bolt.DB, char string, bucket string, key string, newoffset int64) (offsets []int64, err error) {
func (his *HISTORY) boltBucketGetOffsets(db *bolt.DB, char string, bucket string, key string, newoffset int64, returnoffsets *[]int64) (int, error) {
	/*
		if db == nil {
			return 0, fmt.Errorf("ERROR boltBucketGetOffsets char=%s db=nil", char)
		}
		if returnoffsets == nil {
			return 0, fmt.Errorf("ERROR boltBucketGetOffsets returnoffsets=nil")
		}
		if char == "" {
			return 0, fmt.Errorf("ERROR boltBucketGetOffsets char=nil")
		}
		if bucket == "" {
			return 0, fmt.Errorf("ERROR boltBucketGetOffsets char=%s bucket=nil", char)
		}
		if key == "" {
			return 0, fmt.Errorf("ERROR boltBucketGetOffsets [%s|%s] key=nil", char, bucket)
		}
	*/

	//if key == TESTKEY {
	//	log.Printf("bBGOs [%s|%s] TESTKEY=%s newoffset=%d", char, bucket, key, newoffset)
	//}
	var offsets []int64
	his.L3Cache.GetOffsets(char+bucket+key, char, &offsets)
	if offsets != nil && len(offsets) >= 0 {
		//logf(key == TESTKEY, "bBGOs [%s|%s] get CACHED key='%s' offsets='%#v' newoffset=%d", char, bucket, key, offsets, newoffset)
		*returnoffsets = offsets
		return len(offsets), nil
	}

	var encodedOffsets []byte
	if err := db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucket))
		var v []byte
		if his.keyIndex <= 0 {
			v = root.Get([]byte(key)) // don't use a subbucket co-exists in boltBucketPutBatc
		} else {
			// use sub buckets
			subbName := string(key[0:his.keyIndex])
			subb := root.Bucket([]byte(subbName)) // subbucket co-exists in boltBucketPutBatch
			if subb == nil {
				//log.Printf("INFO boltBucketGetOffsets [%s|%s] key='%s' subb='%s' sub bucket not found", char, bucket, key, subbName)
				// bucket not yet created
				return nil
			}
			key := string(key[his.keyIndex:])
			v = subb.Get([]byte(key)) // subbucket co-exists in boltBucketPutBatch
		}
		if v == nil {
			//logf(DEBUG2, "NOTFOUND boltBucketGetOffsets [%s|%s] key=%s", char, bucket, key)
			if newoffset == FlagSearch { // is a search
				his.L3Cache.SetOffsets(char+bucket+key, char, empty_offsets, FlagExpires, "boltBucketGetOffsets:empty") // boltBucketGetOffsets
			}
			return nil
		}
		////logf(DEBUG2, "GOT boltBucketGetOffsets [%s|%s] key=%s bytes=%d", *char, *bucket, *key, len(v))
		encodedOffsets = v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGetOffsets [%s|%s] key=%s err='%v'", char, bucket, key, err)
		return 0, err
	}
	if len(encodedOffsets) > 0 {
		var decodedOffsets []int64
		if b, err := parseByteToSlice(encodedOffsets, &decodedOffsets); b == 0 || err != nil {
			log.Printf("ERROR gobDecodeOffsets parseByteToSlice err='%v' encodedData='%s'", err, string(encodedOffsets))
			return 0, err
		}
		//if len(decodedOffsets) == 0 {
		//	log.Printf("ERROR boltBucketGetOffsets gobDecodeOffsets=%d [%s|%s] key=%s err='%v'", len(decodedOffsets), char, bucket, key, err)
		//	return 0, err
		//}
		go his.Sync_upcounter("BoltDB_decodedOffsets")
		*returnoffsets = decodedOffsets

		if len(offsets) > 0 {
			his.L3Cache.SetOffsets(char+bucket+key, char, offsets, FlagExpires, "boltBucketGetOffsets:got") // boltBucketGetOffsets
		}
		return len(offsets), nil
	}
	//if offsets != nil {
	////logf(DEBUG2, "boltBucketGetOffsets returns [%s|%s] key=%s err='%v' offsetsN=%d", *char, *bucket, *key, err, len(*offsets))
	//}
	return 0, nil
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
		his.LockAllBatchLocks(char)
		if err := his.BoltSync(nil, char, false); err != nil {
			return err
		}
		his.returnLockAllBatchLocks(char)
	}
	return nil
} // end func BoltSyncAll

func (his *HISTORY) returnLockAllBatchLocks(char string) {
	logf(DEBUG9, "UNLOCKING BoltSync BatchLocks")
	if his.BatchLocks != nil {
		for _, bucket := range ROOTBUCKETS {
			// unlocks buckets in this char db
			his.returnBatchLock(char, bucket, nil)
		}
	}
	logf(DEBUG9, "UNLOCKED BoltSync BatchLocks")
} // end func returnLockAllBatchLocks

func (his *HISTORY) LockAllBatchLocks(char string) {
	if his.BatchLocks != nil {
		logf(DEBUG9, "LOCKING BoltSync BatchLocks")
		for _, bucket := range ROOTBUCKETS {
			// locks every bucket in this char db for syncing
			his.BatchLocks[char].bl[bucket].ch <- struct{}{}

		}
		logf(DEBUG9, "LOCKED BoltSync BatchLocks")
	}
} // end func LockAllBatchLocks

// Public: BoltSync - for every DB call function with: db=nil and char=[0-9a-f]
func (his *HISTORY) BoltSync(db *bolt.DB, char string, reopen bool) error {
	if db == nil && char == "" { // both cant be nil/empty
		return fmt.Errorf("ERROR BoltSync db=nil char=nil")
	}
	if db == nil && char != "" {
		his.BoltDBsMap.dbptr[char].mux.Lock()
		if his.BoltDBsMap.dbptr[char].BoltDB != nil {
			db = his.BoltDBsMap.dbptr[char].BoltDB
		}
		his.BoltDBsMap.dbptr[char].mux.Unlock()
	}
	if db == nil {
		return fmt.Errorf("ERROR BoltSync [%s] db=nil", char)
	}
	his.lockBoltSync()
	defer his.returnBoltSync()
	if his.batchQueues != nil {
		his.batchQueues.mux.Lock()
		defer his.batchQueues.mux.Unlock()
	}
	startwait := utils.UnixTimeMilliSec()
	his.LockAllBatchLocks(char)
	start := utils.UnixTimeMilliSec()
	waited := start - startwait
	//logf(DEBUG2, "BoltDB SYNC [%s]", char)
	// Manually sync the database to flush changes to disk
	if err := db.Sync(); err != nil {
		log.Printf("ERROR BoltSync [%s] db.Sync failed err='%v'", char, err)
		return err
	}
	logf(DEBUG, "BoltDB SYNC [%s] reopen=%t (took=%d ms) waited=%d", char, reopen, utils.UnixTimeMilliSec()-start, waited)

	his.returnLockAllBatchLocks(char)

	return nil
} // end func BoltSync

func (his *HISTORY) boltSyncClose(db *bolt.DB, char string, reopen bool, boltOpts *bolt.Options) (*bolt.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("ERROR boltSyncClose [%s] db=nil", char)
	}
	if err := his.BoltSync(db, char, reopen); err != nil {
		return nil, err
	}
	his.GetBoltStats(char, DEBUG)
	err := db.Close()
	if err != nil {
		log.Printf("ERROR boltSyncClose [%s] err='%v'", char, err)
		return nil, err
	}
	his.BoltDBsMap.dbptr[char].mux.Lock()
	defer his.BoltDBsMap.dbptr[char].mux.Unlock()

	his.BoltDBsMap.dbptr[char].BoltDB = nil
	if reopen == true && boltOpts != nil {
		his.LockAllBatchLocks(char)

		dbpath := his.hisDatDB + "." + char
		newDB, err := bolt.Open(dbpath, 0600, boltOpts)
		if err != nil || newDB == nil {
			log.Printf("ERROR boltSyncClose dbpath='%s' err='%v'", dbpath, err)
			return nil, err
		}
		db = nil
		his.BoltDBsMap.dbptr[char].BoltDB = newDB
		his.returnLockAllBatchLocks(char)
		return newDB, nil
	}

	logf(DEBUG2, "BoltDB boltSyncClose [%s]", char)
	return nil, err
} // end func boltSyncClose

func (his *HISTORY) returnBoltSync() {
	<-his.boltSyncChan
} // end func returnBoltSync

func (his *HISTORY) lockBoltSync() {
	his.boltSyncChan <- struct{}{}
} // end func lockBoltSync
