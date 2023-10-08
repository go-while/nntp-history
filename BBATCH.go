package history

import (
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"time"
)

var (
	DBG_BS_LOG           bool          = true                  // debugs BatchLOG for every batch insert! beware of the memory eating dragon!
	AdaptiveBatchSize    bool                                  // adjusts CharBucketBatchSize=>wCBBS=workerCharBucketBatchSize automagically
	BoltDB_MaxBatchSize  int           = 16                    // default value from boltdb:db.go = 1000
	BoltDB_MaxBatchDelay time.Duration = 10 * time.Millisecond // default value from boltdb:db.go = 10 * time.Millisecond
	CharBucketBatchSize  int           = 16                    // default batchsize per 16 queues/buckets in 16 char dbs = 4096 total hashes queued for writing
)

func (his *HISTORY) boltBucketPutBatch(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset, forced bool, src string, looped bool, lastflush int64, workerCharBucketBatchSize int) (inserted uint64, err error, closed bool) {

	//if len(batchQueue) < CharBucketBatchSize && !forced && lastflush < BatchFlushEvery {
	if len(batchQueue) < workerCharBucketBatchSize && !forced {
		return 0, nil, false
	}

	his.BatchLocks[char][bucket] <- struct{}{}
	defer his.returnBatchLock(char, bucket)

	batch1 := []*BatchOffset{}
	//batch2 := []*BatchOffset{}
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

	var inserted1 uint64
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
				inserted1++
			}
			return err
		}); err != nil {
			log.Printf("ERROR boltBucketPutBatch [%s|%s] err='%v'", char, bucket, err)
			return inserted1, err, closed
		}
		insert1_took := utils.UnixTimeMicroSec() - start1
		if DBG_BS_LOG {
			his.batchLog(&BatchLOG{c: &char, b: &bucket, i: inserted1, t: insert1_took})
		}
		//if int(inserted1) != workerCharBucketBatchSize {
		// debugs adaptive batchsize
		//logf(DEBUG, "INFO bboltPutBatch [%s|%s] B=%05d inserted=%05d src='%s' ( took %d micros ) wCBBS=%d f=%t", char, bucket, len(batch1), inserted1, src, insert1_took, workerCharBucketBatchSize, forced)
		logf(DEBUG, "INFO bboltPutBatch [%s|%s] Batch=%05d Ins=%05d lft=%d wCBBS=%d ...... f=%t ( took %d micros ) ", char, bucket, len(batch1), inserted1, lastflush, workerCharBucketBatchSize, forced, insert1_took)

		//}
	}

	/*
		var inserted2 uint64
		if len(batch2) > 0 { // if no batchqueue is set (almost impossible since it is hardcoded...?)
			//start2 := utils.UnixTimeMicroSec()
			if err := db.Update(func(tx *bolt.Tx) error {
				var err error
				b := tx.Bucket([]byte(bucket))
			batch2insert:
				for _, bo := range batch1 {
					puterr := b.Put([]byte(bo.key), bo.gobEncodedOffsets)
					if puterr != nil {
						err = puterr
						break batch2insert
					}
					inserted2++
				}
				return err
			}); err != nil {
				log.Printf("ERROR boltBucketPutBatch [%s|%s] err='%v'", char, bucket, err)
				return inserted2, err, closed
			}

			//if int(inserted2) != workerCharBucketBatchSize {
			//	logf(DEBUG, "INFO bboltPutBat [%s|%s] batch2=%d inserted2=%d f=%t src='%s' ( took %d micros ) wCBBS=%d", char, bucket, len(batch2), inserted2, forced, src, utils.UnixTimeMicroSec() - start2, workerCharBucketBatchSize)
			//}
		}
	*/
	inserted = inserted1 // + inserted2
	his.Sync_upcounterN("inserted1", inserted1)
	//his.Sync_upcounterN("inserted2", inserted2)
	//logf(DEBUG9, "BATCHED boltBucketPutBatch [%s|%s] ins1=%d ins2=%d Q=%d forced=%t (took %d micros) src=%s", char, bucket, inserted1, inserted2, len(batchQueue), forced, utils.UnixTimeMicroSec()-start, src)
	took := utils.UnixTimeMicroSec() - start
	logf(DEBUG9, "BATCHED boltBucketPutBatch [%s|%s] ins1=%d Q=%d f=%t src=%s (took %d micros) ", char, bucket, inserted1, len(batchQueue), forced, src, took)
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
		return
	}
	his.BatchLogs.mux.Lock()
	defer his.BatchLogs.mux.Unlock()
	log.Printf("CrunchLogs: did=%d dat=%d", his.BatchLogs.did, len(his.BatchLogs.dat))
	if !more {
		return
	}
	var ihi, ilo uint64 // inserted
	var thi, tlo int64  // took
	for _, dat := range his.BatchLogs.dat {
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
	}
	log.Printf("CrunchLogs i=%d:%d t=%d:%d", ilo, ihi, tlo, thi)
} // end func CrunchLogs
