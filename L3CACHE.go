package history

import (
	//"fmt"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DefaultL3CacheExpires int64 = DefaultCacheExpires
	L3Purge               int64 = DefaultCachePurge
)

type L3CACHE struct {
	caches   map[string]*L3CACHEMAP
	muxers   map[string]*L3MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
}

type L3CACHEMAP struct {
	cache map[string]*L3ITEM
}

type L3ITEM struct {
	offsets []int64
	expires int64
}

type L3MUXER struct {
	mux sync.Mutex
}

// The L3CACHE_Boot method initializes the L3 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l3 *L3CACHE) L3CACHE_Boot() {
	initsize := 128
	l3.mux.Lock()
	defer l3.mux.Unlock()
	if l3.caches != nil {
		log.Printf("ERROR L3CACHESetup already loaded!")
		return
	}
	l3.caches = make(map[string]*L3CACHEMAP, 16)
	l3.muxers = make(map[string]*L3MUXER, 16)
	l3.mapsizes = make(map[string]*MAPSIZES, 16)
	for _, char := range HEXCHARS {
		l3.caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, initsize)}
		l3.muxers[char] = &L3MUXER{}
		l3.mapsizes[char] = &MAPSIZES{maxmapsize: initsize}
	}
	for _, char := range HEXCHARS {
		go l3.L3Cache_Thread(char)
	}
} // end func L3CACHE_Boot

// The L3Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l3 *L3CACHE) L3Cache_Thread(char string) {
	logf(DEBUG2, "Boot L3Cache_Thread [%s]", char)
	//forever
	for {
		time.Sleep(time.Duration(L3Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		cleanup := []string{}

		l3.muxers[char].mux.Lock()
		for key, item := range l3.caches[char].cache {
			if item.expires < now {
				cleanup = append(cleanup, key)
			}
		}
		l3.muxers[char].mux.Unlock()

		if len(cleanup) > 0 {
			l3.muxers[char].mux.Lock()
			for _, key := range cleanup {
				delete(l3.caches[char].cache, key)
			}
			maplen := len(l3.caches[char].cache)
			max := l3.mapsizes[char].maxmapsize
			if maplen < 1024 && max > 4096 || maplen == 0 && max == 1024 {
				newmap := make(map[string]*L3ITEM, 1024)
				if maplen == 0 {
					l3.caches[char].cache = newmap
				} else {
					for k, v := range l3.caches[char].cache {
						newmap[k] = v
					}
					l3.caches[char].cache = newmap
				}
				l3.mapsizes[char].maxmapsize = 1024
				logf(DEBUG2, "L3Cache_Thread [%s] shrink size to 1024", char)
			}
			newmax := l3.mapsizes[char].maxmapsize
			l3.muxers[char].mux.Unlock()
			logf(DEBUG2, "L3Cache_Thread [%s] deleted=%d maplen=%d/%d oldmax=%d", char, len(cleanup), maplen, newmax, max)
			cleanup = nil
		}
		logf(DEBUG2, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L3Cache_Thread

// The L3CACHE_SetOffsets method sets a cache item in the L3 cache using a key and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) L3CACHE_SetOffsets(key string, char string, offsets *[]int64) {
	if len(key) == 0 {
		log.Printf("ERROR L3CACHESet key=nil")
		return
	}
	if char == "" {
		char = string(key[0])
	}
	start := utils.UnixTimeMilliSec()
	l3.muxers[char].mux.Lock()

	if len(l3.caches[char].cache) >= int(l3.mapsizes[char].maxmapsize/100*98) { // grow map
		newmax := l3.mapsizes[char].maxmapsize * 2
		newmap := make(map[string]*L3ITEM, newmax)
		for k, v := range l3.caches[char].cache {
			newmap[k] = v
		}
		l3.caches[char].cache = newmap
		l3.mapsizes[char].maxmapsize = newmax
		logf(DEBUG1, "L3CACHE char=%s grow newmap=%d/%d (took %d ms)", char, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}

	l3.caches[char].cache[key] = &L3ITEM{offsets: *offsets, expires: utils.UnixTimeSec() + DefaultL3CacheExpires}
	l3.muxers[char].mux.Unlock()
} // end func L3CACHE_SetOffsets

// The L3CACHE_GetOffsets method retrieves a slice of offsets from the L3 cache using a key.
func (l3 *L3CACHE) L3CACHE_GetOffsets(key string, char string) (offsets *[]int64) {
	if key == "" {
		log.Printf("ERROR L3CACHEGet key=nil")
		return
	}
	if char == "" {
		char = string(key[0])
	}
	//now := utils.UnixTimeSec()
	l3.muxers[char].mux.Lock()
	if l3.caches[char].cache[key] != nil {
		item := l3.caches[char].cache[key]
		offsets = &item.offsets
	}
	l3.muxers[char].mux.Unlock()
	return
} // end func L3CACHE_GetOffsets

// The L3CACHE_Del method deletes a cache item from the L3 cache.
func (l3 *L3CACHE) L3CACHE_Del(key *string, char string) {
	if key == nil || *key == "" {
		log.Printf("ERROR L3CACHEDel key=nil")
		return
	}
	if char == "" {
		char = string(string(*key)[0])
	}
	l3.muxers[char].mux.Lock()
	delete(l3.caches[char].cache, *key)
	l3.muxers[char].mux.Unlock()
} // end func L3CACHE_Del