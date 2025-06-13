package history

import (
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

func (his *HISTORY) WatchDB() { // MySQL database performance monitoring
	// this function watches the performance of the MySQL RocksDB
	his.mux.Lock()
	if his.WBR {
		his.mux.Unlock()
		return
	}
	his.WBR = true
	his.mux.Unlock()

	WatchDBTimer := 10 // every N seconds
	uWatchDBTimer := uint64(WatchDBTimer)
	var inserted uint64
	var searches uint64
	ticker := time.NewTicker(time.Duration(WatchDBTimer) * time.Second)
	for range ticker.C {
		insertednow := his.GetCounter("inserted")
		searchesnow := his.GetCounter("searches")
		if insertednow > inserted {
			diff := insertednow - inserted
			pps := float64(diff) / float64(uWatchDBTimer)
			log.Printf("WatchDB: (inserted %.2f/s) (+%d inserted in %ds)", pps, diff, WatchDBTimer)
			inserted = insertednow
		}
		if searchesnow > searches {
			diff := searchesnow - searches
			pps := float64(diff) / float64(uWatchDBTimer)
			log.Printf("WatchDB: (searches %.2f/s) (+%d searches in %ds)", pps, diff, WatchDBTimer)
			searches = searchesnow
		}
	}
	log.Printf("ERROR WatchDB returned!")
} // end func WatchDB
