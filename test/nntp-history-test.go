package main

import (
	"fmt"
	"github.com/go-while/go-utils"
	"github.com/go-while/nntp-history"
	//"github.com/go-while/nntp-overview"
	//"github.com/patrickmn/go-cache"
	bolt "go.etcd.io/bbolt"
	"log"
	//"strings"
	"flag"
	"os"
	"runtime"
	_ "syscall"
	"time"
)

func main() {
	numCPU := runtime.NumCPU()
	var offset int64
	var todo int // todo x parallelTest
	var parallelTest int
	var useHashDB bool
	var boltOpts *bolt.Options
	var KeyAlgo int
	var KeyLen int
	var debugs int
	var BatchSize int
	flag.IntVar(&debugs, "debugs", -1, "-1 = default|0 = more|1 = all")
	flag.Int64Var(&offset, "getHL", -1, "Offset to seek in history")
	flag.IntVar(&todo, "todo", 1000000, "todo per test")
	flag.IntVar(&parallelTest, "p", 4, "runs N tests in parallel")
	flag.IntVar(&numCPU, "gomaxprocs", 4, "Limit CPU cores")
	flag.BoolVar(&useHashDB, "useHashDB", true, "true|false")
	flag.IntVar(&KeyAlgo, "keyalgo", history.HashShort, "11=HashShort|22=FNV32|33=FNV32a|44=FNV64|55=FNV64a")
	flag.IntVar(&KeyLen, "keylen", 6, "md5: 6-32|sha256: 6-64|sha512: 6-128")
	flag.IntVar(&BatchSize, "BatchSize", 1024, "You no mess with Lo Wang!")
	flag.Parse()
	switch debugs {
	case 0:
		history.DEBUG0 = true
	case 1:
		history.DEBUG0 = true
		history.DEBUG1 = true
	case 9:
		history.DEBUG9 = true

	}
	if BatchSize < 16 {
		BatchSize = 16
	}
	runtime.GOMAXPROCS(numCPU)
	fmt.Printf("CPU=%d/%d | useHashDB: %t | jobs=%d | todo=%d | total=%d | keyalgo=%d | keylen=%d | BatchSize=%d\n", numCPU, runtime.NumCPU(), useHashDB, parallelTest, todo, todo*parallelTest, KeyAlgo, KeyLen, BatchSize)
	//time.Sleep(3*time.Second)
	storageToken := "F" // storagetoken flatfile
	HistoryDir := "history"
	HashDBDir := "hashdb"
	// the KeyLen defines length of hash we use as key in 'boltDB[a-f0-9][bucket][key]' minimum is 3
	// KeyLen is only used with `HashShort`. FNV hashes have predefined length.
	// a shorter hash stores more offsets per key
	// a dupecheck checks all offsets per key to match a hash and shorter keys produce more Fseeks to history file.
	// a server with very little messages can go as low as HashLen: 3.
	// one can use debugs to see if keys got added or appended ort if retrieved key has more than 1 offset stored.
	// meaningful range for KeyLen is 5-8. much longer is not better but bloats up the hashdb.
	// KeyLen max 32 with md5
	// KeyLen max 40 with sha1
	// KeyLen max 64 with sha256
	// KeyLen max 128 with sha512
	// KeyLen can be set longer than the hash is, there is a check `cutHashlen` anyways
	// so it should be possible to have variable hashalgos passed in an `HistoryObject` but code tested only with sha256.
	if useHashDB {
		history.BATCHSIZE = BatchSize // this BatchSize queues this amount in db[char][bucket]queuechan
		history.BATCHFLUSH = 2500     // ( can be: 10-5000 ) if BatchSize is not reached within this milliseconds: flush hashdb queues
		// "SYNC" options are only used with 'boltopts.NoSync: true'
		history.Bolt_SYNC_EVERYs = 60    // only used with 'boltopts.NoSync: true'
		history.Bolt_SYNC_EVERYn = 50000 // only used with 'boltopts.NoSync: true'
		//history.BoltSYNCParallel = 1   // ( can be 1-16 ) default: 16 // only used with 'boltopts.NoSync: true' or shutdown
		//history.BoltINITParallel = 4   // ( can be 1-16 ) default: 16
		//history.QueueWriteChan = 1     // ( can be any value > 0 ) default: 16
		//history.QueueIndexChan = 1     // ( can be any value > 0 ) default: 16
		//history.QueueIndexChans = 1    // ( can be any value > 0 ) default: 16
		// DO NOT change any settings while process is running! will produce race conditions!
		bO := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 2 * 1024 * 1024 * 1024, // assign a high value if you expect a lot of load.
			PageSize:        64 * 1024,
			//NoSync:          true,
			//NoFreelistSync: true,
			//FreelistType: "hashmap",
			//FreelistType: "array",
			//MaxBatchSize: 0,
			//AllocSize: 64*1024*1024,
			// If you want to read the entire database fast, you can set MmapFlag to
			// syscall.MAP_POPULATE on Linux 2.6.23+ for sequential read-ahead.
			//MmapFlags: syscall.MAP_POPULATE,
		}
		boltOpts = &bO
	}
	start := utils.UnixTimeSec()
	history.History.History_Boot(HistoryDir, HashDBDir, useHashDB, boltOpts, KeyAlgo, KeyLen)
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
			var IndexRetChan chan int
			if useHashDB {
				responseChan = make(chan int, 1)
				IndexRetChan = make(chan int, 1)
			}
			var spam, spammer, dupes, added, cachehits, retry, adddupes, cachedupes, cacheretry1, errors uint64
			//spammer = uint64(todo)/10
			spammer = 500000
		fortodo:
			for i := 1; i <= todo; i++ {
				if spam >= spammer {
					sum := added + dupes + cachehits + retry + adddupes + cachedupes + cacheretry1
					log.Printf("RUN test p=%d nntp-history added=%d dupes=%d cachehits=%d retry=%d adddupes=%d cachedupes=%d cacheretry1=%d %d/%d", p, added, dupes, cachehits, retry, adddupes, cachedupes, cacheretry1, sum, todo)
					spam = 0
				}
				spam++
				//time.Sleep(time.Nanosecond)
				hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
				//hash := utils.Hash256(fmt.Sprintf("%d", i*p)) // GENERATES DUPLICATES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.Nano())) // GENERATES ALMOST NO DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMicroSec())) // GENERATES VERY SMALL AMOUNT OF DUPES
				//hash := utils.Hash256(fmt.Sprintf("%d", utils.UnixTimeMilliSec())) // GENERATES LOTS OF DUPES
				//log.Printf("hash=%s", hash)
				char := string(hash[0])
				retval := history.History.L1Cache.LockL1Cache(&hash, char, -1) // checks and locks hash for processing
				switch retval {
				case 0:
					history.History.Sync_upcounter("L1CACHE_Lock")
					// pass
				case -1:
					// cache hits, already in processing
					cachehits++
					continue fortodo
				case 1:
					cachedupes++
					continue fortodo
				case 2:
					cacheretry1++
					continue fortodo
				default:
					log.Printf("main: ERROR unknown switch LockL1Cache retval=%d", retval)
					break fortodo
				}

				isDup, err := history.History.IndexQuery(&hash, IndexRetChan)
				if err != nil {
					log.Printf("FALSE IndexQuery hash=%s", hash)
					break fortodo
				}
				switch isDup {
				case 0:
					// pass
				case 1:
					dupes++
					continue fortodo
				case 2:
					retry++
					continue fortodo
				default:
					log.Printf("main: ERROR in response from IndexQuery unknown switch isDup=%d", isDup)
					break fortodo
				}
				// if we are here, hash is not a duplicate in hashdb.
				// place code here to add article to storage and overview
				// when done: send the history object to history_writer

				now := utils.UnixTimeSec()
				expires := now + 86400*10 // expires in 10 days
				//expires := int64(1234) // will expire on next expiry run
				doa := now                // date of article
				// creates a single history object for a usenet article
				hobj := &history.HistoryObject{
					MessageIDHash: &hash,
					Char:          &char,
					StorageToken:  &storageToken,
					Arrival:       now,
					Expires:       expires,
					Date:          doa, // date of article
					ResponseChan:  responseChan,
				}
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
							default:
								errors++
								log.Printf("main: ERROR fortodo unknown switch isDup=%d from responseChan", isDup)
								break fortodo
							}
						}
					} // end select
				} // end responseChan
				//tdone++
			} // end for i
			P_donechan <- struct{}{}
			sum := added + dupes + cachehits + retry + adddupes + cachedupes + cacheretry1
			log.Printf("End test p=%d nntp-history added=%d dupes=%d cachehits=%d retry=%d adddupes=%d cachedupes=%d cacheretry1=%d sum=%d/%d errors=%d", p, added, dupes, cachehits, retry, adddupes, cachedupes, cacheretry1, sum, todo, errors)
		}(p) // end go func parallel
	} // end for parallelTest

	// wait for test to finish
	for {
		if len(P_donechan) == parallelTest {
			break
		}
		time.Sleep(time.Second / 10)
	}

	// close history
	history.History.CLOSE_HISTORY()

	// get some numbers
	key_add := history.History.GetCounter("key_add")
	key_app := history.History.GetCounter("key_app")
	fseeks := history.History.GetCounter("FSEEK")
	fseekeof := history.History.GetCounter("FSEEK_EOF")
	L1CACHE_Lock := history.History.GetCounter("L1CACHE_Lock")
	//L1CACHE_Get := history.History.GetCounter("L1CACHE_Get")
	L2CACHE_Get := history.History.GetCounter("L2CACHE_Get")
	L3CACHE_Get := history.History.GetCounter("L3CACHE_Get")
	BoltDB_decodedOffsets := history.History.GetCounter("BoltDB_decodedOffsets")
	gotmultioffsets := history.History.GetCounter("gotmultioffsets")
	trymultioffsets := history.History.GetCounter("trymultioffsets")
	searches := history.History.GetCounter("searches")
	inserted1 := history.History.GetCounter("inserted1")
	inserted2 := history.History.GetCounter("inserted2")
	total := key_add + key_app
	log.Printf("key_add=%d key_app=%d total=%d fseeks=%d eof=%d BoltDB_decodedOffsets=%d gotmultioffsets=%d trymultioffsets=%d searches=%d inserted1=%d inserted2=%d", key_add, key_app, total, fseeks, fseekeof, BoltDB_decodedOffsets, gotmultioffsets, trymultioffsets, searches, inserted1, inserted2)
	log.Printf("L1LOCK=%d | Get: L2=%d L3=%d", L1CACHE_Lock, L2CACHE_Get, L3CACHE_Get)
	log.Printf("done=%d took %d seconds", todo*parallelTest, utils.UnixTimeSec()-start)
	/*
		runtime.GC()
		time.Sleep(30 * time.Second)
		runtime.GC()
		time.Sleep(30 * time.Second)
	*/
} // end func main
