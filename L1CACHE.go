package history

import (
	//"fmt"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DefaultL1CacheExpires int64 = DefaultCacheExpires
	L1Purge               int64 = DefaultCachePurge
)

type L1CACHE struct {
	caches   map[string]*L1CACHEMAP
	muxers   map[string]*L1MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
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
	initsize := 128
	l1.mux.Lock()
	defer l1.mux.Unlock()
	if l1.caches != nil {
		log.Printf("ERROR L1CACHESetup already loaded!")
		return
	}
	l1.caches = make(map[string]*L1CACHEMAP, 16)
	l1.muxers = make(map[string]*L1MUXER, 16)
	l1.mapsizes = make(map[string]*MAPSIZES, 16)
	for _, char := range HEXCHARS {
		l1.caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, initsize)}
		l1.muxers[char] = &L1MUXER{}
		l1.mapsizes[char] = &MAPSIZES{maxmapsize: initsize}
	}
	for _, char := range HEXCHARS {
		go l1.L1Cache_Thread(char)
	}
} // end func L1CACHE_Boot

// The LockL1Cache method is used to LOCK a `MessageIDHash` for processing.
// If the value is not in the cache or has expired, it locks the cache, updates the cache with a new value, and returns the value.
// Possible return values:
//
//	-1 == in processing
//	 0 == not a duplicate // locked article for processing
//	 1 == is a duplicate
//	 2 == retry later
func (l1 *L1CACHE) LockL1Cache(hash *string, char string, value int) int {
	if hash == nil || *hash == "" {
		log.Printf("ERROR LockL1Cache hash=nil")
		return -999
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	//now := utils.UnixTimeSec()
	l1.muxers[char].mux.Lock()
	if l1.caches[char].cache[*hash] != nil {
		retval := l1.caches[char].cache[*hash].value
		/*
			if l1.caches[char].cache[*hash].expires >= now {
				retval := l1.caches[char].cache[*hash].value
				return retval
			} else {
				// entry expired
			}
		*/
		l1.muxers[char].mux.Unlock()
		return retval
	}
	l1.caches[char].cache[*hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + DefaultL1CacheExpires}
	l1.muxers[char].mux.Unlock()
	return 0
} // end func LockL1Cache

// The L1Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries, and if the cache size is too large, it shrinks the cache.
func (l1 *L1CACHE) L1Cache_Thread(char string) {
	logf(DEBUG2, "Boot L1Cache_Thread [%s]", char)
	//forever
	for {
		time.Sleep(time.Duration(L1Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		cleanup := []string{}

		l1.muxers[char].mux.Lock()
	getexpired:
		for hash, item := range l1.caches[char].cache {
			expired := false
			if item.expires < now {
				// value is cached response:
				switch item.value {
				case -1: // processing
					continue getexpired
				case 1: // duplicate
					expired = true
				case 2: // retry
					expired = true
				}
				if expired {
					cleanup = append(cleanup, hash)
				}
			}
		}
		l1.muxers[char].mux.Unlock()

		if len(cleanup) > 0 {
			l1.muxers[char].mux.Lock()
			for _, hash := range cleanup {
				delete(l1.caches[char].cache, hash)
			}
			maplen := len(l1.caches[char].cache)
			max := l1.mapsizes[char].maxmapsize
			case1 := maplen < 1024 && max > 4096
			case2 := maplen == 0 && max > 1024
			if case1 || case2 {
				newmax := 1024
				if case1 {
					newmax = 4096
				}
				newmap := make(map[string]*L1ITEM, newmax)
				if maplen == 0 {
					l1.caches[char].cache = newmap
				} else {
					for k, v := range l1.caches[char].cache {
						newmap[k] = v
					}
					l1.caches[char].cache = newmap
				}
				l1.mapsizes[char].maxmapsize = newmax
				logf(DEBUG2, "L1Cache_Thread [%s] shrink size to 1024", char)
			}
			newmax := l1.mapsizes[char].maxmapsize
			l1.muxers[char].mux.Unlock()
			logf(DEBUG2, "L1Cache_Thread [%s] deleted=%d maplen=%d/%d oldmax=%d", char, len(cleanup), maplen, newmax, max)
			cleanup = nil
		}
		logf(DEBUG2, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L1Cache_Thread

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash *string, char string, value int) {
	if hash == nil || len(*hash) < 32 { // at least md5
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	start := utils.UnixTimeMilliSec()
	l1.muxers[char].mux.Lock()

	if len(l1.caches[char].cache) >= int(l1.mapsizes[char].maxmapsize/100*98) { // grow map
		newmax := l1.mapsizes[char].maxmapsize * 2
		newmap := make(map[string]*L1ITEM, newmax)
		for k, v := range l1.caches[char].cache {
			newmap[k] = v
		}
		l1.caches[char].cache = newmap
		l1.mapsizes[char].maxmapsize = newmax
		logf(DEBUG1, "L1CACHE char=%s grow newmap=%d/%d (took %d ms)", char, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}
	l1.caches[char].cache[*hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + DefaultL1CacheExpires}
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
	if l1.caches[char].cache[*hash] != nil {
		item := l1.caches[char].cache[*hash]
		retval = &item.value
	}
	l1.muxers[char].mux.Unlock()
	return
} // end func Get


// The Delete method deletes a cache item from the L1 cache.
func (l1 *L1CACHE) Delete(hash *string, char string) {
	if hash == nil || *hash == "" {
		log.Printf("ERROR L1CACHEDel hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	l1.muxers[char].mux.Lock()
	delete(l1.caches[char].cache, *hash)
	l1.muxers[char].mux.Unlock()
} // end func Delete
*/
