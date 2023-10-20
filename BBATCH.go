package history

import (
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"math"
	"time"
)

var (
	DBG_BS_LOG           bool                           // debugs BatchLOG for every batch insert! beware of the memory eating dragon!
	DBG_ABS1             bool                           // debugs adaptive batchsize in boltBucketPutBatch
	DBG_ABS2             bool                           // debugs adaptive batchsize forbatchqueue in boltDB_Worker
	AdaptBatch           bool                           // automagically adjusts CharBucketBatchSize=>wCBBS=workerCharBucketBatchSize to match BatchFlushEvery
	BatchFlushEvery      int64  = 7168                  // flushes boltDB in batch every N milliseconds (1-...)
	BoltDB_MaxBatchDelay        = 10 * time.Millisecond // default value from boltdb:db.go = 10 * time.Millisecond
	BoltDB_MaxBatchSize  int    = 1000                  // default value from boltdb:db.go = 1000
	CharBucketBatchSize  int    = 512                   // default batchsize per char:bucket batchqueues
	EmptyStr             string                         // used as pointer
)

func (his *HISTORY) getNewDB(char string, db *bolt.DB) *bolt.DB {
	if db == nil {
		dbptr := his.BoltDBsMap.dbptr[char]
		dbptr.mux.Lock()
		//his.BoltDBsMap.dbptr[char].mux.Lock()
		if dbptr.BoltDB != nil {
			db = dbptr.BoltDB
		}
		dbptr.mux.Unlock()
		return db
	}
	return nil
} // end func getNewDB

func (his *HISTORY) boltBucketPutBatch(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset, forced bool, src string, lastflush int64, workerCharBucketBatchSize int) (int, uint64, error, bool) {

	//if len(batchQueue) < CharBucketBatchSize && !forced && lastflush < BatchFlushEvery {
	Q := len(batchQueue)
	Qcap := cap(batchQueue)
	if Q >= int(Qcap/100*95) {
		log.Printf("WARN boltBucketPutBatch [%s|%s] Q=%d/Qcap=%d wCBBS=%d forcing flush!", char, bucket, Q, Qcap, workerCharBucketBatchSize)
		//pass
	} else if Q < workerCharBucketBatchSize && !forced {
		return Q, 0, nil, false
	}

	his.BatchLocks[char].bl[bucket].ch <- struct{}{}
	defer his.returnBatchLock(char, bucket)

	var err error
	var closed bool
	batch1 := []*BatchOffset{}
	//start := utils.UnixTimeMicroSec()

fetchbatch:
	for {
		select {
		case bo, ok := <-batchQueue: // channel receives a BatchOffset struct from boltBucketKeyPutOffsets()
			if !ok || bo == nil {
				closed = true
				//logf(DEBUG2, "boltBucketPutBatch received nil pointer [%s|%s]", char, bucket)
				break fetchbatch
			}
			if bo.char != char {
				log.Printf("ERROR boltBucketPutBatch bo.char nil or empty or mismatch!")
				continue fetchbatch
			}
			if bo.bucket == "" {
				log.Printf("ERROR boltBucketPutBatch bo.bucket nil or empty")
				continue fetchbatch
			}
			if bo.bucket != bucket {
				err = fmt.Errorf("ERROR boltBucketPutBatch bo.bucket=%s != bucket=%s", bo.bucket, bucket)
				return 0, 0, err, closed
			}
			if bo.encodedOffsets == nil || len(bo.encodedOffsets) == 0 {
				log.Printf("ERROR boltBucketPutBatch bo.encodedOffsets nil or empty!")
				continue fetchbatch
			}
			if bo.hash == "" {
				log.Printf("ERROR boltBucketPutBatch bo.hash nil or empty!")
				continue fetchbatch
			}
			if bo.key == "" {
				log.Printf("ERROR boltBucketPutBatch bo.key nil or empty!")
				continue fetchbatch
			}
			// appends to tmp batch
			batch1 = append(batch1, bo)
			if !forced && len(batch1) >= workerCharBucketBatchSize {
				break fetchbatch
			}
		default:
			break fetchbatch
		}
	} // end fetchbatch

	//var freelist int
	var inserted uint64
	if len(batch1) > 0 {
		/*
			if db == nil {
				log.Printf("WARN boltBucketPutBatch db=nil")
				//time.Sleep(time.Second * 5)
				his.boltmux.Lock()
				if his.BoltDBsMap[char].BoltDB != nil {
					db = his.BoltDBsMap[char].BoltDB
				}
				his.boltmux.Unlock()
				if db == nil {
					return Q, 0, fmt.Errorf("boltBucketPutBatch [%s] db=nil", char), true // closed
				}
			}
		*/

		start1 := utils.UnixTimeMicroSec()
		if err := db.Batch(func(tx *bolt.Tx) error {
			var err error
			root := tx.Bucket([]byte(bucket))
			root.FillPercent = RootBucketFillPercent
		batch1insert:
			for _, bo := range batch1 {
				//if bo.key == TESTKEY {
				//	log.Printf("DEBUG [%s|%s] db.Batch TESTKEY='%s' TESTBUK='%s' offsets='%s' bo='%#v'", char, bucket, bo.key, TESTBUK, string(bo.encodedOffsets), bo)
				//}
				if his.keyIndex <= 0 {
					puterr := root.Put([]byte(bo.key), bo.encodedOffsets) // don't use a subbucket co-exists in boltBucketGetOffsets
					if puterr != nil {
						err = puterr
						break batch1insert
					}
				} else {
					// use sub buckets
					subbName := string(bo.key[0:his.keyIndex])
					subb, err := root.CreateBucketIfNotExists([]byte(subbName)) // subbucket co-exists in boltBucketGetOffsets
					if err != nil {
						return err
					}
					key := string(bo.key[his.keyIndex:])
					subb.FillPercent = SubBucketFillPercent
					puterr := subb.Put([]byte(key), bo.encodedOffsets) // subbucket co-exists in boltBucketGetOffsets
					if puterr != nil {
						err = puterr
						break batch1insert
					}

					inserted++
				}
			}
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketPutBatch [%s|%s] db.Batch err='%v'", char, bucket, err)
			return 0, inserted, err, true // closed
		}

		// batch insert to boltDB done, pass to CacheEvict
		for _, bo := range batch1 {
			//if bo.offsets == nil || len(*bo.offsets) == 0 {
			//	log.Printf("ERROR boltBucketPutBatch pre DoCacheEvict bo.offsets nil or empty")
			//	continue
			//}
			//logf(DEBUG2, "INFO boltBucketPutBatch pre DoCacheEvict char=%s hash=%s offsets='%#v' key=%s", *bo.char, *bo.hash, *bo.offsets, *bo.key)
			cachekey := bo.char + bo.bucket + bo.key
			his.DoCacheEvict(bo.char, bo.hash, 0, cachekey)
			for _, offset := range bo.offsets {
				// dont pass the hash down with these offsets as the hash does NOT identify the offsets, but the key!
				his.DoCacheEvict(his.L2Cache.OffsetToChar(offset), EmptyStr, offset, EmptyStr)
			}
		}
		insert1_took := utils.UnixTimeMicroSec() - start1
		if DBG_BS_LOG {
			his.batchLog(&BatchLOG{c: char, b: bucket, i: inserted, t: insert1_took, w: workerCharBucketBatchSize})
		}
		// debugs adaptive batchsize
		logf(DBG_ABS1, "INFO bboltPutBatch [%s|%s] DBG_ABS1 Batch=%05d Ins=%05d wCBBS=%05d lft=%04d f=%d ( took %d micros ) ", char, bucket, len(batch1), inserted, workerCharBucketBatchSize, lastflush, bool2int(forced), insert1_took)
	}
	if inserted > 0 {
		go his.Sync_upcounterN("inserted", inserted) // +N
		go his.Sync_upcounter("batchins")            // ++
	}
	//if freelist > 0 {
	//	log.Printf("INFO boltBucketPutBatch freelist=%d", freelist)
	//}
	return len(batchQueue), inserted, err, closed
} // end func boltBucketPutBatch

func (his *HISTORY) returnBatchLock(char string, bucket string) {
	<-his.BatchLocks[char].bl[bucket].ch
	/*
		select {
		case _ = <-his.BatchLocks[char].bl[bucket].ch:
			// pass
		default:
			log.Printf("ERROR returnBatchLock char=%s BatchLocks empty!? CLOSE_HISTORY", char)
			his.CLOSE_HISTORY()
		}
	*/
} // end func returnBatchLock

func (his *HISTORY) batchLog(log *BatchLOG) {
	if !DBG_BS_LOG {
		return
	}
	his.BatchLogs.mux.Lock()
	his.BatchLogs.dat = append(his.BatchLogs.dat, log)
	his.BatchLogs.did++
	his.BatchLogs.mux.Unlock()
} // end func batchLog

func (his *HISTORY) CrunchBatchLogs(more bool) {
	if !DBG_BS_LOG {
		log.Printf("CrunchBatchLogs DBG_BS_LOG=%t", DBG_BS_LOG)
		return
	}
	his.BatchLogs.mux.Lock()
	defer his.BatchLogs.mux.Unlock()
	log.Printf("CrunchBatchLogs: did=%d dat=%d", his.BatchLogs.did, len(his.BatchLogs.dat))
	if !more {
		return
	}

	var ihi, ilo uint64 = 0, math.MaxUint64 // Initialize to the maximum and minimum possible values reversed
	var thi, tlo int64 = 0, math.MaxInt64   // Initialize to the maximum and minimum possible values reversed
	var whi, wlo int = 0, math.MaxInt       // Initialize to the maximum and minimum possible values reversed

	percentiles := []int{}
	step := 5 // N percent
	for i := step; i <= 100; i += step {
		percentiles = append(percentiles, i)
	}

	// Initialize maps to store high and low values for Insert and Took
	insertHi := make(map[int]uint64)
	insertLo := make(map[int]uint64)
	tookHi := make(map[int]int64)
	tookLo := make(map[int]int64)
	wCBBSHi := make(map[int]int)
	wCBBSLo := make(map[int]int)
	percSums := make(map[int]int)

	len_logs := len(his.BatchLogs.dat)
	for j, dat := range his.BatchLogs.dat {

		thispercentile := int(float64(j) / float64(len_logs) * 100)
		percentile := getPercentileRound(thispercentile, percentiles)
		percSums[percentile]++

		// Update ihi and ilo
		if dat.i > ihi {
			ihi = dat.i
		}
		if dat.i < ilo {
			ilo = dat.i
		}

		// Update thi and tlo
		if dat.t > thi {
			thi = dat.t
		}
		if dat.t < tlo {
			tlo = dat.t
		}

		// Update whi and wlo
		if dat.w > whi {
			whi = dat.w
		}
		if dat.w < wlo && dat.w > 0 { // dont count shutdown -1 value
			wlo = dat.w
		}

		// set maps default min/max reversed
		if _, exists := insertHi[percentile]; !exists {
			insertHi[percentile] = 0
		}
		if _, exists := insertLo[percentile]; !exists {
			insertLo[percentile] = math.MaxUint64
		}
		// set maps default min/max reversed
		if _, exists := tookHi[percentile]; !exists {
			tookHi[percentile] = 0
		}
		if _, exists := tookLo[percentile]; !exists {
			tookLo[percentile] = math.MaxInt64
		}
		// set maps default min/max reversed
		if _, exists := wCBBSHi[percentile]; !exists {
			wCBBSHi[percentile] = 0
		}
		if _, exists := wCBBSLo[percentile]; !exists {
			wCBBSLo[percentile] = math.MaxInt
		}

		// update hi/lo percentile maps
		if dat.i > insertHi[percentile] {
			insertHi[percentile] = dat.i
		}
		if dat.i < insertLo[percentile] {
			insertLo[percentile] = dat.i
		}
		// update hi/lo percentile maps
		if dat.t > tookHi[percentile] {
			tookHi[percentile] = dat.t
		}
		if dat.t < tookLo[percentile] {
			tookLo[percentile] = dat.t
		}
		// update hi/lo percentile maps
		if dat.w > wCBBSHi[percentile] {
			wCBBSHi[percentile] = dat.w
		}
		if dat.w < wCBBSLo[percentile] {
			wCBBSLo[percentile] = dat.w
		}
	}

	log.Printf("CrunchLogs: inserted=%05d:%05d wCBBS=%05d:%05d µs=%010d:%010d", ilo, ihi, wlo, whi, tlo, thi)

	// Print specific percentiles
	for _, percentile := range percentiles {
		if percSums[percentile] == 0 {
			continue
		}
		log.Printf("Range %03d%%: inserted=%05d:%05d wCBBS=%05d:%05d µs=%010d:%010d sum=%d", percentile,
			insertLo[percentile], insertHi[percentile],
			wCBBSLo[percentile], wCBBSHi[percentile],
			tookLo[percentile], tookHi[percentile],
			percSums[percentile])
	}
} // end func CrunchBatchLogs

func getPercentileRound(inputperc int, percentiles []int) int {
	for _, perc := range percentiles {
		if inputperc > perc {
			continue
		}
		if inputperc <= perc {
			//log.Printf("getPercentileRound inputperc=%d ret=%d", inputperc, perc)
			return perc
		}
	}
	log.Printf("ERROR getPercentileRound inputperc=%d ret=-1", inputperc)
	return -1
} // end func getPercentileRound

func bool2int(abool bool) int {
	if abool {
		return 1
	}
	return 0
} // end func bool2int
