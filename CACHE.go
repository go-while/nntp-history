package history

import (
	//"fmt"
	"log"
	"time"
	//"sync"
)

const (
	// never change this
	FlagExpires      bool  = true
	FlagNeverExpires bool  = false
	NoExpiresVal     int64 = -1
)

var (
	UseArenas             bool
	DBG_CGS               bool               // DEBUG_CACHE_GROW_SHRINK
	DefaultCacheExpires   int64 = 5          // search only
	DefaultCacheExtend    int64 = 5          // extends cached items after writes
	DefaultCachePurge     int64 = 1          // checks ttl every N seconds. affects CacheExpires/Extend max to + Purge
	DefaultEvictsCapacity int   = 128 * 1024 // his.cEvCap is normally fine as is but higher values can give better performance
)

// CharCacheCounter
type CCC struct {
	Counter map[string]uint64 // counter key: value
}

type ClearCacheChan struct {
	ch chan []*ClearCache
}

// StrExtendChan
type StrECH struct {
	ch chan []string
}

// IntExtendChan
type IntECH struct {
	ch chan []int64
}

type DQSlice []string // delete queue slice

func (his *HISTORY) PrintCacheStats() {
	/*
	 * statsKeys := []string{
	 * 		"Count_Get",
	 * 		"Count_Mis",
	 * 		"Count_Locked",
	 * 		"Count_Set",
	 * 		"Count_Delete",
	 * 		"Count_BatchD",
	 *		}
	 */

	// L1CACHE
	l1cachesize := 0
	l1medium := 0
	l1map := make(map[string]uint64)

	for _, char := range HEXCHARS {
		if his.L1Cache.Muxers[char] == nil {
			continue
		}
		his.L1Cache.Muxers[char].mux.Lock()
		l1cachesize += len(his.L1Cache.Caches[char].cache)
		for k, v := range his.L1Cache.Counter[char].Counter {
			l1map[k] += v
		}
		his.L1Cache.Muxers[char].mux.Unlock()
	}
	if l1cachesize > 0 {
		l1medium = l1cachesize / 16
	}
	log.Printf("L1: [fex=%d/set:%d] [get=%d/mux=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l1map["Count_FlagEx"], l1map["Count_Set"], l1map["Count_Get"], l1map["Count_Locked"], l1map["Count_Delete"], l1map["Count_BatchD"], l1map["Count_Growup"], l1map["Count_Shrink"], l1cachesize, l1medium)

	// L2CACHE
	l2cachesize := 0
	l2medium := 0
	l2map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L2Cache.Muxers[char] == nil {
			continue
		}
		his.L2Cache.Muxers[char].mux.Lock()
		l2cachesize += len(his.L2Cache.Caches[char].cache)
		for k, v := range his.L2Cache.Counter[char].Counter {
			l2map[k] += v
		}
		his.L2Cache.Muxers[char].mux.Unlock()
	}
	if l2cachesize > 0 {
		l2medium = l2cachesize / 16
	}
	log.Printf("L2: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l2map["Count_FlagEx"], l2map["Count_Set"], l2map["Count_Get"], l2map["Count_Mis"], l2map["Count_Delete"], l2map["Count_BatchD"], l2map["Count_Growup"], l2map["Count_Shrink"], l2cachesize, l2medium)

	// L3CACHE
	l3cachesize := 0
	l3medium := 0
	l3map := make(map[string]uint64)
	for _, char := range HEXCHARS {
		if his.L3Cache.Muxers[char] == nil {
			continue
		}
		his.L3Cache.Muxers[char].mux.Lock()
		l3cachesize += len(his.L3Cache.Caches[char].cache)
		for k, v := range his.L3Cache.Counter[char].Counter {
			l3map[k] += v
		}
		his.L3Cache.Muxers[char].mux.Unlock()
	}
	if l3cachesize > 0 {
		l3medium = l3cachesize / 16
	}
	log.Printf("L3: [fex=%d/set:%d] [get=%d/mis=%d] [del:%d/bat:%d] [g/s:%d/%d] cached:%d (~%d/char)", l3map["Count_FlagEx"], l3map["Count_Set"], l3map["Count_Get"], l3map["Count_Mis"], l3map["Count_Delete"], l3map["Count_BatchD"], l3map["Count_Growup"], l3map["Count_Shrink"], l3cachesize, l3medium)
} // end func PrintCacheStats

// gets called in BBATCH.go:boltBucketPutBatch() after boltTX
func (his *HISTORY) DoCacheEvict(char string, hash string, offset int64, key string) {
	if char == "" {
		// char derived from hash or for offset: offset=>hex[lastchar]
		log.Printf("ERROR CacheEvict char empty.")
		return
	}

	if DEBUG {
		lench := len(his.cacheEvicts[char])
		limit := int(float64(his.cEvCap) * 0.95)
		if lench > limit {
			log.Printf("WARN DoCacheEvict cacheEvicts[%s]chan=%d/%d warn>%d near-full", char, lench, his.cEvCap, limit)
		} else {
			//log.Printf("INFO DoCacheEvict cacheEvicts[%s]chan=%d/%d limit=%d OK", char, lench, his.cEvCap, limit)
		}
	}

	// pass ClearCache object to evictChan in CacheEvictThread()
	his.cacheEvicts[char] <- &ClearCache{char: char, offset: offset, hash: hash, key: key}
} // end func DoCacheEvict

func (his *HISTORY) CacheEvictThread(num int) {
	if BootHisCli {
		return
	}
	if his.cacheEvicts != nil {
		log.Printf("ERROR CacheEvictThread already running!")
		return
	}
	his.cacheEvicts = make(map[string]chan *ClearCache)
	//delay, j := 0, 0
	log.Printf("Booting CacheEvictThread")

	for _, char := range HEXCHARS {
		if his.cacheEvicts[char] == nil {
			his.cacheEvicts[char] = make(chan *ClearCache, his.cEvCap)
			log.Printf("CacheEvictThread [%s] created", char)
		}
		//delay = j * int(BatchFlushEvery) / len(HEXCHARS)
		for i := 1; i <= num; i++ {
			// launch a go func for every char with own evictChan
			go func(char string, i int, evictChan chan *ClearCache) {
				//time.Sleep(time.Duration(delay) * time.Millisecond)

				l1MUX := his.L1Cache.Muxers[char]
				l2MUX := his.L2Cache.Muxers[char]
				l3MUX := his.L3Cache.Muxers[char]

				// wait for caches to boot
				l1MUX.mux.Lock()
				l1MUX.mux.Unlock()
				l2MUX.mux.Lock()
				l2MUX.mux.Unlock()
				l3MUX.mux.Lock()
				l3MUX.mux.Unlock()

				l1ext := his.L1Cache.Extend[char]
				l2ext := his.L2Cache.Extend[char]
				l3ext := his.L3Cache.Extend[char]

				clearEveryN := his.cEvCap // DefaultEvictsCapacity
				basetimer := DefaultCachePurge
				tmpHash := []string{}
				tmpOffset := []int64{}
				tmpKey := []string{}

				timer := time.NewTimer(time.Duration(basetimer) * time.Second)
				var del1, del2, del3 bool
				var add1, add2, add3 int
			forever:
				for {
					select {
					case <-timer.C:
						del1 = len(tmpHash) > 0
						del2 = len(tmpOffset) > 0
						del3 = len(tmpKey) > 0
						//Q := len(evictChan)
						//if Q > 0 {
						//	logf(DEBUG2, "CacheEvictThread [%s] case timer evictChan=%d", char, Q)
						//}
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
							tmpOffset = append(tmpOffset, item.offset)
							add2++ // L2
						} else {
							if item.hash != "" { // l1 hash
								tmpHash = append(tmpHash, item.hash)
								add1++ // L1
							}
							if item.key != "" { // l3 key
								tmpKey = append(tmpKey, item.key)
								add3++ // L3
							}
						}
						if add1 >= clearEveryN {
							del1 = true // L1
						}
						if add2 >= clearEveryN {
							del2 = true // L2
						}
						if add3 >= clearEveryN {
							del3 = true // L3
						}
					} // end select
					if del1 { // L1
						//log.Printf("L13 flush tmpHash=%d", len(tmpHash))
						l1ext.ch <- tmpHash
						tmpHash = nil
						del1, add1 = false, 0
					}
					if del2 { // L2
						//log.Printf("L2 flush tmpOffset=%d", len(tmpOffset))
						l2ext.ch <- tmpOffset
						tmpOffset = nil
						del2, add2 = false, 0
					}
					if del3 { // L3
						//log.Printf("L3 flush tmpKey=%d", len(tmpKey))
						l3ext.ch <- tmpKey
						tmpKey = nil
						del3, add3 = false, 0
					}
					timer.Reset(time.Duration(basetimer) * time.Second)
				} // end for fetchdel

			}(char, i, his.cacheEvicts[char])
		} // end for num
	} // end for HEXCHARS
} // end func CacheEvictThread
