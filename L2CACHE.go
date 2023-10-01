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

func (l2 *L2CACHE) L2Cache_Thread() {
	logf(DEBUG1, "Boot L2Cache_Thread")
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
			if maplen < 1024 && max > 4096 || maplen == 0 && max == 1024 {
				newmap := make(map[int64]*L2ITEM, 1024)
				if maplen == 0 {
					l2.cache = newmap
				} else {
					for k, v := range l2.cache {
						newmap[k] = v
					}
					l2.cache = newmap
				}
				l2.mapsize = 1024
				logf(DEBUG1, "L2Cache_Thread shrink size to 1024")
			}
			newmax := l2.mapsize
			l2.mux.Unlock()
			logf(DEBUG1, "L2Cache_Thread deleted=%d maplen=%d/%d oldmax=%d", len(cleanup), maplen, newmax, max)
			cleanup = nil
		}

		logf(DEBUG1, "L2Cache_Thread (took %d ms)", utils.UnixTimeMilliSec()-start)
	} // end for
} //end func L2Cache_Thread

func (l2 *L2CACHE) L2CACHE_SetOffsetHash(offset *int64, hash *string) {
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
} // end func L2CACHE_SetOffsetHash

func (l2 *L2CACHE) L2CACHE_GetHashFromOffset(offset *int64) (hash *string) {
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
} // end func L2CACHE_GetHashFromOffset

func (l2 *L2CACHE) L2CACHE_Del(offset *int64) {
	if offset == nil || *offset <= 0 {
		log.Printf("ERROR L2CACHEDel offset=nil")
		return
	}
	l2.mux.Lock()
	delete(l2.cache, *offset)
	l2.mux.Unlock()
} // end func L2CACHE_Del
