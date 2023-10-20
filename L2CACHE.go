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
	pqMuxer map[string]*L2MUXER      // Priority queue Muxers
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
	l2.Extend = make(map[string]*IntECH, 16)
	l2.Muxers = make(map[string]*L2MUXER, 16)
	l2.Counter = make(map[string]*CCC)
	l2.prioQue = make(map[string]*L2PQ, intBoltDBs)
	l2.pqChans = make(map[string]chan struct{}, intBoltDBs)
	l2.pqMuxer = make(map[string]*L2MUXER, intBoltDBs)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = &IntECH{ch: make(chan []int64, his.cEvCap)}
		l2.Muxers[char] = &L2MUXER{}
		l2.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l2.prioQue[char] = &L2PQ{}
		l2.pqChans[char] = make(chan struct{}, 1)
		l2.pqMuxer[char] = &L2MUXER{}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l2.pqExpire(char)
		go l2.L2Cache_Thread(char)
	}

} // end func L2CACHE_Boot

// The L2Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l2 *L2CACHE) L2Cache_Thread(char string) {
	l2.mux.Lock() // waits for L2CACHE_Boot to unlock
	l2.mux.Unlock()
	//logf(DEBUGL2, "Boot L2Cache_Thread [%s]", char)
	//cleanup := []int64{}
	l2purge := L2Purge
	if l2purge < 1 {
		l2purge = 1
	}

	go func() {
		defer log.Printf("LEFT L2T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l2purge) * time.Second)
		var extends []int64
		ptr := l2.Caches[char]
		cnt := l2.Counter[char]
		extC := l2.Extend[char]
		mux := l2.Muxers[char]
		pq := l2.prioQue[char]
		pqC := l2.pqChans[char]
		pqM := l2.pqMuxer[char]
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					break forextends
				case slice := <-extC.ch: // receives stuff from CacheEvictThread()
					// got offset we will extend in next timer.C run
					extends = slice
					break forextends
				} // end select
			} // end forextends
			if len(extends) > 0 {
				now := time.Now().Unix()
				//logf(DEBUG, "L2 [%s] extends=%d", char, len(extends))
				mux.mux.Lock()
				for _, offset := range extends {
					if _, exists := ptr.cache[offset]; exists {
						pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)
						ptr.cache[offset].expires = now + L2ExtendExpires
						cnt.Counter["Count_BatchD"]++

						pqM.mux.Lock()
						heap.Push(pq, &L2PQItem{
							Key:     offset,
							Expires: pqEX,
						})
						pqM.mux.Unlock()
						select {
						case pqC <- struct{}{}:
							// pass
						default:
							// pass too: notify chan is full
						}
					}
				}
				mux.mux.Unlock()
				extends = nil
				timer.Reset(time.Duration(l2purge) * time.Second)
			}
		} // end forever
	}() // end gofunc1
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
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	pqC := l2.pqChans[char]
	pqM := l2.pqMuxer[char]

	expires := NoExpiresVal
	var pqEX int64

	mux.mux.Lock()
	if flagexpires {
		cnt.Counter["Count_FlagEx"]++
		expires = time.Now().Unix() + L2CacheExpires
		pqEX = time.Now().UnixNano() + (L1CacheExpires * int64(time.Second))
	} else {
		cnt.Counter["Count_Set"]++
	}
	if _, exists := ptr.cache[offset]; exists {
		//if ptr.cache[offset].hash != hash {
		//	log.Printf("ERROR L2Cache [%s] SetOffsetHash extend cached hash=%s@offset=%d != hash=%s", char, l2.Caches[char].cache[offset].hash, offset, hash)
		//	return
		//}
		ptr.cache[offset].expires = expires
		mux.mux.Unlock()
		return
	}
	ptr.cache[offset] = &L2ITEM{hash: hash, expires: expires}

	// Update the priority queue
	if flagexpires {
		//log.Printf("l1.set [%s] heap.push key='%s' expireS=%d", char, hash, (pqEX-time.Now().UnixNano())/int64(time.Second))
		pqM.mux.Lock()
		heap.Push(pq, &L2PQItem{
			Key:     offset,
			Expires: pqEX,
		})
		pqM.mux.Unlock()
		mux.mux.Unlock()
		//log.Printf("l1.set [%s] heap.push unlocked", char)
		select {
		case pqC <- struct{}{}:
			// pass
		default:
			// pass too: notify chan is full
		}
		//log.Printf("l1.set [%s] heap.push passed pqC", char)
		return
	}
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
	l2.mux.Lock() // waits for boot to finish
	l2.mux.Unlock()
	cnt := l2.Counter[char]
	ptr := l2.Caches[char]
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	pqC := l2.pqChans[char]
	pqM := l2.pqMuxer[char]
	//var item *L2PQItem
	var empty bool
	lpq, dqcnt, dqmax := 0, 0, 512
	dq := []int64{}
	lastdel := time.Now().Unix()
forever:
	for {
		if dqcnt >= dqmax || (lastdel < time.Now().Unix()-L2Purge && dqcnt > 0) {
			//log.Printf("L2 pqExpire [%s] cleanup dqcnt=%d lpq=%d", char, dqcnt, lpq)
			mux.mux.Lock()
			for _, delkey := range dq {
				delete(ptr.cache, delkey)
				cnt.Counter["Count_Delete"]++
			}
			mux.mux.Unlock()
			dqcnt, dq, lastdel = 0, nil, time.Now().Unix()
		}

		pqM.mux.RLock()
		lpq = len(*pq)
		if lpq == 0 {
			empty = true
			pqM.mux.RUnlock()
		waiter:
			for {
				//log.Printf("L2 pqExpire [%s] blocking wait dqcnt=%d lpq=%d", char, dqcnt, lpq)
				select {
				case <-pqC: // waits for notify to run
					//log.Printf("L2 pqExpire [%s] got notify <-pqC dqcnt=%d lpq=%d", char, dqcnt, lpq)
				default:
					time.Sleep(time.Duration(L2Purge) * time.Second)
					pqM.mux.RLock() // watch this! RLock gets opened here
					lpq = len(*pq)
					if lpq > 0 {
						empty = false
					}
					if !empty {
						//log.Printf("L2 pqExpire [%s] released wait: !empty dqcnt=%d lpq=%d", char, dqcnt, lpq)
						break waiter
					}
					// RUnlock here but the break !empty before keeps it open to get the item!
					pqM.mux.RUnlock()
				} // end select
				if empty {
					if dqcnt > 0 {
						continue forever
					}
				}
			} // end for waiter
		} // end if len(*pq) == 0 {

		// Get the item with the nearest expiration time
		item := (*pq)[0]
		pqM.mux.RUnlock()

		currentTime := time.Now().UnixNano()

		if item.Expires <= currentTime {
			// This item has expired, remove it from the cache and priority queue
			//log.Printf("L2 pqExpire [%s] key='%d' diff=%d", char, item.Key, item.Expires-currentTime)
			pqM.mux.Lock()
			heap.Pop(pq)
			pqM.mux.Unlock()
			dq = append(dq, item.Key)
			dqcnt++
		} else {
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//log.Printf("L2 pqExpire [%s] key='%s' diff=%d sleepTime=%d", char, item.Key, currentTime-item.Expires, sleepTime)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire
