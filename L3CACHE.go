package history

import (
	"fmt"
	"log"
	"sync"
	"time"
)

/*
 *
 * L3Cache: key => offsets
 * less requests to boltDB
 *
 * disabling L3 is not a good idea!!
 * queues hold offsets which DB does not know about!
 * cache keeps track of duplicate writes
 * duplicate keys will get an empty_offsets
 * and the latest will overwrite the past write maybe still in queue
 *
 */
var (
	DEBUGL3         bool  = false
	L3              bool  = true // do not disable!
	L3CacheExpires  int64 = DefaultCacheExpires
	L3ExtendExpires int64 = DefaultCacheExtend
	L3Purge         int64 = DefaultCachePurge
	L3InitSize      int   = 64 * 1024
)

type L3CACHE struct {
	Caches  map[string]*L3CACHEMAP
	Extend  map[string]*L3ECH
	Muxers  map[string]*L3MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
	prioQue map[string]*L3PrioQue // Priority queue for item expiration

}

type L3CACHEMAP struct {
	cache map[string]*L3ITEM
}

type L3ITEM struct {
	offsets []int64
}

// L3ExtendChan
type L3ECH struct {
	ch chan *L3PQItem
}

type L3MUXER struct {
	mux sync.RWMutex
}

type L3PrioQue struct {
	que *L3PQ
	mux sync.Mutex
	pqC chan struct{}
}

type L3PQ []*L3PQItem

type L3PQItem struct {
	Key     string
	Expires int64
}

// The BootL3Cache method initializes the L3 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l3 *L3CACHE) BootL3Cache(his *HISTORY) {
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
	l3.Extend = make(map[string]*L3ECH, intBoltDBs)
	l3.Muxers = make(map[string]*L3MUXER, intBoltDBs)
	l3.Counter = make(map[string]*CCC)
	l3.prioQue = make(map[string]*L3PrioQue, intBoltDBs)
	for _, char := range HEXCHARS {
		l3.Caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.Extend[char] = &L3ECH{ch: make(chan *L3PQItem, his.cEvCap)}
		l3.Muxers[char] = &L3MUXER{}
		l3.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l3.prioQue[char] = &L3PrioQue{que: &L3PQ{}, pqC: make(chan struct{}, 1)}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l3.pqExpire(char)
		go l3.pqExtend(char)
	}
	log.Printf("L3Cache_Boot")
} // end func BootL3Cache

// The pqExtend function runs as a goroutine for each character.
func (l3 *L3CACHE) pqExtend(char string) {
	if !L3 {
		return
	}
	l3.mux.Lock() // waits for BootL3Cache to unlock
	l3.mux.Unlock()
	//logf(DEBUGL3, "Boot L3pqExtend [%s]", char)
	//defer log.Printf("LEFT L3 [%s] pqExtend", char)

	l3purge := L3Purge
	if l3purge <= 0 {
		l3purge = 1
	}
	clearEv := ClearEveryN
	if clearEv <= 0 {
		clearEv = 1
	}

	ptr := l3.Caches[char]
	cnt := l3.Counter[char]
	extC := l3.Extend[char]
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]
	pushq, pushmax := []*L3PQItem{}, clearEv
	timeout := false
	timer := time.NewTimer(time.Duration(l3purge) * time.Second)

	//forever:
	for {
		select {
		case <-timer.C:
			timeout = true
		case pqitem := <-extC.ch: // receives stuff from DoCacheEvict
			pushq = append(pushq, pqitem)
		} // end select
		if len(pushq) >= pushmax || (timeout && len(pushq) > 0) {
			mux.mux.Lock()
			for _, item := range pushq {
				if _, exists := ptr.cache[item.Key]; exists {
					cnt.Counter["Count_BatchD"]++
				}
			}
			mux.mux.Unlock()
			for _, item := range pushq {
				pq.Push(item)
			}
			pushq = nil
		}
		if timeout {
			timeout = false
		}
		timer.Reset(time.Duration(l3purge) * time.Second)
	} // end forever
} //end func pqExtend

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
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]

	if flagexpires {
		pq.Push(&L3PQItem{Key: key, Expires: L3CacheExpires})

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

func (pq *L3PrioQue) Push(item *L3PQItem) {
	item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
	pq.mux.Lock()
	*pq.que = append(*pq.que, item)
	pq.mux.Unlock()
} // end func Push

func (pq *L3PrioQue) Pop() (*L3PQItem, int) {
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
func (l3 *L3CACHE) pqExpire(char string) {
	if !L3 {
		return
	}
	//log.Printf("L3 pqExpire [%s] wait l3 lock", char)
	l3.mux.Lock() // waits for boot to finish
	l3.mux.Unlock()
	logf(DEBUGL3, "L3 pqExpire [%s] booted", char)

	ptr := l3.Caches[char]
	cnt := l3.Counter[char]
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]
	//lenpq := 0
	var item *L3PQItem
	var isleep int64
	l3purge := L3Purge
	dq, dqmax := []string{}, ClearEveryN
	lf := UnixTimeSec()
	now := UnixTimeSec()
cleanup:
	for {
		now = UnixTimeSec()
		if len(dq) >= dqmax || lf <= now-l3purge {
			if len(dq) > 0 {
				mux.mux.Lock()
				for _, key := range dq {
					delete(ptr.cache, key)
				}
				cnt.Counter["Count_Delete"] += uint64(len(dq))
				mux.mux.Unlock()
			}
			dq, lf = nil, now
		}
		item, _ = pq.Pop()
		if item == nil {
			time.Sleep(time.Duration(l3purge) * time.Second)
			continue cleanup
		}
		if item.Expires > time.Now().UnixNano() {
			isleep = item.Expires - time.Now().UnixNano()
			//logf(DEBUGL1, "L3 pqExpire [%s] sleep=(%d ms) lenpq=%d", char, isleep/1e6, lenpq)
			if isleep > 0 {
				time.Sleep(time.Duration(isleep))
			}
		}
		dq = append(dq, item.Key)
		item = nil
	}
} // end func pqExpire
