package history

import (
	"fmt"
	//"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"time"
)

func (his *HISTORY) PrintGetBoltStatsEveryN(char string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		his.GetBoltStats("", false)
	}
} // end func PrintGetBoltStatsEveryN

func (his *HISTORY) PrintGetBoltStatsEvery(char string, interval time.Duration) {
	var tmpTX int
	ticker := time.NewTicker(interval)
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
} // end func PrintGetBoltStatsEvery

func (his *HISTORY) GetBoltStat(char string, print bool) (OpenTxN int, TxN int) {
	if !his.useHashDB {
		return
	}
	his.boltmux.Lock()
	defer his.boltmux.Unlock()
	if his.BoltDBsMap[char].BoltDB != nil {
		dbstats, err := his.getDBStats(his.BoltDBsMap[char].BoltDB)
		if err != nil {
			log.Printf("ERROR BoltStats [%s] err='%v'", char, err)
			return
		}
		OpenTxN += dbstats.OpenTxN
		TxN += dbstats.TxN
		if print {
			log.Printf("BoltStats [%s] OpenTxN=%d TxN=%d", char, dbstats.OpenTxN, dbstats.TxN)
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
	return OpenTxN, TxN
} // end func GetBoltStats

func (his *HISTORY) GetBoltBucketStats(char string, print bool) {
	if !his.useHashDB {
		return
	}
	if char != "" {
		for _, bucket := range HEXCHARS {
			if his.BoltDBsMap[char].BoltDB != nil {
				stats, err := his.getBucketStats(his.BoltDBsMap[char].BoltDB, bucket)
				if print {
					log.Printf("BoltStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
		}
	}
	for _, char := range HEXCHARS {
		for _, bucket := range HEXCHARS {
			if his.BoltDBsMap[char].BoltDB != nil {
				stats, err := his.getBucketStats(his.BoltDBsMap[char].BoltDB, bucket)
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
		bucket := tx.Bucket([]byte(bucket))
		if bucket == nil {
			return fmt.Errorf("Bucket not found: %s", bucket)
		}
		stats = bucket.Stats()
		return nil
	})

	return stats, err
} // end func getBucketStats
