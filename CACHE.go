package history

import (
	//"fmt"
	"log"
	"time"
	//"sync"
)

const (
	// never change this
	FlagExpires            bool  = true
	FlagNeverExpires       bool  = false
	NoExpiresVal           int64 = -1
	FlagCacheSyncExtend          = 0x42
	FlagCacheSyncDelete          = 0x66
	FlagCacheChanExtend          = 0x99
	DefaultThresholdFactor       = 10 // shrinks caches if cache usage is below N percent
)

var (
	DBG_CGS               bool  // DEBUG_CACHE_GROW_SHRINK
	DefaultCacheExpires   int64 = 15
	DefaultCacheExtend    int64 = DefaultCacheExpires
	DefaultCachePurge     int64 = 5  // seconds
	DefaultTryShrinkEvery int64 = 60 // shrinks cache maps only every N seconds
	DefaultEvictsCapacity int   = 4  // his.cEvCap is normally fine as is
)

func (his *HISTORY) PrintCacheStats() {
	// L1CACHE

	//statsKeys := []string{"Count_Get","Count_GetMiss","Count_Locked","Count_Set","Count_Delete","Count_BatchD","Count_Growup","Count_Shrink"}
	l1cachesize := 0
	l1mapsize := 0
	l1medium := 0
	l1map := make(map[string]uint64)

	for _, char := range HEXCHARS {
		if his.L1Cache.muxers[char] == nil {
			continue
		}
		his.L1Cache.muxers[char].mux.Lock()
		l1cachesize += len(his.L1Cache.Caches[char].cache)
		l1mapsize += his.L1Cache.mapsizes[char].maxmapsize
		for k, v := range his.L1Cache.Counter[char] {
			l1map[k] += v
		}
		his.L1Cache.muxers[char].mux.Unlock()
	}
	if l1cachesize > 0 {
		l1medium = l1cachesize / 16
	}
	log.Printf("L1: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l1map["Count_FlagEx"], l1map["Count_Set"], l1map["Count_Get"], l1map["Count_GetMiss"], l1map["Count_Delete"], l1map["Count_BatchD"], l1map["Count_Growup"], l1map["Count_Shrink"], l1cachesize, l1medium)

	// L2CACHE
	l2cachesize := 0
	l2mapsize := 0
	l2medium := 0
	l2map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L2Cache.muxers[char] == nil {
			continue
		}
		his.L2Cache.muxers[char].mux.Lock()
		l2cachesize += len(his.L2Cache.Caches[char].cache)
		l2mapsize += his.L2Cache.mapsizes[char].maxmapsize
		for k, v := range his.L2Cache.Counter[char] {
			l2map[k] += v
		}
		his.L2Cache.muxers[char].mux.Unlock()
	}
	if l2cachesize > 0 {
		l2medium = l2cachesize / 16
	}
	log.Printf("L2: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l2map["Count_FlagEx"], l2map["Count_Set"], l2map["Count_Get"], l2map["Count_GetMiss"], l2map["Count_Delete"], l2map["Count_BatchD"], l2map["Count_Growup"], l2map["Count_Shrink"], l2cachesize, l2medium)

	// L3CACHE
	l3cachesize := 0
	l3mapsize := 0
	l3medium := 0
	l3map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L3Cache.muxers[char] == nil {
			continue
		}
		his.L3Cache.muxers[char].mux.Lock()
		l3mapsize += his.L3Cache.mapsizes[char].maxmapsize
		l3cachesize += len(his.L3Cache.Caches[char].cache)
		for k, v := range his.L3Cache.Counter[char] {
			l3map[k] += v
		}
		his.L3Cache.muxers[char].mux.Unlock()
	}
	if l3cachesize > 0 {
		l3medium = l3cachesize / 16
	}
	log.Printf("L3: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l3map["Count_FlagEx"], l3map["Count_Set"], l3map["Count_Get"], l3map["Count_GetMiss"], l3map["Count_Delete"], l3map["Count_BatchD"], l3map["Count_Growup"], l3map["Count_Shrink"], l3cachesize, l3medium)
} // end func PrintCacheStats

// gets called in BBATCH.go:boltBucketPutBatch() after boltTX
func (his *HISTORY) DoCacheEvict(char string, hash string, offset int64, key string) {
	// db
	if char == "" {
		log.Printf("ERROR CacheEvict char empty")
		return
	}
	set := 0
	if hash != "" { // l1
		set++
	}
	if offset > 0 { // l2
		set++
	}
	if key != "" { // l3
		set++
	}
	if set <= 0 { // need at least one value
		log.Printf("ERROR DoCacheEvict no values???")
		return
	}
	// pass ClearCache object to evictChan in CacheEvictThread()
	if DEBUG {
		lench := len(his.cacheEvicts[char])
		limit := int(float64(his.cEvCap) * 0.75)
		if lench > limit {
			log.Printf("WARN DoCacheEvict cacheEvicts[%s]chan=%d/%d limit=%d near-full", char, lench, his.cEvCap, limit)
		} else {
			//log.Printf("INFO DoCacheEvict cacheEvicts[%s]chan=%d/%d limit=%d OK", char, lench, his.cEvCap, limit)
		}
	}
	his.cacheEvicts[char] <- &ClearCache{char: char, offset: offset, hash: hash, key: key}
} // end func DoCacheEvict

func (his *HISTORY) CacheEvictThread() {
	if his.cacheEvicts != nil {
		log.Printf("ERROR CacheEvictThread already running!")
		return
	}
	his.cacheEvicts = make(map[string]chan *ClearCache)
	for _, char := range HEXCHARS {
		if his.cacheEvicts[char] != nil {
			log.Printf("ERROR CacheEvictThread [%s] already created!", char)
			continue
		}
		his.cacheEvicts[char] = make(chan *ClearCache, his.cEvCap)
		// launch a go func for every char with own evictChan
		go func(char string, evictChan chan *ClearCache) {
			var tmpHash []*ClearCache
			var tmpOffset []*ClearCache
			var tmpKey []*ClearCache
			clearEveryN := 1024
			timer := time.NewTimer(2500 * time.Millisecond)
			timeout := false
		forever:
			for {
			fetchdel:
				for {
					select {
					case <-timer.C:
						timeout = true
						Q := len(evictChan)
						if Q > 0 {
							logf(DEBUG, "CacheEvictThread [%s] case timer evictChan=%d", char, Q)
						}
						break fetchdel
					case item, ok := <-evictChan: // channel receives a ClearCache struct from DoCacheEvict()
						if !ok {
							log.Printf("evictChan [%s] closed", char)
							break forever
						}
						if item.char != char {
							log.Printf("ERROR evictChan [%s] item.char='%#v' != char", char, item.char)
							break forever
						}

						logf(DEBUG2, "evictChan [%s] item='%#v' to tmp", char, item)
						if item.hash != "" { // l1 hash
							tmpHash = append(tmpHash, item)
						}
						if item.offset > 0 { // l2 offset
							tmpOffset = append(tmpOffset, item)
						}
						if item.key != "" { // l3 key
							tmpKey = append(tmpKey, item)
						}
						if len(tmpHash) >= clearEveryN || len(tmpOffset) >= clearEveryN || len(tmpKey) >= clearEveryN {
							timer.Reset(2500 * time.Millisecond)
							//logf(DEBUG, "CacheEvictThread [%s] break fetchdel evictChan=%d", char, len(evictChan))
							break fetchdel
						}
					} // end select
				} // end for fetchdel
				if timeout || len(tmpHash) >= clearEveryN {
					his.L1Cache.DelExtL1batch(his, char, tmpHash, FlagCacheChanExtend)
					tmpHash = nil
				}
				if timeout || len(tmpOffset) >= clearEveryN {
					his.L2Cache.DelExtL2batch(his, tmpOffset, FlagCacheChanExtend)
					tmpOffset = nil
				}
				if timeout || len(tmpKey) >= clearEveryN {
					his.L3Cache.DelExtL3batch(his, char, tmpKey, FlagCacheChanExtend)
					tmpKey = nil
				}
				if timeout {
					timeout = false
					timer.Reset(2500 * time.Millisecond)
				}
				continue forever
			} // end forever
		}(char, his.cacheEvicts[char])
	} // end for HEXCHARS
} // end func CACHE_EVICTER
