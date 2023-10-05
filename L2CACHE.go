package history

import (
	"fmt"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL2 bool
	DefaultL2CacheExpires int64 = DefaultCacheExpires
	L2Purge               int64 = DefaultCachePurge
	L2InitSize            int   = 16
)

type L2CACHE struct {
	caches   map[string]*L2CACHEMAP
	muxers   map[string]*L2MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
}

type L2CACHEMAP struct {
	cache map[int64]*L2ITEM
}

type L2ITEM struct {
	hash    string
	expires int64
}

type L2MUXER struct {
	mux sync.Mutex
}

// The L2CACHE_Boot method initializes the L2 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l2 *L2CACHE) L2CACHE_Boot() {
	l2.mux.Lock()
	defer l2.mux.Unlock()
	if l2.caches != nil {
		log.Printf("ERROR L2CACHESetup already loaded!")
		return
	}
	l2.caches = make(map[string]*L2CACHEMAP, 16)
	l2.muxers = make(map[string]*L2MUXER, 16)
	l2.mapsizes = make(map[string]*MAPSIZES, 16)
	for _, char := range HEXCHARS {
		l2.caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.muxers[char] = &L2MUXER{}
		l2.mapsizes[char] = &MAPSIZES{maxmapsize: L2InitSize}
		go l2.L2Cache_Thread(char)
	}
} // end func L2CACHE_Boot

// The L2Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l2 *L2CACHE) L2Cache_Thread(char string) {
	logf(DEBUGL2, "Boot L2Cache_Thread [%s]", char)
	//forever
	cleanup := []int64{}
	for {
		time.Sleep(time.Duration(L2Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()

		l2.muxers[char].mux.Lock()
		for key, item := range l2.caches[char].cache {
			if item.expires < now {
				logf(DEBUGL2, "L2 expire key='%#v' item='%#v' age=%d", key, item, now - item.expires)
				cleanup = append(cleanup, key)
			}
		}
		maplen := len(l2.caches[char].cache)
		l2.muxers[char].mux.Unlock()

		if len(cleanup) > 0 {
			maplen -= len(cleanup)
			l2.muxers[char].mux.Lock()
			for _, key := range cleanup {
				delete(l2.caches[char].cache, key)
			}
			max := l2.mapsizes[char].maxmapsize
			l2.muxers[char].mux.Unlock()
			logf(DEBUGL2, "L2Cache_Thread [%s] deleted=%d maplen=%d/%d", char, len(cleanup), maplen, max)
			cleanup = nil
		}
		l2.shrinkMapIfNeeded(char, maplen)
		logf(DEBUGL2, "L2Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L2Cache_Thread

func (l2 *L2CACHE) shrinkMapIfNeeded(char string, maplen int) {
	shrinkmin := L2InitSize
	oldmax := l2.mapsizes[char].maxmapsize
	thresholdFactor := 0.125
	threshold := int(float64(oldmax) * thresholdFactor)
	if maplen > threshold {
		return
	}
	thresmax := threshold * 4
	logf(DEBUGL2, "L2 [%s] PRE-SHRINK maplen=%d threshold=%d oldmax=%d thresmax=%d", char, maplen, threshold, oldmax, thresmax)
	if maplen < threshold && oldmax >= thresmax && thresmax > shrinkmin {
		newmax := threshold * 4
		if newmax < shrinkmin {
			newmax = shrinkmin
		} else if oldmax == shrinkmin && newmax == shrinkmin {
			// dont shrink lower than this
			return
		}
		l2.shrinkMap(char, newmax, maplen)
	}
} // end func shrinkMapIfNeeded

func (l2 *L2CACHE) shrinkMap(char string, newmax int, maplen int) {
	newmap := make(map[int64]*L2ITEM, newmax)
	l2.muxers[char].mux.Lock()
	if maplen > 0 {
		for k, v := range l2.caches[char].cache {
			newmap[k] = v
		}
	}
	l2.caches[char].cache = nil
	l2.caches[char].cache = newmap
	l2.mapsizes[char].maxmapsize = newmax
	l2.muxers[char].mux.Unlock()
	logf(DBG_C, "L2Cache_Thread [%s] shrink size to %d maplen=%d", char, newmax, maplen)
} // end func shrinkMap

// The SetOffsetHash method sets a cache item in the L2 cache using an offset as the key and a hash as the value.
// It also dynamically grows the cache when necessary.
func (l2 *L2CACHE) SetOffsetHash(offset *int64, hash *string) {
	if offset == nil || hash == nil || len(*hash) < 32 { // at least md5
		log.Printf("ERROR L2CACHESet nil pointer")
		return
	}
	char := OffsetToChar(offset)
	if char == nil {
		return
	}
	start := utils.UnixTimeMilliSec()
	l2.muxers[*char].mux.Lock()

	if len(l2.caches[*char].cache) >= int(l2.mapsizes[*char].maxmapsize/100*48) { // grow map
		newmax := l2.mapsizes[*char].maxmapsize * 4
		newmap := make(map[int64]*L2ITEM, newmax)
		for k, v := range l2.caches[*char].cache {
			newmap[k] = v
		}
		l2.caches[*char].cache = newmap
		l2.mapsizes[*char].maxmapsize = newmax
		logf(DBG_C, "L2CACHE grow newmap=%d/%d (took %d ms)", len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}

	l2.caches[*char].cache[*offset] = &L2ITEM{hash: *hash, expires: utils.UnixTimeSec() + DefaultL2CacheExpires}
	l2.muxers[*char].mux.Unlock()
} // end func SetOffsetHash

// The GetHashFromOffset method retrieves a hash from the L2 cache using an offset as the key.
func (l2 *L2CACHE) GetHashFromOffset(offset *int64) (hash *string) {
	if offset == nil || *offset <= 0 {
		log.Printf("ERROR L2CACHEGetHashToOffset offset=nil")
		return
	}
	char := OffsetToChar(offset)
	if char == nil {
		return
	}
	l2.muxers[*char].mux.Lock()
	if l2.caches[*char].cache[*offset] != nil {
		item := l2.caches[*char].cache[*offset]
		hash = &item.hash
	}
	l2.muxers[*char].mux.Unlock()
	return
} // end func GetHashFromOffset

/*
// The Delete method deletes a cache item from the L2 cache using an offset as the key.
func (l2 *L2CACHE) Delete(offset *int64) {
	if offset == nil || *offset <= 0 {
		log.Printf("ERROR L2CACHEDel offset=nil")
		return
	}
	char := OffsetToChar(offset)
	l2.muxers[*char].mux.Lock()
	delete(l2.caches[*char].cache, *offset)
	l2.muxers[*char].mux.Unlock()
} // end func Delete
*/

func OffsetToChar(offset *int64) *string {
	if offset == nil {
		log.Printf("ERROR L2CACHE.OffsetToChar offset=nil")
		return nil
	}
	// get first char from hex: less random. repeats itself for longer period
	//char := string(fmt.Sprintf("%x", *offset)[0])

	// get last char from hex: the last char is more random
	hex := fmt.Sprintf("%x", *offset)
	char := string(hex[len(hex)-1])
	//log.Printf("OffsetToChar = %s", char)
	return &char
} // end func OffsetToChar
