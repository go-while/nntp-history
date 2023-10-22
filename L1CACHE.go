package history

import (
	"container/heap"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL1         bool  = false
	L1              bool  = true // better not disable L1 cache...
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
	prioQue map[string]*L1PrioQue // Priority queue for item expiration
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

type L1PrioQue struct {
	que *L1PQ
	mux sync.Mutex
	pqC chan struct{}
}

type L1PQ []*L1PQItem

type L1PQItem struct {
	Key     string
	Expires int64
}

// The L1CACHE_Boot method initializes the cache system.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically purge expired entries.
func (l1 *L1CACHE) L1CACHE_Boot(his *HISTORY) {
	if !L1 {
		return
	}
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
	//l1.prioQue = make(map[string]*L1PQ, intBoltDBs)
	l1.prioQue = make(map[string]*L1PrioQue, intBoltDBs)
	//l1.pqChans = make(map[string]chan struct{}, intBoltDBs)
	//l1.pqMuxer = make(map[string]*L1PQMUX, intBoltDBs)
	for _, char := range HEXCHARS {
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &StrECH{ch: make(chan *StrItems, his.cEvCap)}
		l1.Muxers[char] = &L1MUXER{}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		//l1.prioQue[char] = &L1PQ{}
		l1.prioQue[char] = &L1PrioQue{que: &L1PQ{}, pqC: make(chan struct{}, 1)}
		//l1.pqChans[char] = make(chan struct{}, 1)
		//l1.pqMuxer[char] = &L1PQMUX{}
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
	if !L1 {
		return CasePass
	}
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
		if L1LockDelay > 0 {
			time.Sleep(time.Duration(L1LockDelay) * time.Millisecond)
		}
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
	if !L1 {
		return
	}
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
		pushq := []string{}
		//forever:
		for {
			select {
			case dat := <-extC.ch: // receives stuff from CacheEvictThread()
				// got hashes we will extend
				if len(dat.extends) > 0 {
					//logf(DEBUGL1, "L1 [%s] extends=%d", char, len(extends))
					//pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)
					mux.mux.Lock()
					for _, hash := range dat.extends {
						if _, exists := ptr.cache[hash]; exists {
							ptr.cache[hash].value = CaseDupes
							cnt.Counter["Count_BatchD"]++
							pushq = append(pushq, hash)
						}
					}
					mux.mux.Unlock()

					for _, hash := range pushq {
						l1.prioPush(char, pq, &L1PQItem{Key: hash, Expires: L1ExtendExpires})
					}
					dat.extends, pushq = nil, nil
					dat = nil
				}
			} // end select
		} // end forever
	}() // end gofunc1
} //end func L1Cache_Thread

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash string, char string, value int, flagexpires bool) {
	if !L1 {
		return
	}
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

	if flagexpires {
		//pqEX := time.Now().UnixNano() + (L1CacheExpires * int64(time.Second))
		l1.prioPush(char, pq, &L1PQItem{Key: hash, Expires: L1CacheExpires})
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
	if !L1 {
		return
	}
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

func (pq *L1PQ) Len() int {
	//log.Printf("Len L1PQ=%d", len(*pq))
	return len(*pq)
}

func (pq *L1PQ) Less(i, j int) bool {
	return false
}

func (pq *L1PQ) Swap(i, j int) {
	return
}

func (pq *L1PQ) Pop() interface{} {
	old := *pq
	*pq = old[1:]
	old = nil
	return nil
}

func (pq *L1PQ) Push(x interface{}) {
	item := x.(*L1PQItem)
	item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
	*pq = append(*pq, item)
}

func (l1 *L1CACHE) prioPush(char string, pq *L1PrioQue, item *L1PQItem) {
	if !L1 {
		return
	}
	//log.Printf("L1 prioPush [%s] heap.push item='%#v' expireS=%d", char, item, (pqEX-time.Now().UnixNano())/int64(time.Second))
	pq.mux.Lock()
	heap.Push(pq.que, item)
	pq.mux.Unlock()

	//log.Printf("L1 prioPush [%s] heap.push unlocked", char)
	select {
	case pq.pqC <- struct{}{}:
		// pass notify to pqExpire()
	default:
		// pass too: notify chan is full
	}
	//log.Printf("L1 prioPush [%s] heap.push passed pqC", char)
} // end func prioPush

// Remove expired items from the cache
func (l1 *L1CACHE) pqExpire(char string) {
	if !L1 {
		return
	}
	log.Printf("L1 pqExpire [%s] wait l1 lock", char)
	l1.mux.Lock() // waits for boot to finish
	l1.mux.Unlock()
	log.Printf("L1 pqExpire [%s] booted", char)

	//start := utils.UnixTimeMilliSec()
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	//extC := l1.Extend[char]
	mux := l1.Muxers[char]
	pq := l1.prioQue[char]
	lpq, dqq, dqmax := 0, uint64(0), uint64(64)
	var item *L1PQItem
	var dq []string
forever:
	for {

		pq.mux.Lock()
		lpq = len(*pq.que)
		if lpq == 0 {
			pq.mux.Unlock()
			//logf(DEBUGL1 || ALWAYS, "L1 pqExpire [%s] wait on <-pqC", char)
			select {
			case <-pq.pqC: // blocking wait for l1.prioPush
				//logf(DEBUGL1 || ALWAYS, "L1 pqExpire [%s] recv on <-pqC", char)
				continue forever
			}
		} else {
			// Get the item with the nearest expiration time
			item = (*pq.que)[0]
		}

		currentTime := time.Now().UnixNano()

		if item.Expires <= currentTime {
			// This item has expired, remove it from the cache and priority queue
			//logf(DEBUGL1 || ALWAYS, "L1 pqExpire [%s] DELETE hash='%s' over=%d", char, item.Key, currentTime-item.Expires)
			heap.Pop(pq.que)
			pq.mux.Unlock()

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
			pq.mux.Unlock()
			// The nearest item hasn't expired yet, sleep until it does
			sleepTime := time.Duration(item.Expires - currentTime)
			//logf(DEBUGL1 || ALWAYS, "L1 pqExpire [%s] SLEEP hash='%s sleep=%d lpq=%d", char, item.Key, sleepTime, lpq)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire
