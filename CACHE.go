package history

import (
	//"fmt"
	"log"
	//"time"
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
	DBG_CGS               bool       // DEBUG_CACHE_GROW_SHRINK
	DefaultCacheExpires   int64 = 5  // gets x2 BatchFlushEvery x2
	DefaultCacheExtend    int64 = 5  // extends cached items after writes
	DefaultCachePurge     int64 = 1  // checks ttl every N seconds. affects CacheExpires/Extend max to + Purge
	DefaultEvictsCapacity       = 32 // his.cEvCap is normally fine as is but higher values can, not will, give better performance by eating memory
	NumCacheEvictThreads        = 1  // 1 is enough
	// cache cleansup every N items or when DefaultCachePurge triggers
	// beware of the appetite!
	// 16 dbs * RootBuckets * ClearEveryN * 2 Caches * 2 PrioQue * 64 bytes (sha256) + maps overhead = lots of bytes
	// 16     *      16     *    32       * 2        * 2         * 64 =   2.097.152 bytes
	// 16     *     256     *    32       * 2        * 2         * 64 =  33.554.432 bytes
	// 16     *    4096     *    32       * 2        * 2         * 64 = 536.870.912 bytes
	ClearEveryN = DefaultEvictsCapacity
)

// CharCacheCounter
type CCC struct {
	Counter map[string]uint64 // counter key: value
}

type ClearCacheChan struct {
	ch chan []*ClearCache
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
		//log.Printf("ERROR CacheEvict char empty.")
		return
	}
	if char != "" {
		if hash != "" {
			l1ext := his.L1Cache.Extend[char]
			//log.Printf("L1Cache.Extend[%s].ch %d/%d his.cEvCap=%d ch='%#v'", char, len(l1ext.ch), cap(l1ext.ch), his.cEvCap, l1ext.ch)
			l1ext.ch <- &L1PQItem{Key: hash, Expires: L1ExtendExpires}
		}
		if offset > 0 {
			l2ext := his.L2Cache.Extend[char]
			//log.Printf("L2Cache.Extend[%s].ch %d/%d his.cEvCap=%d ch='%#v'", char, len(l2ext.ch), cap(l2ext.ch), his.cEvCap, l2ext.ch)
			l2ext.ch <- &L2PQItem{Key: offset, Expires: L2ExtendExpires}
		}
		if key != "" {
			l3ext := his.L3Cache.Extend[char]
			//log.Printf("L3Cache.Extend[%s].ch %d/%d his.cEvCap=%d ch='%#v'", char, len(l3ext.ch), cap(l3ext.ch), his.cEvCap, l3ext.ch)
			l3ext.ch <- &L3PQItem{Key: key, Expires: L3ExtendExpires}
		}
		return
	}
	log.Printf("ERROR CacheEvict char=nil")
} // end func DoCacheEvict
