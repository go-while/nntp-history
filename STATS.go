package history

import (
	"fmt"
	//"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"time"
)

func (his *HISTORY) Sync_upcounter(k string) {
	if !DEBUG {
		return
	}
	//his.cmux.Lock()
	//his.Counter[k]++
	//his.cmux.Unlock()
	/*
		if len(upcounter) == cap(upcounter) {
			log.Printf("ERROR Sync_upcounter upcounter chan full %d/%d", len(upcounter), cap(upcounter))
			//os.Exit(1)
			//return
		}
	*/
	upcounter <- &UPCOUNTER{K: k, V: 1}
} // end func sync_upcounter

func (his *HISTORY) Sync_upcounterN(k string, v uint64) {
	if !DEBUG {
		return
	}
	/*
		if len(upcounter) == cap(upcounter) {
			log.Printf("ERROR Sync_upcounterN upcounter chan full")
			return
		}
	*/
	upcounter <- &UPCOUNTER{K: k, V: v}
	//his.cmux.Lock()
	//his.Counter[counter] += value
	//his.cmux.Unlock()
} // end func Sync_upcounterN

/*
func (his *HISTORY) GetCounter(k string) uint64 {
	//his.cmux.Lock()
	//retval := his.Counter[k]
	//his.cmux.Unlock()
	//return retval
	return his.GetGoCounter(k)
} // end func GetCounter
*/

type UPCOUNTER struct {
	K string
	V uint64
}

func (his *HISTORY) GetCounter(k string) (v uint64) {
	his.cmux.Lock()
	if his.counterRetChan == nil {
		log.Printf("ERROR GetGoCounter his.counterRetChan=nil")
		return 0
	}

	getcounter <- k
	v = <-his.counterRetChan
	his.cmux.Unlock()
	return v
} // end func GetGoCounter

func (his *HISTORY) GoCounter() {
	log.Printf("Booting GoCounter wait locking")
	//his.mux.Lock()
	//his.mux.Unlock()

	his.cmux.Lock()
	if his.counterRetChan != nil {
		his.cmux.Unlock()
		return
	}
	if his.counterRetChan == nil {
		his.counterRetChan = make(chan uint64, 1)
	}
	his.cmux.Unlock()
	logf(DEBUG2, "Booted GoCounter")
	//os.Exit(1)
	cm := make(map[string]uint64) // TODO replace strings with magicnumbers as keys
	for {
		select {
		case k, ok := <-getcounter:
			if !ok || his.counterRetChan == nil {
				break
			}
			//v = cm[k]
			//logf(DEBUG2, "GoCounter GET k='%s' = v=%d retch=%d/%d", k, v, len(his.counterRetChan), cap(his.counterRetChan))
			his.counterRetChan <- cm[k]
		case c, ok := <-upcounter:
			//logf(DEBUG2, "GoCounter ADD k='%s' + v=%d upcounterCh=%d/%d", c.K, c.V, len(upcounter), cap(upcounter))
			if !ok {
				break
			}
			cm[c.K] += c.V
		}
	}
	log.Printf("STOP GoCounter")
	os.Exit(1)
} // end func GoCounter

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
			log.Printf("WatchBolt: (inserted %d/s) (+%d inserted in 10s)", pps, diff)
			inserted = insertednow
		}
		if batchinsnow > batchins {
			if insertednow > 0 {
				diff := batchinsnow - batchins
				pps := diff / uWatchBoltTimer
				medbatchsize := uint64(insertednow / batchinsnow)
				log.Printf("WatchBolt: (batchins %d/s) (+%d batchins in 10s) medBS=~%d", pps, diff, medbatchsize)
			}
			batchins = batchinsnow
		}
		if searchesnow > searches {
			diff := searchesnow - searches
			pps := diff / uWatchBoltTimer
			log.Printf("WatchBolt: (searches %d/s) (+%d searches in 10s)", pps, diff)
			searches = searchesnow
		}
	}
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
} // end func PrintBoltPerformance

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
		for _, bucket := range ALLBUCKETS {
			if his.BoltDBsMap[char].BoltDB != nil {
				stats, err := his.getBucketStats(his.BoltDBsMap[char].BoltDB, bucket)
				if print {
					log.Printf("BoltStats [%s|%s]:'%+v' err='%#v'", char, bucket, stats, err)
				}
			}
		}
	}
	for _, char := range HEXCHARS {
		for _, bucket := range ALLBUCKETS {
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
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("Bucket not found: %s", bucket)
		}
		stats = b.Stats()
		return nil
	})

	return stats, err
} // end func getBucketStats
