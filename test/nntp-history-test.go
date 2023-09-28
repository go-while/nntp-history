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
	"os"
	"syscall"
	"runtime"
	"time"
)

func main() {
	start := utils.UnixTimeSec()
	numCPU := runtime.NumCPU()
	var offset int64
	var todo int // todo x parallelTest
	var parallelTest int
	var useHashDB bool
	var useGoCache bool
	var gocache *cache.Cache
	var boltOpts *bolt.Options
	var KeyAlgo int
	var KeyLen int
	var debugs int
	flag.Int64Var(&offset, "getHL", -1, "Offset to seek in history")
	flag.IntVar(&todo, "todo", 1000000, "todo per test")
	flag.IntVar(&parallelTest, "p", 4, "runs N tests in parallel")
	flag.IntVar(&numCPU, "gomaxprocs", 4, "Limit CPU cores")
	flag.BoolVar(&useHashDB, "useHashDB", true, "true|false")
	flag.IntVar(&KeyAlgo, "keyalgo", history.HashShort, "11=HashShort|22=FNV32|33=FNV32a|44=FNV64|55=FNV64a")
	flag.IntVar(&KeyLen, "keylen", 6, "md5: 3-32|sha256: 3-64|sha512: 3-128")
	flag.BoolVar(&useGoCache, "useGoCache", true, "true|false")
	flag.IntVar(&debugs, "debugs", -1, "-1 = default|0 = more|1 = all")
	flag.Parse()
	switch debugs {
		case 0:
			history.DEBUG0 = true
		case 1:
			history.DEBUG0 = true
			history.DEBUG1 = true
	}
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("Number of CPU cores: %d/%d\n", numCPU, runtime.NumCPU())
	fmt.Printf("useHashDB: %t | useGoCache: %t\n", useHashDB, useGoCache)
	time.Sleep(3*time.Second)
	storageToken := "F"                                       // storagetoken flatfile
	expireCache, purgeCache := history.DefaultCacheExpires, history.DefaultCachePurge // cache
	readq, writeq := parallelTest, parallelTest
	Bolt_SYNC_EVERYs := history.Bolt_SYNC_EVERYs // gets defaults
	Bolt_SYNC_EVERYn := history.Bolt_SYNC_EVERYn // gets defaults
	HistoryDir := "history"
	HashDBDir := "hashdb"
	// the KeyLen defines length of hash we use as key in 'boltDB[a-f0-9][bucket][key]' minimum is 3
	// KeyLen is only used with `HashShort`. FNV hashes have predefined length.
	// a shorter hash stores more offsets per key
	// a dupecheck checks all offsets per key to match a hash and shorter keys produce more Fseeks to history file.
	// a server with very little messages can go as low as HashLen: 3.
	// one can use debugs to see if keys got added or appended ort if retrieved key has more than 1 offset stored.
	// meaningful range for KeyLen is 5-8. much longer is not better but bloats up the hashdb.
	// KeyLen max 32(-4) with md5
	// KeyLen max 40(-4) with sha1
	// KeyLen max 64(-4) with sha256
	// KeyLen max 128(-4) with sha512
	// KeyLen can be set longer than the hash is, there is a check `cutHashlen` anyways
	// so it should be possible to have variable hashalgos passed in an `HistoryObject` but code tested only with sha256.
	if useHashDB {
		Bolt_SYNC_EVERYs = 60
		Bolt_SYNC_EVERYn = 50000
		history.BoltINITParallel = 4 // default: history.BoltDBs ( can be 1-16 )
		history.BoltSYNCParallel = 1 // default: history.BoltDBs ( can be 1-16 )
		bO := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 1024 * 1024 * 1024,
			PageSize:        4 * 1024,
			NoSync:          true,
			NoFreelistSync: true,
			// If you want to read the entire database fast, you can set MmapFlag to
			// syscall.MAP_POPULATE on Linux 2.6.23+ for sequential read-ahead.
			MmapFlags: syscall.MAP_POPULATE,
		}
		boltOpts = &bO
	}
	if useGoCache {
		gocache = cache.New(expireCache, purgeCache)
	}
	history.History.History_Boot(HistoryDir, HashDBDir, useHashDB, readq, writeq, boltOpts, Bolt_SYNC_EVERYs, Bolt_SYNC_EVERYn, KeyAlgo, KeyLen, gocache)
	if offset >= 0 {
		result, err := history.History.FseekHistoryLine(offset)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("History @offset=%d line='%s'\n", offset, *result)
		os.Exit(0)
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
			var done, tdone, dupes, added, cachehits, retry, adddupes, cachedupes, cacheretry uint64
		fortodo:
			for i := 1; i <= todo; i++ {
				if done >= 250000 {
					log.Printf("RUN test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d retry=%d adddupes=%d cachedupes=%d cacheretry=%d", p, tdone, todo, added, dupes, cachehits, retry, adddupes, cachedupes, cacheretry)
					done = 0
				}
				done++
				//time.Sleep(time.Nanosecond)
				//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
				//hash := utils.Hash256(fmt.Sprintf("%d", i*p)) // GENERATES DUPLICATES
				hash := utils.Hash256(fmt.Sprintf("%d", utils.Nano())) // GENERATES ALMOST NO DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMicroSec())) // GENERATES VERY SMALL AMOUNT OF DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMilliSec())) // GENERATES LOTS OF DUPES

				if gocache != nil { // check go-cache for hash
					//history.History.RMUX.RLock()
					if val, found := gocache.Get(hash); found {
						//history.History.RMUX.RUnlock()
						switch val {
						case "-1":
							// cache hits, already in processing
							cachehits++
						case "1":
							cachedupes++
						case "2":
							cacheretry++
						}
						continue
					}
					//history.History.RMUX.RUnlock()
					//history.History.RMUX.Lock()
					gocache.Set(hash, "-1", expireCache) // adds hash to temporary go-cache with value "-1"
					//history.History.RMUX.Unlock()
				}
				now := utils.UnixTimeSec()
				expires := now + 86400*10 // expires in 10 days
				//log.Printf("hash=%s", hash)

				// creates a single history object for a usenet article
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
							// pass
						case 1:
							dupes++
							// DUPLICATE entry
							//log.Printf("main: DUP hash='%s'", hash)
							continue fortodo
						case 2:
							retry++
							continue fortodo
						}
					} // end select
				}
				// if we are here, hash is not a duplicate in hashdb.
				// place code here to add article to storage and overview
				// when done: send the history object to history_writer
				history.History.WriterChan <- hobj
				if useHashDB && responseChan != nil {
					select {
					case isDup, ok := <-responseChan:
						if !ok {
							// error: responseChan got closed
							log.Printf("ERROR test p=%d responseChan closed! i=%d hash=%s", p, i, hash)
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
			log.Printf("End test p=%d nntp-history done=%d/%d added=%d dupes=%d cachehits=%d retry=%d adddupes=%d cachedupes=%d cacheretry=%d", p, tdone, todo, added, dupes, cachehits, retry, adddupes, cachedupes, cacheretry)
		}(p) // end go func parallel

	} // end for parallel

	for {
		if len(P_donechan) == parallelTest {
			break
		}
		time.Sleep(time.Second)
	}
	history.History.WriterChan <- nil // closes workers
	for {
		if len(history.HISTORY_WRITER_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK) == 0 &&
			len(history.HISTORY_INDEX_LOCK16) == 0 &&
			history.History.GetBoltHashOpen() == 0 {
			break
		}
		time.Sleep(time.Second)
	}
	key_add := history.History.GetCounter("key_add")
	key_app := history.History.GetCounter("key_app")
	total := key_add + key_app
	log.Printf("key_add=%d key_app=%d total=%d", key_add, key_app, total)
	log.Printf("done=%d took %d seconds", todo*parallelTest, utils.UnixTimeSec() - start)
} // end func main
