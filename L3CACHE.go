package history

import (
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL3         bool
	L3CacheExpires  int64 = DefaultCacheExpires
	L3ExtendExpires int64 = DefaultCacheExtend
	L3Purge         int64 = DefaultCachePurge
	L3InitSize      int   = 256 * 1024
)

type L3CACHE struct {
	Caches   map[string]*L3CACHEMAP
	Extend   map[string]chan string
	muxers   map[string]*L3MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
	Counter  map[string]map[string]uint64
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
func (l3 *L3CACHE) L3CACHE_Boot(his *HISTORY) {
	l3.mux.Lock()
	defer l3.mux.Unlock()
	if l3.Caches != nil {
		log.Printf("ERROR L3CACHESetup already loaded!")
		return
	}
	l3.Caches = make(map[string]*L3CACHEMAP, 16)
	l3.Extend = make(map[string]chan string, 16)
	l3.muxers = make(map[string]*L3MUXER, 16)
	l3.mapsizes = make(map[string]*MAPSIZES, 16)
	l3.Counter = make(map[string]map[string]uint64)
	for _, char := range HEXCHARS {
		l3.Caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.Extend[char] = make(chan string, his.cEvCap)
		l3.muxers[char] = &L3MUXER{}
		l3.mapsizes[char] = &MAPSIZES{maxmapsize: L3InitSize}
		l3.Counter[char] = make(map[string]uint64)
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l3.L3Cache_Thread(char)
	}

} // end func L3CACHE_Boot

// The L3Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l3 *L3CACHE) L3Cache_Thread(char string) {
	l3.mux.Lock() // waits for L3CACHE_Boot to unlock
	l3.mux.Unlock()
	logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
	//lastshrink := utils.UnixTimeSec()
	cleanup := []string{}
	l3purge := L3Purge
	if l3purge < 1 {
		l3purge = 1
	}
	extends := make(map[string]bool, 1024)
	timer := time.NewTimer(time.Duration(l3purge) * time.Second)
	timeout := false
	start := utils.UnixTimeMilliSec()
forever:
	for {
		if timeout {
			timeout = false
			timer.Reset(time.Duration(l3purge) * time.Second)
		}
		select {
		case key := <-l3.Extend[char]: // receives stuff from DelExtL3batch()
			// got key we will extend in next timer.C run
			if key != "" {
				extends[key] = true
			}
		case <-timer.C:
			timeout = true
			start = utils.UnixTimeMilliSec()
			now := int64(start / 1000)
			l3.muxers[char].mux.Lock()
		getexpired:
			for key, item := range l3.Caches[char].cache {
				if extends[key] {
					if len(item.offsets) > 0 {
						l3.Caches[char].cache[key].offsets = item.offsets
						l3.Caches[char].cache[key].expires = now + L3ExtendExpires
						l3.Counter[char]["Count_BatchD"] += 1
					}
					//delete(extends, key)
					continue getexpired
				} else if item.expires > 0 && item.expires < now {
					//logf(DEBUG, "L3 expire [%s] key='%#v' item='%#v'", char, key, item)
					cleanup = append(cleanup, key)
				}
			} // end for getexpired
			maplen := len(l3.Caches[char].cache)
			//oldmax := l3.mapsizes[char].maxmapsize
			l3.muxers[char].mux.Unlock()
			clear(extends)
			if len(cleanup) > 0 {
				maplen -= len(cleanup)
				l3.muxers[char].mux.Lock()
				for _, key := range cleanup {
					delete(l3.Caches[char].cache, key)
					l3.Counter[char]["Count_Delete"] += 1
				}
				//max := l3.mapsizes[char].maxmapsize
				l3.muxers[char].mux.Unlock()
				logf(DEBUGL3, "L3Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
				cleanup = nil
			}
			logf(DEBUGL3, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
			continue forever
		} // end select
	} // end for

} //end func L3Cache_Thread

// The SetOffsets method sets a cache item in the L3 cache using a key, char and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets *[]int64, flagexpires bool, src string) {
	if key == "" {
		return
	}
	if char == "" {
		char = string(key[0])
	}
	if offsets == nil {
		return
	}
	l3.muxers[char].mux.Lock()
	defer l3.muxers[char].mux.Unlock()

	expires := NoExpiresVal
	if flagexpires {
		if len(*offsets) > 0 {
			l3.Counter[char]["Count_FlagEx"] += 1
		}
		expires = utils.UnixTimeSec() + L3CacheExpires
	} else {
		l3.Counter[char]["Count_Set"] += 1
	}

	if l3.Caches[char].cache[key] != nil {
		// cache entry exists
		l3.Caches[char].cache[key].expires = expires
		cachedlen := len(l3.Caches[char].cache[key].offsets)
		if cachedlen == 0 {
			// there is an empty offsets-slice cached: set this
			l3.Caches[char].cache[key].offsets = *offsets
			return
		}
		allexist := allValuesExistInSlice(*offsets, l3.Caches[char].cache[key].offsets)
		if allexist {
			return
		}
		cacheex := utils.UnixTimeSec() - l3.Caches[char].cache[key].expires
		newoffsetslen := len(*offsets)
		for _, offset := range *offsets {
			if !valueExistsInSlice(offset, l3.Caches[char].cache[key].offsets) {
				l3.Caches[char].cache[key].offsets = append(l3.Caches[char].cache[key].offsets, offset)
				log.Printf("INFO L3CACHE [%s] SetOffsets append key='%s' cached=%d='%#v' set?=%d='%#v' cacheex=%d newexpi=%d !valueExistsInSlice offset=%d src='%s'", char, key, cachedlen, l3.Caches[char].cache[key].offsets, newoffsetslen, offsets, cacheex, expires, offset, src)
			}
		}
		return
	}
	l3.Caches[char].cache[key] = &L3ITEM{offsets: *offsets, expires: expires}
	l3.mapsizes[char].maxmapsize++
} // end func SetOffsets

func allValuesExistInSlice(values []int64, slice []int64) bool {
	for _, v := range values {
		if !valueExistsInSlice(v, slice) {
			return false
		}
	}
	return true
}

func valueExistsInSlice(value int64, slice []int64) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// The GetOffsets method retrieves a slice of offsets from the L3 cache using a key and a char.
func (l3 *L3CACHE) GetOffsets(key string, char string) (offsets *[]int64) {
	if key == "" {
		log.Printf("ERROR L3CACHEGet key=nil")
		return
	}
	if char == "" {
		char = string(key[0])
	}
	l3.muxers[char].mux.Lock()
	if l3.Caches[char].cache[key] != nil {
		l3.Counter[char]["Count_Get"] += 1
		item := l3.Caches[char].cache[key]
		offsets = &item.offsets
		l3.muxers[char].mux.Unlock()
		return
	}
	l3.Counter[char]["Count_GetMiss"] += 1
	l3.muxers[char].mux.Unlock()
	return
} // end func GetOffsets

// The DelExtL3batch method deletes multiple cache items from the L3 cache.
func (l3 *L3CACHE) DelExtL3batch(his *HISTORY, char string, tmpKey []*ClearCache, flagCacheDelExt int) {
	if char == "" {
		log.Printf("ERROR DelExtL3batch char=nil")
		return
	}
	if len(tmpKey) == 0 {
		log.Printf("DelExtL3batch [%s] tmpKey empty", char)
		return
	}
	if flagCacheDelExt == FlagCacheChanExtend {
		for _, item := range tmpKey {
			if item.key != "" {
				if DEBUG {
					lench := len(l3.Extend[char])
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L3 Extend[%s]chan=%d/his.cEvCap=%d half-full", char, lench, his.cEvCap)
					}
				}
				l3.Extend[char] <- item.key
			}
		}
		return
	}
	now := utils.UnixTimeSec()
	l3.muxers[char].mux.Lock()
	for _, item := range tmpKey {
		if item.key != "" {
			if _, exists := l3.Caches[char].cache[item.key]; exists {
				switch flagCacheDelExt {
				case FlagCacheSyncDelete:
					delete(l3.Caches[char].cache, item.key)
				case FlagCacheSyncExtend:
					// dont delete from cache but extend expiry time
					l3.Caches[char].cache[item.key].expires = now + L3ExtendExpires
				}
				l3.Counter[char]["Count_BatchD"] += 1
			}
		}
	}
	l3.muxers[char].mux.Unlock()
} // end func DelExtL3batch

func (l3 *L3CACHE) L3Stats(key string) (retval uint64, retmap map[string]uint64) {
	if key == "" {
		retmap = make(map[string]uint64)
	}
	if l3 == nil || l3.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		l3.muxers[char].mux.Lock()
		switch key {
		case "":
			// key is empty, get all key=>stats to retmap
			for k, v := range l3.Counter[char] {
				retmap[k] += v
			}
		default:
			// key is set
			if _, exists := l3.Counter[char][key]; exists {
				retval += l3.Counter[char][key]
			}
		}
		l3.muxers[char].mux.Unlock()
	}
	return
} // end func L3Stats
