package history

/*
 * WARNING: the `char` used by L2Cache is NOT boltDB char!
 *          it is derived from offset via OffsetToChar !
 */
import (
	//"fmt"
	"github.com/go-while/go-utils"
	"log"
	"strconv"
	"sync"
	"time"
)

var (
	DEBUGL2         bool  = false
	L2CacheExpires  int64 = DefaultCacheExpires
	L2ExtendExpires int64 = DefaultCacheExtend
	L2Purge         int64 = DefaultCachePurge
	L2InitSize      int   = 256 * 1024
)

type L2CACHE struct {
	Caches  map[string]*L2CACHEMAP
	Extend  map[string]chan *ClearCache
	muxers  map[string]*L2MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
}

type L2CACHEMAP struct {
	cache map[int64]*L2ITEM
}

type L2ITEM struct {
	hash    string
	expires int64
}

type L2MUXER struct {
	mux sync.RWMutex
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
	l2.Counter = make(map[string]*CCC)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = make(chan *ClearCache, his.cEvCap)
		l2.muxers[char] = &L2MUXER{}
		l2.Counter[char] = &CCC{Counter: make(map[string]uint64)}

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
	//logf(DEBUGL2, "Boot L2Cache_Thread [%s]", char)
	cleanup := []int64{}
	l2purge := L2Purge
	if l2purge < 1 {
		l2purge = 1
	}

	go func(ptr *L2CACHEMAP, mux *sync.RWMutex, cnt *CCC, extendChan chan *ClearCache) {
		defer log.Printf("LEFT L2T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l2purge) * time.Second)
		timeout := false
		ext, emax := 0, 16384
		extends := make([]int64, emax)
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					timeout = true
					break forextends
				case item := <-extendChan: // receives stuff from DelExtL2batch()
					// got offset we will extend in next timer.C run
					if char != item.char {
						log.Printf("ERROR L2 [%s] char != extends[offset=%d].char='%s'", char, item.offset, item.char)
						continue forextends
					}
					if item.offset > 0 {
						extends = append(extends, item.offset)
						ext++
						if ext >= emax {
							timeout = true
							break forextends
						}
					}
				} // end select
			} // end forextends
			if (timeout && ext > 0) || ext >= emax {
				now := utils.UnixTimeSec()
				//logf(DEBUG, "L2 [%s] extends=%d", char, len(extends))
				mux.Lock()
				for _, offset := range extends {
					if _, exists := ptr.cache[offset]; exists {
						ptr.cache[offset].expires = now + L2ExtendExpires
						cnt.Counter["Count_BatchD"]++
					}
				}
				mux.Unlock()
				extends = nil
				timeout = false
				ext = 0
				timer.Reset(time.Duration(l2purge) * time.Second)
			}
		} // end forever
	}(l2.Caches[char], &l2.muxers[char].mux, l2.Counter[char], l2.Extend[char]) // end gofunc1

	go func(ptr *L2CACHEMAP, mux *sync.RWMutex, cnt *CCC) {
		defer log.Printf("LEFT L2T gofunc2 delete [%s]", char)
		timer := time.NewTimer(time.Duration(l2purge) * time.Second)
		timeout := false
		start := utils.UnixTimeMilliSec()
		now := int64(start / 1000)
	forever:
		for {
			if timeout {
				timeout = false
				timer.Reset(time.Duration(l2purge) * time.Second)
			}
			select {
			case <-timer.C:
				timeout = true
				start = utils.UnixTimeMilliSec()
				now = int64(start / 1000)

				mux.Lock()
				//getexpired:
				for offset, item := range ptr.cache {
					if item.expires > 0 && item.expires < now {
						//logf(DEBUG, "L2 expire [%s] offset='%#v' item='%#v' age=%d l2exp=%d", char, offset, item, now-item.addtime, L2CacheExpires)
						cleanup = append(cleanup, offset)
					}
				} // end for getexpired

				//maplen := len(ptr.cache)
				if len(cleanup) > 0 {
					//maplen -= len(cleanup)
					for _, offset := range cleanup {
						delete(ptr.cache, offset)
						cnt.Counter["Count_Delete"]++
					}
					//logf(DEBUG, "L2Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
				}
				mux.Unlock()
				cleanup = nil
				//logf(DEBUG, "L2Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
				continue forever
			} // end select
		} // end for
	}(l2.Caches[char], &l2.muxers[char].mux, l2.Counter[char]) // end gofunc2
} //end func L2Cache_Thread

// The SetOffsetHash method sets a cache item in the L2 cache using an offset as the key and a hash as the value.
// It also dynamically grows the cache when necessary.
func (l2 *L2CACHE) SetOffsetHash(offset int64, hash string, flagexpires bool) {
	//if hash == TESTHASH {
	//	log.Printf("L2CAC Set hash='%s' @offset=%d expires=%t", hash, offset, flagexpires)
	//}
	if offset <= 0 || len(hash) < 32 { // at least md5
		log.Printf("ERROR L2CACHESet nil pointer")
		return
	}
	char := l2.OffsetToChar(offset)

	//start := utils.UnixTimeMilliSec()

	ptr := l2.Caches[char]
	cnt := l2.Counter[char]
	mux := l2.muxers[char]

	mux.mux.Lock()

	expires := NoExpiresVal
	if flagexpires {
		expires = utils.UnixTimeSec() + L2CacheExpires
		cnt.Counter["Count_FlagEx"]++
	} else {
		cnt.Counter["Count_Set"]++
	}
	if _, exists := ptr.cache[offset]; exists {
		//if l2.Caches[char].cache[offset].hash != hash {
		//	log.Printf("ERROR L2Cache [%s] SetOffsetHash extend cached hash=%s@offset=%d != hash=%s", char, l2.Caches[char].cache[offset].hash, offset, hash)
		//	return
		//}
		ptr.cache[offset].expires = expires
		mux.mux.Unlock()
		return
	}
	ptr.cache[offset] = &L2ITEM{hash: hash, expires: expires}
	mux.mux.Unlock()
} // end func SetOffsetHash

// The GetHashFromOffset method retrieves a hash from the L2 cache using an offset as the key.
func (l2 *L2CACHE) GetHashFromOffset(offset int64, rethash *string) {
	if offset <= 0 || rethash == nil {
		log.Printf("ERROR L2CACHEGetHashToOffset io nil")
		return
	}
	char := l2.OffsetToChar(offset)

	ptr := l2.Caches[char]
	//cnt := l3.Counter[char]
	mux := l2.muxers[char]

	mux.mux.RLock()
	if _, exists := ptr.cache[offset]; exists {
		//cnt.Counter["Count_Get"]++ // cant count this here! we only have RLOCK!
		hash := ptr.cache[offset].hash
		*rethash = hash
		mux.mux.RUnlock()
		return
	}
	mux.mux.RUnlock()
	//cnt.Counter["Count_Mis"]++ // cant count this here! we only have RLOCK!
	return
} // end func GetHashFromOffset

// The DelExtL2batch method deletes multiple cache items from the L2 cache.
func (l2 *L2CACHE) DelExtL2batch(his *HISTORY, tmpOffset []*ClearCache) {
	if len(tmpOffset) == 0 {
		log.Printf("DelExtL2batch tmpOffset empty")
		return
	}
	for _, item := range tmpOffset {
		//if item.offset > 0 && item.hash != "" && item.char != "" {
		if item.offset > 0 && item.char != "" {
			/*
				testchar := l2.OffsetToChar(item.offset)
				if testchar != item.char {
					log.Printf("ERROR DelExtL2batch1 testchar=%s != item.char=%s", testchar, item.char)
					continue
				}
			*/
			/*
				if DEBUG {
					lench := len(l2.Extend[item.char])
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L2 Extend[%s]chan=%d/his.cEvCap=%d half-full", item.char, lench, his.cEvCap)
					}
				}
			*/
			//char := l2.OffsetToChar(item.offset)
			l2.Extend[item.char] <- item
		} else {
			log.Printf("ERROR DelExtL2batch item='%#v'", item)
		}
	}
} // end func DelExtL2batch

func (l2 *L2CACHE) OffsetToChar(offset int64) (retval string) {
	if offset <= 0 {
		log.Printf("ERROR L2CACHE.OffsetToChar offset=%d <=0", offset)
		return ""
	}
	// Compute the last hex character directly
	retval = strconv.FormatInt(offset%16, 16)
	//log.Printf("offset=%d lastHexDigitStr=%#v", offset, retval)
	return
} // end func OffsetToChar

func (l2 *L2CACHE) L2Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l2 == nil || l2.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l2.Counter[char]
		mux := l2.muxers[char]
		mux.mux.RLock()
		switch statskey {
		case "":
			// key is empty, get all key=>stats to retmap
			for k, v := range cnt.Counter {
				retmap[k] += v
			}
		default:
			// key is set, returns retval
			if _, exists := cnt.Counter[statskey]; exists {
				retval += cnt.Counter[statskey]
			}
		}
		mux.mux.RUnlock()
	}
	return
} // end func L2Stats
