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

func (l1 *L1CACHE) LockL1Cache(hash *string, char string, value int) int {
	if hash == nil || *hash == "" {
		log.Printf("ERROR LockL1Cache hash=nil")
		return -999
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	now := utils.UnixTimeSec()
	l1.muxers[char].mux.Lock()
	defer l1.muxers[char].mux.Unlock()
	if l1.caches[char].cache[*hash] != nil {
		if l1.caches[char].cache[*hash].expires >= now {
			retval := l1.caches[char].cache[*hash].value
			return retval
		} else {
			// entry expired
		}
	}
	l1.caches[char].cache[*hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + DefaultL1CacheExpires}
	return 0
} // end func LockL1Cache

func (l1 *L1CACHE) L1Cache_Thread(char string) {
	logf(DEBUG1, "Boot L1Cache_Thread [%s]", char)
	//forever
	for {
		time.Sleep(time.Duration(L1Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		cleanup := []string{}

		l1.muxers[char].mux.Lock()
		for hash, item := range l1.caches[char].cache {
			if item.expires < now {
				cleanup = append(cleanup, hash)
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
			if maplen < 1024 && max > 4096 || maplen == 0 && max == 1024 {
				newmap := make(map[string]*L1ITEM, 1024)
				if maplen == 0 {
					l1.caches[char].cache = newmap
				} else {
					for k, v := range l1.caches[char].cache {
						newmap[k] = v
					}
					l1.caches[char].cache = newmap
				}
				l1.mapsizes[char].maxmapsize = 1024
				logf(DEBUG1, "L1Cache_Thread [%s] shrink size to 1024", char)
			}
			newmax := l1.mapsizes[char].maxmapsize
			l1.muxers[char].mux.Unlock()
			logf(DEBUG1, "L1Cache_Thread [%s] deleted=%d maplen=%d/%d oldmax=%d", char, len(cleanup), maplen, newmax, max)
			cleanup = nil
		}
		logf(DEBUG1, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
	} // end for

} //end func L1Cache_Thread

func (l1 *L1CACHE) L1CACHE_Set(hash *string, char string, value int) {
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
} // end func L1CACHE_Set

func (l1 *L1CACHE) L1CACHE_Get(hash *string, char string) (retval *int) {
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
} // end func L1CACHE_Get

/*
func (l1 *L1CACHE) L1CACHE_GetSet(hash *string, char string, want int, setval int, offset int64) (retval *int) {
	if hash == nil || *hash == "" {
		log.Printf("ERROR L1CACHEGet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	//now := utils.UnixTimeSec()
	l1.muxers[char].mux.Lock()
	if l1.caches[char].cache[*hash] != nil {
		item := l1.caches[char].cache[*hash]
		retval = &item.value
		if offset > 0 && item.value == want {
			l1.caches[char].cache[*hash].value = setval
			l1.caches[char].cache[*hash].expires += 9
		}
	}
	l1.muxers[char].mux.Unlock()
	return
} // end func L1CACHE_GetSet
*/

func (l1 *L1CACHE) L1CACHE_Del(hash *string, char string) {
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
} // end func L1CACHE_Del
