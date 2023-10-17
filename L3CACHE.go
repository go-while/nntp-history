package history

import (
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL3         bool  = false
	L3CacheExpires  int64 = DefaultCacheExpires
	L3ExtendExpires int64 = DefaultCacheExtend
	L3Purge         int64 = DefaultCachePurge
	L3InitSize      int   = 256 * 1024
)

type L3CACHE struct {
	Caches  map[string]*L3CACHEMAP
	Extend  map[string]*StrECH
	muxers  map[string]*L3MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
}

type L3CACHEMAP struct {
	cache map[string]*L3ITEM
}

type L3ITEM struct {
	offsets []int64
	expires int64
}

type L3MUXER struct {
	mux sync.RWMutex
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
	l3.Caches = make(map[string]*L3CACHEMAP, intBoltDBs)
	l3.Extend = make(map[string]*StrECH, intBoltDBs)
	l3.muxers = make(map[string]*L3MUXER, intBoltDBs)
	l3.Counter = make(map[string]*CCC)
	for _, char := range HEXCHARS {
		l3.Caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.Extend[char] = &StrECH{ch: make(chan *string, his.cEvCap)}
		l3.muxers[char] = &L3MUXER{}
		l3.Counter[char] = &CCC{Counter: make(map[string]uint64)}
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
	//logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
	cleanup := []string{}
	l3purge := L3Purge
	if l3purge < 1 {
		l3purge = 1
	}

	go func(ptr *L3CACHEMAP, mux *sync.RWMutex, cnt *CCC, extendChan *StrECH) {
		defer log.Printf("LEFT L3T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l3purge) * time.Second)
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
				case key := <-extendChan.ch: // receives stuff from DelExtL3batch()
					if key == nil {
						log.Printf("ERROR L3 extend ch received nil hash")
						continue forextends
					}
					// got key we will extend in next timer.C run
					extends = append(extends, *key)
					ext++
					if ext >= emax {
						timeout = true
						break forextends
					}
				} // end select
			} // end forextends
			if (timeout && ext > 0) || ext >= emax {
				now := utils.UnixTimeSec()
				//logf(DEBUG, "L3 [%s] extends=%d", char, len(extends))
				mux.Lock()
				for _, key := range extends {
					if _, exists := ptr.cache[key]; exists {
						ptr.cache[key].expires = now + L3ExtendExpires
						cnt.Counter["Count_BatchD"]++
					}
				}
				mux.Unlock()
				extends = nil
				timeout = false
				ext = 0
				timer.Reset(time.Duration(l3purge) * time.Second)
			}
		} // end forever
	}(l3.Caches[char], &l3.muxers[char].mux, l3.Counter[char], l3.Extend[char]) // end gofunc1

	go func(ptr *L3CACHEMAP, mux *sync.RWMutex, cnt *CCC) {
		defer log.Printf("LEFT L3T gofunc2 delete [%s]", char)
		timer := time.NewTimer(time.Duration(l3purge) * time.Second)
		timeout := false
		start := utils.UnixTimeMilliSec()
		now := int64(start / 1000)
	forever:
		for {
			if timeout {
				timeout = false
				timer.Reset(time.Duration(l3purge) * time.Second)
			}
			select {
			case <-timer.C:
				timeout = true
				start = utils.UnixTimeMilliSec()
				now = int64(start / 1000)

				mux.Lock()
				//getexpired:
				for key, item := range ptr.cache {
					if item.expires > 0 && item.expires < now {
						//logf(DEBUG, "L3 expire [%s] key='%#v' item='%#v'", char, key, item)
						cleanup = append(cleanup, key)
					}
				} // end for getexpired

				//maplen := len(ptr.cache)
				if len(cleanup) > 0 {
					//maplen -= len(cleanup)
					for _, key := range cleanup {
						delete(ptr.cache, key)
						l3.Counter[char].Counter["Count_Delete"]++
					}
					//logf(DEBUG, "L3Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
				}
				mux.Unlock()
				cleanup = nil
				//logf(DEBUG, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
				continue forever
			} // end select
		} // end for
	}(l3.Caches[char], &l3.muxers[char].mux, l3.Counter[char]) // end gofunc2
} //end func L3Cache_Thread

// The SetOffsets method sets a cache item in the L3 cache using a key, char and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets []int64, flagexpires bool, src string) {
	//if key == TESTCACKEY {
	//	log.Printf("L3CAC [%s|  ] SetOffsets key='%s' offsets='%#v' flagexpires=%t src='%s'", char, key, offsets, flagexpires, src)
	//}
	if key == "" {
		return
	}
	if char == "" {
		char = string(key[0])
	}
	if offsets == nil {
		return
	}

	ptr := l3.Caches[char]
	cnt := l3.Counter[char]
	mux := l3.muxers[char]

	mux.mux.Lock()

	expires := NoExpiresVal
	if flagexpires {
		if len(offsets) > 0 {
			cnt.Counter["Count_FlagEx"]++
		}
		expires = utils.UnixTimeSec() + L3CacheExpires
	} else {
		cnt.Counter["Count_Set"]++
	}
	//var tailstr string
	if _, exists := ptr.cache[key]; exists {
		// cache entry exists
		ptr.cache[key].expires = expires
		cachedlen := len(ptr.cache[key].offsets)
		if cachedlen == 0 {
			// there is an empty offsets-slice cached: set this
			ptr.cache[key].offsets = offsets
			mux.mux.Unlock()
			return
		}
		// loops in reversed order backwards over new offsets
		for i := len(offsets) - 1; i >= 0; i-- {
			if offsets[i] <= 0 {
				log.Printf("ERROR L3CACHE SetOffsets offsets[i]=%d", offsets[i])
				continue
			}
			// checks cached offsets backwards too
			//tailstr := fmt.Sprintf("key='%s' cached=%d='%#v' i=%d %d/%d='%#v' cacheex=%d newexpi=%d offsets[i]=%d src='%s'", key, cachedlen, l3.Caches[char].cache[key].offsets, i, i+1, len(offsets), offsets, l3.Caches[char].cache[key].expires, expires, offsets[i], src)
			if !valueExistsInSliceReverseOrder(offsets[i], ptr.cache[key].offsets) {
				//logf(DEBUG, "INFO L3CACHE [%s] SetOffsets append %s", char, tailstr)
				ptr.cache[key].offsets = append(ptr.cache[key].offsets, offsets[i])
			} else {
				//logf(DEBUG, "INFO L3CACHE [%s] SetOffsets exists %s", char, tailstr)
				// NOTE with valueExistsInSliceReverseOrder first hit returns fast now
				mux.mux.Unlock()
				return
			}
		}
		mux.mux.Unlock()
		return
	}
	ptr.cache[key] = &L3ITEM{offsets: offsets, expires: expires}
	mux.mux.Unlock()
} // end func SetOffsets

// The GetOffsets method retrieves a slice of offsets from the L3 cache using a key and a char.
func (l3 *L3CACHE) GetOffsets(key string, char string, offsets *[]int64) int {
	if key == "" || offsets == nil || len(*offsets) > 0 {
		log.Printf("ERROR L3CACHEGet key or io nil or var `offsets` not empty")
		return 0
	}
	if char == "" {
		char = string(key[0])
	}
	ptr := l3.Caches[char]
	//cnt := l3.Counter[char]
	mux := l3.muxers[char]

	mux.mux.RLock()
	if _, exists := ptr.cache[key]; exists {
		//cnt.Counter["Count_Get"]++ // cant count this here! we only have RLOCK!
		*offsets = ptr.cache[key].offsets
		mux.mux.RUnlock()
		return len(*offsets)
	}
	mux.mux.RUnlock()
	//cnt.Counter["Count_Mis"]++ // cant count this here! we only have RLOCK!
	return 0
} // end func GetOffsets

// The DelExtL3batch method deletes multiple cache items from the L3 cache.
func (l3 *L3CACHE) DelExtL3batch(his *HISTORY, char string, tmpKey []*ClearCache) {
	if char == "" {
		log.Printf("ERROR DelExtL3batch char=nil")
		return
	}
	if len(tmpKey) == 0 {
		log.Printf("DelExtL3batch [%s] tmpKey empty", char)
		return
	}
	for _, item := range tmpKey {
		if item.key != nil && *item.key != "" {
			/*
				if DEBUG {
					lench := len(l3.Extend[char].ch)
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L3 Extend[%s]chan=%d/his.cEvCap=%d half-full", char, lench, his.cEvCap)
					}
				}
			*/
			l3.Extend[char].ch <- item.key
		}
	}
} // end func DelExtL3batch

func (l3 *L3CACHE) L3Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l3 == nil || l3.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l3.Counter[char]
		mux := l3.muxers[char]
		mux.mux.RLock()
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
		mux.mux.RUnlock()
	}
	return
} // end func L3Stats

func valueExistsInSliceReverseOrder(value int64, slice []int64) bool {
	for i := len(slice) - 1; i >= 0; i-- {
		if slice[i] == value {
			return true
		}
	}
	return false
}
