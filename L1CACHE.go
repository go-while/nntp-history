package history

import (
	"container/heap"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL1         bool  = false
	L1CacheExpires  int64 = DefaultCacheExpires
	L1ExtendExpires int64 = DefaultCacheExtend
	L1Purge         int64 = DefaultCachePurge
	L1InitSize      int   = 256 * 1024

	// L1LockDelay: delays L1 locking by N milliseconds
	// L1 locking is most likely done per client-connection
	// settings this greater 0 limits the amount of articles a client can lock&send
	//    1ms is a max of 1000 messages/sec per conn
	//  100ms is a max of   10 messages/sec per conn
	// 1000ms is a max of    1 message /sec per conn
	// text peers mostly dont need more than 4 msg per sec
	L1LockDelay int = 0
)

type L1CACHE struct {
	mux     sync.Mutex // global L1 mutex
	Caches  map[string]*L1CACHEMAP
	Extend  map[string]*StrECH
	muxers  map[string]*L1MUXER
	Counter map[string]*CCC
	prioQue map[string]*L1PQ         // Priority queue for item expiration
	pqChans map[string]chan struct{} // Priority queue notify channels
}

type L1CACHEMAP struct {
	cache map[string]*L1ITEM
}

type L1ITEM struct {
	value   int
	expires int64
}

type L1MUXER struct {
	mux sync.RWMutex
}

// The L1CACHE_Boot method initializes the cache system.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically purge expired entries.
func (l1 *L1CACHE) L1CACHE_Boot(his *HISTORY) {
	l1.mux.Lock()
	defer l1.mux.Unlock()
	if l1.Caches != nil {
		log.Printf("ERROR L1CACHESetup already loaded!")
		return
	}
	l1.Caches = make(map[string]*L1CACHEMAP, intBoltDBs)
	l1.Extend = make(map[string]*StrECH, intBoltDBs)
	l1.muxers = make(map[string]*L1MUXER, intBoltDBs)
	l1.Counter = make(map[string]*CCC)
	l1.prioQue = make(map[string]*L1PQ)
	l1.pqChans = make(map[string]chan struct{})
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &StrECH{ch: make(chan string, his.cEvCap)}
		l1.muxers[char] = &L1MUXER{}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l1.prioQue[char] = &L1PQ{}
		l1.pqChans[char] = make(chan struct{}, 1)
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l1.pqExpire(char)
		go l1.L1Cache_Thread(char)
	}
} // end func L1CACHE_Boot

// The LockL1Cache method is used to LOCK a `MessageIDHash` for processing.
// If the value is not in the cache or has expired, it locks the cache, updates the cache with a new value, and returns the value.
// Possible return values:
//
//	CaseLock == already in processing
//	CaseWrite == already in processing
//	CaseDupes == is a duplicate
//	CasePass == not a duplicate == locked article for processing
func (l1 *L1CACHE) LockL1Cache(hash string, char string, value int, useHashDB bool) int {
	//if hash == TESTHASH {
	//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d tryLock", char, hash, value)
	//}
	if hash == "" {
		log.Printf("ERROR LockL1Cache hash=nil")
		return -999
	}
	if char == "" {
		char = string(hash[0])
	}
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.muxers[char]

	mux.mux.RLock()
	if _, exists := ptr.cache[hash]; exists {
		retval := ptr.cache[hash].value
		//if hash == TESTHASH {
		//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d isLocked", char, hash, value)
		//}
		mux.mux.RUnlock()
		return retval
	}
	mux.mux.RUnlock()

	if L1LockDelay > 0 {
		time.Sleep(time.Duration(L1LockDelay) * time.Millisecond)
	}

	//if hash == TESTHASH {
	//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d weLocked", char, hash, value)
	//}

	mux.mux.Lock()
	if _, exists := ptr.cache[hash]; !exists {
		cnt.Counter["Count_Locked"]++
		if !useHashDB {
			value = CaseDupes
		}
		ptr.cache[hash] = &L1ITEM{value: value, expires: utils.UnixTimeSec() + L1CacheExpires}
		mux.mux.Unlock()
		return CasePass
	} else {
		retval := ptr.cache[hash].value
		mux.mux.Unlock()
		return retval
	}
	mux.mux.Unlock()

	return CaseError
} // end func LockL1Cache

// The L1Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries, and if the cache size is too large, it shrinks the cache.
func (l1 *L1CACHE) L1Cache_Thread(char string) {
	l1.mux.Lock() // waits for L1CACHE_Boot to unlock
	l1.mux.Unlock()
	//logf(DEBUGL1, "Boot L1Cache_Thread [%s]", char)
	//cleanup := []string{}
	l1purge := L1Purge
	if l1purge < 1 {
		l1purge = 1
	}

	go func(ptr *L1CACHEMAP, mux *sync.RWMutex, cnt *CCC, extendChan *StrECH) {
		defer log.Printf("LEFT L1T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l1purge) * time.Second)
		timeout := false
		ext, emax := 0, 16384
		extends := make([]string, emax)
		pq := l1.prioQue[char]
		pqC := l1.pqChans[char]
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					timeout = true
					break forextends
				case hash := <-extendChan.ch: // receives stuff from DelExtL1batch()
					if hash == "" {
						log.Printf("ERROR L1 extend ch received nil hash")
						continue forextends
					}
					// got hash we will extend in next timer.C run
					extends = append(extends, hash)
					//log.Printf("L1 extend hash='%s'", *hash)
					ext++
					if ext >= emax {
						timeout = true
						break forextends
					}
				} // end select
			} // end forextends
			if (timeout && ext > 0) || ext >= emax {
				now := utils.UnixTimeSec()
				mux.Lock()
				//logf(DEBUG, "L1 [%s] extends=%d", char, len(extends))
				for _, hash := range extends {
					if _, exists := ptr.cache[hash]; exists {
						pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)
						ptr.cache[hash].expires = now + L1ExtendExpires
						ptr.cache[hash].value = CaseDupes
						cnt.Counter["Count_BatchD"]++
						heap.Push(pq, &L1PQItem{
							Key:     hash,
							Expires: pqEX,
						})
						select {
						case pqC <- struct{}{}:
							// pass
						default:
							// pass too: notify chan is full
						}
					}
				}
				mux.Unlock()
				extends = nil
				timeout = false
				ext = 0
				timer.Reset(time.Duration(l1purge) * time.Second)
			}
		} //end forever
	}(l1.Caches[char], &l1.muxers[char].mux, l1.Counter[char], l1.Extend[char]) // end gofunc1

	/*
		go func(ptr *L1CACHEMAP, mux *sync.RWMutex, cnt *CCC) {
			defer log.Printf("LEFT L1T gofunc2 delete [%s]", char)
			timer := time.NewTimer(time.Duration(l1purge) * time.Second)
			start := utils.UnixTimeMilliSec()
			now := int64(start / 1000)
			return // disables old expiry
			//forever:
			for {
				select {
				case <-timer.C:
					start = utils.UnixTimeMilliSec()
					now = int64(start / 1000)

					//mux.RLock()
					mux.Lock()
					//getexpired:
					for hash, item := range ptr.cache {
						if (item.value == CaseDupes || item.value == CaseRetry) && item.expires > 0 && item.expires < now {
							//if hash == TESTHASH {
							//	log.Printf("L1CAC [%s|  ] ADD2CLEANUP TESTHASH='%s'", char, hash)
							//}
							//cleanup = append(cleanup, hash)
							cnt.Counter["Count_Delete"]++
							delete(ptr.cache, hash)
						}
					} // end for getexpired
					mux.Unlock()
					//mux.RUnlock()

					//maplen := len(ptr.cache)
					/,*
						if len(cleanup) > 0 {
							mux.Lock()
							//maplen -= len(cleanup)
							for _, hash := range cleanup {
								delete(ptr.cache, hash)
								cnt.Counter["Count_Delete"]++
							}
							mux.Unlock()
							cleanup = nil
							//logf(DEBUG, "L1Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
						}
					*,/
					//logf(DEBUG, "L1Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
					timer.Reset(time.Duration(l1purge) * time.Second)
				} // end select
			} // end for
		}(l1.Caches[char], &l1.muxers[char].mux, l1.Counter[char]) // end gofunc2
	*/

} //end func L1Cache_Thread

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash string, char string, value int, flagexpires bool) {
	if len(hash) < 32 { // at least md5
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(hash[0])
	}
	//start := utils.UnixTimeMilliSec()
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]

	mux.mux.Lock()

	expires := NoExpiresVal
	var pqEX int64
	if flagexpires {
		cnt.Counter["Count_FlagEx"]++
		expires = utils.UnixTimeSec() + L1CacheExpires
		pqEX = time.Now().UnixNano() + (L1CacheExpires * int64(time.Second))
	} else if !flagexpires && value == CaseWrite {
		cnt.Counter["Count_Set"]++
	}

	if _, exists := ptr.cache[hash]; exists {
		ptr.cache[hash].expires = expires
		ptr.cache[hash].value = value
		mux.mux.Unlock()
		return
	}
	ptr.cache[hash] = &L1ITEM{value: value, expires: expires}

	// Update the priority queue
	if flagexpires {
		log.Printf("l1.set [%s] heap.push key='%s' expireS=%d", char, hash, (pqEX-time.Now().UnixNano())/int64(time.Second))
		heap.Push(pq, &L1PQItem{
			Key:     hash,
			Expires: pqEX,
		})
		mux.mux.Unlock()
		log.Printf("l1.set [%s] heap.push unlocked", char)
		select {
		case pqC <- struct{}{}:
			// pass
		default:
			// pass too: notify chan is full
		}
		log.Printf("l1.set [%s] heap.push passed pqC", char)
		return
	}
	mux.mux.Unlock()
} // end func Set

// The DelExtL1batch method deletes multiple cache items from the L1 cache.
func (l1 *L1CACHE) DelExtL1batch(his *HISTORY, char string, tmpHash []*ClearCache) {
	if char == "" {
		log.Printf("ERROR DelExtL1batch char=nil")
		return
	}
	if len(tmpHash) == 0 {
		log.Printf("DelExtL1batch [%s] tmpHash empty", char)
		return
	}
	ext := l1.Extend[char]
	for _, item := range tmpHash {
		if item.hash != "" {
			/*
				if DEBUG {
					lench := len(l1.Extend[char].ch)
					if lench >= his.cEvCap/2 {
						log.Printf("WARN L1 Extend[%s]chan=%d/his.cEvCap=%d half-full", char, lench, his.cEvCap)
					}
				}
			*/
			ext.ch <- item.hash
		}
	}
	return
} // end func DelExtL1batch

func (l1 *L1CACHE) L1Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l1 == nil || l1.muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l1.Counter[char]
		mux := l1.muxers[char]
		mux.mux.Lock()
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
		mux.mux.Unlock()
	}
	return
} // end func L1Stats

type L1PQ []*L1PQItem

type L1PQItem struct {
	Key     string
	Expires int64
}

func (pq L1PQ) Len() int { return len(pq) }

func (pq L1PQ) Less(i, j int) bool {
	//log.Printf("L1PQ Less()")
	return pq[i].Expires < pq[j].Expires
}

func (pq L1PQ) Swap(i, j int) {
	//log.Printf("L1PQ Swap()")
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *L1PQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	//log.Printf("L1PQ POP() item='%#v", item)
	return item
}

func (pq *L1PQ) Push(x interface{}) {
	item := x.(*L1PQItem)
	*pq = append(*pq, item)
}

/*
type L1PQ []*L1PQItem

type L1PQItem struct {
	Key     string
	Expires int64
	index   int // New field to track index
}

func (pq L1PQ) Len() int { return len(pq) }

func (pq L1PQ) Less(i, j int) bool {
	return pq[i].Expires < pq[j].Expires
}

func (pq L1PQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i // Update index of swapped items
	pq[j].index = j
}

func (pq *L1PQ) Push(x interface{}) {
	item := x.(*L1PQItem)
	item.index = len(*pq) // Set the index for the new item
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1  // Reset index
	*pq = old[0 : n-1]
	//log.Printf("L1PQ POP() item='%#v", item)
	return item
}
*/

func NewL1CACHE() *L1CACHE {
	return &L1CACHE{
		Caches:  make(map[string]*L1CACHEMAP),
		Extend:  make(map[string]*StrECH),
		muxers:  make(map[string]*L1MUXER),
		Counter: make(map[string]*CCC),
		prioQue: make(map[string]*L1PQ),
	}
}

// Remove expired items from the cache
func (l1 *L1CACHE) pqExpire(char string) {
	l1.mux.Lock() // waits for boot to finish
	l1.mux.Unlock()
	cnt := l1.Counter[char]
	ptr := l1.Caches[char]
	mux := l1.muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]
	var item *L1PQItem
	var empty bool
	for {
		mux.mux.RLock()
		if len(*pq) == 0 {
			empty = true
			mux.mux.RUnlock()
		waiter:
			for {
				//log.Printf("pqExpire [%s] blocking wait", char)
				select {
				case <-pqC:
					// pass
				default:
					time.Sleep(time.Duration(L1Purge) * time.Second)
					mux.mux.RLock() // watch this! RLock gets opened here
					if len(*pq) > 0 {
						empty = false
					}
					if !empty {
						//log.Printf("pqExpire [%s] released wait: !empty", char)
						break waiter
					}
					// RUnlock here but the break !empty before keeps it open to get the item!
					mux.mux.RUnlock()
				} // blocking wait for a notify
				if empty {
					continue waiter
				}
			} // end for waiter
		} // end if len(*pq) == 0 {

		// Get the item with the nearest expiration time
		item = (*pq)[0]
		mux.mux.RUnlock()

		currentTime := time.Now().UnixNano()

		if item.Expires <= currentTime {
			// This item has expired, remove it from the cache and priority queue
			//log.Printf("pqExpire [%s] key='%s' diff=%d", char, *item.Key, item.Expires-currentTime)
			mux.mux.Lock()
			delete(ptr.cache, item.Key)
			cnt.Counter["Count_Delete"]++
			heap.Pop(pq)
			mux.mux.Unlock()
		} else {
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//log.Printf("pqExpire [%s] key='%s' diff=%d sleepTime=%d", char, item.Key, item.Expires-currentTime, sleepTime)
			time.Sleep(sleepTime)
		}
	}
}
