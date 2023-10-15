package history

import (
	"github.com/go-while/go-utils"
	"log"
	//"os"
	"sync"
	"time"
)

var (
	DEBUGL1         bool  = false
	L1CacheExpires  int64 = DefaultCacheExpires
	L1ExtendExpires int64 = DefaultCacheExtend
	L1Purge         int64 = DefaultCachePurge
	L1InitSize      int   = 256 * 1024
)

type L1CACHE struct {
	Caches  map[string]*L1CACHEMAP
	Extend  map[string]chan string
	muxers  map[string]*L1MUXER
	mux     sync.Mutex
	Counter map[string]map[string]uint64
}

type L1CACHEMAP struct {
	cache map[string]*L1ITEM
}

type L1ITEM struct {
	value   int
	expires int64
}

type L1MUXER struct {
	mux sync.Mutex
}

// The L1CACHE_Boot method initializes the cache system.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically purge expired entries.
func (l1 *L1CACHE) L1CACHE_Boot(his *HISTORY) {
	l1.mux.Lock()
	defer l1.mux.Unlock()
	if l1.Caches != nil {
		log.Printf("ERROR L1CACHESetup already loaded!")
		return
	}
	l1.Caches = make(map[string]*L1CACHEMAP, 16)
	l1.Extend = make(map[string]chan string, 16)
	l1.muxers = make(map[string]*L1MUXER, 16)
	l1.Counter = make(map[string]map[string]uint64)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = make(chan string, his.cEvCap)
		l1.muxers[char] = &L1MUXER{}
		l1.Counter[char] = make(map[string]uint64)
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l1.L1Cache_Thread(char)
	}
} // end func L1CACHE_Boot

// The LockL1Cache method is used to LOCK a `MessageIDHash` for processing.
// If the value is not in the cache or has expired, it locks the cache, updates the cache with a new value, and returns the value.
// Possible return values:
//
//	CaseLock == already in processing
//	CaseWrite == already in processing
//	CaseDupes == is a duplicate
//	CasePass == not a duplicate == locked article for processing
func (l1 *L1CACHE) LockL1Cache(hash string, char string, value int, useHashDB bool) int {
	//if hash == TESTHASH {
	//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d tryLock", char, hash, value)
	//}
	if hash == "" {
		log.Printf("ERROR LockL1Cache hash=nil")
		return -999
	}
	if char == "" {
		char = string(hash[0])
	}
	l1.muxers[char].mux.Lock()
	defer l1.muxers[char].mux.Unlock()

	if l1.Caches[char].cache[hash] != nil {
		l1.Counter[char]["Count_Get"] += 1
		retval := l1.Caches[char].cache[hash].value
		//if hash == TESTHASH {
		//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d isLocked", char, hash, value)
		//}

		return retval
	}

	if !useHashDB {
		value = CaseDupes
	}
	l1.Counter[char]["Count_Locked"] += 1
	l1.Caches[char].cache[hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + L1CacheExpires}
	//if hash == TESTHASH {
	//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d weLocked", char, hash, value)
	//}
	return CasePass
} // end func LockL1Cache

// The L1Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries, and if the cache size is too large, it shrinks the cache.
func (l1 *L1CACHE) L1Cache_Thread(char string) {
	l1.mux.Lock() // waits for L2CACHE_Boot to unlock
	l1.mux.Unlock()
	//logf(DEBUGL1, "Boot L1Cache_Thread [%s]", char)
	cleanup := []string{}
	l1purge := L1Purge
	if l1purge < 1 {
		l1purge = 1
	}
	extends := make(map[string]bool, 1024)
	timer := time.NewTimer(time.Duration(l1purge) * time.Second)
	timeout := false
forever:
	for {
		if timeout {
			timeout = false
			timer.Reset(time.Duration(l1purge) * time.Second)
		}
		select {
		case hash := <-l1.Extend[char]: // receives stuff from DelExtL1batch()
			// got hash we will extend in next timer.C run
			if hash != "" {
				extends[hash] = true
			}
		case <-timer.C:
			timeout = true
			start := utils.UnixTimeMilliSec()
			now := int64(start / 1000)
			l1.muxers[char].mux.Lock()
		getexpired:
			for hash, item := range l1.Caches[char].cache {
				if extends[hash] {
					l1.Caches[char].cache[hash].expires = now + L1ExtendExpires
					// has been written to boltDB and is now a Duplicate in response
					l1.Caches[char].cache[hash].value = CaseDupes
					l1.Counter[char]["Count_BatchD"] += 1
					continue getexpired
				} else if item.expires > 0 && item.expires < now && item.value == CaseDupes {
					//if hash == TESTHASH {
					//	log.Printf("L1CAC [%s|  ] ADD2CLEANUP TESTHASH='%s'", char, hash)
					//}
					cleanup = append(cleanup, hash)
				}
			} // end for getexpired
			maplen := len(l1.Caches[char].cache)
			if len(cleanup) > 0 {
				maplen -= len(cleanup)
				for _, hash := range cleanup {
					delete(l1.Caches[char].cache, hash)
					l1.Counter[char]["Count_Delete"] += 1
				}
				logf(DEBUGL1, "L1Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
				cleanup = nil
			}
			l1.muxers[char].mux.Unlock()
			clear(extends)
			//logf(DEBUGL1, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
			continue forever
		} // end select
	} // end for

} //end func L1Cache_Thread

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash string, char string, value int, flagexpires bool) {
	if len(hash) < 32 { // at least md5
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(hash[0])
	}
	//start := utils.UnixTimeMilliSec()
	l1.muxers[char].mux.Lock()
	defer l1.muxers[char].mux.Unlock()

	expires := NoExpiresVal
	if flagexpires {
		l1.Counter[char]["Count_FlagEx"]++
		expires = utils.UnixTimeSec() + L1CacheExpires
	} else if !flagexpires && value == CaseWrite {
		l1.Counter[char]["Count_Set"]++
	}

	if l1.Caches[char].cache[hash] != nil {
		l1.Caches[char].cache[hash].expires = expires
		return
	}
	l1.Caches[char].cache[hash] = &L1ITEM{value: value, expires: expires}
} // end func Set

// The DelExtL1batch method deletes multiple cache items from the L1 cache.
func (l1 *L1CACHE) DelExtL1batch(his *HISTORY, char string, tmpHash []*ClearCache, flagCacheDelExt int) {
	if char == "" {
		log.Printf("ERROR DelExtL1batch char=nil")
		return
	}
	if len(tmpHash) == 0 {
		log.Printf("DelExtL1batch [%s] tmpHash empty", char)
		return
	}
	if flagCacheDelExt == FlagCacheChanExtend {
		for _, item := range tmpHash {
			if item.hash != "" {
				if DEBUG {
					lench := len(l1.Extend[char])
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L1 Extend[%s]chan=%d/his.cEvCap=%d half-full", char, lench, his.cEvCap)
					}
				}
				l1.Extend[char] <- item.hash
			}
		}
		return
	}
	now := utils.UnixTimeSec()
	l1.muxers[char].mux.Lock()
	defer l1.muxers[char].mux.Unlock()

	for _, item := range tmpHash {
		if item.hash != "" {
			if _, exists := l1.Caches[char].cache[item.hash]; exists {
				switch flagCacheDelExt {
				case FlagCacheSyncDelete:
					delete(l1.Caches[char].cache, item.hash)
				case FlagCacheSyncExtend:
					// dont delete from cache but extend expiry time
					l1.Caches[char].cache[item.hash].expires = now + L1ExtendExpires
					// has been written to boltDB and is now a Duplicate in response
					l1.Caches[char].cache[item.hash].value = CaseDupes
				}
				l1.Counter[char]["Count_BatchD"] += 1
			}
		}
	}
} // end func DelExtL1batch

func (l1 *L1CACHE) L1Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l1 == nil || l1.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		l1.muxers[char].mux.Lock()
		switch statskey {
		case "":
			// key is empty, get all key=>stats to retmap
			for k, v := range l1.Counter[char] {
				retmap[k] += v
			}
		default:
			// key is set, returns retval
			if _, exists := l1.Counter[char][statskey]; exists {
				retval += l1.Counter[char][statskey]
			}
		}
		l1.muxers[char].mux.Unlock()
	}
	return
} // end func L1Stats
