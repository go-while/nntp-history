package main

import (
	"fmt"
	"github.com/go-while/go-utils"
	"github.com/go-while/nntp-history"
	"github.com/patrickmn/go-cache"
	bolt "go.etcd.io/bbolt"
	"log"
	"time"
)

func main() {
	useHashDB := true
	storageToken := "F" // storagetoken flatfile
	todo := 100000
	parallelTest := 4
	expire, purge := 5*time.Second, 15*time.Second // cache

	// dont change below
	var boltOpts *bolt.Options
	Bolt_SYNC_EVERY := history.Bolt_SYNC_EVERY
	if useHashDB {
		Bolt_SYNC_EVERY = 30
		bO := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 1024 * 1024,
			PageSize:        4 * 1024,
			NoSync:          true,
		}
		boltOpts = &bO
	}
	c := cache.New(expire, purge)
	history.History.History_Boot("", useHashDB, 4, 4, boltOpts, Bolt_SYNC_EVERY)
	P_donechan := make(chan struct{}, parallelTest)
	for p := 1; p <= parallelTest; p++ {

		go func(p int) {
			done, tdone, dupes, added, cachehits := 0, 0, 0, 0, 0 // init counters
			for i := 1; i <= todo; i++ {
				//time.Sleep(time.Nanosecond)
				//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATE DUPLICATES
				hash := utils.Hash256(fmt.Sprintf("%d", utils.Nano())) // GENERATE MORE UNIQ, LESS DUPES
				if _, found := c.Get(hash); found {
					// cache hits
					cachehits++
					continue
				}
				c.Set(hash, "1", cache.DefaultExpiration)
				now := utils.Nano()
				expires := now + 86400*10 // expires in 10 days
				//log.Printf("hash=%s", hash)
				responseChan := make(chan struct{}, 1)
				history.HISTORY_WRITER_CHAN <- &history.HistoryObject{
					MessageIDHash: &hash,
					StorageToken:  &storageToken,
					Arrival:       expires,
					Expires:       now,
					Date:          now - 86400*7,
					ResponseChan:  responseChan,
				}

				select {
				case _, ok := <-responseChan:
					if !ok {
						// responseChan got closed: is not a duplicate
						added++
					} else {
						dupes++
					}
				}
				tdone++
				done++
				if done >= 10000 {
					log.Printf("test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d", p, tdone, todo, added, dupes, cachehits)
					done = 0
				}
			} // end for i
			P_donechan <- struct{}{}
		}(p) // end go func parallel

	} // end for parallel

	for {
		if len(P_donechan) == parallelTest {
			break
		}
		time.Sleep(time.Second)
	}
	history.HISTORY_WRITER_CHAN <- nil // closes workers
	for {
		if len(history.HISTORY_WRITER_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK16) == 0 &&
			history.GetBoltHashOpen() == 0 {
			break
		}
		time.Sleep(time.Second)
	}
	time.Sleep(time.Second * 5)
} // end func main
