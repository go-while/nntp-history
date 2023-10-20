package main

import (
	"fmt"
	"github.com/go-while/go-utils"
	"github.com/go-while/nntp-history"
	"github.com/gorilla/mux"
	//"github.com/go-while/nntp-overview"
	//"github.com/patrickmn/go-cache"
	bolt "go.etcd.io/bbolt"
	"log"
	"net/http"
	hpprof "net/http/pprof"
	//"strings"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"
)

var TESTHASH = history.TESTHASH

func main() {
	numCPU := runtime.NumCPU()
	debug.SetGCPercent(100)
	var offset int64
	var hexoff string
	var todo int // todo x parallelTest
	var parallelTest int
	var useHashDB bool
	var boltOpts *bolt.Options
	var BoltDB_MaxBatchDelay int
	var KeyAlgo int
	var KeyLen int
	var debugs int
	var RebuildHashDB bool
	var PprofAddr string
	var isleep int
	var useL1Cache bool
	var NoSync bool
	var NoGrowSync bool
	var NoFreelistSync bool
	var pprofmem bool
	var RunTCPonly bool
	var BoltDB_PageSize int
	var InitialMmapSize int
	flag.IntVar(&isleep, "isleep", 0, "sleeps N ms in main fortodo")
	flag.StringVar(&PprofAddr, "pprof", "", " listen address:port")
	flag.IntVar(&numCPU, "numcpu", 4, "Limit your CPU cores to Threads/2 !")
	flag.IntVar(&todo, "todo", 1000000, "todo per test")
	flag.IntVar(&parallelTest, "p", 4, "runs N tests in parallel")
	flag.IntVar(&debugs, "debugs", -1, "default:-1|stats:0|more:1|spam:2|batch:9")

	flag.Int64Var(&offset, "getHL", -1, "Offset to seek in history")
	flag.StringVar(&hexoff, "getHEX", "", "Hex 'f075322c83d4ea' Offset to seek in history")

	flag.BoolVar(&useHashDB, "useHashDB", true, "true | false (no dupe check, only history.dat writing)")
	flag.BoolVar(&RebuildHashDB, "RebuildHashDB", false, "rebuild hashDB from history.dat file")

	flag.BoolVar(&useL1Cache, "useL1Cache", true, "[true | false] (works only with useHashDB=false)")

	// keylen & keyalgo are constants once hashdb is initalized!
	// change needs a RebuildHashDB and updating history.dat settings header. but latter func does not exist yet!
	// these 4 make up the HistorySettings too and are constants once DBs are initialized!

	flag.IntVar(&KeyAlgo, "keyalgo", history.HashShort, "11=HashShort (default, no other option)")
	flag.IntVar(&KeyLen, "keylen", 8+3, "min:8 | default:8")
	flag.IntVar(&history.KeyIndex, "keyindex", 3, "1-...") // use n chars of key for sub buckets. cuts KeyLen by this.
	flag.IntVar(&history.RootBUCKETSperDB, "RootBUCKETSperDB", 16, "16, 256, 4096")
	// experimental flags
	flag.BoolVar(&history.BootHisCli, "BootHistoryClient", false, "experimental client/server")
	flag.BoolVar(&RunTCPonly, "RunTCPonly", false, "experimental client/server")

	// profiling
	flag.BoolVar(&history.CPUProfile, "pprofcpu", false, "goes to file 'cpu.pprof.out'")
	flag.BoolVar(&pprofmem, "pprofmem", false, "goes to file 'mem.pprof.out.unixtime()'")

	// options for our pre-batching
	flag.BoolVar(&history.DBG_ABS1, "DBG_ABS1", false, "default: false (debugs adaptive batchsize/wCBBS)")
	flag.BoolVar(&history.DBG_ABS2, "DBG_ABS2", false, "default: false (debugs adaptive batchsize/wCBBS)")
	flag.BoolVar(&history.DBG_BS_LOG, "DBG_BS_LOG", true, "true | false (debug batchlogs)") // debug batchlogs
	flag.BoolVar(&history.AdaptBatch, "AdaptBatch", false, "true | false  (experimental)")  // adaptive batchsize
	flag.IntVar(&history.CharBucketBatchSize, "wCBBS", 1024, "1-... (default: 1024)")

	// lower value than 10000ms produces heavy write loads (only tested on ZFS)
	// detailed insert performance: DBG_ABS1 / DBG_ABS2
	// 32768 ms / 256 RootBucketsPerDB = every 128ms will one of the buckets put its batch to bbolt
	// depends on your load. if you have low volume you can batchflush more frequently.
	// but hiload (100k tx/sec) benefit from higher batchflush values 16k-64k ms.
	// watch out for bboltDB option MaxBatchDelay. affects this somehow. how? do your tests with your load xD
	// start of the app will be delayed by this timeframe to start workers within this timeframe
	// to get a better flushing distribution over the timeframe.
	// choose a pow2 number because buckets are pow2 too
	flag.Int64Var(&history.BatchFlushEvery, "BatchFlushEvery", 32768, "1-.... ms (choose a pow2 number because buckets are pow2 too)")

	// bbolt options

	// a higher BoltDB_MaxBatchDelay than 10ms can boost performance and reduce write bandwidth to disk
	// up to a point where performance degrades but write BW stays very low.
	flag.IntVar(&BoltDB_MaxBatchDelay, "BoltDB_MaxBatchDelay", 25, "milliseconds (default: 10)")

	// BoltDB_MaxBatchSize can change disk write behavior in positive and negative. needs testing.
	flag.IntVar(&history.BoltDB_MaxBatchSize, "BoltDB_MaxBatchSize", 16384, "0-65536 default: -1 = 1000")

	// lower RootBucketFillPercent produces page splits early
	// higher values produce pagesplits at a later time? choose your warrior!
	flag.Float64Var(&history.RootBucketFillPercent, "RootBucketFillPercent", 0.25, "0.1-0.9 default: 0.5")
	flag.Float64Var(&history.SubBucketFillPercent, "SubBucketFillPercent", 0.25, "0.1-0.9 default: 0.5")

	// lower pagesize produces more pagesplits too
	flag.IntVar(&BoltDB_PageSize, "BoltDB_PageSize", 1024, "KB (default: 4)")

	// no need to grow before 1G of size per db
	flag.IntVar(&InitialMmapSize, "BoltDB_InitialMmapSize", 1, "MB (default: 1024)")

	// NoSync: When set to true, the database skips fsync() calls after each commit.
	// This can be useful for bulk loading data, but it's not recommended for normal use.
	flag.BoolVar(&NoSync, "NoSync", false, "bbolt.NoSync: default false!")

	// NoGrowSync: When true, skips the truncate call when growing the database,
	//  but it's only safe on non-ext3/ext4 systems.
	flag.BoolVar(&NoGrowSync, "NoGrowSync", false, "bbolt.NoGrowSync: default false!")

	// NoFreelistSync: When true, the database skips syncing the freelist to disk.
	// This can improve write performance but may require a full database re-sync during recovery.
	flag.BoolVar(&NoFreelistSync, "NoFreelistSync", false, "bbolt.NoFreelistSync")

	flag.BoolVar(&history.ForcedReplay, "ForcedReplay", false, "default: false --- broken!")
	flag.BoolVar(&history.NoReplayHisDat, "NoReplayHisDat", false, "default: false")

	flag.Parse()

	BoltDB_MaxBatchDelay = int(history.BatchFlushEvery / int64(history.RootBUCKETSperDB) / 4) // tuneable
	log.Printf("DEBUG AUTO SETTING: MaxBatchDelay: %d ms", BoltDB_MaxBatchDelay)
	time.Sleep(3 * time.Second)

	if numCPU > 0 {
		runtime.GOMAXPROCS(numCPU)
	}
	if todo <= 0 {
		log.Printf("??? todo=0")
		os.Exit(1)
	}
	if PprofAddr != "" {
		go debug_pprof(PprofAddr)
		time.Sleep(time.Second)
	}
	history.History.SET_DEBUG(debugs)

	//history.DBG_GOB_TEST = true // costly check: test decodes gob encoded data after encoding
	//history.DBG_CGS = true // prints cache grow/shrink
	//history.DBG_BS_LOG = true // this debug eats memory to log all batched writes
	//history.DBG_ABS1 = true // prints adaptive batchsize queue
	//history.DBG_ABS2 = true // prints adaptive batchsize decr/incr
	storageToken := "F" // storagetoken flatfile
	HistoryDir := "history"
	HashDBDir := "hashdb"
	// the KeyLen defines length of hash we use as key in 'boltDB[a-f0-9][bucket][key]' minimum is 3
	// KeyLen is only used with `HashShort`. FNV hashes have predefined length.
	// a shorter hash stores more offsets per key
	// a dupecheck checks all offsets per key to match a hash and shorter keys produce more Fseeks to history file.
	// a server with very little messages can go as low as HashLen: 4.
	// one can use debugs to see if keys got added or appended or if retrieved key has more than 1 offset stored.
	// meaningful range for KeyLen is 5-8. much longer is not better but bloats up the hashdb.
	// KeyLen max 32 with md5
	// KeyLen max 40 with sha1
	// KeyLen max 64 with sha256
	// KeyLen max 128 with sha512
	// KeyLen can be set longer than the hash is, there is a check `cutHashlen` anyways
	// so it should be possible to have variable hashalgos passed in an `HistoryObject` but code tested only with sha256.
	if useHashDB {
		//history.BoltDB_MaxBatchSize = 256                    // = history.his.rootBUCKETS // 0 disables boltdb internal batching. default: 1000
		history.BoltDB_MaxBatchDelay = time.Duration(BoltDB_MaxBatchDelay) * time.Millisecond // default: 10 * time.Millisecond

		// AllocSize is the amount of space allocated when the database
		// needs to create new pages. This is done to amortize the cost
		// of truncate() and fsync() when growing the data file.
		//history.BoltDB_AllocSize = 128 * 1024 * 1024 // default: 16 * 1024 * 1024

		//history.AdaptBatch = true        // automagically adjusts CharBucketBatchSize to match history.BatchFlushEvery // default: false
		//history.CharBucketBatchSize = 256 // ( can be: 1-65536 ) BatchSize per db[char][bucket]queuechan (16*16). default: 64
		//history.BatchFlushEvery = 5000 // ( can be: 500-5000 ) if CharBucketBatchSize is not reached within this milliseconds: flush hashdb queues
		// "SYNC" options are only used with 'boltopts.NoSync: true'
		history.BoltSyncEveryS = 60     // only used with 'boltopts.NoSync: true'
		history.BoltSyncEveryN = 500000 // only used with 'boltopts.NoSync: true'
		//history.BoltSYNCParallel = 1   // ( can be 1-16 ) default: 16 // only used with 'boltopts.NoSync: true' or shutdown
		//history.BoltINITParallel = 4   // ( can be 1-16 ) default: 16 // used when booting and initalizing bolt databases
		//history.NumQueueWriteChan = 1  // ( can be any value > 0 ) default: 16 [note: keep it low!]
		//history.QIndexChan = 1     // ( can be any value > 0 ) default: 16 [note: keep it low(er)!]
		//history.QindexChans = 2 // ( can be any value > 0 ) default: 16 [note: keep it low(er)!]
		//history.IndexParallel = 1 // default: 16
		// DO NOT change any settings while process is running! will produce race conditions!
		bO := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: InitialMmapSize * 1024 * 1024, // assign a high value if you expect a lot of load.
			PageSize:        BoltDB_PageSize * 1024,
			//FreelistType:    bolt.FreelistArrayType,
			FreelistType:   bolt.FreelistMapType,
			NoSync:         NoSync,
			NoGrowSync:     NoGrowSync,
			NoFreelistSync: NoFreelistSync,
			//FreelistType: "hashmap",
			//FreelistType: "array",
			//PreLoadFreelist: ?,
			// If you want to read the entire database fast, you can set MmapFlag to
			// syscall.MAP_POPULATE on Linux 2.6.23+ for sequential read-ahead.
			MmapFlags: syscall.MAP_POPULATE,
		}
		boltOpts = &bO
	}
	start := time.Now().Unix()
	fmt.Printf("ARGS: CPU=%d/%d | jobs=%d | todo=%d | total=%d | keyalgo=%d | keylen=%d | BatchSize=%d | useHashDB: %t\n", numCPU, runtime.NumCPU(), parallelTest, todo, todo*parallelTest, KeyAlgo, KeyLen, history.CharBucketBatchSize, useHashDB)

	history.DefaultACL = make(map[string]bool)
	if history.DEBUG {
		// default: all is blocked from talking to tcp port
		// allows localhost while debugging
		history.DefaultACL["::1"] = true
		history.DefaultACL["127.0.0.1"] = true
	}

	if RunTCPonly {
		history.History.History_Boot(HistoryDir, HashDBDir, useHashDB, boltOpts, KeyAlgo, KeyLen)
		history.History.Wait4HashDB()
		select {}

	} else if history.BootHisCli {
		log.Printf("main: history.BootHisCli=%t", history.BootHisCli)
		//if BootHisCli {
		//	return
		//}
		history.NoReplayHisDat = true
		P_donechan := make(chan struct{}, parallelTest)
		go MemoryProfile(time.Second*30, time.Second*15, pprofmem)
		for p := 1; p <= parallelTest; p++ {

			go history.History.BootHistoryClient("")

			go func(p int) {
				var added, adddupes, addretry, errors uint64
				responseChan := make(chan int, 1)
			fortodo:
				for i := int64(1); i <= int64(todo); i++ {
					//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
					hash := utils.Hash256(strconv.FormatInt(i, 10)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
					//hash := utils.Hash256(strconv.FormatInt(i*p, 10)) // GENERATES DUPLICATES
					//hash := utils.Hash256(strconv.FormatInt(utils.Nano(), 10)) // GENERATES ALMOST NO DUPES
					//hash := utils.Hash256(strconv.FormatInt(utils.UnixTimeMicroSec(), 10)) // GENERATES VERY SMALL AMOUNT OF DUPES
					//hash := utils.Hash256(strconv.FormatInt(utils.UnixTimeMilliSec(), 10)) // GENERATES LOTS OF DUPES
					char := string(hash[0])
					//log.Printf("hash=%s char=%s", hash, char)
					//if hash == TESTHASH {
					//	log.Printf("p=%d processing TESTHASH=%s i=%d", p, hash, i)
					//}
					now := time.Now().Unix()
					expires := now + 86400*10 // expires in 10 days
					//expires := int64(1234) // will expire on next expiry run
					doa := now // date of article
					// creates a single history object for a usenet article
					hobj := &history.HistoryObject{
						MessageIDHash: hash,
						Char:          char,
						StorageToken:  "F",
						Arrival:       now,
						Expires:       expires,
						Date:          doa, // date of article
						ResponseChan:  responseChan,
					}

					// pass to handleRConn()
					history.History.TCPchan <- hobj

					isDup := <-responseChan
					switch isDup {
					case history.CaseAdded:
						added++
					case history.CaseDupes:
						adddupes++
					case history.CaseRetry:
						addretry++
					default:
						errors++
						log.Printf("main: ERROR fortodo unknown switch isDup=%d from TCPchan", isDup)
						break fortodo
					}
				} // end fortodo
				P_donechan <- struct{}{}
			}(p)
		}
		// wait for test to finish
		for {
			if len(P_donechan) == parallelTest {
				break
			}
			time.Sleep(time.Second / 10)
		}
		log.Printf("QUIT HistoryClient")
		os.Exit(0)
	} // end if BootHistoryClient

	// cmdline -getHL offset | -getHEX hexoff
	if hexoff != "" {
		value, err := strconv.ParseInt(hexoff, 16, 64) // reads hex
		if value <= 0 || err != nil {
			log.Printf("main: ERROR hexoff ParseInt err='%v'", err)
			os.Exit(1)
		}
		log.Printf("HEX='%s' => offset=%d", hexoff, offset)
		offset = value
	}
	if offset >= 0 {
		history.NoReplayHisDat = true
		history.BootVerbose = false
	}
	if useHashDB {
		fmt.Printf(" boltOpts='%#v'\n", boltOpts)
	}
	history.History.History_Boot(HistoryDir, HashDBDir, useHashDB, boltOpts, KeyAlgo, KeyLen)
	history.History.Wait4HashDB()
	// check command line arguments to execute commands
	if RebuildHashDB {
		defer history.History.CLOSE_HISTORY()
		err := history.History.RebuildHashDB()
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if offset >= 0 {
		// command line: '-getHL offset'
		result, err := history.History.FseekHistoryLine(offset)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("History @offset=%d line='%s'\n", offset, result)
		os.Exit(0)
	}
	if useHashDB {
		go history.History.WatchBolt()
	}
	//go history.PrintMemoryStatsEvery(30 * time.Second)
	testhashes := []string{} // sequential

	// start test
	/*
		if pprofcpu {
			cpuProfileFile, err := startCPUProfile()
			if err != nil {
				log.Fatal("Could not start CPU profile: ", err)
			}
			defer stopCPUProfile(cpuProfileFile)
		}
	*/
	LOCKONLYTEST := false

	P_donechan := make(chan struct{}, parallelTest)
	go MemoryProfile(time.Second*30, time.Second*15, pprofmem)
	for p := 1; p <= parallelTest; p++ {

		go func(p int, testhashes []string) {
			if offset > 0 {
				return
			}
			// delay start
			//time.Sleep(time.Duration(time.Duration(p*p*p) * time.Second))
			log.Printf("Launch p=%d", p)
			responseChan := make(chan int, 1)
			var IndexRetChan chan int
			if useHashDB {
				IndexRetChan = make(chan int, 1)
			}
			var spam, spammer, dupes, added, cLock, addretry, retry, adddupes, cdupes, cretry1, cretry2, errors, locked uint64
			spammer = 1000 * 1000      // default: spam every 1m done
			if todo >= 100*1000*1000 { // todo 100m
				spammer = 10 * 1000 * 1000 // spam every 10m done
			} else if todo < 1000*1000 { // todo less than 1m
				spammer = uint64(todo) / 25 // spams every 25%
			}
		fortodo:
			for i := int64(1); i <= int64(todo); i++ {
				if isleep > 0 {
					time.Sleep(time.Duration(isleep) * time.Millisecond)
				}
				if spam >= spammer {
					sum := added + dupes + cLock + addretry + retry + adddupes + cdupes + cretry1 + cretry2
					log.Printf("RUN test p=%d nntp-history added=%d dupes=%d cLock=%d addretry=%d retry=%d adddupes=%d cdupes=%d cretry1=%d cretry2=%d %d/%d", p, added, dupes, cLock, addretry, retry, adddupes, cdupes, cretry1, cretry2, sum, todo)
					spam = 0
				}
				spam++
				//hash := utils.Hash256(fmt.Sprintf("%d", i)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
				hash := utils.Hash256(strconv.FormatInt(i, 10)) // GENERATES ONLY DUPLICATES (in parallel or after first run)
				//hash := utils.Hash256(strconv.FormatInt(i*p, 10)) // GENERATES DUPLICATES
				//hash := utils.Hash256(strconv.FormatInt(utils.Nano(), 10)) // GENERATES ALMOST NO DUPES
				//hash := utils.Hash256(strconv.FormatInt(utils.UnixTimeMicroSec(), 10)) // GENERATES VERY SMALL AMOUNT OF DUPES
				//hash := utils.Hash256(strconv.FormatInt(utils.UnixTimeMilliSec(), 10)) // GENERATES LOTS OF DUPES

				char := string(hash[0])
				//log.Printf("hash=%s char=%s", hash, char)
				//if hash == TESTHASH {
				//	log.Printf("p=%d processing TESTHASH=%s i=%d", p, hash, i)
				//}

				if useHashDB || useL1Cache {
					retval := history.History.L1Cache.LockL1Cache(hash, char, history.CaseLock, useHashDB) // checks and locks hash for processing
					switch retval {
					case history.CasePass:
						//history.History.Sync_upcounter("L1CACHE_Lock")
						locked++
						// pass
					case history.CaseLock:
						// cache hits, already in processing
						cLock++
						continue fortodo
					case history.CaseDupes:
						cdupes++
						continue fortodo
					case history.CaseWrite:
						cretry1++
						continue fortodo
					case history.CaseRetry:
						cretry2++
						continue fortodo
					default:
						log.Printf("main: ERROR LockL1Cache unknown switch retval=%d=0x%X", retval, retval)
						break fortodo
					}

					if LOCKONLYTEST {
						continue fortodo
					}
				}

				if useHashDB {
					isDup, err := history.History.IndexQuery(hash, IndexRetChan, history.FlagSearch)
					if err != nil {
						log.Printf("FALSE IndexQuery hash=%s", hash)
						break fortodo
					}
					switch isDup {
					case history.CasePass:
						// pass
					case history.CaseDupes:
						// we locked the hash but IndexQuery replied with Duplicate
						// set L1 cache to Dupe and expire
						//history.History.L1Cache.Set(hash, char, history.CaseDupes, history.FlagExpires)
						history.History.DoCacheEvict(char, hash, 0, history.EmptyStr)
						dupes++
						continue fortodo
					case history.CaseRetry:
						// we locked the hash but IndexQuery replied with Retry
						// set L1 cache to Retry and expire
						//history.History.L1Cache.Set(hash, char, history.CaseRetry, history.FlagExpires)
						history.History.DoCacheEvict(char, hash, 0, history.EmptyStr)
						retry++
						continue fortodo
					default:
						log.Printf("main: ERROR in response from IndexQuery unknown switch isDup=%d", isDup)
						break fortodo
					}
				}

				// if we are here, hash is not a duplicate in hashdb.
				// place code here to add article to storage and overview
				// when done: send the history object to history_writer

				now := time.Now().Unix()
				expires := now + 86400*10 // expires in 10 days
				//expires := int64(1234) // will expire on next expiry run
				doa := now // date of article
				// creates a single history object for a usenet article
				hobj := &history.HistoryObject{
					MessageIDHash: hash,
					Char:          char,
					StorageToken:  storageToken,
					Arrival:       now,
					Expires:       expires,
					Date:          doa, // date of article
					ResponseChan:  responseChan,
				}

				//log.Printf("p=%d i=%d -> AddHistory", p, i)
				isDup := history.History.AddHistory(hobj, useL1Cache)
				switch isDup {
				case history.CaseAdded:
					added++
				case history.CaseDupes:
					adddupes++
				case history.CaseRetry:
					addretry++
				default:
					errors++
					log.Printf("main: ERROR fortodo unknown switch isDup=%d from AddHistory", isDup)
					break fortodo
				}
				//log.Printf("main: p=%d i=%d isDup=%x", p, i, isDup)

			} // end for i todo
			sum := added + dupes + cLock + addretry + retry + adddupes + cdupes + cretry1 + cretry2
			log.Printf("End test p=%d nntp-history added=%d dupes=%d cLock=%d addretry=%d retry=%d adddupes=%d cdupes=%d cretry1=%d cretry2=%d sum=%d/%d errors=%d locked=%d", p, added, dupes, cLock, addretry, retry, adddupes, cdupes, cretry1, cretry2, sum, todo, errors, locked)
			P_donechan <- struct{}{}
		}(p, testhashes) // end go func parallel
	} // end for parallelTest

	// wait for test to finish
	for {
		if len(P_donechan) == parallelTest {
			break
		}
		time.Sleep(time.Second / 10)
	}
	took := time.Now().Unix() - start
	log.Printf("main: CLOSING HISTORY")
	// close history
	closewait := time.Now().Unix()
	history.History.CLOSE_HISTORY()
	go MemoryProfile(time.Second*10, 0, pprofmem)
	waited := time.Now().Unix() - closewait

	// get some numbers
	key_add := history.History.GetCounter("key_add")
	key_app := history.History.GetCounter("key_app")
	fseeks := history.History.GetCounter("FSEEK")
	fseekeof := history.History.GetCounter("FSEEK_EOF")
	L1CACHE_Get, _ := history.History.L1Cache.L1Stats("Count_Get")
	L2CACHE_Get, _ := history.History.L2Cache.L2Stats("Count_Get")
	L3CACHE_Get, _ := history.History.L3Cache.L3Stats("Count_Get")
	L1CACHE_Set, _ := history.History.L1Cache.L1Stats("Count_Set")
	L2CACHE_Set, _ := history.History.L2Cache.L2Stats("Count_Set")
	L3CACHE_Set, _ := history.History.L3Cache.L3Stats("Count_Set")
	L1CACHE_SetX, _ := history.History.L1Cache.L1Stats("Count_FlagEx")
	L2CACHE_SetX, _ := history.History.L2Cache.L2Stats("Count_FlagEx")
	L3CACHE_SetX, _ := history.History.L3Cache.L3Stats("Count_FlagEx")
	BoltDB_decodedOffsets := history.History.GetCounter("BoltDB_decodedOffsets")
	addoffset := history.History.GetCounter("addoffset")
	appoffset := history.History.GetCounter("appoffset")
	trymultioffsets := history.History.GetCounter("trymultioffsets")
	tryoffset := history.History.GetCounter("tryoffset")
	searches := history.History.GetCounter("searches")
	inserted := history.History.GetCounter("inserted")
	total := key_add + key_app
	log.Printf("key_add=%d key_app=%d total=%d fseeks=%d eof=%d BoltDB_decodedOffsets=%d addoffset=%d appoffset=%d trymultioffsets=%d tryoffset=%d searches=%d inserted=%d", key_add, key_app, total, fseeks, fseekeof, BoltDB_decodedOffsets, addoffset, appoffset, trymultioffsets, tryoffset, searches, inserted)
	log.Printf("L1=%d:%d:%d L2=%d:%d:%d L3=%d:%d:%d", L1CACHE_Get, L1CACHE_Set, L1CACHE_SetX, L2CACHE_Get, L2CACHE_Set, L2CACHE_SetX, L3CACHE_Get, L3CACHE_Set, L3CACHE_SetX)
	log.Printf("done=%d (took %d seconds) (closewait %d seconds)", todo*parallelTest, took, waited)

	if total > 0 {
		history.History.CrunchBatchLogs(true)
	}

	tmax := 5
	for t := 1; t <= tmax; t++ {
		history.PrintMemoryStats()
		log.Printf("runtime.GC() [ %d / %d ] sleep 10 sec", t, tmax)
		runtime.GC()
		time.Sleep(10 * time.Second)
	}
} // end func main

func debug_pprof(addr string) {
	router := mux.NewRouter()
	router.Handle("/debug/pprof/", http.HandlerFunc(hpprof.Index))
	router.Handle("/debug/pprof/cmdline", http.HandlerFunc(hpprof.Cmdline))
	router.Handle("/debug/pprof/profile", http.HandlerFunc(hpprof.Profile))
	router.Handle("/debug/pprof/symbol", http.HandlerFunc(hpprof.Symbol))
	router.Handle("/debug/pprof/trace", http.HandlerFunc(hpprof.Trace))
	router.Handle("/debug/pprof/{cmd}", http.HandlerFunc(hpprof.Index)) // special handling for Gorilla mux
	server := &http.Server{Addr: addr, Handler: router}

	// go launch debug http
	go func() {
		if err := server.ListenAndServe(); err != nil {
			// handle err
			log.Printf("debug_pprof ERROR server.ListenAndServe err='%v'", err)
			os.Exit(1)
		}
	}()
} // end func debug_pprof

func MemoryProfile(duration time.Duration, wait time.Duration, pprofmem bool) error {
	if !pprofmem {
		return nil
	}
	// Generate a unique filename with a timestamp
	filename := fmt.Sprintf("mem.pprof.out.%d", time.Now().Unix())
	time.Sleep(wait)
	log.Printf("capture MemoryProfile duration=%#v waited=%#v", duration, wait)
	// Create the profile file
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	// Start memory profiling
	pprof.Lookup("heap").WriteTo(f, 0)
	// Sleep for the specified duration to capture the memory profile
	time.Sleep(duration)
	return nil
}

func startCPUProfile() (*os.File, error) {
	log.Printf("startCPUProfile")
	cpuProfileFile, err := os.Create("cpu.pprof.out")
	if err != nil {
		log.Printf("ERROR startCPUProfile err1='%v'", err)
		return nil, err
	}
	if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
		log.Printf("ERROR startCPUProfile err2='%v'", err)
		return nil, err
	}
	return cpuProfileFile, nil
}

func stopCPUProfile(cpuProfileFile *os.File) {
	log.Printf("stopCPUProfile")
	pprof.StopCPUProfile()
	cpuProfileFile.Close()
}

func captureInterruptSignal(cpuProfileFile *os.File) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)
	go func() {
		<-c
		stopCPUProfile(cpuProfileFile)
		os.Exit(1)
	}()
}

func shuffleStrings(strings []string) {
	n := len(strings)
	rand.Shuffle(n, func(i, j int) {
		strings[i], strings[j] = strings[j], strings[i]
	})
}
