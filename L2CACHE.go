package history

import (
	//"fmt"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DefaultL2CacheExpires int64 = DefaultCacheExpires
	L2Purge               int64 = DefaultCachePurge
)

type L2CACHE struct {
	cache   map[int64]*L2ITEM
	mapsize int
	mux     sync.Mutex
}

type L2ITEM struct {
	hash    string
	expires int64
}

// The L2CACHE_Boot method initializes the L2 cache. It creates a cache map with an initial size and starts a goroutine to periodically clean up expired entries.
func (l2 *L2CACHE) L2CACHE_Boot() {
	initsize := 128
	l2.mux.Lock()
	defer l2.mux.Unlock()
	if l2.cache != nil {
		log.Printf("ERROR L2CACHESetup already loaded!")
		return
	}
	l2.cache = make(map[int64]*L2ITEM, initsize)
	l2.mapsize = initsize
	go l2.L2Cache_Thread()
} // end func L2CACHE_Boot

// The L2Cache_Thread function runs as a goroutine to periodically clean up expired cache items, similar to the L1 cache's thread.
func (l2 *L2CACHE) L2Cache_Thread() {
	logf(DEBUG2, "Boot L2Cache_Thread")
	//forever
	for {
		time.Sleep(time.Duration(L2Purge) * time.Second)
		now := utils.UnixTimeSec()
		start := utils.UnixTimeMilliSec()
		cleanup := []int64{}

		l2.mux.Lock()
		for offset, item := range l2.cache {
			if item.expires < now {
				cleanup = append(cleanup, offset)
			}
		}
		l2.mux.Unlock()

		if len(cleanup) > 0 {
			l2.mux.Lock()
			for _, offset := range cleanup {
				delete(l2.cache, offset)
			}
			maplen := len(l2.cache)
			max := l2.mapsize
			case1 := maplen < 1024 && max > 4096
			case2 := maplen == 0 && max > 1024
			if case1 || case2 {
				newmax := 1024
				if case1 {
					newmax = 4096
				}
				newmap := make(map[int64]*L2ITEM, newmax)
				if maplen == 0 {
					l2.cache = newmap
				} else {
					for k, v := range l2.cache {
						newmap[k] = v
					}
					l2.cache = newmap
				}
				l2.mapsize = newmax
				logf(DEBUG2, "L2Cache_Thread shrink size to 1024")
			}
			newmax := l2.mapsize
			l2.mux.Unlock()
			logf(DEBUG2, "L2Cache_Thread deleted=%d maplen=%d/%d oldmax=%d", len(cleanup), maplen, newmax, max)
			cleanup = nil
		}

		logf(DEBUG2, "L2Cache_Thread (took %d ms)", utils.UnixTimeMilliSec()-start)
	} // end for
} //end func L2Cache_Thread

// The SetOffsetHash method sets a cache item in the L2 cache using an offset as the key and a hash as the value.
// It also dynamically grows the cache when necessary.
func (l2 *L2CACHE) SetOffsetHash(offset *int64, hash *string) {
	if hash == nil || len(*hash) < 32 || offset == nil { // at least md5
		log.Printf("ERROR L2CACHESet nil pointer")
		return
	}
	start := utils.UnixTimeMilliSec()
	l2.mux.Lock()

	if len(l2.cache) >= int(l2.mapsize/100*98) { // grow map
		newmax := l2.mapsize * 2
		newmap := make(map[int64]*L2ITEM, newmax)
		for k, v := range l2.cache {
			newmap[k] = v
		}
		l2.cache = newmap
		l2.mapsize = newmax
		logf(DEBUG1, "L2CACHE grow newmap=%d/%d (took %d ms)", len(newmap), newmax, utils.UnixTimeMilliSec()-start)
	}

	l2.cache[*offset] = &L2ITEM{hash: *hash, expires: utils.UnixTimeSec() + DefaultL2CacheExpires}
	l2.mux.Unlock()
} // end func SetOffsetHash

// The GetHashFromOffset method retrieves a hash from the L2 cache using an offset as the key.
func (l2 *L2CACHE) GetHashFromOffset(offset *int64) (hash *string) {
	if offset == nil || *offset <= 0 {
		log.Printf("ERROR L2CACHEGetHashToOffset offset=nil")
		return
	}
	l2.mux.Lock()
	if l2.cache[*offset] != nil {
		item := l2.cache[*offset]
		hash = &item.hash
	}
	l2.mux.Unlock()
	return
} // end func GetHashFromOffset

// The Delete method deletes a cache item from the L2 cache using an offset as the key.
func (l2 *L2CACHE) Delete(offset *int64) {
	if offset == nil || *offset <= 0 {
		log.Printf("ERROR L2CACHEDel offset=nil")
		return
	}
	l2.mux.Lock()
	delete(l2.cache, *offset)
	l2.mux.Unlock()
} // end func Delete
