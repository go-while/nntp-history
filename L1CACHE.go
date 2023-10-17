package history

import (
	"github.com/go-while/go-utils"
	"log"
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
	Extend  map[string]*StrECH
	muxers  map[string]*L1MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
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
	l1.Caches = make(map[string]*L1CACHEMAP, intBoltDBs)
	l1.Extend = make(map[string]*StrECH, intBoltDBs)
	l1.muxers = make(map[string]*L1MUXER, intBoltDBs)
	l1.Counter = make(map[string]*CCC)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &StrECH{ch: make(chan *string, his.cEvCap)}
		l1.muxers[char] = &L1MUXER{}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
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
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.muxers[char]

	mux.mux.Lock()

	if _, exists := ptr.cache[hash]; exists {
		cnt.Counter["Count_Get"]++
		retval := ptr.cache[hash].value
		//if hash == TESTHASH {
		//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d isLocked", char, hash, value)
		//}
		mux.mux.Unlock()
		return retval
	}

	if !useHashDB {
		value = CaseDupes
	}
	cnt.Counter["Count_Locked"]++
	ptr.cache[hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + L1CacheExpires}
	//if hash == TESTHASH {
	//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d weLocked", char, hash, value)
	//}
	mux.mux.Unlock()
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

	go func(ptr *L1CACHEMAP, mux *sync.Mutex, cnt *CCC, extendChan *StrECH) {
		defer log.Printf("LEFT L1T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l1purge) * time.Second)
		timeout := false
		ext, emax := 0, 16384
		extends := make([]string, emax)
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					timeout = true
					break forextends
				case hash := <-extendChan.ch: // receives stuff from DelExtL1batch()
					if hash == nil {
						log.Printf("ERROR L1 extend ch received nil hash")
						continue forextends
					}
					// got hash we will extend in next timer.C run
					extends = append(extends, *hash)
					//log.Printf("L1 extend hash='%s'", *hash)
					ext++
					if ext >= emax {
						timeout = true
						break forextends
					}
				} // end select
			} // end forextends
			if (timeout && ext > 0) || ext >= emax {
				now := utils.UnixTimeSec()
				mux.Lock()
				//logf(DEBUG, "L1 [%s] extends=%d", char, len(extends))
				for _, hash := range extends {
					if _, exists := ptr.cache[hash]; exists {
						ptr.cache[hash].expires = now + L1ExtendExpires
						ptr.cache[hash].value = CaseDupes
						cnt.Counter["Count_BatchD"]++
					}
				}
				mux.Unlock()
				extends = nil
				timeout = false
				ext = 0
				timer.Reset(time.Duration(l1purge) * time.Second)
			}
		} //end forever
	}(l1.Caches[char], &l1.muxers[char].mux, l1.Counter[char], l1.Extend[char]) // end gofunc1

	go func(ptr *L1CACHEMAP, mux *sync.Mutex, cnt *CCC) {
		defer log.Printf("LEFT L1T gofunc2 delete [%s]", char)
		timer := time.NewTimer(time.Duration(l1purge) * time.Second)
		timeout := false
		start := utils.UnixTimeMilliSec()
		now := int64(start / 1000)
	forever:
		for {
			if timeout {
				timeout = false
				timer.Reset(time.Duration(l1purge) * time.Second)
			}
			select {
			case <-timer.C:
				timeout = true
				start = utils.UnixTimeMilliSec()
				now = int64(start / 1000)
				mux.Lock()
				//getexpired:
				for hash, item := range ptr.cache {
					if (item.value == CaseDupes || item.value == CaseRetry) && item.expires > 0 && item.expires < now {
						//if hash == TESTHASH {
						//	log.Printf("L1CAC [%s|  ] ADD2CLEANUP TESTHASH='%s'", char, hash)
						//}
						cleanup = append(cleanup, hash)
					}
				} // end for getexpired

				//maplen := len(ptr.cache)
				if len(cleanup) > 0 {
					//maplen -= len(cleanup)
					for _, hash := range cleanup {
						delete(ptr.cache, hash)
						cnt.Counter["Count_Delete"]++
					}
					//logf(DEBUG, "L1Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
				}
				mux.Unlock()
				cleanup = nil
				//logf(DEBUG, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
				continue forever
			} // end select
		} // end for
	}(l1.Caches[char], &l1.muxers[char].mux, l1.Counter[char]) // end gofunc2
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
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.muxers[char]

	mux.mux.Lock()

	expires := NoExpiresVal
	if flagexpires {
		cnt.Counter["Count_FlagEx"]++
		expires = utils.UnixTimeSec() + L1CacheExpires
	} else if !flagexpires && value == CaseWrite {
		cnt.Counter["Count_Set"]++
	}

	if _, exists := ptr.cache[hash]; exists {
		ptr.cache[hash].expires = expires
		ptr.cache[hash].value = value
		l1.muxers[char].mux.Unlock()
		return
	}
	ptr.cache[hash] = &L1ITEM{value: value, expires: expires}
	mux.mux.Unlock()
} // end func Set

// The DelExtL1batch method deletes multiple cache items from the L1 cache.
func (l1 *L1CACHE) DelExtL1batch(his *HISTORY, char string, tmpHash []*ClearCache) {
	if char == "" {
		log.Printf("ERROR DelExtL1batch char=nil")
		return
	}
	if len(tmpHash) == 0 {
		log.Printf("DelExtL1batch [%s] tmpHash empty", char)
		return
	}
	for _, item := range tmpHash {
		if item.hash != nil && *item.hash != "" {
			/*
				if DEBUG {
					lench := len(l1.Extend[char].ch)
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L1 Extend[%s]chan=%d/his.cEvCap=%d half-full", char, lench, his.cEvCap)
					}
				}
			*/
			l1.Extend[char].ch <- item.hash
		}
	}
	return
} // end func DelExtL1batch

func (l1 *L1CACHE) L1Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l1 == nil || l1.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l1.Counter[char]
		mux := l1.muxers[char]
		mux.mux.Lock()
		switch statskey {
		case "":
			// key is empty, get all key=>stats to retmap
			for k, v := range cnt.Counter {
				retmap[k] += v
			}
		default:
			// key is set, returns retval
			if _, exists := cnt.Counter[statskey]; exists {
				retval += cnt.Counter[statskey]
			}
		}
		mux.mux.Unlock()
	}
	return
} // end func L1Stats
