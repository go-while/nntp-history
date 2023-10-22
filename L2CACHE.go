package history

/*
 * WARNING: the `char` used by L2Cache is NOT boltDB char!
 *          it is derived from offset via OffsetToChar !
 */
import (
	//"fmt"
	"container/heap"
	"log"
	"strconv"
	"sync"
	"time"
)

/*
L2Cache: offset => hash
less requests to hisDat
*/
var (
	DEBUGL2         bool  = false
	L2              bool  = false
	L2CacheExpires  int64 = DefaultCacheExpires
	L2ExtendExpires int64 = DefaultCacheExtend
	L2Purge         int64 = DefaultCachePurge
	L2InitSize      int   = 64 * 1024
)

type L2CACHE struct {
	Caches  map[string]*L2CACHEMAP
	Extend  map[string]*IntECH
	Muxers  map[string]*L2MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
	prioQue map[string]*L2PQ         // Priority queue for item expiration
	pqChans map[string]chan struct{} // Priority queue notify channels
	pqMuxer map[string]*L2PQMUX      // Priority queue Muxers
}

type L2CACHEMAP struct {
	cache map[int64]*L2ITEM
}

type L2ITEM struct {
	hash string
}

type L2MUXER struct {
	mux sync.RWMutex
}

type L2PQMUX struct {
	mux sync.Mutex
}

// The L2CACHE_Boot method initializes the L2 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l2 *L2CACHE) L2CACHE_Boot(his *HISTORY) {
	if !L2 {
		return
	}
	l2.mux.Lock()
	defer l2.mux.Unlock()
	if l2.Caches != nil {
		log.Printf("ERROR L2CACHESetup already loaded!")
		return
	}
	l2.Caches = make(map[string]*L2CACHEMAP, 16)
	l2.Extend = make(map[string]*IntECH, 16)
	l2.Muxers = make(map[string]*L2MUXER, 16)
	l2.Counter = make(map[string]*CCC)
	l2.prioQue = make(map[string]*L2PQ, intBoltDBs)
	l2.pqChans = make(map[string]chan struct{}, intBoltDBs)
	l2.pqMuxer = make(map[string]*L2PQMUX, intBoltDBs)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = &IntECH{ch: make(chan *IntItems, his.cEvCap)}
		l2.Muxers[char] = &L2MUXER{}
		l2.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l2.prioQue[char] = &L2PQ{}
		l2.pqChans[char] = make(chan struct{}, 1)
		l2.pqMuxer[char] = &L2PQMUX{}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l2.pqExpire(char)
		go l2.L2Cache_Thread(char)
	}

} // end func L2CACHE_Boot

// The L2Cache_Thread function runs as a goroutine for each character.
func (l2 *L2CACHE) L2Cache_Thread(char string) {
	if !L2 {
		return
	}
	l2.mux.Lock() // waits for L2CACHE_Boot to unlock
	l2.mux.Unlock()
	//logf(DEBUGL2, "Boot L2Cache_Thread [%s]", char)
	l2purge := L2Purge
	if l2purge < 1 {
		l2purge = 1
	}

	go func() {
		defer log.Printf("LEFT L2T gofunc1 extend [%s]", char)
		ptr := l2.Caches[char]
		cnt := l2.Counter[char]
		extC := l2.Extend[char]
		mux := l2.Muxers[char]
		pq := l2.prioQue[char]
		pqC := l2.pqChans[char]
		pqM := l2.pqMuxer[char]
		//forever:
		for {
			select {
			case dat := <-extC.ch: // receives stuff from CacheEvictThread()
				// got offset we will extend
				if len(*dat.extends) > 0 {
					//logf(DEBUG, "L2 [%s] extends=%d", char, len(extends))
					pqEX := time.Now().UnixNano() + L2ExtendExpires*int64(time.Second)
					mux.mux.Lock()
					for _, offset := range *dat.extends {
						if _, exists := ptr.cache[offset]; exists {
							cnt.Counter["Count_BatchD"]++
							l2.prioPush(char, pq, pqC, pqM, &L2PQItem{Key: offset, Expires: pqEX})
						}
					}
					mux.mux.Unlock()
				}
			} // end select

		} // end forever
	}() // end gofunc1
} //end func L2Cache_Thread

// The SetOffsetHash method sets a cache item in the L2 cache using an offset as the key and a hash as the value.
// It also dynamically grows the cache when necessary.
func (l2 *L2CACHE) SetOffsetHash(offset int64, hash string, flagexpires bool) {
	if !L2 {
		return
	}
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
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	pqC := l2.pqChans[char]
	pqM := l2.pqMuxer[char]

	if flagexpires {
		pqEX := time.Now().UnixNano() + (L2CacheExpires * int64(time.Second))
		l2.prioPush(char, pq, pqC, pqM, &L2PQItem{Key: offset, Expires: pqEX})
	}

	mux.mux.Lock()
	if _, exists := ptr.cache[offset]; !exists {
		ptr.cache[offset] = &L2ITEM{hash: hash}
	}
	switch flagexpires {
	case true:
		cnt.Counter["Count_FlagEx"]++
	case false:
		cnt.Counter["Count_Set"]++
	}
	mux.mux.Unlock()

} // end func SetOffsetHash

// The GetHashFromOffset method retrieves a hash from the L2 cache using an offset as the key.
func (l2 *L2CACHE) GetHashFromOffset(offset int64, rethash *string) {
	if !L2 {
		return
	}
	if offset <= 0 || rethash == nil {
		log.Printf("ERROR L2CACHEGetHashToOffset io nil")
		return
	}
	char := l2.OffsetToChar(offset)

	ptr := l2.Caches[char]
	//cnt := l2.Counter[char]
	mux := l2.Muxers[char]

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

func (l2 *L2CACHE) OffsetToChar(offset int64) (retval string) {
	if !L2 {
		return
	}
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
	if !L2 {
		return
	}
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l2 == nil || l2.Muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l2.Counter[char]
		mux := l2.Muxers[char]
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

type L2PQ []*L2PQItem

type L2PQItem struct {
	Key     int64
	Expires int64
}

func (pq L2PQ) Len() int { return len(pq) }

func (pq L2PQ) Less(i, j int) bool {
	//log.Printf("L2PQ Less()")
	return pq[i].Expires < pq[j].Expires
}

func (pq L2PQ) Swap(i, j int) {
	//log.Printf("L2PQ Swap()")
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *L2PQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	old = nil
	//log.Printf("L2PQ POP() item='%#v", item)
	return item
}

func (pq *L2PQ) Push(x interface{}) {
	item := x.(*L2PQItem)
	*pq = append(*pq, item)
}

// Remove expired items from the cache
func (l2 *L2CACHE) pqExpire(char string) {
	if !L2 {
		return
	}
	l2.mux.Lock() // waits for boot to finish
	l2.mux.Unlock()
	cnt := l2.Counter[char]
	ptr := l2.Caches[char]
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	pqC := l2.pqChans[char]
	pqM := l2.pqMuxer[char]
	lpq, dqq, dqmax := 0, uint64(0), uint64(1024)
	var item *L2PQItem
	var dq []int64
forever:
	for {
		pqM.mux.Lock()
		lpq = len(*pq)
		if lpq == 0 {
			pqM.mux.Unlock()
			//logf(DEBUGL2, "L2 pqExpire [%s] wait on <-pqC", char)
			select {
			case <-pqC: // blocking wait for his.prioPush()
				//logf(DEBUGL2, "L2 pqExpire [%s] recv on <-pqC", char)
				continue forever
			}
		} else {
			// Get the item with the nearest expiration time
			item = (*pq)[0]
		}

		currentTime := time.Now().UnixNano()

		if item.Expires <= currentTime {
			// This item has expired, remove it from the cache and priority queue
			//logf(DEBUGL2, "L2 pqExpire [%s] DELETE offset='%d' over=%d", char, item.Key, currentTime-item.Expires)
			heap.Pop(pq)
			pqM.mux.Unlock()

			dq = append(dq, item.Key)
			dqq++
			if dqq >= dqmax {
				mux.mux.Lock()
				for _, key := range dq {
					delete(ptr.cache, key)
				}
				cnt.Counter["Count_Delete"] += dqq
				mux.mux.Unlock()
				dq, dqq = nil, 0
			}
		} else {
			pqM.mux.Unlock()
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//logf(DEBUGL2, "L2 pqExpire [%s] SLEEP offset='%d' sleep=%d lpq=%d", char, item.Key, sleepTime, lpq)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire

func (l2 *L2CACHE) prioPush(char string, pq *L2PQ, pqC chan struct{}, pqM *L2PQMUX, item *L2PQItem) {
	if !L2 {
		return
	}
	//log.Printf("L2 prioPush [%s] heap.push item='%#v' expireS=%d", char, item, (pqEX-time.Now().UnixNano())/int64(time.Second))
	pqM.mux.Lock()
	heap.Push(pq, item)
	pqM.mux.Unlock()

	//log.Printf("L2 prioPush [%s] heap.push unlocked", char)
	select {
	case pqC <- struct{}{}:
		// pass notify to pqExpire()
	default:
		// pass too: notify chan is full
	}
	//log.Printf("L2 prioPush [%s] heap.push passed pqC", char)
} // end func prioPush
