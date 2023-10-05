package history

import (
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL3 bool
	DefaultL3CacheExpires int64 = DefaultCacheExpires
	L3Purge               int64 = DefaultCachePurge
	L3InitSize            int   = 128
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
		l3.caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.muxers[char] = &L3MUXER{}
		l3.mapsizes[char] = &MAPSIZES{maxmapsize: L3InitSize}
		go l3.L3Cache_Thread(char)
	}
} // end func L3CACHE_Boot

// The L3Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l3 *L3CACHE) L3Cache_Thread(char string) {
	logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
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
		maplen := len(l3.caches[char].cache)
		l3.muxers[char].mux.Unlock()

		if len(cleanup) > 0 {
			maplen -= len(cleanup)
			l3.muxers[char].mux.Lock()
			for _, key := range cleanup {
				delete(l3.caches[char].cache, key)
			}
			max := l3.mapsizes[char].maxmapsize
			l3.muxers[char].mux.Unlock()
			logf(DEBUGL3, "L3Cache_Thread [%s] deleted=%d maplen=%d/%d", char, len(cleanup), maplen, max)
			cleanup = nil
		}
		l3.shrinkMapIfNeeded(char, maplen)
		logf(DEBUGL3, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L3Cache_Thread

func (l3 *L3CACHE) shrinkMapIfNeeded(char string, maplen int) {
	shrinkmin := L2InitSize
	oldmax := l3.mapsizes[char].maxmapsize
	thresholdFactor := 0.125
	threshold := int(float64(oldmax) * thresholdFactor)
	if maplen > threshold {
		return
	}
	thresmax := threshold * 4
	logf(DEBUGL3, "L3 [%s] PRE-SHRINK maplen=%d threshold=%d oldmax=%d thresmax=%d", char, maplen, threshold, oldmax, thresmax)
	if maplen < threshold && oldmax >= thresmax && thresmax > shrinkmin {
		newmax := threshold * 4
		if newmax < shrinkmin {
			newmax = shrinkmin
		} else if oldmax == shrinkmin && newmax == shrinkmin {
			// dont shrink lower than this
			return
		}
		l3.shrinkMap(char, newmax, maplen)
	}
} // end func shrinkMapIfNeeded

func (l3 *L3CACHE) shrinkMap(char string, newmax int, maplen int) {
	newmap := make(map[string]*L3ITEM, newmax)
	l3.muxers[char].mux.Lock()
	if maplen > 0 {
		for k, v := range l3.caches[char].cache {
			newmap[k] = v
		}
	}
	l3.caches[char].cache = nil
	l3.caches[char].cache = newmap
	l3.mapsizes[char].maxmapsize = newmax
	l3.muxers[char].mux.Unlock()
	logf(DEBUGL3, "L3Cache_Thread [%s] shrink size to %d maplen=%d", char, newmax, maplen)
} // end func shrinkMap

// The SetOffsets method sets a cache item in the L3 cache using a key and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets *[]int64) {
	if len(key) == 0 {
		log.Printf("ERROR L3CACHESet key=nil")
		return
	}
	if char == "" {
		char = string(key[0])
	}
	start := utils.UnixTimeMilliSec()
	l3.muxers[char].mux.Lock()

	if len(l3.caches[char].cache) >= int(l3.mapsizes[char].maxmapsize/100*48) { // grow map
		newmax := l3.mapsizes[char].maxmapsize * 4
		newmap := make(map[string]*L3ITEM, newmax)
		for k, v := range l3.caches[char].cache {
			newmap[k] = v
		}
		l3.caches[char].cache = newmap
		l3.mapsizes[char].maxmapsize = newmax
		logf(DEBUGL3, "L3CACHE char=%s grow newmap=%d/%d (took %d ms)", char, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}

	l3.caches[char].cache[key] = &L3ITEM{offsets: *offsets, expires: utils.UnixTimeSec() + DefaultL3CacheExpires}
	l3.muxers[char].mux.Unlock()
} // end func SetOffsets

// The GetOffsets method retrieves a slice of offsets from the L3 cache using a key.
func (l3 *L3CACHE) GetOffsets(key string, char string) (offsets *[]int64) {
	if key == "" {
		log.Printf("ERROR L3CACHEGet key=nil")
		return
	}
	if char == "" {
		char = string(key[0])
	}
	l3.muxers[char].mux.Lock()
	if l3.caches[char].cache[key] != nil {
		item := l3.caches[char].cache[key]
		offsets = &item.offsets
	}
	l3.muxers[char].mux.Unlock()
	return
} // end func GetOffsets

// The Delete method deletes a cache item from the L3 cache.
func (l3 *L3CACHE) Delete(key *string, char string) {
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
} // end func Delete
