package history

import (
	//"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	//"github.com/go-while/go-utils"
	//"github.com/edsrzf/mmap-go"
	"hash/fnv"
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
	boltChanInit      chan struct{}
	BoltHashOpen      = make(chan struct{}, boltDBs) // dont change this
)

type HistoryAddIndex struct {
	Hash         *string
	Offset       int64
	IndexRetChan chan bool
}

func (his *HISTORY) History_DBZinit(boltOpts *bolt.Options) {
	log.Printf("his.History_DBZinit()")
	if boltChanInit != nil {
		log.Printf("ERROR History_DBZinit already loaded")
		return
	}
	for i, char := range HEXCHARS {
		his.IndexChans[i] = make(chan *HistoryAddIndex, 2)
		his.charsMap[char] = i
		go his.History_DBZ_Worker(char, i, his.IndexChans[i], boltOpts)
	}
	boltChanInit = make(chan struct{}, BoltCheckParallel)
	go his.History_DBZ()
	time.Sleep(time.Millisecond)
} // end func History_DBZinit

func (his *HISTORY) History_DBZ() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "History_DBZ")
	log.Printf("OK History_DBZ")
forever:
	for {
		select {
		case hi, ok := <-his.IndexChan: // recevies a HistoryAddIndex struct
			if !ok || hi == nil || hi.Hash == nil {
				// receiving a nil object stops history_dbz
				for _, achan := range his.IndexChans {
					// passing nils to IndexChans will stop them too
					achan <- nil
				}
				break forever
			}
			C1 := string(string(*hi.Hash)[0]) // gets first char of hash
			if C1 != "" {
				//i := his.charsMap[C1]
				if his.IndexChans[his.charsMap[C1]] != nil {
					his.IndexChans[his.charsMap[C1]] <- hi // sends object to hash History_DBZ_Worker char
				}
			}
		} // end select
	} // end for
} // end func History_DBZ

func returnBoltHashOpen() {
	<-BoltHashOpen
}

func setBoltHashOpen() {
	BoltHashOpen <- struct{}{}
}

func GetBoltHashOpen() int {
	return len(BoltHashOpen)
}

func (his *HISTORY) History_DBZ_Worker(char string, i int, indexchan chan *HistoryAddIndex, boltOpts *bolt.Options) {
	if !LOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char) {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK16, "History_DBZ_Worker "+char)
	dbpath := his.HF_hash + "." + char
	if boltOpts == nil {
		defboltOpts := bolt.Options{
			//ReadOnly: true,
			Timeout:         9 * time.Second,
			InitialMmapSize: 1024 * 1024,
			PageSize:        4 * 1024,
			NoSync:          true,
		}
		boltOpts = &defboltOpts
	}
	db, err := bolt.Open(dbpath, 0600, boltOpts)
	if err != nil {
		log.Printf("Error HashDB dbpath='%s' err='%v'", dbpath, err)
		return
	}
	defer db.Close()
	if err != nil {
		log.Fatal(err)
	}
	defer boltSyncClose(db, char)
	//defer log.Printf("Stopped HDBZW=(%02d) char=%s ", i, char)
	testkey := "1"
	testoffsets := []int64{1}
	tcheck := 4096
	checked := 0
	created := 0
	setempty := false
	initLongTest := false
	boltChanInit <- struct{}{}
	log.Printf("HDBZW: INIT HashDB char=%s", char)
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
						log.Printf("ERROR HDBZW INIT HashDB boltBucketPutOffsets1 char=%s bucket=%s err='%v' retbool=%t", char, bucket, err, retbool)
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
						log.Printf("ERROR HDBZW INIT HashDB boltBucketPutOffsets2 char=%s bucket=%s err='%v'", char, bucket, err)
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
	<-boltChanInit
	//log.Printf("HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tcheck, created, tcheck)
	if checked != 4096 {
		log.Printf("ERROR HDBZW char=%s checked %d/%d created=%d/%d", char, checked, tcheck, created, tcheck)
		return
	}
	setBoltHashOpen()
	defer returnBoltHashOpen()
	lastsync := utils.UnixTimeSec()
	added, total, processed := 0, uint64(0), uint64(0)
	useShorthash := false // can not be changed once db has been created!
forever:
	for {
		select {
		case hi, ok := <-indexchan: // recevies a HistoryAddIndex struct
			if !ok || hi == nil || hi.Hash == nil || len(*hi.Hash) < 32 { // at least md5
				// receiving a nil object stops history_dbz_worker
				break forever
			}
			hash := hi.Hash
			if useShorthash {
				ahash := string(string(*hi.Hash)[4:]) // shorterkey
				hash = &ahash
			}
			var key *string
			switch HashType {
			case HashFNV32:
				key = FNV32S(hash)
			case HashFNV32a:
				key = FNV32aS(hash)
			case HashFNV64:
				key = FNV64S(hash)
			case HashFNV64a:
				key = FNV64aS(hash)
			default:
				key = FNV32S(hash)
			}
			bucket := string(string(*hi.Hash)[1:4])
			//log.Printf("WORKER HDBZW char=%s hash=%s bucket=%s akey=%s numhash=%s @0x%010x|%d|%s", char, *hi.Hash, bucket, akey, numhash, hi.Offset, hi.Offset, hexoffset)
			isDup, err := his.DupeCheck(db, &char, &bucket, key, hi.Hash, &hi.Offset, false, true)
			if err != nil {
				log.Printf("ERROR HDBZW char=%s DupeCheck bucket=%s key=%s hash='%s' err='%v'", char, bucket, *key, *hi.Hash, err)
				if hi.IndexRetChan != nil {
					close(hi.IndexRetChan)
				}
				//break forever
				continue forever
			}
			if hi.IndexRetChan != nil {
				hi.IndexRetChan <- isDup
			}
			if !isDup {
				added++
				total++
				processed++
			}
			if added > 0 && lastsync <= utils.UnixTimeSec()-Bolt_SYNC_EVERY {
				if err := BoltSync(db, char); err != nil {
					break forever
				}
				added, lastsync = 0, utils.UnixTimeSec()
			}
		} // end select
	} // end for
} // end func History_DBZ_Worker

/*
func (his *HISTORY) DupeCheck(db *bolt.DB, char *string, bucket *string, key *string, hi *HistoryAddIndex, setempty bool) (err error) {
	err := DupeCheck(db, char, bucket, key, hi, setempty)
	if err != nil {
		return err
	}
	AppendOffset(offsets, &his.Offset)
	if err := boltBucketKeyPutOffsets(db, char, bucket, key, offsets, setempty); err != nil {
		log.Printf("ERROR HDBZW APPEND boltBucketPutOffsets char=%s bucket=%s err='%v'", *char, *bucket, err)
		return err
	}
	log.Printf("HDBZW char=%s APPENDED key=%s hash=%s offset=0x%08x=%d offsets=%d", *char, *key, *hi.Hash, hi.Offset, hi.Offset, len(*offsets))
	return
}
*/

func (his *HISTORY) DupeCheck(db *bolt.DB, char *string, bucket *string, key *string, hash *string, offset *int64, setempty bool, add bool) (isDup bool, err error) {
	offsets, err := boltBucketGetOffsets(db, char, bucket, key)
	if err != nil {
		log.Printf("ERROR HDBZW DupeCheck boltBucketGetOffsets char=%s bucket=%s key=%s hash='%s' err='%v'", *char, *bucket, *key, *hash, err)
		return
	}
	if offsets == nil { // no offsets stored for numhash
		newoffsets := []int64{*offset}
		// add hash to db
		if err := boltBucketKeyPutOffsets(db, char, bucket, key, &newoffsets, setempty); err != nil {
			log.Printf("ERROR HDBZW DupeCheck char=%s Add boltBucketPutOffsets bucket=%s err='%v'", *char, *bucket, err)
			return false, err
		}
		logf(DEBUG0, "HDBZW char=%s DupeCheck CREATED key=%s hash=%s offset=0x%08x=%d", *char, *key, *hash, *offset, *offset)
		return
	}

	lo := len(*offsets)
	if lo == 0 {
		// error: 0 offsets stored at this key?!
		log.Printf("ERROR HDBZW char=%s DupeCheck NO OFFSETS bucket=%s key=%s hash=%s", *char, *bucket, *key, *hash)
		return false, fmt.Errorf("NO OFFSETS bucket=%s key=%s hash=%s", *bucket, *key, *hash)
	}

	//log.Printf("HDBZW char=%s GOT key=%s offsets=%d", *char, *key, lo)
	//if lo == 1 {
	//	// only 1 offset stored at this key: return quick duplicate hit
	//	return true, nil
	//}

	// got offsets stored for numhash
	if lo > 1 {
		log.Printf("HDBZW char=%s GOT key=%s hash='%s' offsets=%d", *char, *key, *hash, lo)
	}
	//check_offsets: // LABEL for loop
	for _, offset := range *offsets {
		// check history for duplicate hash / evades collissions
		logf(DEBUG1, "HDBZW char=%s CHECK DUP key=%s lo=%d offset=%d", *char, *key, lo, offset)
		historyHash, err := his.FseekHistoryMessageHash(offset)
		if err != nil {
			log.Printf("ERROR HDBZW char=%s FseekHistoryMessageHash bucket=%s err='%v' offset=%d", *char, *bucket, err, offset)
			return false, err
		}
		if historyHash != nil {
			if *historyHash == *hash {
				// hash is a duplicate in history
				logf(DEBUG1, "WARN HDBZW DUPLICATE historyHash=%s @offset=%d +his.Offset=%d", *historyHash, offset, his.Offset)
				return true, nil
			}
		}
		if historyHash == nil && err == nil {
			log.Printf("ERROR HDBZW char=%s CHECK DUP bucket=%s historyHash=nil err=nil hash=%s", *char, *bucket, err, *hash)
			return false, fmt.Errorf("ERROR historyHash=nil err=nil @offset=%d +his.Offset=%d", *historyHash, offset, his.Offset)
		}
	}

	if add {
		AppendOffset(offsets, &his.Offset)
		if err := boltBucketKeyPutOffsets(db, char, bucket, key, offsets, setempty); err != nil {
			log.Printf("ERROR HDBZW APPEND boltBucketPutOffsets char=%s bucket=%s err='%v'", *char, *bucket, err)
			return false, err
		}
		log.Printf("HDBZW char=%s APPENDED key=%s hash=%s offset=0x%08x=%d offsets=%d", *char, *key, *hash, *offset, *offset, len(*offsets))
	}
	return
} // end func DupeCheck

func BoltSync(db *bolt.DB, char string) error {
	log.Printf("BoltDB SYNC char=%s", char)
	// Manually sync the database to flush changes to disk
	if err := db.Sync(); err != nil {
		log.Printf("ERROR BoltSync char=%s db.Sync failed err='%v'", char, err)
		return err
	}
	return nil
} // end func BoltSync

func boltSyncClose(db *bolt.DB, char string) error {
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
} // end func boltBucketPut
*/

func AppendOffset(offsets *[]int64, offset *int64) {
	*offsets = append(*offsets, *offset)
}

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
	gobEncodedOffsets := gobEncodeOffsets(offsets)
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), *gobEncodedOffsets)
		return err
	}); err != nil {
		log.Printf("Error boltBucketKeyPutOffsets char=%s buk=%s key=%s offsetsN=%d err='%v'", *char, *bucket, *key, len(*offsets), err)
		return err
	}
	return
} // end func boltBucketPutOffsets

/*
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
*/

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

func gobEncodeOffsets(offsets *[]int64) *[]byte {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(offsets)
	if err != nil {
		log.Fatal(err)
	}
	encodedData := buf.Bytes()
	return &encodedData
} // end func gobEncodeOffsets

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