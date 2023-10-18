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
	L1InitSize      int   = 64 * 1024

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
	Muxers  map[string]*L1MUXER
	Counter map[string]*CCC
	prioQue map[string]*L1PQ         // Priority queue for item expiration
	pqChans map[string]chan struct{} // Priority queue notify channels
	pqMuxer map[string]*L1MUXER      // Priority queue Muxers
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
	l1.Muxers = make(map[string]*L1MUXER, intBoltDBs)
	l1.Counter = make(map[string]*CCC, intBoltDBs)
	l1.prioQue = make(map[string]*L1PQ, intBoltDBs)
	l1.pqChans = make(map[string]chan struct{}, intBoltDBs)
	l1.pqMuxer = make(map[string]*L1MUXER, intBoltDBs)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &StrECH{ch: make(chan []string, his.cEvCap)}
		l1.Muxers[char] = &L1MUXER{}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l1.prioQue[char] = &L1PQ{}
		l1.pqChans[char] = make(chan struct{}, 1)
		l1.pqMuxer[char] = &L1MUXER{}
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
	mux := l1.Muxers[char]

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

	go func() {
		defer log.Printf("LEFT L1T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l1purge) * time.Second)
		var extends []string
		ptr := l1.Caches[char]
		cnt := l1.Counter[char]
		extC := l1.Extend[char]
		mux := l1.Muxers[char]
		pq := l1.prioQue[char]
		pqC := l1.pqChans[char]
		pqM := l1.pqMuxer[char]
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					break forextends
				case slice := <-extC.ch: // receives stuff from DelExtL1batch()
					// got hashes we will extend
					extends = slice
					//log.Printf("L1 extend hash='%s'", *hash)
					break forextends
				} // end select
			} // end forextends
			if len(extends) > 0 {
				now := utils.UnixTimeSec()
				mux.mux.Lock()
				//logf(DEBUG, "L1 [%s] extends=%d", char, len(extends))
				for _, hash := range extends {
					if _, exists := ptr.cache[hash]; exists {
						pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)
						ptr.cache[hash].expires = now + L1ExtendExpires
						ptr.cache[hash].value = CaseDupes
						cnt.Counter["Count_BatchD"]++

						pqM.mux.Lock()
						heap.Push(pq, &L1PQItem{
							Key:     hash,
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
				timer.Reset(time.Duration(l1purge) * time.Second)
			}
		} //end forever
	}() // end gofunc1

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
	mux := l1.Muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]
	pqM := l1.pqMuxer[char]

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
		pqM.mux.Lock()
		heap.Push(pq, &L1PQItem{
			Key:     hash,
			Expires: pqEX,
		})
		pqM.mux.Unlock()
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

func (l1 *L1CACHE) L1Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l1 == nil || l1.Muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l1.Counter[char]
		mux := l1.Muxers[char]
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
	old = nil
	//log.Printf("L1PQ POP() item='%#v", item)
	return item
}

func (pq *L1PQ) Push(x interface{}) {
	item := x.(*L1PQItem)
	*pq = append(*pq, item)
}

// Remove expired items from the cache
func (l1 *L1CACHE) pqExpire(char string) {
	l1.mux.Lock() // waits for boot to finish
	l1.mux.Unlock()
	cnt := l1.Counter[char]
	ptr := l1.Caches[char]
	mux := l1.Muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]
	pqM := l1.pqMuxer[char]
	//var item *L1PQItem
	var empty bool
	dqcnt, dqmax := 0, 512
	dq := make([]string, dqmax)
	lastdel := time.Now().Unix()
forever:
	for {
		if dqcnt >= dqmax || (lastdel < time.Now().Unix()-L1Purge && dqcnt > 0) {
			//log.Printf("pqExpire [%s] cleanup dqcnt=%d", char, dqcnt)
			mux.mux.Lock()
			for _, delkey := range dq {
				delete(ptr.cache, delkey)
				cnt.Counter["Count_Delete"]++
			}
			mux.mux.Unlock()
			dqcnt, dq, lastdel = 0, nil, time.Now().Unix()
		}

		pqM.mux.RLock()
		if len(*pq) == 0 {
			empty = true
			pqM.mux.RUnlock()
		waiter:
			for {
				//log.Printf("pqExpire [%s] blocking wait dqcnt=%d", char, dqcnt)
				select {
				case <-pqC: // waits for notify to run
					// pass
				default:
					//time.Sleep(time.Duration(L1Purge/2*1000) * time.Millisecond)
					time.Sleep(time.Duration(L1Purge) * time.Second)
					pqM.mux.RLock() // watch this! RLock gets opened here
					if len(*pq) > 0 {
						empty = false
					}
					if !empty {
						//log.Printf("pqExpire [%s] released wait: !empty", char)
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
			//log.Printf("pqExpire [%s] key='%s' diff=%d", char, item.Key, item.Expires-currentTime)
			pqM.mux.Lock()
			//delete(ptr.cache, item.Key)
			//cnt.Counter["Count_Delete"]++
			heap.Pop(pq)
			pqM.mux.Unlock()
			dq = append(dq, item.Key)
			dqcnt++
		} else {
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//log.Printf("pqExpire [%s] key='%s' diff=%d sleepTime=%d", char, item.Key, currentTime-item.Expires, sleepTime)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire
