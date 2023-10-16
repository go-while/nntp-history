package history

import (
	//"fmt"
	"log"
	"math/rand"
	"time"
	//"sync"
)

const (
	// never change this
	FlagExpires      bool  = true
	FlagNeverExpires bool  = false
	NoExpiresVal     int64 = -1
	// CacheEvictThread => his.LXCache.DelExtLXbatch
	FlagCacheSyncExtend = 0x66
	FlagCacheSyncDelete = 0x99
	FlagCacheChanExtend = 0x42
)

var (
	DBG_CGS               bool       // DEBUG_CACHE_GROW_SHRINK
	DefaultCacheExpires   int64 = 15 // search only
	DefaultCacheExtend    int64 = 15 // extends cached items after writes
	DefaultCachePurge     int64 = 1  // seconds
	DefaultEvictsCapacity int   = 64 // his.cEvCap is normally fine as is
)

// CharCacheCounter
type CCC struct {
	Counter map[string]uint64 // counter key: value
}

// StrExtendChan
type StrECH struct {
	ch chan string
}

// IntExtendChan
type IntECH struct {
	ch chan int64
}

func (his *HISTORY) PrintCacheStats() {
	/*
	 * statsKeys := []string{
	 * 		"Count_Get",
	 * 		"Count_Mis",
	 * 		"Count_Locked",
	 * 		"Count_Set",
	 * 		"Count_Delete",
	 * 		"Count_BatchD",
	 * 		"Count_Growup",
	 * 		"Count_Shrink"
	 *		}
	 */

	// L1CACHE
	l1cachesize := 0
	//l1mapsize := 0
	l1medium := 0
	l1map := make(map[string]uint64)

	for _, char := range HEXCHARS {
		if his.L1Cache.muxers[char] == nil {
			continue
		}
		his.L1Cache.muxers[char].mux.Lock()
		l1cachesize += len(his.L1Cache.Caches[char].cache)
		for k, v := range his.L1Cache.Counter[char].Counter {
			l1map[k] += v
		}
		his.L1Cache.muxers[char].mux.Unlock()
	}
	if l1cachesize > 0 {
		l1medium = l1cachesize / 16
	}
	log.Printf("L1: [fex=%d/set:%d] [get=%d/mux=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l1map["Count_FlagEx"], l1map["Count_Set"], l1map["Count_Get"], l1map["Count_Locked"], l1map["Count_Delete"], l1map["Count_BatchD"], l1map["Count_Growup"], l1map["Count_Shrink"], l1cachesize, l1medium)

	// L2CACHE
	l2cachesize := 0
	//l2mapsize := 0
	l2medium := 0
	l2map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L2Cache.muxers[char] == nil {
			continue
		}
		his.L2Cache.muxers[char].mux.Lock()
		l2cachesize += len(his.L2Cache.Caches[char].cache)
		for k, v := range his.L2Cache.Counter[char].Counter {
			l2map[k] += v
		}
		his.L2Cache.muxers[char].mux.Unlock()
	}
	if l2cachesize > 0 {
		l2medium = l2cachesize / 16
	}
	log.Printf("L2: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l2map["Count_FlagEx"], l2map["Count_Set"], l2map["Count_Get"], l2map["Count_Mis"], l2map["Count_Delete"], l2map["Count_BatchD"], l2map["Count_Growup"], l2map["Count_Shrink"], l2cachesize, l2medium)

	// L3CACHE
	l3cachesize := 0
	//l3mapsize := 0
	l3medium := 0
	l3map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L3Cache.muxers[char] == nil {
			continue
		}
		his.L3Cache.muxers[char].mux.Lock()
		l3cachesize += len(his.L3Cache.Caches[char].cache)
		for k, v := range his.L3Cache.Counter[char].Counter {
			l3map[k] += v
		}
		his.L3Cache.muxers[char].mux.Unlock()
	}
	if l3cachesize > 0 {
		l3medium = l3cachesize / 16
	}
	log.Printf("L3: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l3map["Count_FlagEx"], l3map["Count_Set"], l3map["Count_Get"], l3map["Count_Mis"], l3map["Count_Delete"], l3map["Count_BatchD"], l3map["Count_Growup"], l3map["Count_Shrink"], l3cachesize, l3medium)
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
	/*
		 *
			if DEBUG {
				lench := len(his.cacheEvicts[char])
				limit := int(float64(his.cEvCap) * 0.50)
				if lench > limit {
					log.Printf("WARN DoCacheEvict cacheEvicts[%s]chan=%d/%d limit=%d near-full", char, lench, his.cEvCap, limit)
				} else {
					//log.Printf("INFO DoCacheEvict cacheEvicts[%s]chan=%d/%d limit=%d OK", char, lench, his.cEvCap, limit)
				}
			}
		*
	*/
	// pass down to CacheEvictThread
	his.cacheEvicts[char] <- &ClearCache{char: char, offset: offset, hash: hash, key: key}
} // end func DoCacheEvict

func jitter(j int, timer int) int {
	randInt := rand.Intn(j)
	if randInt < j/2 {
		return timer - randInt
	}
	return timer + randInt
}

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
		j := 50 // jitter
		his.cacheEvicts[char] = make(chan *ClearCache, his.cEvCap)
		// launch a go func for every char with own evictChan
		go func(char string, evictChan chan *ClearCache) {
			var tmpHash []*ClearCache
			var tmpOffset []*ClearCache
			var tmpKey []*ClearCache
			clearEveryN := 500
			basetimer := 500
			timer := time.NewTimer(time.Duration(jitter(j, basetimer)) * time.Millisecond)
			//timeout := false
			var del1, del2, del3 bool
		forever:
			for {
			fetchdel:
				for {
					select {
					case <-timer.C:
						if len(tmpHash) > 0 {
							del1 = true
						}
						if len(tmpOffset) > 0 {
							del2 = true
						}
						if len(tmpKey) > 0 {
							del3 = true
						}
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

						//logf(DEBUG2, "evictChan [%s] item='%#v' to tmp", char, item)
						if item.offset > 0 { // l2 offset
							tmpOffset = append(tmpOffset, item)
						} else {
							if item.hash != "" { // l1 hash
								tmpHash = append(tmpHash, item)
							}
							if item.key != "" { // l3 key
								tmpKey = append(tmpKey, item)
							}
						}

						if len(tmpHash) >= clearEveryN {
							del1 = true
						}
						if len(tmpOffset) >= clearEveryN {
							del2 = true
						}
						if len(tmpKey) >= clearEveryN {
							del3 = true
						}
						if del1 || del2 || del3 {
							break fetchdel
						}
					} // end select
				} // end for fetchdel
				if del1 {
					his.L1Cache.DelExtL1batch(his, char, tmpHash, FlagCacheChanExtend)
					tmpHash = nil
					del1 = false
				}
				if del2 {
					his.L2Cache.DelExtL2batch(his, tmpOffset, FlagCacheChanExtend)
					tmpOffset = nil
					del2 = false
				}
				if del3 {
					his.L3Cache.DelExtL3batch(his, char, tmpKey, FlagCacheChanExtend)
					tmpKey = nil
					del3 = false
				}
				timer.Reset(time.Duration(jitter(j, basetimer)) * time.Millisecond)
				continue forever
			} // end forever
		}(char, his.cacheEvicts[char])
	} // end for HEXCHARS
} // end func CACHE_EVICTER
