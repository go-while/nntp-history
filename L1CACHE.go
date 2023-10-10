package history

import (
	"github.com/go-while/go-utils"
	"log"
	"os"
	"sync"
	"time"
)

var (
	DEBUGL1               bool
	DefaultL1CacheExpires int64 = DefaultCacheExpires
	L1Purge               int64 = DefaultCachePurge
	L1InitSize            int   = 64
)

type L1CACHE struct {
	Caches   map[string]*L1CACHEMAP
	muxers   map[string]*L1MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
	Counter  map[string]map[string]uint64
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

type MAPSIZES struct {
	maxmapsize int
}

// The L1CACHE_Boot method initializes the cache system.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically purge expired entries.
func (l1 *L1CACHE) L1CACHE_Boot() {
	l1.mux.Lock()
	defer l1.mux.Unlock()
	if l1.Caches != nil {
		log.Printf("ERROR L1CACHESetup already loaded!")
		return
	}
	l1.Caches = make(map[string]*L1CACHEMAP, 16)
	l1.muxers = make(map[string]*L1MUXER, 16)
	l1.mapsizes = make(map[string]*MAPSIZES, 16)
	l1.Counter = make(map[string]map[string]uint64)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.muxers[char] = &L1MUXER{}
		l1.mapsizes[char] = &MAPSIZES{maxmapsize: L1InitSize}
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
//	CaseLock == in processing
//	CaseDupes == is a duplicate
//	CaseRetry == retry later
//	CasePass == not a duplicate // locked article for processing
func (l1 *L1CACHE) LockL1Cache(hash *string, char string, value int) int {
	if hash == nil || *hash == "" {
		log.Printf("ERROR LockL1Cache hash=nil")
		return -999
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	l1.muxers[char].mux.Lock()
	if l1.Caches[char].cache[*hash] != nil {
		retval := l1.Caches[char].cache[*hash].value
		l1.muxers[char].mux.Unlock()
		return retval
	}
	l1.Caches[char].cache[*hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + DefaultL1CacheExpires}
	l1.muxers[char].mux.Unlock()
	return CasePass
} // end func LockL1Cache

// The L1Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries, and if the cache size is too large, it shrinks the cache.
func (l1 *L1CACHE) L1Cache_Thread(char string) {
	l1.mux.Lock() // waits for CACHE_Boot to unlock
	l1.mux.Unlock()
	logf(DEBUGL1, "Boot L1Cache_Thread [%s]", char)
	cleanup := []string{}
	lastshrink := utils.UnixTimeSec()
	//forever
	for {
		time.Sleep(time.Duration(L1Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		l1.muxers[char].mux.Lock()
	getexpired:
		for hash, item := range l1.Caches[char].cache {
			expired := false
			if item.expires > 0 && item.expires < now {
				// value is cached response:
				switch item.value {
				case CaseLock: // processing
					continue getexpired
				case CaseWrite: // processing
					continue getexpired
				case CaseDupes: // duplicate
					expired = true
				case CaseRetry: // retry
					expired = true
				default:
					log.Printf("ERROR L1 unknown switch item.value=%d=%x", item.value, item.value)
					os.Exit(1)
				}
				if expired {
					cleanup = append(cleanup, hash)
				}
			}
		}
		maplen := len(l1.Caches[char].cache)
		oldmax := l1.mapsizes[char].maxmapsize
		l1.muxers[char].mux.Unlock()

		if len(cleanup) > 0 {
			maplen -= len(cleanup)
			l1.muxers[char].mux.Lock()
			for _, hash := range cleanup {
				delete(l1.Caches[char].cache, hash)
				l1.Counter[char]["Count_Delete"] += 1
			}
			max := l1.mapsizes[char].maxmapsize
			l1.muxers[char].mux.Unlock()
			logf(DEBUGL1, "L1Cache_Thread [%s] deleted=%d maplen=%d/%d", char, len(cleanup), maplen, max)
			cleanup = nil
		}
		if lastshrink < now-DefaultTryShrinkEvery {
			l1.shrinkMapIfNeeded(char, maplen, oldmax)
			lastshrink = now
		}
		logf(DEBUGL1, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L1Cache_Thread

func (l1 *L1CACHE) shrinkMapIfNeeded(char string, maplen int, oldmax int) bool {
	shrinkmin := L1InitSize
	thresholdFactor := 20 // percent
	threshold := int(oldmax / 100 * thresholdFactor)
	if maplen > threshold {
		return true
	}
	thresmax := threshold * 4
	logf(DEBUGL1, "L1 [%s] PRE-SHRINK maplen=%d threshold=%d oldmax=%d thresmax=%d", char, maplen, threshold, oldmax, thresmax)
	if maplen < threshold && oldmax >= thresmax && thresmax > shrinkmin {
		newmax := threshold * 2
		if newmax < shrinkmin {
			newmax = shrinkmin
		} else if oldmax == shrinkmin && newmax == shrinkmin {
			// dont shrink lower than this
			return true
		}
		return l1.shrinkMap(&char, newmax, maplen)
	}
	return false
} // end func shrinkMapIfNeeded

func (l1 *L1CACHE) shrinkMap(char *string, newmax int, maplen int) bool {
	if char == nil {
		log.Printf("ERROR L1CACHE shrinkMap char=nil")
		return false
	}
	newmap := make(map[string]*L1ITEM, newmax)
	l1.muxers[*char].mux.Lock()
	if maplen > 0 {
		for k, v := range l1.Caches[*char].cache {
			newmap[k] = v
		}
	}
	clear(l1.Caches[*char].cache)
	l1.Caches[*char].cache = nil
	l1.Caches[*char].cache = newmap
	l1.mapsizes[*char].maxmapsize = newmax
	l1.Counter[*char]["Count_Shrink"] += 1
	l1.muxers[*char].mux.Unlock()
	logf(DBG_CGS, "L1Cache_Thread [%s] shrink size to %d maplen=%d", *char, newmax, maplen)
	return true
} // end func shrinkMap

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash *string, char string, value int, flagexpires bool) {
	if hash == nil || len(*hash) < 32 { // at least md5
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	start := utils.UnixTimeMilliSec()
	l1.muxers[char].mux.Lock()

	if len(l1.Caches[char].cache) >= int(l1.mapsizes[char].maxmapsize/100*98) { // grow map
		newmax := l1.mapsizes[char].maxmapsize * 4
		newmap := make(map[string]*L1ITEM, newmax)
		for k, v := range l1.Caches[char].cache {
			newmap[k] = v
		}
		clear(l1.Caches[char].cache)
		l1.Caches[char].cache = nil
		l1.Caches[char].cache = newmap
		l1.mapsizes[char].maxmapsize = newmax
		l1.Counter[char]["Count_Growup"] += 1
		logf(DBG_CGS, "L1CACHE char=%s grow newmap=%d/%d (took %d ms)", char, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}
	expires := NoExpiresVal
	if flagexpires {
		l1.Counter[char]["Count_FlagEx"] += 1
		expires = utils.UnixTimeSec() + DefaultL1CacheExpires
	} else if !flagexpires && value == CaseWrite {
		l1.Counter[char]["Count_Insert"] += 1
	}
	l1.Caches[char].cache[*hash] = &L1ITEM{value: value, expires: expires}
	l1.muxers[char].mux.Unlock()
} // end func Set

/*
// The Get method retrieves a value from the cache.
func (l1 *L1CACHE) Get(hash *string, char string) (retval *int) {
	if hash == nil || *hash == "" {
		log.Printf("ERROR L1CACHEGet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	l1.muxers[char].mux.Lock()
	if l1.Caches[char].cache[*hash] != nil {
		item := l1.Caches[char].cache[*hash]
		retval = &item.value
	}
	l1.muxers[char].mux.Unlock()
	return
} // end func Get
*/

/*
// The Delete method deletes a cache item from the L1 cache.
func (l1 *L1CACHE) DeleteL1(char *string, hash *string) {
	if char == nil || *char == "" {
		achar := string(string(*hash)[0])
		char = &achar
	}
	if hash == nil || *hash == "" {
		log.Printf("ERROR L1CACHEDel hash=nil")
		return
	}
	l1.muxers[*char].mux.Lock()
	if _, exists := l1.Caches[*char].cache[*hash]; exists {
		delete(l1.Caches[*char].cache, *hash)
	}
	l1.muxers[*char].mux.Unlock()
} // end func DeleteL1
*/

// The DeleteL1batch method deletes multiple cache items from the L1 cache.
func (l1 *L1CACHE) DeleteL1batch(char *string, tmpHash *[]*ClearCache) {
	if char == nil {
		log.Printf("ERROR DeleteL1batch char=nil")
		return
	}
	if tmpHash == nil {
		log.Printf("ERROR DeleteL1batch [%s] tmpHash=nil", *char)
	}
	now := utils.UnixTimeSec()
	l1.muxers[*char].mux.Lock()
	for _, item := range *tmpHash {
		if item.hash != nil {
			if _, exists := l1.Caches[*char].cache[*item.hash]; exists {
				//delete(l1.Caches[*char].cache, *item.hash)
				// dont delete from cache but extend expiry time
				l1.Caches[*char].cache[*item.hash].expires = now + DefaultL1CacheExpires
				l1.Caches[*char].cache[*item.hash].value = CaseDupes
				l1.Counter[*char]["Count_BatchD"] += 1
			}
		}
	}
	l1.muxers[*char].mux.Unlock()
} // end func DeleteL1batch
