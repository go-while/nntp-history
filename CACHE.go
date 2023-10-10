package history

import (
	//"fmt"
	"log"
	"time"
)

const (
	DefaultTryShrinkEvery int64 = 5 // shrinks cache maps only every N seconds
	FlagExpires           bool  = true
	FlagNeverExpires      bool  = false
	NoExpiresVal          int64 = -1
)

var (
	DBG_CGS bool // DEBUG_CACHE_GROW_SHRINK
)

func (his *HISTORY) PrintCacheStats() {
	// L1CACHE
	//his.L1Cache.mux.Lock()
	//statsKeys := []string{"Count_Locked","Count_Insert","Count_Delete","Count_BatchD","Count_Growup","Count_Shrink"}
	l1cachesize := 0
	l1mapsize := 0
	l1medium := 0
	l1map := make(map[string]uint64)

	for _, char := range HEXCHARS {
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
	log.Printf("L1: [fex=%d/set:%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l1map["Count_FlagEx"], l1map["Count_Insert"], l1map["Count_Delete"], l1map["Count_BatchD"], l1map["Count_Growup"], l1map["Count_Shrink"], l1cachesize, l1medium)
	//his.L1Cache.mux.Unlock()

	// L2CACHE
	//his.L2Cache.mux.Lock()
	l2cachesize := 0
	l2mapsize := 0
	l2medium := 0
	l2map := make(map[string]uint64)
	for _, char := range HEXCHARS {
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
	log.Printf("L2: [fex=%d/set:%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l2map["Count_FlagEx"], l2map["Count_Insert"], l2map["Count_Delete"], l2map["Count_BatchD"], l2map["Count_Growup"], l2map["Count_Shrink"], l2cachesize, l2medium)
	//his.L2Cache.mux.Unlock()

	// L3CACHE
	//his.L3Cache.mux.Lock()
	l3cachesize := 0
	l3mapsize := 0
	l3medium := 0
	l3map := make(map[string]uint64)
	for _, char := range HEXCHARS {
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
	log.Printf("L3: [fex=%d/set:%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l3map["Count_FlagEx"], l3map["Count_Insert"], l3map["Count_Delete"], l3map["Count_BatchD"], l3map["Count_Growup"], l3map["Count_Shrink"], l3cachesize, l3medium)
	//his.L3Cache.mux.Unlock()
} // end func PrintCacheStats

// gets called in BBATCH.go:boltBucketPutBatch() after boltTX
func (his *HISTORY) DoCacheEvict(char string, hash string, offset int64, key string) {
	// db
	if char == "" {
		log.Printf("ERROR CacheEvict char empty")
		return
	}
	set := 0
	// l1 key
	if hash != "" {
		set++
	}
	// l2 key
	if offset > 0 {
		set++
	}
	// l3 key
	if key != "" {
		set++
	}
	if set <= 0 { // need at least one value
		log.Printf("ERROR DoCacheEvict no values???")
		return
	}
	// pass ClearCache object to evictChan in CacheEvictThread()
	his.CacheEvicts[char] <- &ClearCache{char: &char, offset: &offset, hash: &hash, key: &key}
} // end func DoCacheEvict

func (his *HISTORY) CacheEvictThread() {
	if his.CacheEvicts != nil {
		log.Printf("ERROR CacheEvictThread already running!")
		return
	}
	his.CacheEvicts = make(map[string]chan *ClearCache)
	for _, char := range HEXCHARS {
		if his.CacheEvicts[char] != nil {
			log.Printf("ERROR CacheEvictThread [%s] already created!", char)
			continue
		}
		evictChan := make(chan *ClearCache, 1000)
		his.CacheEvicts[char] = evictChan
		// launch a go func for every char with own evictChan
		go func(char string, evictChan chan *ClearCache) {
			var tmpHash []*ClearCache
			var tmpOffset []*ClearCache
			var tmpKey []*ClearCache
			clearEveryN := 100
			timer := time.NewTimer(5000 * time.Millisecond)
			timeout := false
		forever:
			for {
			fetchdel:
				for {
					select {
					case <-timer.C:
						timeout = true
						timer.Reset(5000 * time.Millisecond)
						//logf(DEBUG2, "CacheEvictThread [%s] case timer", char)
						break fetchdel
					case item, ok := <-evictChan: // channel receives a ClearCache struct from DoCacheEvict()
						if !ok {
							log.Printf("evictChan [%s] closed", char)
							break forever
						}
						if item.char == nil || *item.char != char {
							log.Printf("ERROR evictChan [%s] item.char='%#v' != char", char, item.char)
							break forever
						}

						logf(DEBUG2, "evictChan [%s] item='%#v' to tmp", char, item)
						if item.hash != nil { // l1 key
							tmpHash = append(tmpHash, item)
						}
						if item.offset != nil { // l2key
							tmpOffset = append(tmpOffset, item)
						}
						if item.key != nil { // l3key
							tmpKey = append(tmpKey, item)
						}
						if len(tmpHash) >= clearEveryN || len(tmpOffset) >= clearEveryN || len(tmpKey) >= clearEveryN {
							timer.Reset(5000 * time.Millisecond)
							break fetchdel
						}
					} // end select
				} // end for fetchdel
				if timeout || len(tmpHash) >= clearEveryN {
					his.L1Cache.DeleteL1batch(&char, &tmpHash)
					tmpHash = nil
				}
				if timeout || len(tmpOffset) >= clearEveryN {
					his.L2Cache.DeleteL2batch(&tmpOffset)
					tmpOffset = nil
				}
				if timeout || len(tmpKey) >= clearEveryN {
					his.L3Cache.DeleteL3batch(&char, &tmpKey)
					tmpKey = nil
				}
				continue forever
			} // end forever
		}(char, evictChan)
	} // end for HEXCHARS
} // end func CACHE_EVICTER
