package history

import (
	//"container/heap"
	"log"
	"sync"
	"time"
)

var (
	DEBUGL1         bool  = true
	L1              bool  = true // better not disable L1 cache...
	L1CacheExpires  int64 = DefaultCacheExpires
	L1ExtendExpires int64 = DefaultCacheExtend
	L1Purge         int64 = DefaultCachePurge
	L1InitSize      int   = 128

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
	Extend  map[string]*L1ECH
	Muxers  map[string]*L1MUXER
	Counter map[string]*CCC
	pqQueue map[string]*L1pqQ // Priority queue for item expiration
}

type L1CACHEMAP struct {
	cache map[string]*L1ITEM
}

type L1ITEM struct {
	value int
}

// L1ExtendChan
type L1ECH struct {
	ch chan *L1PQItem
}

type L1MUXER struct {
	mux sync.Mutex
}

type L1pqQ struct {
	que *L1PQ
	mux sync.Mutex
	pqC chan struct{}
}

type L1PQ []*L1PQItem

type L1PQItem struct {
	Key     string
	Expires int64
}

// The BootL1Cache method initializes the cache system.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically purge expired entries.
func (l1 *L1CACHE) BootL1Cache(his *HISTORY) {
	if !L1 {
		return
	}
	l1.mux.Lock()
	defer l1.mux.Unlock()
	if l1.Caches != nil {
		log.Printf("ERROR L1CACHESetup already loaded!")
		return
	}
	l1.Caches = make(map[string]*L1CACHEMAP, 16)
	l1.Extend = make(map[string]*L1ECH, 16)
	l1.Muxers = make(map[string]*L1MUXER, 16)
	l1.Counter = make(map[string]*CCC, 16)
	l1.pqQueue = make(map[string]*L1pqQ, 16)
	for _, char := range HEXCHARS {
		//log.Printf("L1 Boot [%s]", char)
		l1.Caches[char] = &L1CACHEMAP{cache: make(map[string]*L1ITEM, L1InitSize)}
		l1.Extend[char] = &L1ECH{ch: make(chan *L1PQItem, his.cEvCap)}
		l1.Muxers[char] = &L1MUXER{mux: sync.Mutex{}}
		l1.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l1.pqQueue[char] = &L1pqQ{mux: sync.Mutex{}, que: &L1PQ{}, pqC: make(chan struct{}, 1)}
	}
	time.Sleep(time.Millisecond)
	for _, char := range ROOTDBS {
		// stupid race condition on boot when placed in loop before
		go l1.pqExpire(char)
		go l1.pqExtend(char)
	}
	log.Printf("L1Cache_Boot")
} // end func BootL1Cache

// The LockL1Cache method is used to LOCK a `MessageIDHash` for processing.
// If the value is not in the cache or has expired, it locks the cache, updates the cache with a new value, and returns the value.
// Possible return values:
//
//	CaseLock == already in processing
//	CaseWrite == already in processing
//	CaseDupes == is a duplicate
//	CasePass == not a duplicate == locked article for processing
func (l1 *L1CACHE) LockL1Cache(hash string, value int, his *HISTORY) int {
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

	char := string(hash[:his.cutChar])

	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.Muxers[char]

	mux.mux.Lock()
	if _, exists := ptr.cache[hash]; !exists {
		//if hash == TESTHASH {
		//	log.Printf("L1CAC [%s|  ] LockL1Cache TESTHASH='%s' v=%d isLocked", char, hash, value)
		//}
		cnt.Counter["Count_Locked"]++
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

// The L1pqExtend function runs as a goroutine for each character.
func (l1 *L1CACHE) pqExtend(char string) {
	if !L1 {
		return
	}
	logf(DEBUGL1, "L1 pqExtend [%s] wait l1 lock", char)
	l1.mux.Lock() // waits for BootL1Cache to unlock
	logf(DEBUGL1, "L1 pqExtend [%s] booted", char)
	l1.mux.Unlock()
	//logf(DEBUGL1, "Boot L1pqExtend [%s]", char)
	//defer log.Printf("LEFT L1 [%s] pqExtend", char)

	l1purge := L1Purge
	if l1purge <= 0 {
		l1purge = 1
	}
	clearEv := ClearEveryN
	if clearEv <= 0 {
		clearEv = 1
	}

	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	extC := l1.Extend[char]
	mux := l1.Muxers[char]
	pq := l1.pqQueue[char]
	pushq, pushmax, pushcnt := make([]*L1PQItem, clearEv), clearEv, 0
	timeout := false
	timer := time.NewTimer(time.Duration(l1purge) * time.Second)

	//forever:
	for {
		select {
		case <-timer.C:
			timeout = true
		case pqitem := <-extC.ch: // receives stuff from DoCacheEvict
			if pqitem != nil {
				//log.Printf("L1 pushq append pqitem=%#v", pqitem)
				pushq[pushcnt] = pqitem
				pushcnt++
			} else {
				log.Printf("ERROR L1 pqExtend extC.ch <- nil pointer")
				return
			}
		} // end select
		if pushcnt >= pushmax || (timeout && pushcnt > 0) {
			if pushcnt > 0 {

				mux.mux.Lock()
				for i := 0; i < pushcnt; i++ {
					if _, exists := ptr.cache[pushq[i].Key]; exists {
						cnt.Counter["Count_BatchD"]++
					}
				}
				mux.mux.Unlock()

				pq.mux.Lock()
				for i := 0; i < pushcnt; i++ {
					pq.Push(pushq[i])
				}
				pq.mux.Unlock()

				pushq, pushcnt = make([]*L1PQItem, clearEv), 0
			}
		}
		if timeout {
			timeout = false
		}
		timer.Reset(time.Duration(l1purge) * time.Second)
	} // end forever
} //end func pqExtend

// The Set method is used to set a value in the cache.
// If the cache size is close to its maximum, it grows the cache.
func (l1 *L1CACHE) Set(hash string, char string, value int, flagexpires bool, his *HISTORY) {
	if !L1 {
		return
	}
	if len(hash) < 64 { // sha256
		log.Printf("ERROR L1CACHESet hash=nil")
		return
	}
	if char == "" {
		char = string(hash[:his.cutChar])
	}
	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.Muxers[char]
	pq := l1.pqQueue[char]

	if flagexpires {
		pq.mux.Lock()
		pq.Push(&L1PQItem{Key: hash, Expires: L1CacheExpires})
		pq.mux.Unlock()
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
	for _, char := range ROOTDBS {
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

func (pq *L1pqQ) Push(item *L1PQItem) {
	item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
	*pq.que = append(*pq.que, item)
} // end func Push

func (pq *L1pqQ) Pop() (*L1PQItem, int) {
	pq.mux.Lock()
	lenpq := len(*pq.que)
	if lenpq == 0 {
		pq.mux.Unlock()
		return nil, 0
	}
	old := *pq.que
	*pq.que = old[1:]
	pq.mux.Unlock()
	item := old[0]
	old = nil
	return item, lenpq
} // end func Pop

// Remove expired items from the cache
func (l1 *L1CACHE) pqExpire(char string) {
	if !L1 {
		return
	}
	logf(DEBUGL1, "L1 pqExpire [%s] wait l1 lock", char)
	l1.mux.Lock() // waits for BootL1Cache to unlock
	logf(DEBUGL1, "L1 pqExpire [%s] booted", char)
	l1.mux.Unlock()

	ptr := l1.Caches[char]
	cnt := l1.Counter[char]
	mux := l1.Muxers[char]
	pq := l1.pqQueue[char]
	lenpq := 0
	var item *L1PQItem
	var isleep int64
	l1purge := L1Purge
	//dq, dqmax, dqcnt := make([]string, ClearEveryN), ClearEveryN, 0
	//now := UnixTimeSec()
	//lf := now
cleanup:
	for {
		/*
			now = UnixTimeSec()
			if dqcnt >= dqmax || lf <= now-l1purge {
				if dqcnt > 0 {
					mux.mux.Lock()
					for i := 0; i < dqcnt; i++ {
						delete(ptr.cache, dq[i])
					}
					cnt.Counter["Count_Delete"] += uint64(dqcnt)
					mux.mux.Unlock()
				}
				dq, lf, dqcnt = make([]string, ClearEveryN), now, 0
			}
		*/
		logf(DEBUGL1, "L1 pqExpire [%s] pq.Pop item='%v'", char, item)
		item, lenpq = pq.Pop()
		if item == nil {
			time.Sleep(time.Duration(l1purge) * time.Second)
			continue cleanup
		}
		if item.Expires > time.Now().UnixNano() {
			isleep = item.Expires - time.Now().UnixNano()
			if isleep >= int64(1*time.Millisecond) {
				logf(DEBUGL1, "L1 pqExpire [%s] POS sleep=(%d ms) nanos=(%d) lenpq=%d", char, isleep/1e6, isleep, lenpq)
				time.Sleep(time.Duration(isleep))
			} else {
				logf(DEBUGL1, "L1 pqExpire [%s] NEG sleep=(%d ms) nanos=(%d) lenpq=%d", char, isleep/1e6, isleep, lenpq)
			}
		}

		mux.mux.Lock()
		delete(ptr.cache, item.Key)
		cnt.Counter["Count_Delete"]++
		mux.mux.Unlock()
		item = nil
		continue cleanup

		//dq = append(dq, item.Key)
		//dqcnt++
		//item = nil
	}
} // end func pqExpire
