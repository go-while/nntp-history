package history

import (
	// "arena"
	"container/heap"
	"fmt"
	"log"
	"sync"
	"time"
)

/*
L3Cache: key => offsets
less requests to boltDB

disabling L3 is not a good idea!!
queues hold offsets which DB does not know about!
cache keeps track of duplicate writes
duplicate keys will get an empty_offsets
* and the latest will overwrite the past write maybe still in queue
*/
var (
	DEBUGL3         bool  = false
	L3              bool  = true
	L3CacheExpires  int64 = DefaultCacheExpires
	L3ExtendExpires int64 = DefaultCacheExtend
	L3Purge         int64 = DefaultCachePurge
	L3InitSize      int   = 64 * 1024
)

type L3CACHE struct {
	Caches  map[string]*L3CACHEMAP
	Extend  map[string]*StrECH
	Muxers  map[string]*L3MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
	prioQue map[string]*L3PQ         // Priority queue for item expiration
	pqChans map[string]chan struct{} // Priority queue notify channels
	pqMuxer map[string]*L3PQMUX      // Priority queue Muxers
}

type L3CACHEMAP struct {
	cache map[string]*L3ITEM
}

type L3ITEM struct {
	offsets []int64
}

type L3MUXER struct {
	mux sync.RWMutex
}

type L3PQMUX struct {
	mux sync.Mutex
}

// The L3CACHE_Boot method initializes the L3 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l3 *L3CACHE) L3CACHE_Boot(his *HISTORY) {
	if !L3 {
		return
	}
	l3.mux.Lock()
	defer l3.mux.Unlock()
	if l3.Caches != nil {
		log.Printf("ERROR L3CACHESetup already loaded!")
		return
	}
	l3.Caches = make(map[string]*L3CACHEMAP, intBoltDBs)
	l3.Extend = make(map[string]*StrECH, intBoltDBs)
	l3.Muxers = make(map[string]*L3MUXER, intBoltDBs)
	l3.Counter = make(map[string]*CCC)
	l3.prioQue = make(map[string]*L3PQ, intBoltDBs)
	l3.pqChans = make(map[string]chan struct{}, intBoltDBs)
	l3.pqMuxer = make(map[string]*L3PQMUX, intBoltDBs)
	for _, char := range HEXCHARS {
		l3.Caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.Extend[char] = &StrECH{ch: make(chan *StrItems, his.cEvCap)}
		l3.Muxers[char] = &L3MUXER{}
		l3.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l3.prioQue[char] = &L3PQ{}
		l3.pqChans[char] = make(chan struct{}, 1)
		l3.pqMuxer[char] = &L3PQMUX{}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l3.pqExpire(char)
		go l3.L3Cache_Thread(char)
	}

} // end func L3CACHE_Boot

// The L3Cache_Thread function runs as a goroutine for each character.
func (l3 *L3CACHE) L3Cache_Thread(char string) {
	if !L3 {
		return
	}
	l3.mux.Lock() // waits for L3CACHE_Boot to unlock
	l3.mux.Unlock()
	//logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
	l3purge := L3Purge
	if l3purge < 1 {
		l3purge = 1
	}

	go func() {
		defer log.Printf("LEFT L3T gofunc1 extend [%s]", char)
		ptr := l3.Caches[char]
		cnt := l3.Counter[char]
		extC := l3.Extend[char]
		mux := l3.Muxers[char]
		pq := l3.prioQue[char]
		pqC := l3.pqChans[char]
		pqM := l3.pqMuxer[char]
		//forever:
		for {
			select {
			case dat := <-extC.ch: // receives stuff from CacheEvictThread()
				// got keys we will extend
				if len(dat.extends) > 0 {
					//logf(DEBUG, "L3 [%s] extends=%d", char, len(extends))
					pqEX := time.Now().UnixNano() + L3ExtendExpires*int64(time.Second)
					mux.mux.Lock()
					for _, key := range dat.extends {
						if _, exists := ptr.cache[key]; exists {
							cnt.Counter["Count_BatchD"]++
							l3.prioPush(char, pq, pqC, pqM, &L3PQItem{Key: key, Expires: pqEX})
						}
					}
					mux.mux.Unlock()
				}
			} // end select

		} // end forever
	}() // end gofunc1
} //end func L3Cache_Thread

// The SetOffsets method sets a cache item in the L3 cache using a key, char and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets []int64, flagexpires bool, src string) {
	if !L3 {
		return
	}
	//if key == TESTCACKEY {
	//	log.Printf("L3CAC [%s|  ] SetOffsets key='%s' offsets='%#v' flagexpires=%t src='%s'", char, key, offsets, flagexpires, src)
	//}
	if key == "" {
		return
	}
	if char == "" {
		char = string(key[0])
	}
	if offsets == nil {
		return
	}

	ptr := l3.Caches[char]
	cnt := l3.Counter[char]
	//extC := l3.Extend[char]
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]
	pqC := l3.pqChans[char]
	pqM := l3.pqMuxer[char]

	if flagexpires {
		pqEX := time.Now().UnixNano() + (L3CacheExpires * int64(time.Second))
		l3.prioPush(char, pq, pqC, pqM, &L3PQItem{Key: key, Expires: pqEX})
	}
	mux.mux.Lock()
	switch flagexpires {
	case true:
		if len(offsets) > 0 {
			cnt.Counter["Count_FlagEx"]++
		}
	case false:
		cnt.Counter["Count_Set"]++
	}

	//var tailstr string
	if _, exists := ptr.cache[key]; exists {
		// cache entry exists
		//ptr.cache[key].expires = expires
		cachedlen := len(ptr.cache[key].offsets)
		if cachedlen == 0 {
			// there is an empty offsets-slice cached: set this
			ptr.cache[key].offsets = offsets
			mux.mux.Unlock()
			return
		}
		// loops in reversed order backwards over new offsets
		for i := len(offsets) - 1; i >= 0; i-- {
			if offsets[i] <= 0 {
				log.Printf("ERROR L3CACHE SetOffsets offsets[i]=%d", offsets[i])
				continue
			}
			// checks cached offsets backwards too
			tailstr := fmt.Sprintf("key='%s' cached=%d='%#v' i=%d %d/%d='%#v' offsets[i]=%d src='%s'", key, cachedlen, l3.Caches[char].cache[key].offsets, i, i+1, len(offsets), offsets, offsets[i], src)
			if !valueExistsInSliceReverseOrder(offsets[i], ptr.cache[key].offsets) {
				logf(DEBUG, "INFO L3CACHE [%s] SetOffsets append %s", char, tailstr)
				ptr.cache[key].offsets = append(ptr.cache[key].offsets, offsets[i])
			} else {
				//logf(DEBUG, "INFO L3CACHE [%s] SetOffsets exists %s", char, tailstr)
				// NOTE with valueExistsInSliceReverseOrder first hit returns fast now
				mux.mux.Unlock()
				return
			}
		}
		mux.mux.Unlock()
		return
	} else {
		// !exists
		ptr.cache[key] = &L3ITEM{offsets: offsets}
	}
	mux.mux.Unlock()
} // end func SetOffsets

// The GetOffsets method retrieves a slice of offsets from the L3 cache using a key and a char.
func (l3 *L3CACHE) GetOffsets(key string, char string, offsets *[]int64) int {
	if !L3 {
		return 0
	}
	if key == "" || offsets == nil || len(*offsets) > 0 {
		log.Printf("ERROR L3CACHEGet key or io nil or var `offsets` not empty")
		return 0
	}
	if char == "" {
		char = string(key[0])
	}
	ptr := l3.Caches[char]
	//cnt := l3.Counter[char]
	mux := l3.Muxers[char]

	mux.mux.RLock()
	if _, exists := ptr.cache[key]; exists {
		//cnt.Counter["Count_Get"]++ // cant count this here! we only have RLOCK!
		*offsets = ptr.cache[key].offsets
		mux.mux.RUnlock()
		return len(*offsets)
	}
	mux.mux.RUnlock()
	//cnt.Counter["Count_Mis"]++ // cant count this here! we only have RLOCK!
	return 0
} // end func GetOffsets

func (l3 *L3CACHE) L3Stats(statskey string) (retval uint64, retmap map[string]uint64) {
	if statskey == "" {
		retmap = make(map[string]uint64)
	}
	if l3 == nil || l3.Muxers == nil {
		return
	}
	for _, char := range HEXCHARS {
		cnt := l3.Counter[char]
		mux := l3.Muxers[char]
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
} // end func L3Stats

func valueExistsInSliceReverseOrder(value int64, slice []int64) bool {
	for i := len(slice) - 1; i >= 0; i-- {
		if slice[i] == value {
			return true
		}
	}
	return false
}

type L3PQ []*L3PQItem

type L3PQItem struct {
	Key     string
	Expires int64
}

func (pq L3PQ) Len() int { return len(pq) }

func (pq L3PQ) Less(i, j int) bool {
	//logf(DEBUGL3, "L3PQ Less()")
	return pq[i].Expires < pq[j].Expires
}

func (pq L3PQ) Swap(i, j int) {
	//logf(DEBUGL3, "L3PQ Swap()")
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *L3PQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	old = nil
	//logf(DEBUGL3, "L3PQ POP() item='%#v", item)
	return item
}

func (pq *L3PQ) Push(x interface{}) {
	item := x.(*L3PQItem)
	*pq = append(*pq, item)
}

// Remove expired items from the cache
func (l3 *L3CACHE) pqExpire(char string) {
	if !L3 {
		return
	}
	l3.mux.Lock() // waits for boot to finish
	l3.mux.Unlock()
	//start := utils.UnixTimeMilliSec()
	ptr := l3.Caches[char]
	cnt := l3.Counter[char]
	//extC := l3.Extend[char]
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]
	pqC := l3.pqChans[char]
	pqM := l3.pqMuxer[char]
	lpq, dqq, dqmax := 0, uint64(0), uint64(64)
	var item *L3PQItem
	var dq []string
forever:
	for {
		pqM.mux.Lock()
		lpq = len(*pq)
		if lpq == 0 {
			pqM.mux.Unlock()
			//logf(DEBUGL3, "L3 pqExpire [%s] wait on <-pqC", char)
			select {
			case <-pqC: // blocking wait for his.prioPush()
				//logf(DEBUGL3, "L3 pqExpire [%s] recv on <-pqC", char)
				continue forever
			}
		} else {
			// Get the item with the nearest expiration time
			item = (*pq)[0]
		}

		currentTime := time.Now().UnixNano()

		if item.Expires <= currentTime {
			// This item has expired, remove it from the cache and priority queue
			//logf(DEBUGL3 || ALWAYS, "L3 pqExpire [%s] DELETE key='%s' over=%d", char, item.Key, currentTime-item.Expires)
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
			//logf(DEBUGL3, "L3 pqExpire [%s] SLEEP key='%s' sleep=%d lpq=%d", char, item.Key, sleepTime, lpq)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire

func (l3 *L3CACHE) prioPush(char string, pq *L3PQ, pqC chan struct{}, pqM *L3PQMUX, item *L3PQItem) {
	if !L3 {
		return
	}
	//log.Printf("L3 prioPush [%s] heap.push item='%#v' expireS=%d", char, item, (pqEX-time.Now().UnixNano())/int64(time.Second))
	pqM.mux.Lock()
	heap.Push(pq, item)
	pqM.mux.Unlock()

	//log.Printf("L3 prioPush [%s] heap.push unlocked", char)
	select {
	case pqC <- struct{}{}:
		// pass notify to pqExpire()
	default:
		// pass too: notify chan is full
	}
	//log.Printf("L3 prioPush [%s] heap.push passed pqC", char)
} // end func prioPush
