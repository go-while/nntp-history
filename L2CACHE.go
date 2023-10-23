package history

/*
 * WARNING: the `char` used by L2Cache is NOT boltDB char!
 *          it is derived from offset via OffsetToChar !
 */
import (
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
	L2              bool  = true
	L2CacheExpires  int64 = DefaultCacheExpires
	L2ExtendExpires int64 = DefaultCacheExtend
	L2Purge         int64 = DefaultCachePurge
	L2InitSize      int   = 64 * 1024
)

type L2CACHE struct {
	Caches  map[string]*L2CACHEMAP
	Extend  map[string]*L2ECH
	Muxers  map[string]*L2MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
	prioQue map[string]*L2PrioQue // Priority queue for item expiration
}

type L2CACHEMAP struct {
	cache map[int64]*L2ITEM
}

type L2ITEM struct {
	hash string
}

// L2ExtendChan
type L2ECH struct {
	ch chan *L2PQItem
}

type L2MUXER struct {
	mux sync.RWMutex
}

type L2PrioQue struct {
	que *L2PQ
	mux sync.Mutex
	pqC chan struct{}
}

type L2PQ []*L2PQItem

type L2PQItem struct {
	Key     int64
	Expires int64
}

// The BootL2Cache method initializes the L2 cache.
// It creates cache maps, initializes them with initial sizes, and starts goroutines to periodically clean up expired entries.
func (l2 *L2CACHE) BootL2Cache(his *HISTORY) {
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
	l2.Extend = make(map[string]*L2ECH, 16)
	l2.Muxers = make(map[string]*L2MUXER, 16)
	l2.Counter = make(map[string]*CCC)
	l2.prioQue = make(map[string]*L2PrioQue, intBoltDBs)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = &L2ECH{ch: make(chan *L2PQItem, his.cEvCap)}
		l2.Muxers[char] = &L2MUXER{}
		l2.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l2.prioQue[char] = &L2PrioQue{que: &L2PQ{}, pqC: make(chan struct{}, 1)}
	}
	time.Sleep(time.Millisecond)
	for _, char := range HEXCHARS {
		// stupid race condition on boot when placed in loop before
		go l2.pqExpire(char)
		go l2.pqExtend(char)
	}
	log.Printf("L2Cache_Boot")
} // end func BootL2Cache

// The pqExtend function runs as a goroutine for each character.
func (l2 *L2CACHE) pqExtend(char string) {
	if !L2 {
		return
	}
	l2.mux.Lock() // waits for BootL2Cache to unlock
	l2.mux.Unlock()
	//logf(DEBUGL2, "Boot L2pqExtend [%s]", char)
	//defer log.Printf("LEFT L2 [%s] pqExtend", char)

	l2purge := L2Purge
	if l2purge <= 0 {
		l2purge = 1
	}
	clearEv := ClearEveryN
	if clearEv <= 0 {
		clearEv = 1
	}

	ptr := l2.Caches[char]
	cnt := l2.Counter[char]
	extC := l2.Extend[char]
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	pushq, pushmax := []*L2PQItem{}, clearEv
	timeout := false
	timer := time.NewTimer(time.Duration(l2purge) * time.Second)

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
		timer.Reset(time.Duration(l2purge) * time.Second)
	} // end forever
} //end func pqExtend

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

	ptr := l2.Caches[char]
	cnt := l2.Counter[char]
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]

	if flagexpires {
		pq.Push(&L2PQItem{Key: offset, Expires: L2CacheExpires})
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

func (pq *L2PrioQue) Push(item *L2PQItem) {
	item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
	pq.mux.Lock()
	*pq.que = append(*pq.que, item)
	pq.mux.Unlock()
} // end func Push

func (pq *L2PrioQue) Pop() (*L2PQItem, int) {
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
func (l2 *L2CACHE) pqExpire(char string) {
	if !L2 {
		return
	}
	//log.Printf("L2 pqExpire [%s] wait l2lock", char)
	l2.mux.Lock() // waits for boot to finish
	l2.mux.Unlock()
	logf(DEBUGL2, "L2 pqExpire [%s] booted", char)

	ptr := l2.Caches[char]
	cnt := l2.Counter[char]
	mux := l2.Muxers[char]
	pq := l2.prioQue[char]
	//lenpq := 0
	var item *L2PQItem
	var isleep int64
	l2purge := L2Purge
	dq, dqmax := []int64{}, ClearEveryN
	lf := UnixTimeSec()
	now := UnixTimeSec()
cleanup:
	for {
		now = UnixTimeSec()
		if len(dq) >= dqmax || lf <= now-l2purge {
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
			time.Sleep(time.Duration(l2purge) * time.Second)
			continue cleanup
		}
		if item.Expires > time.Now().UnixNano() {
			isleep = item.Expires - time.Now().UnixNano()
			//logf(DEBUGL1, "L2 pqExpire [%s] sleep=(%d ms) lenpq=%d", char, isleep/1e6, lenpq)
			if isleep > 0 {
				time.Sleep(time.Duration(isleep))
			}
		}
		dq = append(dq, item.Key)
		item = nil
	}
} // end func pqExpire
