package history

import (
	//"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	//"github.com/go-while/go-utils"
	//"github.com/edsrzf/mmap-go"
	"hash/fnv"
	"io"
	"log"
	//"os"
	//"github.com/nutsdb/nutsdb"
	//"strings"
	//"sync"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"time"
)

const (
	boltDBs int = 16 // never change this
)

var (
	BoltCheckParallel int = boltDBs // set this via 'history.BoltCheckParallel = 1' before calling History_Boot.
	BoltHashOpen      = make(chan struct{}, boltDBs) // dont change this
)

type HistoryIndex struct {
	Hash         *string
	Offset       int64
	IndexRetChan chan int
}

// History_DBZinit initializes the history database (HashDB) and starts the worker goroutines for processing historical data.
// It creates worker channels for each character in HEXCHARS and launches corresponding worker goroutines.
// The provided boltOpts parameter allows configuring the BoltDB database options.
func (his *HISTORY) History_DBZinit(boltOpts *bolt.Options) {
	logf(DEBUG1, "his.History_DBZinit()")
	if his.boltChanInit != nil {
		log.Printf("ERROR History_DBZinit already loaded")
		return
	}
	his.boltChanInit = make(chan struct{}, BoltCheckParallel)
	for i, char := range HEXCHARS {
		indexchan := make(chan *HistoryIndex, 1)
		his.IndexChans[i] = indexchan
		his.charsMap[char] = i
		go his.History_DBZ_Worker(char, i, indexchan, boltOpts)
	}
	go his.History_DBZ()
} // end func History_DBZinit

// History_DBZ is the main routine for managing the historical data processing.
// It listens to incoming HistoryIndex structs on the IndexChan channel and distributes them to corresponding worker goroutines.
func (his *HISTORY) History_DBZ() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ")
	his.wait4HashDB()
	logf(DEBUG1,"Boot History_DBZ")
	defer log.Printf("Quit History_DBZ")
forever:
	for {
		select {
		case hi, ok := <-his.IndexChan: // recevies a HistoryIndex struct and passes it down to '0-9a-f' workers
			if !ok {
				log.Printf("Stopping History_DBZ IndexChan closed")
				break forever
			}
			if hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // allow at least md5
				log.Printf("Stopping History_DBZ IndexChan received nil pointer")
				break forever
			}
			if hi.Offset == 0 {
				log.Printf("ERROR History_DBZ offset=0") // must: Offset -1 to checkonly OR Offset > 0 adds to hashDB
				break forever
			}
			// gets first char of hash: hash must be lowercase!
			// hex.EncodeToString returns a lowercased string of a hashsum
			C1 := string((string(*hi.Hash)[0]))
			if his.IndexChans[his.charsMap[C1]] != nil {
				his.IndexChans[his.charsMap[C1]] <- hi // sends object to hash History_DBZ_Worker char
			} else {
				log.Printf("Error History_DBZ IndexChan C1=%s=nil", C1)
				break forever
			}
		} // end select
	} // end for
	for _, achan := range his.IndexChans {
		// passing nils to IndexChans will stop History_DBZ_Worker
		// achan <- nil
		close(achan)
	}
} // end func History_DBZ

// History_DBZ_Worker is a worker function responsible for processing historical data.
// It manages BoltDB operations, including storing and retrieving offsets, and handles duplicate checks
// to ensure message-ID hashes are correctly tracked in the history file.
func (his *HISTORY) History_DBZ_Worker(char string, i int, indexchan chan *HistoryIndex, boltOpts *bolt.Options) {
	if !LOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char) {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char)
	dbpath := his.HF_hash + "." + char
	if boltOpts == nil {
		defboltOpts := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 16 * 1024 * 1024,
			PageSize:        4 * 1024,
			NoSync:          true,
			//NoFreelistSync: true,
		}
		boltOpts = &defboltOpts
	}
	db, err := bolt.Open(dbpath, 0600, boltOpts)
	if err != nil {
		log.Printf("Error HashDB dbpath='%s' err='%v'", dbpath, err)
		return
	}
	defer boltSyncClose(db, char)
	testkey := "1"
	testoffsets := []int64{1}
	tcheck := 4096
	checked := 0
	created := 0
	setempty := false
	initLongTest := false
	his.boltChanInit <- struct{}{}
	logf(DEBUG1, "HDBZW: INIT HashDB char=%s", char)
	for _, c1 := range HEXCHARS {
		for _, c2 := range HEXCHARS {
			for _, c3 := range HEXCHARS {
				bucket := c1 + c2 + c3
				retbool, err := boltCreateBucket(db, &char, &bucket)
				if err != nil || !retbool {
					if err == bolt.ErrBucketExists {
						if !initLongTest {
							checked++
							continue
						}
					} else {
						log.Printf("ERROR HDBZW INIT HashDB boltCreateBucket char=%s bucket=%s err='%v' retbool=%t", char, bucket, err, retbool)
						return
					}
				} else if retbool {
					created++ // <= bucket has been created
					if !initLongTest {
						checked++
						continue
					}
					// put1
					if err := boltBucketKeyPutOffsets(db, &char, &bucket, &testkey, &testoffsets, setempty); err != nil {
						log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets1 char=%s bucket=%s err='%v' retbool=%t", char, bucket, err, retbool)
						return
					}
					// get1
					offsets1, err := boltBucketGetOffsets(db, &char, &bucket, &testkey)
					if err != nil || offsets1 == nil {
						log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 char=%s bucket=%s key=%s err='%v'", char, bucket, testkey, err)
						return
					}
					if len(*offsets1) != 1 {
						log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets1 char=%s bucket=%s len(offsets)=%d != 1 ", char, bucket, len(*offsets1))
						return
					}
					// put2
					*offsets1 = append(*offsets1, 2)
					if err := boltBucketKeyPutOffsets(db, &char, &bucket, &testkey, offsets1, setempty); err != nil {
						log.Printf("ERROR HDBZW INIT HashDB boltBucketKeyPutOffsets2 char=%s bucket=%s err='%v'", char, bucket, err)
						return
					}
				}
				if !initLongTest {
					continue
				}
				// get2
				offsets2, err := boltBucketGetOffsets(db, &char, &bucket, &testkey)
				if err != nil || offsets2 == nil || len(*offsets2) != 2 {
					log.Printf("ERROR HDBZW INIT HashDB boltBucketGetOffsets2 char=%s bucket=%s key=%s err='%v'", char, bucket, testkey, err)
					return
				}
				if keys, err := boltGetAllKeys(db, &char, &bucket); err != nil || keys == nil {
					log.Printf("ERROR HDBZW INIT HashDB boltGetAllKeys char=%s bucket=%s err='%v'", char, bucket, err)
					return
				} else if len(*keys) == 1 {
					checked++
				}
			} // enf for c3
		} // end for c2
		//log.Printf("HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tcheck, created, tcheck)
	} // end for c1
	<-his.boltChanInit
	logf(DEBUG1,"HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tcheck, created, tcheck)
	if checked != 4096 {
		log.Printf("ERROR HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tcheck, created, tcheck)
		return
	}
	his.setBoltHashOpen()
	defer his.returnBoltHashOpen()
	lastsync := utils.UnixTimeSec()
	var added, total, processed, dupes, searches, retry uint64
	cutHashlen := 9 // 4:9 = 5 chars
	if his.hashtype == HashShort {
		if his.hashlen >= 5 {
			cutHashlen = 4 + his.hashlen
		}
	}
forever:
	for {
		select {
		case hi, ok := <-indexchan: // recevies a HistoryIndex struct
			if !ok || hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // at least md5
				// receiving a nil object stops history_dbz_worker
				break forever
			}
			var key *string
			bucket := string(string(*hi.Hash)[1:4]) // get 3 chars for bucket
			switch his.hashtype {
			case HashShort:
				max := len(*hi.Hash)
				if cutHashlen > max {
					cutHashlen = max
				}
				shorthash := string(string(*hi.Hash)[4:cutHashlen])
				key = &shorthash
			case HashFNV32:
				key = FNV32S(hi.Hash)
			case HashFNV32a:
				key = FNV32aS(hi.Hash)
			case HashFNV64:
				key = FNV64S(hi.Hash)
			case HashFNV64a:
				key = FNV64aS(hi.Hash)
			}
			logf(DEBUG1,"WORKER HDBZW char=%s hash=%s bucket=%s key=%s @0x%010x|%d|%x", char, *hi.Hash, bucket, *key, hi.Offset, hi.Offset, hi.Offset)
			isDup, err := his.DupeCheck(db, &char, &bucket, key, hi.Hash, &hi.Offset, false)
			if err != nil {
				if err != io.EOF {
					if hi.IndexRetChan != nil {
						close(hi.IndexRetChan)
					}
					log.Printf("ERROR HDBZW char=%s DupeCheck err='%v'", char, err)
					break forever
				}
				// dupecheck got EOF from history file
			}

			if hi.IndexRetChan != nil {
				hi.IndexRetChan <- isDup
			}
			if hi.Offset == -1 {
				searches++
				continue forever
			} else if hi.Offset > 0 {
				processed++
			}
			switch isDup {
			case 0:
				added++
				total++
			case 1:
				dupes++
			case 2:
				retry++
			}
			if added >= Bolt_SYNC_EVERYn || (added > 0 && lastsync <= utils.UnixTimeSec()-Bolt_SYNC_EVERYs) {
				if err := BoltSync(db, char); err != nil {
					break forever
				}
				added, lastsync = 0, utils.UnixTimeSec()
			}
		} // end select
	} // end for
	log.Printf("Quit HDBZW char=%s added=%d dupes=%d processed=%d searches=%d retry=%d", char, total, dupes, processed, searches, retry)
} // end func History_DBZ_Worker

// DupeCheck checks for duplicate message-ID hashes in a BoltDB bucket.
// It manages offsets associated with message hashes and handles duplicates, ensuring the integrity of the historical data.
// If a hash is a duplicate, it returns 1, otherwise, it returns 0.
// It also handles the creation of new hash entries in the bucket when needed.
func (his *HISTORY) DupeCheck(db *bolt.DB, char *string, bucket *string, key *string, hash *string, offset *int64, setempty bool) (int, error) {
	if db == nil {
		return -1, fmt.Errorf("Error DupeCheck db=nil")
	}
	if char == nil {
		return -1, fmt.Errorf("Error DupeCheck char=nil")
	}
	if bucket == nil {
		return -1, fmt.Errorf("Error DupeCheck char=%s bucket=nil", *char)
	}
	if key == nil {
		return -1, fmt.Errorf("Error DupeCheck char=%s bucket=%s key=nil", *char, *bucket)
	}
	if hash == nil {
		return -1, fmt.Errorf("Error DupeCheck char=%s bucket=%s key=%s hash=nil", *char, *bucket, *key)
	}
	if offset == nil {
		return -1, fmt.Errorf("Error DupeCheck char=%s bucket=%s key=%s hash=%s offset=nil", *char, *bucket, *key, *hash)
	}
	offsets, err := boltBucketGetOffsets(db, char, bucket, key)
	if err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets char=%s bucket=%s key=%s hash='%s' err='%v'", *char, *bucket, *key, *hash, err)
		return -1, err
	}
	// check go-cache for hash
	if his.Cache != nil {
		if val, found := his.Cache.Get(*hash); found {
			// cache hits
			switch val {
				case "-1":
					// pass
				case "1":
					return 1, nil
				case "2":
					return 2, nil
			}
		}
	}
	if offsets == nil { // no offsets stored for numhash
		if *offset == -1 {
			//isDup = false
			return 0, nil
		}
		newoffsets := []int64{*offset}
		// add hash to db
		if err := boltBucketKeyPutOffsets(db, char, bucket, key, &newoffsets, setempty); err != nil {
			log.Printf("ERROR HDBZW DupeCheck char=%s Add boltBucketKeyPutOffsets bucket=%s err='%v'", *char, *bucket, err)
			return -1, err
		}
		logf(DEBUG0, "HDBZW char=%s DupeCheck CREATED key=%s hash=%s offset=0x%08x=%d", *char, *key, *hash, *offset, *offset)
		if his.Cache != nil {
			his.Cache.Set(*hash, "1", 15) // offset of history entry added to key: hash is a duplicate in cached response now
		}
		return 0, nil
	}

	lo := len(*offsets)
	if lo == 0 {
		// error: 0 offsets stored at this key?!
		log.Printf("ERROR HDBZW char=%s DupeCheck NO OFFSETS bucket=%s key=%s hash=%s", *char, *bucket, *key, *hash)
		return -1, fmt.Errorf("NO OFFSETS bucket=%s key=%s hash=%s", *bucket, *key, *hash)
	}
	//if lo == 1 && *offset == -1 { // DoCheckHashDupOnly
	//	// only 1 offset stored at this key: return quick duplicate hit
	//	//isDup = true
	//	return true, nil
	//}
	if lo > 0 { // got offsets stored for numhash
		if lo > 1 {
			log.Printf("INFO HDBZW char=%s GOT key=%s hash='%s' multiple offsets=%d=%#v", *char, *key, *hash, lo, *offsets)
		}
		for _, check_offset := range *offsets {
			// check history for duplicate hash / evades collissions
			logf(DEBUG1, "HDBZW char=%s CHECK DUP key=%s lo=%d offset=%d", *char, *key, lo, check_offset)
			historyHash, err := his.FseekHistoryMessageHash(check_offset)
			if historyHash == nil && err == nil {
				log.Printf("ERROR HDBZW char=%s CHECK DUP bucket=%s historyHash=nil err=nil hash=%s", *char, *bucket, err, *hash)
				return -1, fmt.Errorf("ERROR historyHash=nil err=nil @offset=%d +offset=%d", *historyHash, check_offset, *offset)
			}
			if err != nil {
				log.Printf("ERROR HDBZW char=%s FseekHistoryMessageHash bucket=%s err='%v' offset=%d", *char, *bucket, err, check_offset)
				return -1, err
			}
			if historyHash != nil {
				if len(*historyHash) == 3 && *historyHash == eofhash {
					// The history file reached EOF for check_offset, which means the entry was not flushed. Retry later.
					if his.Cache != nil {
						his.Cache.Set(*hash, "2", 10)
					}
					return 2, nil
				} else if *historyHash == *hash {
					// hash is a duplicate in history
					logf(DEBUG1, "WARN HDBZW DUPLICATE historyHash=%s @offset=%d +offset=%d", *historyHash, check_offset, *offset)
					//isDup = true
					if his.Cache != nil {
						his.Cache.Set(*hash, "1", 15)
					}
					return 1, nil
				}
			}
		}
	}

	if *offset > 0 {
		if err := AppendOffset(offsets, offset); err != nil {
			return -1, err
		}
		if err := boltBucketKeyPutOffsets(db, char, bucket, key, offsets, setempty); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketKeyPutOffsets char=%s bucket=%s err='%v'", *char, *bucket, err)
			return -1, err
		}
		log.Printf("HDBZW char=%s APPENDED key=%s hash=%s offset=0x%08x=%d offsets=%d='%#v'", *char, *key, *hash, *offset, *offset, len(*offsets), *offsets)
		if his.Cache != nil {
			his.Cache.Set(*hash, "1", 15) // offset of history entry added to key: hash is a duplicate in cached response now
		}
	}
	return 0, nil
} // end func DupeCheck

func BoltSync(db *bolt.DB, char string) error {
	if db == nil {
		return fmt.Errorf("ERROR BoltSync db=nil")
	}
	log.Printf("BoltDB SYNC char=%s", char)
	// Manually sync the database to flush changes to disk
	if err := db.Sync(); err != nil {
		log.Printf("ERROR BoltSync char=%s db.Sync failed err='%v'", char, err)
		return err
	}
	return nil
} // end func BoltSync

func boltSyncClose(db *bolt.DB, char string) error {
	if db == nil {
		return fmt.Errorf("ERROR boltSyncClose db=nil")
	}
	if err := BoltSync(db, char); err != nil {
		return err
	}
	log.Printf("BoltDB Close char=%s", char)
	return db.Close()
} // end func boltSyncClose

func boltCreateBucket(db *bolt.DB, char *string, bucket *string) (retbool bool, err error) {
	if char == nil {
		return false, fmt.Errorf("Error boltCreateBucket char=nil")
	}
	if db == nil {
		return false, fmt.Errorf("Error boltCreateBucket char=%s db=nil", *char)
	}
	if bucket == nil {
		return false, fmt.Errorf("Error boltCreateBucket char=%s bucket=nil", *char)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		//_, err := tx.CreateBucketIfNotExists([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		if err != nil {
			return err
		}
		logf(DEBUG1, "OK boltCreateBucket char=%s buk='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s buk='%s' err='%v'", *char, *bucket, err)
		}
		return false, err
	}
	return
} // end func boltCreateBucket

/*
func boltBucketPutString(db *bolt.DB, char *string, bucket *string, key *string, val *string) (err error) {
	if char == nil {
		return fmt.Errorf("Error boltBucketPut char=nil")
	}
	if db == nil {
		return fmt.Errorf("Error boltBucketPut char=%s db=nil", *char)
	}
	if bucket == nil {
		return fmt.Errorf("Error boltBucketPut char=%s bucket=nil", char)
	}
	if key == nil {
		return fmt.Errorf("Error boltBucketPut char=%s bucket=%s key=nil", char, *bucket)
	}
	if val == nil {
		return fmt.Errorf("Error boltBucketPut char=%s bucket=%s val=nil", char, *bucket)
	}
	if err:= db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), []byte(*val))
		return err
	}); err != nil {
		log.Printf("Error boltBucketPut char=%s buk=%s key=%s val=%s err='%v'", *char, *bucket, *key, *val, err)
		return err
	}
	return
} // end func boltBucketPutString
*/

func AppendOffset(offsets *[]int64, offset *int64) error {
	if offsets == nil || offset == nil || *offset <= 0 {
		return fmt.Errorf("Error AppendOffset i/o=nil")
	}
	*offsets = append(*offsets, *offset)
	return nil
} // end func AppendOffset

func boltBucketKeyPutOffsets(db *bolt.DB, char *string, bucket *string, key *string, offsets *[]int64, setempty bool) (err error) {
	if char == nil {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=nil")
	}
	if db == nil {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s bucket=nil", char)
	}
	if key == nil || *key == "" {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s bucket=%s key=nil", char, *bucket)
	}
	if offsets == nil {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s bucket=%s offsets=nil", char, *bucket)
	}
	if !setempty && len(*offsets) == 0 {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s bucket=%s offsetsN=0 setempty=%t", char, *bucket, setempty)
	}
	if setempty && len(*offsets) != 0 {
		return fmt.Errorf("Error boltBucketKeyPutOffsets char=%s bucket=%s offsetsN=%d setempty=%t", char, *bucket, len(*offsets), setempty)
	}
	gobEncodedOffsets, err := gobEncodeOffsets(offsets)
	if err != nil {
		log.Printf("ERROR boltBucketKeyPutOffsets gobEncodedOffsets err='%v'", err)
		return err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), *gobEncodedOffsets)
		return err
	}); err != nil {
		log.Printf("Error boltBucketKeyPutOffsets char=%s buk=%s key=%s offsetsN=%d err='%v'", *char, *bucket, *key, len(*offsets), err)
		return err
	}
	return
} // end func boltBucketKeyPutOffsets

func boltBucketGetBytes(db *bolt.DB, char *string, bucket *string, key *string) (retval *[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("Error boltBucketGet char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("Error boltBucketGet char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("Error boltBucketGet char=%s bucket=nil", char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("Error boltBucketGet char=%s bucket=%s key=nil", char, *bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet char=%s buk=%s key=%s", *char, *bucket, *key)
			return nil
		}
		//log.Printf("GOT boltBucketGet char=%s buk=%s key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = &v
		return nil
	}); err != nil {
		log.Printf("Error boltBucketGet char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	return
} // end func boltBucketGetBytes

func boltBucketGetOffsets(db *bolt.DB, char *string, bucket *string, key *string) (offsets *[]int64, err error) {
	if char == nil {
		return nil, fmt.Errorf("Error boltBucketGetOffsets char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("Error boltBucketGetOffsets char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("Error boltBucketGetOffsets char=%s bucket=nil", char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("Error boltBucketGetOffsets char=%s bucket=%s key=nil", char, *bucket)
	}
	var gobEncodedOffsets []byte
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			logf(DEBUG1, "NOTFOUND boltBucketGetOffsets char=%s buk=%s key=%s", *char, *bucket, *key)
			return nil
		}
		logf(DEBUG1, "GOT boltBucketGetOffsets char=%s buk=%s key=%s bytes=%d", *char, *bucket, *key, len(v))
		gobEncodedOffsets = v
		return nil
	}); err != nil {
		log.Printf("Error boltBucketGetOffsets char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	if len(gobEncodedOffsets) > 0 {
		gobDecodeOffsets, err := gobDecodeOffsets(gobEncodedOffsets)
		if err != nil || gobDecodeOffsets == nil {
			log.Printf("Error boltBucketGetOffsets gobDecodeOffsets char=%s buk=%s key=%s err='%v'", *char, *bucket, *key, err)
			return nil, err
		}
		offsets = gobDecodeOffsets
	}
	if offsets != nil {
		logf(DEBUG1, "boltBucketGetOffsets returns char=%s buk=%s key=%s err='%v' offsetsN=%d", *char, *bucket, *key, err, len(*offsets))
	}
	return
} // end func boltBucketGet

func boltGetAllKeysVals(db *bolt.DB, char *string, bucket *string) (keyvals map[*string]*[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("Error boltGetAllKeysVals char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("Error boltGetAllKeysVals char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("Error boltGetAllKeysVals char=%s bucket=nil", char)
	}
	keyvals = make(map[*string]*[]byte)
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			key := string(k)
			keyvals[&key] = &v
			//*keys = append(*keys, &key)
			//*vals = append(*vals, &v)
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return
} // end func boltGetAllKeysVals

func boltGetAllKeys(db *bolt.DB, char *string, bucket *string) (retkeys *[]*string, err error) {
	if char == nil {
		return nil, fmt.Errorf("Error boltGetAllKeys char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("Error boltGetAllKeys char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("Error boltGetAllKeys char=%s bucket=nil", char)
	}
	var keys []*string
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(*bucket))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			//fmt.Printf("key=%s", k)
			key := string(k)
			keys = append(keys, &key)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	retkeys = &keys
	return
} // end func boltGetAllKeys

func isPrime(n int64) bool {
	// If the number is divisible by 2 or 3, it's not prime.
	if n%2 == 0 || n%3 == 0 {
		return false
	}

	// Check divisibility for numbers from 5 up to the square root of n.
	for i := int64(5); i*i <= n; i += 6 {
		if n%i == 0 || n%(i+2) == 0 {
			return false
		}
	}
	return true
}

func nextPrime(number int64) int64 {
	for {
		if isPrime(number) {
			log.Printf("%d is a prime.", number)
			return number
		} /* else {
			//log.Printf("%d is not a prime number.\n", number)
		}*/
		number++
	}
} // end func nextPrime

func FNV32(data *string) (*string, *uint32) {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV32

func FNV64(data *string) (*string, *uint64) {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV64

func FNV32S(data *string) *string {
	hash := fnv.New32()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32S

func FNV32aS(data *string) *string {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32aS

func FNV64S(data *string) *string {
	hash := fnv.New64()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64S

func FNV64aS(data *string) *string {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64aS

func gobEncodeOffsets(offsets *[]int64) (*[]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(offsets)
	if err != nil {
		return nil, err
	}
	encodedData := buf.Bytes()
	return &encodedData, nil
} // end func gobEncodeOffsets

func gobEncodeHeader(settings *HistorySettings) (*[]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		return nil, err
	}
	encodedData := buf.Bytes()
	return &encodedData, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData []byte) (*HistorySettings, error) {
	buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	settings := &HistorySettings{}
	err := decoder.Decode(&settings)
	if err != nil {
		return nil, err
	}
	return settings, nil
} // end func gobDecodeHeader

func gobDecodeOffsets(encodedData []byte) (*[]int64, error) {
	buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	var decodedOffsets []int64
	err := decoder.Decode(&decodedOffsets)
	if err != nil {
		return nil, err
	}
	return &decodedOffsets, nil
} // end func gobDecodeOffsets

func (his *HISTORY) returnBoltHashOpen() {
	<-BoltHashOpen
}

func (his *HISTORY) setBoltHashOpen() {
	BoltHashOpen <- struct{}{}
}

func (his *HISTORY) GetBoltHashOpen() int {
	return len(BoltHashOpen)
}
