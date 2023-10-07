package history

import (
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	//"time"
)

func (his *HISTORY) boltBucketPutBatch(db *bolt.DB, char string, bucket string, batchQueue chan *BatchOffset, forced bool, src string, looped bool, lastflush int64, workerCharBucketBatchSize int) (inserted uint64, err error, closed bool) {

	if batchQueue == nil {
		log.Printf("ERROR boltBucketPutBatch batchQueue=nil")
		return
	}
	//if len(batchQueue) < CharBucketBatchSize && !forced && lastflush < BatchFlushEvery {
	if len(batchQueue) < workerCharBucketBatchSize && !forced {
		return 0, nil, false
	}

	his.BatchLocks[char][bucket] <- struct{}{}
	defer his.returnBatchLock(char, bucket)

	batch1 := []*BatchOffset{}
	batch2 := []*BatchOffset{}
	start := utils.UnixTimeMilliSec()
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
	}

	var inserted1 uint64
	if len(batch1) > 0 {
		start := utils.UnixTimeMicroSec()
		// locks this char db:bucket for batch writing

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
			log.Printf("ERROR boltBucketPutBatch char=%s bucket=%s err='%v'", char, bucket, err)
			return inserted1, err, closed
		}

		//if int(inserted1) != CharBucketBatchSize && lastflush < BatchFlushEvery {
		if int(inserted1) != workerCharBucketBatchSize {
			took := utils.UnixTimeMicroSec() - start
			logf(DEBUG, "INFO bboltPutBat char=%s bucket=%s batch1=%d forced=%t inserted1=%d src='%s' ( took %d micros ) wCBBS=%d", char, bucket, len(batch1), forced, inserted1, src, took, workerCharBucketBatchSize)
		}
	}

	var inserted2 uint64
	if len(batch2) > 0 { // if no batchqueue is set (almost impossible since it is hardcoded...?)
		// locks this char db:bucket for batch writing

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
			log.Printf("ERROR boltBucketPutBatch char=%s bucket=%s err='%v'", char, bucket, err)
			return inserted2, err, closed
		}
	}

	inserted = inserted1 + inserted2
	his.Sync_upcounterN("inserted1", inserted1)
	his.Sync_upcounterN("inserted2", inserted2)
	logf(DEBUG9, "BATCHED boltBucketPutBatch char=%s bucket=%s ins1=%d ins2=%d Q=%d forced=%t (took %d ms) src=%s", char, bucket, inserted1, inserted2, len(batchQueue), forced, utils.UnixTimeMilliSec()-start, src)
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
