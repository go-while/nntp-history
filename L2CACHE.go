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
	L2pqSize        int   = 256 * 1024
)

type L2CACHE struct {
	Caches  map[string]*L2CACHEMAP
	Extend  map[string]*L2ECH
	Muxers  map[string]*L2MUXER
	mux     sync.Mutex
	Counter map[string]*CCC
	pqQueue map[string]*L2pqQ // Priority queue for item expiration
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

type L2pqQ struct {
	que chan *L2PQItem
	mux sync.RWMutex // pq.mux.Lock()
	pqC chan struct{}
	char string
}

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
	l2.Caches = make(map[string]*L2CACHEMAP, NumBBoltDBs)
	l2.Extend = make(map[string]*L2ECH, NumBBoltDBs)
	l2.Muxers = make(map[string]*L2MUXER, NumBBoltDBs)
	l2.Counter = make(map[string]*CCC, NumBBoltDBs)
	l2.pqQueue = make(map[string]*L2pqQ, NumBBoltDBs)
	for _, char := range HEXCHARS {
		l2.Caches[char] = &L2CACHEMAP{cache: make(map[int64]*L2ITEM, L2InitSize)}
		l2.Extend[char] = &L2ECH{ch: make(chan *L2PQItem, his.cEvCap)}
		l2.Muxers[char] = &L2MUXER{}
		l2.Counter[char] = &CCC{Counter: make(map[string]uint64)}
		l2.pqQueue[char] = &L2pqQ{que: make(chan *L2PQItem, L2pqSize), pqC: make(chan struct{}, 1), char: char}
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
	pq := l2.pqQueue[char]
	pushq, pushmax, pushcnt := make([]*L2PQItem, clearEv), clearEv, 0
	timeout := false
	timer := time.NewTimer(time.Duration(l2purge) * time.Second)

	//forever:
	for {
		select {
		case <-timer.C:
			timeout = true
		case pqitem := <-extC.ch: // receives stuff from DoCacheEvict
			if pqitem != nil {
				//log.Printf("L2 pushq append pqitem=%#v", pqitem)
				pushq[pushcnt] = pqitem
				pushcnt++
			} else {
				log.Printf("ERROR L2 pqExtend extC.ch <- nil pointer")
				return
			}
		} // end select
		if pushcnt >= pushmax || (timeout && pushcnt > 0) {

			if DEBUGL2 {
				logf(DEBUGL2, "DEBUGL2 ptr.cache Count_BatchD")
				mux.mux.Lock()
				for i := 0; i < pushcnt; i++ {
					if _, exists := ptr.cache[pushq[i].Key]; exists {
						cnt.Counter["Count_BatchD"]++
					}
				}
				mux.mux.Unlock()
			}

			//pq.mux.Lock()
			for i := 0; i < pushcnt; i++ {
				pq.Push(pushq[i])
			}
			//pq.mux.Unlock()

			pushq, pushcnt = make([]*L2PQItem, clearEv), 0
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
	pq := l2.pqQueue[char]

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

func (pq *L2pqQ) Push(item *L2PQItem) {
forever:
	for {
		item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
		select {
		case pq.que <- item:
			// pushed
			break forever
		default:
			// channel full!
			log.Printf("WARN L2pqQ char=%s Push channel is full!", pq.char)
			time.Sleep(time.Millisecond)
		}
	}
} // end func Push

func (pq *L2pqQ) Push_TryNewCode(item *L2PQItem) {
	 // everyone readLocks on entering the function
	pq.mux.RLock()
	rlocked := true
forever:
	for {
		item.Expires = time.Now().UnixNano() + item.Expires*int64(time.Second)
		select {
		case pq.que <- item:
			// pushed
			if rlocked {
				// release readLock
				pq.mux.RUnlock()
			}
			break forever
		default:
			// channel full!
			log.Printf("WARN L2pqQ char=%s Push channel is full! LOCKING", pq.char)
			//pq.mux.Lock() // lock others out but not ourself
			if !tryRWLockWithTimeout(&pq.mux, time.Second, rlocked) {
				log.Printf("WARN L2pqQ char=%s tryRWLockWithTimeout failed", pq.char)
				rlocked = false
				continue forever
			}
			//pq.mux.RUnlock() // release our readLock
			rlocked = false

			oldsize := cap(pq.que)
			newsize := oldsize * 4
			time.Sleep(time.Second)
			// waited long enough for other lockers to replace the chan?
			checkcap := cap(pq.que)
			if checkcap != oldsize {
				// channel capacity has been updated by someone else
				// continue to try putting our item in pq.que
				pq.mux.Unlock()
				log.Printf("ERROR L2pqQ char=%s Push channel has changed!", pq.char)
				continue forever
			}

			newchan := make(chan *L2PQItem, newsize)
			suck:
			for {
				select {
					case aitem := <- pq.que:
						newchan <- aitem
					default:
						// sucked channel empty
						break suck
				}
			}
			// replace channel
			pq.que = newchan
			log.Printf("INFO L2pqQ char=%s Push channel replaced newcap=%d filled=%d", pq.char, newsize, len(pq.que))
			pq.mux.Unlock()
		}
	}
} // end func Push

func (pq *L2pqQ) Pop() (*L2PQItem, int) {
	select {
	case item := <-pq.que:
		return item, len(pq.que)
	default:
		// pq empty
	}
	return nil, 0
} // end func Pop

// Remove expired items from the cache
func (l2 *L2CACHE) pqExpire(char string) {
	if !L2 {
		return
	}
	//log.Printf("L2 pqExpire [%s] wait l2 lock", char)
	l2.mux.Lock() // waits for boot to finish
	l2.mux.Unlock()
	logf(DEBUGL2, "L2 pqExpire [%s] booted", char)

	ptr := l2.Caches[char]
	cnt := l2.Counter[char]
	mux := l2.Muxers[char]
	pq := l2.pqQueue[char]
	lenpq := 0
	var item *L2PQItem
	var isleep int64
	l2purge := L2Purge
	//dq, dqmax, dqcnt := make([]int64, ClearEveryN), ClearEveryN, 0
	//now := UnixTimeSec()
	//lf := now
cleanup:
	for {
		/*
			now = UnixTimeSec()
			if dqcnt >= dqmax || lf <= now-l2purge {
				if dqcnt > 0 {
					mux.mux.Lock()
					for i := 0; i < dqcnt; i++ {
						delete(ptr.cache, dq[i])
					}
					cnt.Counter["Count_Delete"] += uint64(len(dq))
					mux.mux.Unlock()
				}
				dq, lf, dqcnt = make([]int64, ClearEveryN), now, 0
			}
		*/
		item, lenpq = pq.Pop()
		if item == nil {
			time.Sleep(time.Duration(l2purge) * time.Second)
			continue cleanup
		}
		if item.Expires > time.Now().UnixNano() {
			isleep = item.Expires - time.Now().UnixNano()
			if isleep >= int64(1*time.Millisecond) {
				logf(DEBUGL2, "L2 pqExpire [%s] sleep=(%d ms) nanos=(%d) lenpq=%d", char, isleep/1e6, isleep, lenpq)
				time.Sleep(time.Duration(isleep))
			} else {
				logf(DEBUGL2, "L2 pqExpire [%s] NEG sleep=(%d ms) nanos=(%d) lenpq=%d", char, isleep/1e6, isleep, lenpq)
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
