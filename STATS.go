package history

import (
	//"fmt"
	//"github.com/go-while/go-utils"
	//bolt "go.etcd.io/bbolt"
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
	return
	his.mux.Lock()
	if his.WBR {
		his.mux.Unlock()
		return
	}
	his.WBR = true
	//go his.PrintBoltPerformance()
	his.mux.Unlock()

	WatchBoltTimer := 10 // every N seconds
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
			pps := float64(diff) / float64(uWatchBoltTimer)
			log.Printf("WatchBolt: (inserted %.2f/s) (+%d inserted in %ds)", pps, diff, WatchBoltTimer)
			inserted = insertednow
		}
		if batchinsnow > batchins {
			if insertednow > 0 {
				diff := batchinsnow - batchins
				pps := float64(diff) / float64(uWatchBoltTimer)
				medbatchsize := uint64(insertednow / batchinsnow)
				log.Printf("WatchBolt: (batchins %.2f/s) (+%d batchins in %ds) medBS=~%d", pps, diff, WatchBoltTimer, medbatchsize)
			}
			batchins = batchinsnow
		}
		if searchesnow > searches {
			diff := searchesnow - searches
			pps := float64(diff) / float64(uWatchBoltTimer)
			log.Printf("WatchBolt: (searches %.2f/s) (+%d searches in %ds)", pps, diff, WatchBoltTimer)
			searches = searchesnow
		}
	}
	log.Printf("ERROR WatchBolt returned!")
} // end func WatchBolt
