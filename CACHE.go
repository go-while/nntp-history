package history

import (
	"log"
)

const (
	// never change this
	FlagExpires      bool = true
	FlagNeverExpires bool = false
)

var (
	DBG_CGS               bool       // DEBUG_CACHE_GROW_SHRINK
	DefaultCacheExpires   int64 = 5  // gets x2 BatchFlushEvery x2
	DefaultCacheExtend    int64 = 5  // extends cached items after writes
	DefaultCachePurge     int64 = 1  // checks ttl every N seconds. affects CacheExpires/Extend max to + Purge
	DefaultEvictsCapacity       = 16 // his.cEvCap (size of Extend chan) is normally fine as is.
	ClearEveryN                 = 16
)

// CharCacheCounter
type CCC struct {
	Counter map[string]uint64 // counter key: value
}

type ClearCacheChan struct {
	ch chan []*ClearCache
}

// DoCacheEvict - stub function (cache layers removed)
func (his *HISTORY) DoCacheEvict(char string, hash string, offset int64, key string) {
	// Cache functionality removed - this is now a no-op stub
} // end func DoCacheEvict

func (his *HISTORY) PrintCacheStats() {
	// Cache functionality removed - this is now a no-op stub
	log.Printf("Cache stats: L1/L2/L3 cache layers removed, using MySQL RocksDB directly")
} // end func PrintCacheStats
