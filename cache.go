package history

import (
	//"fmt"
	"log"
	"sync"
	"time"
	"github.com/go-while/go-utils"
)

type L1ITEM struct {
	value   int
	expires int64
	offset int64
}

type L1CACHE struct {
	caches map[string]*L1CACHEMAP
	muxers map[string]*L1MUXER
	mapsizes map[string]*MAPSIZES
	mux sync.Mutex
	//cmux sync.RWMutex
}

type L1CACHEMAP struct {
	cache map[string]*L1ITEM
}

type L1MUXER struct {
	mux sync.Mutex
}

type MAPSIZES struct {
	maxmapsize int
}

func (l1 *L1CACHE) L1CACHESetup() {
	initsize := 1024
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
		l1.caches[char] = &L1CACHEMAP{ cache: make(map[string]*L1ITEM, initsize) }
		l1.muxers[char] = &L1MUXER{}
		l1.mapsizes[char] = &MAPSIZES{ maxmapsize: initsize }
		syncgrowmap[char] = make(chan struct{}, 1)
	}
	go l1.L1CACHE_THREAD()
} // end func L1CACHESet

func (l1 *L1CACHE) LockL1Cache(hash *string, char string, value int) (int) {
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
	l1.caches[char].cache[*hash] = &L1ITEM{ value: value, expires: utils.UnixTimeSec()+DefaultL1CacheExpires }
	return 0
} // end func LockL1Cache

func (l1 *L1CACHE) L1CACHE_THREAD() {
//forever:
	for {
		time.Sleep(2500 * time.Millisecond)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		for _, char := range HEXCHARS {
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
				if maplen < 1024 && max > 1024 || maplen == 0 && max == 1024 {
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

					logf(DEBUG1,"L1CACHE_THREAD [%s] shrink size to 1024", char)
				}
				newmax := l1.mapsizes[char].maxmapsize
				l1.muxers[char].mux.Unlock()
				logf(DEBUG1,"L1CACHE_THREAD [%s] deleted=%d maplen=%d/%d oldmax=%d", char, len(cleanup), maplen, newmax, max)
			}
			cleanup = nil
		}
		logf(DEBUG1,"L1CACHE_THREAD (took %d ms)", utils.UnixTimeMilliSec()-start)
	} // end forever
} //end func L1CACHE_THREAD

func (l1 *L1CACHE) L1CACHESet(hash *string, char string, value int) {
	if hash == nil || len(*hash) < 32 { // at least md5
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}

	//l1.muxers[char].mux.RLock()
	l1.muxers[char].mux.Lock()
	mapsize := len(l1.caches[char].cache)
	max := l1.mapsizes[char].maxmapsize
	limit := int(l1.mapsizes[char].maxmapsize/100*75) // grow at 75% fill
	//l1.muxers[char].mux.RUnlock()
	if mapsize >= limit {
		/*
		doresize := false
		select {
			case syncgrowmap[char] <- struct{}{}:
				doresize = true
			default:
				log.Printf("INFO grow L1CACHE[%s] already locked", char)
				// grow is already locked
		}
		if doresize {
		*/
			start := utils.UnixTimeMilliSec()
			//l1.muxers[char].mux.Lock()
			// grow map
			//newmax := max + (max/100*50)
			newmax := max*2
			newmap := make(map[string]*L1ITEM, newmax)
			for k, v := range l1.caches[char].cache {
				newmap[k] = v
			}
			//l1.mux.Lock()
			//l1.caches[char].cache = nil
			l1.caches[char].cache = newmap
			l1.mapsizes[char].maxmapsize = newmax
			//l1.mux.Unlock()
			logf(DEBUG1,"L1CACHE char=%s oldmap=%d newmap=%d newmax=%d (took %d ms)", char, mapsize, len(newmap), newmax, utils.UnixTimeMilliSec()-start)
			//l1.muxers[char].mux.Unlock()
			//<- syncgrowmap[char] // unlocks channel
		//}
	}

	l1.caches[char].cache[*hash] = &L1ITEM{ value: value, expires: utils.UnixTimeSec()+DefaultL1CacheExpires }
	l1.muxers[char].mux.Unlock()
	//l1.muxers[char].mux.Unlock()
} // end func L1CACHESet

func (l1 *L1CACHE) L1CACHEGet(hash *string, char string) (retval *int) {
	if hash == nil || *hash == "" {
		log.Printf("ERROR L1CACHEGet hash=nil")
		return
	}
	if char == "" {
		char = string(string(*hash)[0])
	}
	now := utils.UnixTimeSec()
	l1.muxers[char].mux.Lock()
	if l1.caches[char].cache[*hash] != nil {
		item := l1.caches[char].cache[*hash]
		if item.expires >= now {
			retval = &item.value
		} else {
			//go l1.L1CACHEDel(hash, char)
		}
	}
	l1.muxers[char].mux.Unlock()
	return
} // end func L1CACHEGet

func (l1 *L1CACHE) L1CACHEDel(hash string, char string) () {
	if hash == "" {
		log.Printf("ERROR L1CACHEDel hash empty")
		return
	}
	if char == "" {
		char = string(hash[0])
	}
	l1.muxers[char].mux.Lock()
	delete(l1.caches[char].cache, hash)
	l1.muxers[char].mux.Unlock()
} // end func L1CACHEDel
