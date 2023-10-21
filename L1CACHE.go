package history

import (
	//"arena"
	"container/heap"
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
	//  250ms is a max of    4 messages/sec per conn
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
	pqMuxer map[string]*L1PQMUX      // Priority queue Muxers
	//arenas  map[string]*L1Arena
}

type L1CACHEMAP struct {
	cache map[string]*L1ITEM
}

type L1ITEM struct {
	value int
}

type L1MUXER struct {
	mux sync.Mutex
}

type L1PQMUX struct {
	mux sync.RWMutex
}

/*
mem := arena.NewArena()
//defer mem.Free()
memhash := arena.New[AHASH](mem)
memhash.buffer = make([]byte, bufferSize)

type L1Arena struct {
	mux sync.RWMutex

	pqmem   *arena.Arena
	prioQue *L1PQ

	dqmem   *arena.Arena
	dqslice *DQSlice
}
*/

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
	l1.pqMuxer = make(map[string]*L1PQMUX, intBoltDBs)
	//l1.arenas = make(map[string]*L1Arena, intBoltDBs)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &StrECH{ch: make(chan []string, his.cEvCap)}
		l1.Muxers[char] = &L1MUXER{}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l1.prioQue[char] = &L1PQ{}
		l1.pqChans[char] = make(chan struct{}, 1)
		l1.pqMuxer[char] = &L1PQMUX{}
		//l1.arenas[char] = &L1Arena{pqmem: arena.NewArena(), dqmem: arena.NewArena()}
		//l1.arenas[char].prioQue = arena.New[L1PQ](l1.arenas[char].pqmem)
		//l1.arenas[char].dqslice = arena.New[DQSlice](l1.arenas[char].dqmem)
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

	if L1LockDelay > 0 {
		time.Sleep(time.Duration(L1LockDelay) * time.Millisecond)
	}

	mux.mux.Lock()
	if _, exists := ptr.cache[hash]; !exists {
		//if hash == TESTHASH {
		//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d isLocked", char, hash, value)
		//}
		cnt.Counter["Count_Locked"]++
		if !useHashDB {
			value = CaseDupes
		}
		ptr.cache[hash] = &L1ITEM{value: value}
		mux.mux.Unlock()
		return CasePass
	} else {
		retval := ptr.cache[hash].value
		mux.mux.Unlock()
		return retval
	}
	return CaseError
} // end func LockL1Cache

// The L1Cache_Thread function runs as a goroutine for each character.
func (l1 *L1CACHE) L1Cache_Thread(char string) {
	l1.mux.Lock() // waits for L1CACHE_Boot to unlock
	l1.mux.Unlock()
	//logf(DEBUGL1, "Boot L1Cache_Thread [%s]", char)
	l1purge := L1Purge
	if l1purge < 1 {
		l1purge = 1
	}

	go func() {
		defer log.Printf("LEFT L1T gofunc1 extend [%s]", char)
		ptr := l1.Caches[char]
		cnt := l1.Counter[char]
		extC := l1.Extend[char]
		mux := l1.Muxers[char]
		pq := l1.prioQue[char]
		pqC := l1.pqChans[char]
		pqM := l1.pqMuxer[char]
		var extends []string
		//if UseArenas {
		//	a := l1.arenas[char]
		//	pq = a.prioQue
		//	pqM = a
		//}

		//forever:
		for {
			select {
			case extends = <-extC.ch: // receives stuff from CacheEvictThread()
				// got hashes we will extend
				if len(extends) > 0 {
					//logf(DEBUGL1, "L1 [%s] extends=%d", char, len(extends))
					pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)
					mux.mux.Lock()
					for _, hash := range extends {
						if _, exists := ptr.cache[hash]; exists {
							ptr.cache[hash].value = CaseDupes
							cnt.Counter["Count_BatchD"]++
							l1.prioPush(char, pq, pqC, pqM, &L1PQItem{Key: hash, Expires: pqEX})
						}
					}
					mux.mux.Unlock()
					extends = nil
				}
			} // end select
		} // end forever
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
	//extC := l1.Extend[char]
	mux := l1.Muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]
	pqM := l1.pqMuxer[char]
	/*if UseArenas {
		a := l1.arenas[char]
		pq := a.prioQue
		pqC := l1.pqChans[char]
		pqM := a
	}*/

	if flagexpires {
		pqEX := time.Now().UnixNano() + (L1CacheExpires * int64(time.Second))
		l1.prioPush(char, pq, pqC, pqM, &L1PQItem{Key: hash, Expires: pqEX})
	}
	mux.mux.Lock()
	if _, exists := ptr.cache[hash]; !exists {
		ptr.cache[hash] = &L1ITEM{value: value}
	} else {
		ptr.cache[hash].value = value
	}
	switch flagexpires {
	case true:
		cnt.Counter["Count_FlagEx"]++
	case false:
		if value == CaseWrite {
			cnt.Counter["Count_Set"]++
		}
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
	//if len(pq) == 0 {
	//	return false
	//}
	return pq[i].Expires < pq[j].Expires
}

func (pq L1PQ) Swap(i, j int) {
	//if len(pq) == 0 {
	//	return
	//}
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

	//start := utils.UnixTimeMilliSec()
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	//extC := l1.Extend[char]
	mux := l1.Muxers[char]
	pq := l1.prioQue[char]
	pqC := l1.pqChans[char]
	pqM := l1.pqMuxer[char]
	dq := []string{} // with arenas needs to change dq to *dq below
	/*if UseArenas {
		a := l1.arenas[char]
		pq := a.prioQue
		dq := a.dqslice // delete queue slice in memory arena
		pqM := a
	}*/
	//var item *L1PQItem
	var empty bool
	lpq, dqcnt, dqmax := 0, 0, 512

	lastdel := time.Now().Unix()
forever:
	for {
		if dqcnt >= dqmax || (lastdel < time.Now().Unix()-L1Purge && dqcnt > 0) {
			//log.Printf("L1 pqExpire [%s] cleanup dqcnt=%d lpq=%d", char, dqcnt, lpq)
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
				//log.Printf("L1 pqExpire [%s] blocking wait dqcnt=%d lpq=%d", char, dqcnt, lpq)
				select {
				case <-pqC: // waits for notify to run
					//log.Printf("L1 pqExpire [%s] got notify <-pqC dqcnt=%d lpq=%d", char, dqcnt, lpq)
				default:
					time.Sleep(time.Duration(L1Purge) * time.Second)
					pqM.mux.RLock() // watch this! RLock gets opened here
					lpq = len(*pq)
					if lpq > 0 {
						empty = false
					}
					if !empty {
						//log.Printf("L1 pqExpire [%s] released wait: !empty dqcnt=%d lpq=%d", char, dqcnt, lpq)
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
			//log.Printf("L1 pqExpire [%s] key='%s' diff=%d", char, item.Key, item.Expires-currentTime)
			pqM.mux.Lock()
			heap.Pop(pq)
			pqM.mux.Unlock()
			dq = append(dq, item.Key)
			dqcnt++
		} else {
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//log.Printf("L1 pqExpire [%s] key='%s' diff=%d sleepTime=%d", char, item.Key, currentTime-item.Expires, sleepTime)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire

func (l1 *L1CACHE) prioPush(char string, pq *L1PQ, pqC chan struct{}, pqM *L1PQMUX, item *L1PQItem) {
	//log.Printf("l1.prioPush [%s] heap.push key='%s' expireS=%d", char, hash, (pqEX-time.Now().UnixNano())/int64(time.Second))
	pqM.mux.Lock()
	heap.Push(pq, item)
	pqM.mux.Unlock()

	//log.Printf("l1.prioPush [%s] heap.push unlocked", char)
	select {
	case pqC <- struct{}{}:
		// pass notify to pqExpire()
	default:
		// pass too: notify chan is full
	}
	//log.Printf("l1.prioPush [%s] heap.push passed pqC", char)
} // end func prioPush
