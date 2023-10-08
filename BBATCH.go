package history

import (
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"math"
	"os"
	"time"
)

var (
	DBG_BS_LOG           bool               // debugs BatchLOG for every batch insert! beware of the memory eating dragon!
	DBG_FBQ1             bool               // debugs adaptive batchsize in boltBucketPutBatch
	DBG_FBQ2             bool               // debugs adaptive batchsize forbatchqueue in boltDB_Worker
	AdaptiveBatchSize    bool               // adjusts CharBucketBatchSize=>wCBBS=workerCharBucketBatchSize automagically
	BoltDB_MaxBatchDelay time.Duration      // default value from boltdb:db.go = 10 * time.Millisecond
	BoltDB_MaxBatchSize  int           = 16 // default value from boltdb:db.go = 1000
	CharBucketBatchSize  int           = 16 // default batchsize per *16 queues (buckets) in *16 char dbs = 4096 total hashes queued for writing
)

func (his *HISTORY) boltBucketPutBatch(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset, forced bool, src string, looped bool, lastflush int64, workerCharBucketBatchSize int) (inserted uint64, err error, closed bool) {

	//if len(batchQueue) < CharBucketBatchSize && !forced && lastflush < BatchFlushEvery {
	if len(batchQueue) < workerCharBucketBatchSize && !forced {
		return 0, nil, false
	}

	his.BatchLocks[char][bucket] <- struct{}{}
	defer his.returnBatchLock(char, bucket)

	batch1 := []*BatchOffset{}
	start := utils.UnixTimeMicroSec()

fetchbatch:
	for {
		select {
		case bo, ok := <-batchQueue:
			if !ok || bo == nil {
				closed = true
				logf(DEBUG2, "boltBucketPutBatch received nil pointer [%s|%s]", char, bucket)
				break fetchbatch
			}
			if bo.bucket == "" {
				log.Printf("ERROR boltBucketPutBatch bo.bucket empty!")
				continue fetchbatch
			}
			if bo.bucket != bucket {
				err = fmt.Errorf("ERROR boltBucketPutBatch bo.bucket=%s != bucket=%s", bo.bucket, bucket)
				return 0, err, closed
			}
			batch1 = append(batch1, bo)
			if !forced && len(batch1) >= workerCharBucketBatchSize {
				break fetchbatch
			}
		default:
			break fetchbatch
		}
	} // end fetchbatch

	var inserted uint64
	if len(batch1) > 0 {
		start1 := utils.UnixTimeMicroSec()
		if err := db.Batch(func(tx *bolt.Tx) error {
			var err error
			b := tx.Bucket([]byte(bucket))
		batch1insert:
			for _, bo := range batch1 {
				puterr := b.Put([]byte(bo.key), bo.gobEncodedOffsets)
				if puterr != nil {
					err = puterr
					break batch1insert
				}
				inserted++
			}
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketPutBatch [%s|%s] err='%v'", char, bucket, err)
			return inserted, err, closed
		}
		insert1_took := utils.UnixTimeMicroSec() - start1
		if DBG_BS_LOG {
			his.batchLog(&BatchLOG{c: &char, b: &bucket, i: inserted, t: insert1_took})
		}
		// debugs adaptive batchsize
		logf(DBG_FBQ1, "INFO bboltPutBatch [%s|%s] Batch=%05d Ins=%05d wCBBS=%05d lft=%04d f=%d ( took %d micros ) ", char, bucket, len(batch1), inserted, workerCharBucketBatchSize, lastflush, bool2int(forced), insert1_took)
	}

	his.Sync_upcounterN("inserted", inserted)
	//took := utils.UnixTimeMicroSec() - start
	//logf(DEBUG9, "BATCHED boltBucketPutBatch [%s|%s] ins=%d Q=%d f=%t src=%s (took %d micros) ", char, bucket, inserted, len(batchQueue), forced, src, took)
	return inserted, err, closed
} // end func boltBucketPutBatch

func (his *HISTORY) returnBatchLock(char string, bucket string) {
	select {
	case _ = <-his.BatchLocks[char][bucket]:
		// pass
	default:
		log.Printf("ERROR returnBatchLock char=%s BatchLocks empty!?", char)
		os.Exit(1)
	}
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

		// set maps default min/max reversed
		if _, exists := insertHi[percentile]; !exists {
			insertHi[percentile] = 0
		}
		if _, exists := insertLo[percentile]; !exists {
			insertLo[percentile] = math.MaxUint64
		}
		if _, exists := tookHi[percentile]; !exists {
			tookHi[percentile] = 0
		}
		if _, exists := tookLo[percentile]; !exists {
			tookLo[percentile] = math.MaxInt64
		}

		// update hi/lo percentile maps
		if dat.i > insertHi[percentile] {
			insertHi[percentile] = dat.i
		}
		if dat.i < insertLo[percentile] {
			insertLo[percentile] = dat.i
		}
		if dat.t > tookHi[percentile] {
			tookHi[percentile] = dat.t
		}
		if dat.t < tookLo[percentile] {
			tookLo[percentile] = dat.t
		}
	}

	log.Printf("CrunchLogs: BatchSize=%05d:%05d t=%011d:%011d µs", ilo, ihi, tlo, thi)

	// Print specific percentiles
	for _, percentile := range percentiles {
		log.Printf("Percentile %03d%%: BatchSize=%05d:%05d t=%011d:%011d µs percSum=%d",
			percentile, insertLo[percentile], insertHi[percentile], tookLo[percentile], tookHi[percentile],
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
