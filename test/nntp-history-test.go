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
	todo := 100000      // todo x parallelTest
	parallelTest := 4
	expireCache, purgeCache := 10*time.Second, 30*time.Second // cache

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
	c := cache.New(expireCache, purgeCache)
	history.History.History_Boot("", useHashDB, 4, 4, boltOpts, Bolt_SYNC_EVERY)
	P_donechan := make(chan struct{}, parallelTest)
	for p := 1; p <= parallelTest; p++ {

		go func(p int) {
			var responseChan chan bool
			var indexRetChan chan bool
			if useHashDB {
				responseChan = make(chan bool, 1)
				indexRetChan = make(chan bool, 1)
			}
			done, tdone, dupes, added, cachehits := 0, 0, 0, 0, 0 // init counters
		fortodo:
			for i := 1; i <= todo; i++ {
				//time.Sleep(time.Nanosecond)
				//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES DUPLICATES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.Nano())) // GENERATE MORE UNIQ, ALMOST NO DUPES
				hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMicroSec())) // GENERATES VERY SMALL AMOUNT OF DUPES

				// check go-cache for hash
				if _, found := c.Get(hash); found {
					// cache hits, already in processing
					cachehits++
					tdone++
					continue
				}
				c.Set(hash, "1", expireCache) // adds key=hash to temporary go-cache with value "1"
				now := utils.UnixTimeSec()
				expires := now + 86400*10 // expires in 10 days
				//log.Printf("hash=%s", hash)
				//Do := history.DoCheckAdd
				//Do := history.DoCheckHashDupOnly

				// creates the history object
				hobj := &history.HistoryObject{
					MessageIDHash: &hash,
					StorageToken:  &storageToken,
					Arrival:       now,
					Expires:       expires,
					Date:          now - 86400*7,
					ResponseChan:  responseChan,
				}

				// check only if hash is in hashdb: uses offset: -1 !!!
				if useHashDB && history.History.IndexChan != nil {
					var hi = history.HistoryIndex{Hash: hobj.MessageIDHash, Offset: -1, IndexRetChan: indexRetChan}
					//log.Printf("check hashDB hi='%#v'", hi)
					history.History.IndexChan <- &hi
					select {
					case isDup, ok := <-indexRetChan:
						if !ok {
							log.Printf("main: ERROR indexRetChan closed! error in History_DBZ_Worker")
							break fortodo
						}
						if isDup {
							dupes++
							tdone++
							//log.Printf("main: DUP hash='%s'", hash)
							// DUPLICATE entry
							continue fortodo
						}
					} // end select
				}
				// if we are here, hash is not a duplicate in hashdb.
				// place code here to add article to storage

				// send the history object to history_writer
				history.HISTORY_WRITER_CHAN <- hobj
				if useHashDB && responseChan != nil {
					select {
					case isDup, ok := <-responseChan:
						if !ok {
							// error: responseChan got closed
							log.Printf("Error test p=%d responseChan closed! i=%d hash=%s", p, i, hash)
							break fortodo
						} else {
							if isDup {
								dupes++
							} else {
								added++
							}
						}
					} // end select
				} // end responseChan
				tdone++
				done++
				if done >= 10000 {
					log.Printf("test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d", p, tdone, todo, added, dupes, cachehits)
					done = 0
				}
			} // end for i
			P_donechan <- struct{}{}
			log.Printf("End test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d", p, tdone, todo, added, dupes, cachehits)
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
