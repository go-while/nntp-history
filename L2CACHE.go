package history

/*
 * WARNING: the `char` used by L2Cache is NOT boltDB char!
 *          it is derived from offset via OffsetToChar !
 */
import (
	"fmt"
	"github.com/go-while/go-utils"
	"log"
	//"os"
	"sync"
	"time"
)

var (
	DEBUGL2         bool
	L2CacheExpires  int64 = DefaultCacheExpires
	L2ExtendExpires int64 = DefaultCacheExtend
	L2Purge         int64 = DefaultCachePurge
	L2InitSize      int   = 256 * 1024
)

type L2CACHE struct {
	Caches   map[string]*L2CACHEMAP
	Extend   map[string]chan *ClearCache
	muxers   map[string]*L2MUXER
	mapsizes map[string]*MAPSIZES
	mux      sync.Mutex
	Counter  map[string]map[string]uint64
}

type L2CACHEMAP struct {
	cache map[int64]*L2ITEM
}

type L2ITEM struct {
	hash    string
	expires int64
}

type L2MUXER struct {
	mux sync.Mutex
}

// The L2CACHE_Boot method initializes the L2 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l2 *L2CACHE) L2CACHE_Boot(his *HISTORY) {
	l2.mux.Lock()
	defer l2.mux.Unlock()
	if l2.Caches != nil {
		log.Printf("ERROR L2CACHESetup already loaded!")
		return
	}
	l2.Caches = make(map[string]*L2CACHEMAP, 16)
	l2.Extend = make(map[string]chan *ClearCache, 16)
	l2.muxers = make(map[string]*L2MUXER, 16)
	l2.mapsizes = make(map[string]*MAPSIZES, 16)
	l2.Counter = make(map[string]map[string]uint64)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = make(chan *ClearCache, his.cEvCap)
		l2.muxers[char] = &L2MUXER{}
		l2.mapsizes[char] = &MAPSIZES{maxmapsize: L2InitSize}
		l2.Counter[char] = make(map[string]uint64)
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l2.L2Cache_Thread(char)
	}

} // end func L2CACHE_Boot

// The L2Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l2 *L2CACHE) L2Cache_Thread(char string) {
	l2.mux.Lock() // waits for L2CACHE_Boot to unlock
	l2.mux.Unlock()
	logf(DEBUGL2, "Boot L2Cache_Thread [%s]", char)
	//lastshrink := utils.UnixTimeSec()
	cleanup := []int64{}
	l2purge := L2Purge
	if l2purge < 1 {
		l2purge = 1
	}
	extends := make(map[int64]bool, 1024)
	timer := time.NewTimer(time.Duration(l2purge) * time.Second)
	timeout := false
forever:
	for {
		if timeout {
			timeout = false
			timer.Reset(time.Duration(l2purge) * time.Second)
		}
		select {
		case item := <-l2.Extend[char]: // receives stuff from DelExtL2batch()
			// got offset we will extend in next timer.C run
			if item.offset > 0 {
				extends[item.offset] = true
				if char != item.char {
					log.Printf("ERROR L2 [%s] char != extends[offset=%d].char='%s'", char, item.offset, item.char)
					continue forever
				}
			}
		case <-timer.C:
			timeout = true
			start := utils.UnixTimeMilliSec()
			now := int64(start / 1000)

			l2.muxers[char].mux.Lock()
		getexpired:
			for offset, item := range l2.Caches[char].cache {
				if extends[offset] {
					l2.Caches[char].cache[offset].expires = now + L2ExtendExpires
					l2.Counter[char]["Count_BatchD"] += 1
					//delete(extends, offset)
					continue getexpired
				} else if item.expires > 0 && item.expires < now {
					//logf(DEBUG, "L2 expire [%s] offset='%#v' item='%#v' age=%d l2exp=%d", char, offset, item, now-item.addtime, L2CacheExpires)
					cleanup = append(cleanup, offset)
				}
			} // end for getexpired
			maplen := len(l2.Caches[char].cache)
			l2.muxers[char].mux.Unlock()
			clear(extends)
			if len(cleanup) > 0 {
				maplen -= len(cleanup)
				l2.muxers[char].mux.Lock()
				for _, offset := range cleanup {
					delete(l2.Caches[char].cache, offset)
					l2.Counter[char]["Count_Delete"] += 1
				}
				max := l2.mapsizes[char].maxmapsize
				l2.muxers[char].mux.Unlock()
				logf(DEBUGL2, "L2Cache_Thread [%s] deleted=%d/%d max=%d", char, len(cleanup), maplen, max)
				cleanup = nil
			}
			logf(DEBUGL2, "L2Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
			continue forever
		} // end select
	} // end for

} //end func L2Cache_Thread

// The SetOffsetHash method sets a cache item in the L2 cache using an offset as the key and a hash as the value.
// It also dynamically grows the cache when necessary.
func (l2 *L2CACHE) SetOffsetHash(offset int64, hash string, flagexpires bool) {
	if offset <= 0 || len(hash) < 32 { // at least md5
		log.Printf("ERROR L2CACHESet nil pointer")
		return
	}
	char := l2.OffsetToChar(offset)
	//start := utils.UnixTimeMilliSec()
	l2.muxers[char].mux.Lock()
	defer l2.muxers[char].mux.Unlock()

	expires := NoExpiresVal
	if flagexpires {
		expires = utils.UnixTimeSec() + L2CacheExpires
		l2.Counter[char]["Count_FlagEx"] += 1
	} else {
		l2.Counter[char]["Count_Set"] += 1
	}
	if l2.Caches[char].cache[offset] != nil {
		if l2.Caches[char].cache[offset].hash != hash {
			log.Printf("ERROR L2Cache [%s] SetOffsetHash extend cached hash=%s@offset=%d != hash=%s", char, l2.Caches[char].cache[offset].hash, offset, hash)
			return
		}
		l2.Caches[char].cache[offset].expires = expires
		return
	}
	l2.Caches[char].cache[offset] = &L2ITEM{hash: hash, expires: expires}
	l2.mapsizes[char].maxmapsize++
} // end func SetOffsetHash

// The GetHashFromOffset method retrieves a hash from the L2 cache using an offset as the key.
func (l2 *L2CACHE) GetHashFromOffset(offset int64) (hash string) {
	if offset <= 0 {
		log.Printf("ERROR L2CACHEGetHashToOffset offset=nil")
		return
	}
	char := l2.OffsetToChar(offset)
	l2.muxers[char].mux.Lock()
	if l2.Caches[char].cache[offset] != nil {
		l2.Counter[char]["Count_Get"] += 1
		item := l2.Caches[char].cache[offset]
		hash = item.hash
		l2.muxers[char].mux.Unlock()
		return
	}
	l2.Counter[char]["Count_Mis"] += 1
	l2.muxers[char].mux.Unlock()
	return
} // end func GetHashFromOffset

// The DelExtL2batch method deletes multiple cache items from the L2 cache.
func (l2 *L2CACHE) DelExtL2batch(his *HISTORY, tmpOffset []*ClearCache, flagCacheDelExt int) {
	if len(tmpOffset) == 0 {
		log.Printf("DelExtL2batch tmpOffset empty")
		return
	}
	if flagCacheDelExt == FlagCacheChanExtend {
		for _, item := range tmpOffset {
			//if item.offset > 0 && item.hash != "" && item.char != "" {
			if item.offset > 0 && item.char != "" {
				testchar := l2.OffsetToChar(item.offset)
				if testchar != item.char {
					log.Printf("ERROR DelExtL2batch1 testchar=%s != item.char=%s", testchar, item.char)
					continue
				}
				if DEBUG {
					lench := len(l2.Extend[item.char])
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L2 Extend[%s]chan=%d/his.cEvCap=%d half-full", item.char, lench, his.cEvCap)
					}
				}
				//char := l2.OffsetToChar(item.offset)
				l2.Extend[item.char] <- item
			} else {
				log.Printf("ERROR DelExtL2batch item='%#v'", item)
			}
		}
		return
	}
	now := utils.UnixTimeSec()
	for _, item := range tmpOffset {
		if item.offset > 0 && item.char != "" {
			testchar := l2.OffsetToChar(item.offset)
			if testchar != item.char {
				log.Printf("ERROR DelExtL2batch2 testchar=%s != item.char=%s", testchar, item.char)
				continue
			}
			//log.Printf("DelExtL2batch char=%s offset=%d offsets=%d", char, item.offset, len(tmpOffset))
			l2.muxers[item.char].mux.Lock()
			if _, exists := l2.Caches[item.char].cache[item.offset]; exists {
				switch flagCacheDelExt {
				case FlagCacheSyncDelete:
					delete(l2.Caches[item.char].cache, item.offset)
				case FlagCacheSyncExtend:
					// dont delete from cache but extend expiry time
					l2.Caches[item.char].cache[item.offset].expires = now + L2ExtendExpires
				}
				l2.Counter[item.char]["Count_BatchD"] += 1
			}
			l2.muxers[item.char].mux.Unlock()
		}
	}
} // end func DelExtL2batch

func (l2 *L2CACHE) OffsetToChar(offset int64) string {
	if offset <= 0 {
		log.Printf("ERROR L2CACHE.OffsetToChar offset=%d <=0", offset)
		return ""
	}
	// get first char from hex: less random. repeats itself for longer period
	//char := string(fmt.Sprintf("%x", *offset)[0])

	// get last char from hex: the last char is more random
	hex := fmt.Sprintf("%x", offset)
	char := string(hex[len(hex)-1])
	if char == "" {
		log.Printf("ERROR L2CACHE.OffsetToChar char empty fallback 0")
		char = "0"
	}
	//log.Printf("OffsetToChar offset=%d => hex=%s => char=%s", offset, hex, char)
	return char
} // end func OffsetToChar

func (l2 *L2CACHE) L2Stats(key string) (retval uint64, retmap map[string]uint64) {
	if key == "" {
		retmap = make(map[string]uint64)
	}
	if l2 == nil || l2.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		l2.muxers[char].mux.Lock()
		switch key {
		case "":
			// key is empty, get all key=>stats to retmap
			for k, v := range l2.Counter[char] {
				retmap[k] += v
			}
		default:
			// key is set, returns retval
			if _, exists := l2.Counter[char][key]; exists {
				retval += l2.Counter[char][key]
			}
		}
		l2.muxers[char].mux.Unlock()
	}
	return
} // end func L2Stats
