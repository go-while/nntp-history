package history

import (
	"fmt"
	//"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"time"
)

func (his *HISTORY) Sync_upcounter(k string) {
	if !DEBUG {
		return
	}
	his.cmux.Lock()
	his.Counter[k]++
	his.cmux.Unlock()
} // end func sync_upcounter

func (his *HISTORY) Sync_upcounterN(k string, v uint64) {
	if !DEBUG {
		return
	}
	his.cmux.Lock()
	his.Counter[k] += v
	his.cmux.Unlock()
} // end func Sync_upcounterN

func (his *HISTORY) GetCounter(k string) uint64 {
	his.cmux.Lock()
	retval := his.Counter[k]
	his.cmux.Unlock()
	return retval
} // end func GetCounter

func (his *HISTORY) WatchBolt() {
	his.mux.Lock()
	if his.WBR {
		his.mux.Unlock()
		return
	}
	his.WBR = true
	go his.PrintBoltPerformance()
	his.mux.Unlock()

	uWatchBoltTimer := uint64(WatchBoltTimer)
	var inserted uint64
	var batchins uint64
	var searches uint64
	ticker := time.NewTicker(time.Duration(WatchBoltTimer) * time.Second)
	for range ticker.C {
		insertednow := his.GetCounter("inserted")
		batchinsnow := his.GetCounter("batchins")
		searchesnow := his.GetCounter("searches")
		if insertednow > inserted {
			diff := insertednow - inserted
			pps := diff / uWatchBoltTimer
			log.Printf("WatchBolt: (inserted %d/s) (+%d inserted in %ds)", pps, diff, WatchBoltTimer)
			inserted = insertednow
		}
		if batchinsnow > batchins {
			if insertednow > 0 {
				diff := batchinsnow - batchins
				pps := diff / uWatchBoltTimer
				medbatchsize := uint64(insertednow / batchinsnow)
				log.Printf("WatchBolt: (batchins %d/s) (+%d batchins in %ds) medBS=~%d", pps, diff, medbatchsize, WatchBoltTimer)
			}
			batchins = batchinsnow
		}
		if searchesnow > searches {
			diff := searchesnow - searches
			pps := diff / uWatchBoltTimer
			log.Printf("WatchBolt: (searches %d/s) (+%d searches in %ds)", pps, diff, WatchBoltTimer)
			searches = searchesnow
		}
	}
	log.Printf("ERROR WatchBolt returned!")
} // end func WatchBolt

func (his *HISTORY) PrintBoltPerformance() {
	var tmpTX int
	ticker := time.NewTicker(time.Duration(WatchBoltTimer) * time.Second)
	prevTimestamp := time.Now()
	for range ticker.C {
		his.PrintCacheStats()

		currentTimestamp := time.Now()
		timePassed := currentTimestamp.Sub(prevTimestamp)
		prevTimestamp = currentTimestamp

		// Get the BoltStats
		otx, tx := his.GetBoltStats("", false)
		did := tx - tmpTX
		passed := timePassed.Seconds()
		performance := float64(did) / passed
		if performance > 0 {
			log.Printf("BoltSpeed: %.2f tx/s ( did=%d in %.1f sec ) totalTX=%d otx=%d", performance, did, passed, tx, otx)
		}
		tmpTX = tx
	}
	log.Printf("ERROR PrintBoltPerformance returned")
} // end func PrintBoltPerformance

func (his *HISTORY) GetBoltStat(char string, print bool) (OpenTxN int, TxN int) {
	if !his.useHashDB || char == "" {
		return
	}
	his.BoltDBsMap.dbptr[char].mux.Lock()
	defer his.BoltDBsMap.dbptr[char].mux.Unlock()
	if his.BoltDBsMap.dbptr[char].BoltDB != nil {
		dbstats, err := his.getDBStats(his.BoltDBsMap.dbptr[char].BoltDB)
		if err != nil {
			log.Printf("ERROR BoltStats [%s] err='%v'", char, err)
			return
		}
		OpenTxN += dbstats.OpenTxN
		TxN += dbstats.TxN
		if print {
			logf(DEBUG2, "BoltStats [%s] OpenTxN=%d TxN=%d", char, dbstats.OpenTxN, dbstats.TxN)
		}
	}

	return
} // end func GetBoltStat

func (his *HISTORY) GetBoltStats(char string, print bool) (OpenTxN int, TxN int) {
	if !his.useHashDB {
		return
	}
	if char != "" {
		return his.GetBoltStat(char, print)
	}
	for _, char := range HEXCHARS {
		otx, tx := his.GetBoltStat(char, print)
		OpenTxN += otx
		TxN += tx
	}
	if print {
		log.Printf("BoltStats [%s] OpenTxN=%d TxN=%d", char, OpenTxN, TxN)
	}
	return OpenTxN, TxN
} // end func GetBoltStats

func (his *HISTORY) GetBoltBucketStats(char string, print bool) {
	if !his.useHashDB {
		return
	}
	if char != "" {
		for _, bucket := range ROOTBUCKETS {
			if his.BoltDBsMap.dbptr[char].BoltDB != nil {
				stats, err := his.getBucketStats(his.BoltDBsMap.dbptr[char].BoltDB, bucket)
				if print {
					log.Printf("BoltStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
		}
	}
	for _, char := range HEXCHARS {
		for _, bucket := range ROOTBUCKETS {
			if his.BoltDBsMap.dbptr[char].BoltDB != nil {
				stats, err := his.getBucketStats(his.BoltDBsMap.dbptr[char].BoltDB, bucket)
				if print {
					log.Printf("BucketStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
		}
	}
} // end func GetBoltBucketStats

func (his *HISTORY) getDBStats(db *bolt.DB) (bolt.Stats, error) {
	var stats bolt.Stats
	if !his.useHashDB {
		return stats, nil
	}
	if db == nil {
		return stats, fmt.Errorf("ERROR getDBStats db=nil")
	}
	err := db.View(func(tx *bolt.Tx) error {
		stats = db.Stats()
		return nil
	})
	return stats, err
} // end func getDBStats

func (his *HISTORY) getBucketStats(db *bolt.DB, bucket string) (bolt.BucketStats, error) {
	var stats bolt.BucketStats
	if !his.useHashDB {
		return stats, nil
	}
	if db == nil {
		return stats, fmt.Errorf("ERROR getBucketStats db=nil")
	}
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("Bucket not found: %s", bucket)
		}
		stats = b.Stats()
		return nil
	})

	return stats, err
} // end func getBucketStats
