package main

import (
	"fmt"
	"github.com/go-while/go-utils"
	"github.com/go-while/nntp-history"
	"github.com/patrickmn/go-cache"
	bolt "go.etcd.io/bbolt"
	"log"
	//"strings"
	"flag"
	"time"
)

func main() {
	var offset int64
	var todo int // todo x parallelTest
	var parallelTest int
	var useHashDB bool
	flag.Int64Var(&offset, "getHL", -1, "Offset to seek in history")
	flag.IntVar(&todo, "todo", 1000000, "todo per test")
	flag.IntVar(&parallelTest, "p", 4, "runs N tests in parallel")
	flag.BoolVar(&useHashDB, "useHashDB", true, "<----")
	flag.Parse()
	storageToken := "F"                                       // storagetoken flatfile
	expireCache, purgeCache := 10*time.Second, 30*time.Second // cache
	// dont change below
	var boltOpts *bolt.Options
	Bolt_SYNC_EVERY := history.Bolt_SYNC_EVERY
	HashAlgo := history.HashFNV32 // default hashalgo if no shorthash is used
	// shorthash=true overwrites HashAlgo FNV!
	// and the HashLen defines length of hash we use in hashdb
	// minimum is 5.
	// a shorter hash stores more offsets per key
	// a dupecheck checks all offsets per key to match a hash
	// meaningful range for HashLen is 6-8. longer is not better.
	// max 32 with md5
	// max 40 with sha1
	// max 64 with sha256
	// max 128 with sha512
	ShortHash, HashLen := true, 6
	if useHashDB {
		Bolt_SYNC_EVERY = 30
		bO := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 1024 * 1024 * 1024,
			PageSize:        4 * 1024,
			NoSync:          true,
		}
		boltOpts = &bO
	}
	c := cache.New(expireCache, purgeCache)
	history.History.History_Boot("", useHashDB, 4, 4, boltOpts, Bolt_SYNC_EVERY, HashAlgo, ShortHash, HashLen)
	if offset >= 0 {
		result, err := history.History.FseekHistoryLine(offset)
		// Check for errors.
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		/*
			if strings.HasPrefix(*result, "HT="){
				hh := strings.Split(*result, "=")[1]
			}*/
		fmt.Printf("History @offset=%d line='%s'\n", offset, *result)
		return
	}
	P_donechan := make(chan struct{}, parallelTest)
	for p := 1; p <= parallelTest; p++ {

		go func(p int) {
			var responseChan chan int
			var indexRetChan chan int
			if useHashDB {
				responseChan = make(chan int, 1)
				indexRetChan = make(chan int, 1)
			}
			var done, tdone, dupes, added, cachehits, retry, adddupes uint64
		fortodo:
			for i := 1; i <= todo; i++ {
				if done >= 10000 {
					log.Printf("RUN test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d retry=%d adddupes=%d", p, tdone, todo, added, dupes, cachehits, retry, adddupes)
					done = 0
				}
				done++
				//time.Sleep(time.Nanosecond)
				//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES ONLY DUPLICATES
				hash := utils.Hash256(fmt.Sprintf("%d", utils.Nano())) // GENERATES ALMOST NO DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMicroSec())) // GENERATES VERY SMALL AMOUNT OF DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMilliSec())) // GENERATES LOTS OF DUPES

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
					history.History.IndexChan <- &history.HistoryIndex{Hash: hobj.MessageIDHash, Offset: -1, IndexRetChan: indexRetChan}
					select {
					case isDup, ok := <-indexRetChan:
						if !ok {
							log.Printf("main: ERROR indexRetChan closed! error in History_DBZ_Worker")
							break fortodo
						}
						switch isDup {
						case 0:
							added++
						case 1:
							dupes++
							// DUPLICATE entry
							log.Printf("main: DUP hash='%s'", hash)
							continue fortodo
						case 2:
							retry++
							go func(hobj *history.HistoryObject) {
								time.Sleep(10 * time.Second)
								// retry object ...
							}(hobj)
							continue fortodo
						}
					} // end select
				}
				// if we are here, hash is not a duplicate in hashdb.
				// place code here to add article to storage and overview
				// when done: send the history object to history_writer
				history.HISTORY_WRITER_CHAN <- hobj
				if useHashDB && responseChan != nil {
					select {
					case isDup, ok := <-responseChan:
						if !ok {
							// error: responseChan got closed
							log.Printf("Error test p=%d responseChan closed! i=%d hash=%s", p, i, hash)
							break fortodo
						} else {
							switch isDup {
							case 0:
								added++
							case 1:
								adddupes++
							case 2:
								retry++
							}
						}
					} // end select
				} // end responseChan
				tdone++
			} // end for i
			P_donechan <- struct{}{}
			log.Printf("End test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d retry=%d adddupes=%d", p, tdone, todo, added, dupes, cachehits, retry, adddupes)
		}(p) // end go func parallel

	} // end for parallel

	for {
		if len(P_donechan) == parallelTest {
			break
		}
		time.Sleep(time.Second)
	}
	close(history.HISTORY_WRITER_CHAN) // <- nil // closes workers
	for {
		if len(history.HISTORY_WRITER_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK16) == 0 &&
			history.GetBoltHashOpen() == 0 {
			break
		}
		time.Sleep(time.Second)
	}
} // end func main
