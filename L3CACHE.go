package history

import (
	"container/heap"
	"github.com/go-while/go-utils"
	"log"
	"sync"
	"time"
)

/*
L3Cache: key => offsets
less requests to boltDB
*/
var (
	DEBUGL3         bool  = false
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
	pqMuxer map[string]*L3MUXER      // Priority queue Muxers
}

type L3CACHEMAP struct {
	cache map[string]*L3ITEM
}

type L3ITEM struct {
	offsets []int64
	expires int64
}

type L3MUXER struct {
	mux sync.RWMutex
}

// The L3CACHE_Boot method initializes the L3 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l3 *L3CACHE) L3CACHE_Boot(his *HISTORY) {
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
	l3.pqMuxer = make(map[string]*L3MUXER, intBoltDBs)
	for _, char := range HEXCHARS {
		l3.Caches[char] = &L3CACHEMAP{cache: make(map[string]*L3ITEM, L3InitSize)}
		l3.Extend[char] = &StrECH{ch: make(chan []string, his.cEvCap)}
		l3.Muxers[char] = &L3MUXER{}
		l3.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l3.prioQue[char] = &L3PQ{}
		l3.pqChans[char] = make(chan struct{}, 1)
		l3.pqMuxer[char] = &L3MUXER{}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l3.pqExpire(char)
		go l3.L3Cache_Thread(char)
	}

} // end func L3CACHE_Boot

// The L3Cache_Thread function runs as a goroutine for each character.
// It periodically cleans up expired cache entries and dynamically shrinks the cache size if needed.
func (l3 *L3CACHE) L3Cache_Thread(char string) {
	l3.mux.Lock() // waits for L3CACHE_Boot to unlock
	l3.mux.Unlock()
	//logf(DEBUGL3, "Boot L3Cache_Thread [%s]", char)
	//cleanup := []string{}
	l3purge := L3Purge
	if l3purge < 1 {
		l3purge = 1
	}

	go func() {
		defer log.Printf("LEFT L3T gofunc1 extend [%s]", char)
		timer := time.NewTimer(time.Duration(l3purge) * time.Second)
		var extends []string
		ptr := l3.Caches[char]
		cnt := l3.Counter[char]
		extC := l3.Extend[char]
		mux := l3.Muxers[char]
		pq := l3.prioQue[char]
		pqC := l3.pqChans[char]
		pqM := l3.pqMuxer[char]
		//forever:
		for {
		forextends:
			for {
				select {
				case <-timer.C:
					break forextends
				case slice := <-extC.ch: // receives stuff from CacheEvictThread()
					// got keys we will extend in next timer.C run
					extends = slice
					break forextends
				} // end select
			} // end forextends
			if len(extends) > 0 {
				now := utils.UnixTimeSec()
				//logf(DEBUG, "L3 [%s] extends=%d", char, len(extends))
				mux.mux.Lock()
				for _, key := range extends {
					if _, exists := ptr.cache[key]; exists {
						ptr.cache[key].expires = now + L3ExtendExpires
						cnt.Counter["Count_BatchD"]++
						pqEX := time.Now().UnixNano() + L1ExtendExpires*int64(time.Second)

						pqM.mux.Lock()
						heap.Push(pq, &L3PQItem{
							Key:     key,
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
				timer.Reset(time.Duration(l3purge) * time.Second)
			}
		} // end forever
	}() // end gofunc1

	/*
		go func(ptr *L3CACHEMAP, mux *sync.RWMutex, cnt *CCC) {
			defer log.Printf("LEFT L3T gofunc2 delete [%s]", char)
			timer := time.NewTimer(time.Duration(l3purge) * time.Second)
			start := utils.UnixTimeMilliSec()
			now := int64(start / 1000)
			//forever:
			for {
				select {
				case <-timer.C:
					start = utils.UnixTimeMilliSec()
					now = int64(start / 1000)

					//mux.RLock()
					mux.Lock()
					//getexpired:
					for key, item := range ptr.cache {
						if item.expires > 0 && item.expires < now {
							//logf(DEBUG, "L3 expire [%s] key='%#v' item='%#v'", char, key, item)
							//cleanup = append(cleanup, key)
							cnt.Counter["Count_Delete"]++
							delete(ptr.cache, key)
						}
					} // end for getexpired
					mux.Unlock()
					//mux.RUnlock()

					//maplen := len(ptr.cache)
					/,*
						if len(cleanup) > 0 {
							mux.Lock()
							//maplen -= len(cleanup)
							for _, key := range cleanup {
								delete(ptr.cache, key)
								cnt.Counter["Count_Delete"]++
							}
							mux.Unlock()
							//logf(DEBUG, "L3Cache_Thread [%s] deleted=%d/%d", char, len(cleanup), maplen)
							cleanup = nil
						}
						//logf(DEBUG, "L3Cache_Thread [%s] (took %d ms)", char, utils.UnixTimeMilliSec()-start)
					*,/
					timer.Reset(time.Duration(l3purge) * time.Second)
				} // end select
			} // end for
		}(l3.Caches[char], &l3.Muxers[char].mux, l3.Counter[char]) // end gofunc2
	*/
} //end func L3Cache_Thread

// The SetOffsets method sets a cache item in the L3 cache using a key, char and a slice of offsets as the value.
// It also dynamically grows the cache when necessary.
func (l3 *L3CACHE) SetOffsets(key string, char string, offsets []int64, flagexpires bool, src string) {
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
	pqC := l3.pqChans[char]
	pqM := l3.pqMuxer[char]

	mux.mux.Lock()

	expires := NoExpiresVal
	var pqEX int64
	if flagexpires {
		if len(offsets) > 0 {
			cnt.Counter["Count_FlagEx"]++
		}
		expires = utils.UnixTimeSec() + L3CacheExpires
		pqEX = time.Now().UnixNano() + (L3CacheExpires * int64(time.Second))
	} else {
		cnt.Counter["Count_Set"]++
	}
	//var tailstr string
	if _, exists := ptr.cache[key]; exists {
		// cache entry exists
		ptr.cache[key].expires = expires
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
			//tailstr := fmt.Sprintf("key='%s' cached=%d='%#v' i=%d %d/%d='%#v' cacheex=%d newexpi=%d offsets[i]=%d src='%s'", key, cachedlen, l3.Caches[char].cache[key].offsets, i, i+1, len(offsets), offsets, l3.Caches[char].cache[key].expires, expires, offsets[i], src)
			if !valueExistsInSliceReverseOrder(offsets[i], ptr.cache[key].offsets) {
				//logf(DEBUG, "INFO L3CACHE [%s] SetOffsets append %s", char, tailstr)
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
	}
	ptr.cache[key] = &L3ITEM{offsets: offsets, expires: expires}
	// Update the priority queue
	if flagexpires {
		log.Printf("l3.set [%s] heap.push key='%s' expireS=%d", char, key, (pqEX-time.Now().UnixNano())/int64(time.Second))
		pqM.mux.Lock()
		heap.Push(pq, &L3PQItem{
			Key:     key,
			Expires: pqEX,
		})
		pqM.mux.Unlock()
		mux.mux.Unlock()
		log.Printf("l3.set [%s] heap.push unlocked", char)
		select {
		case pqC <- struct{}{}:
			// pass
		default:
			// pass too: notify chan is full
		}
		log.Printf("l3.set [%s] heap.push passed pqC", char)
		return
	}
	mux.mux.Unlock()
} // end func SetOffsets

// The GetOffsets method retrieves a slice of offsets from the L3 cache using a key and a char.
func (l3 *L3CACHE) GetOffsets(key string, char string, offsets *[]int64) int {
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
	//log.Printf("L3PQ Less()")
	return pq[i].Expires < pq[j].Expires
}

func (pq L3PQ) Swap(i, j int) {
	//log.Printf("L3PQ Swap()")
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *L3PQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	old = nil
	//log.Printf("L3PQ POP() item='%#v", item)
	return item
}

func (pq *L3PQ) Push(x interface{}) {
	item := x.(*L3PQItem)
	*pq = append(*pq, item)
}

// Remove expired items from the cache
func (l3 *L3CACHE) pqExpire(char string) {
	l3.mux.Lock() // waits for boot to finish
	l3.mux.Unlock()
	cnt := l3.Counter[char]
	ptr := l3.Caches[char]
	mux := l3.Muxers[char]
	pq := l3.prioQue[char]
	pqC := l3.pqChans[char]
	pqM := l3.pqMuxer[char]
	//var item *L3PQItem
	var empty bool
	dqcnt, dqmax := 0, 512
	dq := make([]string, dqmax)
	lastdel := time.Now().Unix()
forever:
	for {
		if dqcnt >= dqmax || (lastdel < time.Now().Unix()-L3Purge && dqcnt > 0) {
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
				//log.Printf("L3 pqExpire [%s] blocking wait dqcnt=%d", char, dqcnt)
				select {
				case <-pqC: // waits for notify to run
					// pass
				default:
					time.Sleep(time.Duration(L3Purge) * time.Second)
					pqM.mux.RLock() // watch this! RLock gets opened here
					if len(*pq) > 0 {
						empty = false
					}
					if !empty {
						//log.Printf("L3 pqExpire [%s] released wait: !empty", char)
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
			//log.Printf("L3 pqExpire [%s] key='%s' diff=%d", char, item.Key, item.Expires-currentTime)
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
			//log.Printf("L3 pqExpire [%s] key='%s' diff=%d sleepTime=%d", char, item.Key, currentTime-item.Expires, sleepTime)
			time.Sleep(sleepTime)
		}
	} // end for
} // end func pqExpire
