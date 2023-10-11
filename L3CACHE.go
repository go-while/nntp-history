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
	L3InitSize      int   = 64
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
	l3.mux.Lock() // waits for CACHE_Boot to unlock
	l3.mux.Unlock()
	logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
	lastshrink := utils.UnixTimeSec()
	cleanup := []string{}
	l3purge := L3Purge
	if l3purge < 1 {
		l3purge = 1
	}
	mapsize := 1024 * l3purge
	extends := make(map[string]bool, mapsize)
	timer := time.NewTimer(time.Duration(l3purge) * time.Second)
	timeout := false
	didnotexist := 0
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
			lenExt := len(extends)
			if didnotexist > 0 {
				log.Printf("INFO L3 [%s] extends=%d/%d didnotexist=%d", char, lenExt, mapsize, didnotexist)
			}
			now := utils.UnixTimeSec()
			start := utils.UnixTimeMilliSec()
			l3.muxers[char].mux.Lock()
		getexpired:
			for key, item := range l3.Caches[char].cache {
				if extends[key] {
					if len(l3.Caches[char].cache[key].offsets) > 0 {
						l3.Caches[char].cache[key].expires = now + L3ExtendExpires
						l3.Counter[char]["Count_BatchD"] += 1
					} else {
						didnotexist++
					}
					delete(extends, key)
					continue getexpired
				} else if item.expires > 0 && item.expires < now {
					//logf(DEBUG, "L3 expire [%s] key='%#v' item='%#v'", char, key, item)
					cleanup = append(cleanup, key)
				}
			} // end for getexpired
			maplen := len(l3.Caches[char].cache)
			oldmax := l3.mapsizes[char].maxmapsize
			l3.muxers[char].mux.Unlock()

			if len(cleanup) > 0 {
				maplen -= len(cleanup)
				l3.muxers[char].mux.Lock()
				for _, key := range cleanup {
					delete(l3.Caches[char].cache, key)
					l3.Counter[char]["Count_Delete"] += 1
				}
				max := l3.mapsizes[char].maxmapsize
				l3.muxers[char].mux.Unlock()
				logf(DEBUGL3, "L3Cache_Thread [%s] deleted=%d maplen=%d/%d", char, len(cleanup), maplen, max)
				cleanup = nil
			}
			if lastshrink < now-DefaultTryShrinkEvery {
				l3.shrinkMapIfNeeded(char, maplen, oldmax)
				lastshrink = now
			}
			if len(extends) != 0 {
				log.Printf("ERROR L3 [%s] extends=%d='%#v' != 0", char, len(extends), extends)
				//log.Printf("ERROR L3 [%s] extends=%d != 0", char, len(extends))
			} else {
				//clear(extends)
				//extends = nil
				if int64(lenExt) >= mapsize {
					mapsize = int64(lenExt) * 3
					logf(DEBUGL3, "INFO L3 [%s] grow extends=%d/%d didnotexist=%d", char, lenExt, mapsize, didnotexist)
				}
				extends = make(map[string]bool, mapsize)
			}
			logf(DEBUGL3, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
			continue forever
		} // end select
	} // end for

} //end func L3Cache_Thread

func (l3 *L3CACHE) shrinkMapIfNeeded(char string, maplen int, oldmax int) bool {
	shrinkmin := L3InitSize
	thresholdFactor := 20 // shrink if percentage used of cache is lower than N
	threshold := int(oldmax / 100 * thresholdFactor)
	if maplen > threshold {
		return true
	}
	thresmax := threshold * 2
	logf(DEBUGL3, "L3 [%s] PRE-SHRINK maplen=%d threshold=%d oldmax=%d thresmax=%d", char, maplen, threshold, oldmax, thresmax)
	if maplen < threshold && oldmax >= thresmax && thresmax > shrinkmin {
		newmax := threshold * 4
		if newmax < shrinkmin {
			newmax = shrinkmin
		} else if oldmax == shrinkmin && newmax == shrinkmin {
			// dont shrink lower than this
			return true
		}
		return l3.shrinkMap(char, newmax, maplen)
	}
	return false
} // end func shrinkMapIfNeeded

func (l3 *L3CACHE) shrinkMap(char string, newmax int, maplen int) bool {
	if char == "" {
		log.Printf("ERROR L3CACHE shrinkMap char=nil")
		return false
	}
	newmap := make(map[string]*L3ITEM, newmax)
	l3.muxers[char].mux.Lock()
	if maplen > 0 {
		for k, v := range l3.Caches[char].cache {
			newmap[k] = v
		}
	}
	//clear(l3.Caches[char].cache)
	//l3.Caches[char].cache = nil
	l3.Caches[char].cache = newmap
	l3.mapsizes[char].maxmapsize = newmax
	l3.Counter[char]["Count_Shrink"] += 1
	l3.muxers[char].mux.Unlock()
	logf(DBG_CGS, "L3Cache_Thread [%s] shrink size to %d maplen=%d", char, newmax, maplen)
	return true
} // end func shrinkMap

// The SetOffsets method sets a cache item in the L3 cache using a key, char and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets *[]int64, flagexpires bool) {
	if offsets == nil {
		return
	}
	if char == "" {
		char = string(key[0])
	}
	start := utils.UnixTimeMilliSec()
	l3.muxers[char].mux.Lock()

	if len(l3.Caches[char].cache) >= int(l3.mapsizes[char].maxmapsize/100*98) { // grow map
		newmax := l3.mapsizes[char].maxmapsize * 4
		newmap := make(map[string]*L3ITEM, newmax)
		for k, v := range l3.Caches[char].cache {
			newmap[k] = v
		}
		//clear(l3.Caches[char].cache)
		//l3.Caches[char].cache = nil
		l3.Caches[char].cache = newmap
		l3.mapsizes[char].maxmapsize = newmax
		l3.Counter[char]["Count_Growup"] += 1
		logf(DBG_CGS, "L3CACHE char=%s grow newmap=%d/%d (took %d ms)", char, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}
	expires := NoExpiresVal
	if flagexpires {
		if len(*offsets) > 0 {
			l3.Counter[char]["Count_FlagEx"] += 1
		}
		expires = utils.UnixTimeSec() + L3CacheExpires
	} else {
		l3.Counter[char]["Count_Set"] += 1
	}
	l3.Caches[char].cache[key] = &L3ITEM{offsets: *offsets, expires: expires}
	l3.muxers[char].mux.Unlock()
} // end func SetOffsets

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

/*
// The Delete method deletes a cache item from the L3 cache.
func (l3 *L3CACHE) DelExtL3(char *string, key *string) {
	if key == nil || *key == "" {
		log.Printf("ERROR L3CACHEDel key=nil")
		return
	}
	if char == nil || *char == "" {
		achar := string(string(*key)[0])
		char = &achar
	}
	l3.muxers[*char].mux.Lock()
	if _, exists := l3.Caches[*char].cache[*key]; exists {
		delete(l3.Caches[*char].cache, *key)
	}
	l3.muxers[*char].mux.Unlock()
} // end func DelExtL3
*/

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
					if lench >= int(float64(his.cEvCap)*0.75) {
						log.Printf("WARN L3 Extend[%s]chan=%d/his.cEvCap=%d near-full", char, lench, his.cEvCap)
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
