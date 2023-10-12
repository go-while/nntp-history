package history

import (
	"bufio"
	"fmt"
	//"github.com/go-while/go-utils"
	//"golang.org/x/exp/mmap"
	"github.com/edsrzf/mmap-go"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"strings"
)

func (his *HISTORY) ReplayHistoryDat(quickreplay bool, testmax int) {
	if !his.useHashDB || his.hisDat == "" {
		log.Printf("return ReplayHistoryDat !his.useHashDB || his.hisDat empty")
		return
	}
	if testmax < 1 {
		testmax = 1
	}
	his.wait4HashDB()
	indexRetChan := make(chan int, 1)
	// oldestFirst=true checks up to `testmax` lines from oldest to latest.
	// oldestFirst=false checks from latest and stops if latest (first) hash is in DB.
	oldestFirst := true
	if quickreplay {
		oldestFirst = false
		testmax = QuickReplayTestMax
	}
	lines, err := readLastNLinesFromFile(his.hisDat, testmax, oldestFirst)
	lenlines := len(lines)
	log.Printf("ReplayHistoryDat quickreplay=%t testmax=%d lines=%d ", quickreplay, testmax, lenlines)
	if err != nil {
		log.Printf("ERROR ReplayHistoryDat err='%#v'", err)
		os.Exit(1)
	}
	if lenlines <= 1 { // nothing or header only
		return
	}
	ok := 0
replay:
	for i, line := range lines {
		hash := extractHash(line)
		if hash == "" {
			log.Printf("ERROR ReplayHistoryDat i=%d hash empty line='%s'", i, line)
			os.Exit(1)
		}
		logf(DEBUG, "ReplayHistoryDat: replay hash='%s'", hash)
		isDup, err := his.IndexQuery(&hash, indexRetChan, -1)
		if err != nil {
			log.Printf("ERROR ReplayHistoryDat IndexQuery hash='%s' err='%v'", hash, err)
			os.Exit(1)
		}
		switch isDup {
		case CaseDupes:
			ok++
			if !oldestFirst && ok == testmax {
				break replay
			}
		default:
			log.Printf("Error ReplayHistoryDat bad response from IndexQuery isDup=%x", isDup)
			os.Exit(1)
		}
	}
	log.Printf("ReplayHistoryDat ok=%d/%d testmax=%d quickreplay=%t oldestFirst=%t", ok, lenlines, testmax, quickreplay, oldestFirst)
} // end func ReplayHistoryDat

func extractHash(line string) string {
	parts := strings.Split(line, "\t") // Split the line at the first tab character
	if len(parts) < 3 || len(parts[0]) < 32+2 || parts[0][0] != '{' || parts[0][len(parts[0])-1] != '}' {
		return ""
	}
	hash := string(parts[0][1 : len(parts[0])-1])
	return hash
} // end func extractHash

func (his *HISTORY) RebuildHashDB() error {
	if !his.useHashDB || his.hisDat == "" {
		log.Printf("return RebuildHashDB !his.useHashDB || his.hisDat empty")
		return nil
	}
	// Open the history.dat file for reading
	file, err := os.Open(his.hisDat)
	if err != nil {
		log.Printf("Error opening file:", err)
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("ERROR RebuildHashDB fh open Stat err='%v'", err)
		os.Exit(1)
	}
	filesize := fileInfo.Size()
	// assume 102 bytes per line
	estimate := filesize / 102
	log.Printf("RebuildHashDB: his.hisDat='%s' filesize=%d estimate=%d", his.hisDat, filesize, estimate)
	var offset, added, passed, skipped, dupes, retry, did, total int64
	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	indexRetChan := make(chan int, 1)
	for scanner.Scan() {
		line := scanner.Text()
		ll := len(line) + 1 // +1 accounts for LF
		/*
			parts := strings.Split(line, "\t") // Split the line at the first tab character
			if len(parts) < 3 || len(parts[0]) < 32+2 || parts[0][0] != '{' || parts[0][len(parts[0])-1] != '}' {
				// Skip lines that don't have correct fields or not { } character in first
				if offset > 0 { // ignores header line @ offset 0
					log.Printf("skipped: line @offset=%d ll=%d", offset, ll)
					skipped++
				}
				offset += int64(ll)
				continue
			}
			hash := string(parts[0][1 : len(parts[0])-1])
		*/
		hash := extractHash(line)
		if len(hash) < 32 { // at least md5
			if offset > 0 { // ignores header line @ offset 0
				log.Printf("skipped: line @offset=%d ll=%d", offset, ll)
				skipped++
			}
			continue
		}
		//log.Printf("RebuildHashDB hash='%s' @offset=%d", hash, offset)
		// pass hash:offset to IndexChan

		isDup, err := his.IndexQuery(&hash, indexRetChan, offset)
		if err != nil {
			log.Printf("ERROR RebuildHashDB IndexQuery hash='%s' err='%v'", hash, err)
			return err
		}
		switch isDup {
		case CasePass:
			passed++
		case CaseAdded:
			added++
		case CaseDupes:
			dupes++
		case CaseRetry:
			retry++
		default:
			log.Printf("main: ERROR in RebuildHashDB response from IndexQuery unknown switch isDup=%x", isDup)
			os.Exit(1)
		}
		offset += int64(ll)

		if did >= 100000 {
			perc1 := int(float64(offset) / float64(filesize) * 100)
			perc2 := int(float64(total) / float64(estimate) * 100)
			log.Printf("RebuildHashDB did=%d offset=%d (%d%%) estimate=%d%%", total, offset, perc1, perc2)
			did = 0
		}
		did++
		total++
	}
	log.Printf("RebuildHashDB: his.hisDat='%s' added=%d passed=%d skipped=%d dupes=%d retry=%d", his.hisDat, added, passed, skipped, dupes, retry)
	return err
} // end func RebuildHashDB

func boltCreateBucket(db *bolt.DB, char *string, bucket *string) (retbool bool, err error) {
	if char == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=nil")
	}
	if db == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s db=nil", *char)
	}
	if bucket == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s bucket=nil", *char)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		//_, err := tx.CreateBucketIfNotExists([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		if err != nil {
			return err
		}
		//logf(DEBUG2, "OK boltCreateBucket char=%s bucket='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s bucket='%s' err='%v'", *char, *bucket, err)
		}
		return false, err
	}
	return
} // end func boltCreateBucket

func boltBucketPutString(db *bolt.DB, char *string, bucket *string, key *string, val *string) (err error) {
	if char == nil {
		return fmt.Errorf("ERROR boltBucketPut char=nil")
	}
	if db == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s db=nil", *char)
	}
	if bucket == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=nil", *char)
	}
	if key == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] key=nil", *char, *bucket)
	}
	if val == nil {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] val=nil", *char, *bucket)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		err := b.Put([]byte(*key), []byte(*val))
		return err
	}); err != nil {
		log.Printf("ERROR boltBucketPut [%s|%s] key=%s val=%s err='%v'", *char, *bucket, *key, *val, err)
		return err
	}
	return
} // end func boltBucketPutString

func boltBucketGetBytes(db *bolt.DB, char *string, bucket *string, key *string) (retval *[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s bucket=nil", *char)
	}
	if key == nil || *key == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet [%s|%s] key=nil", *char, *bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(*bucket))
		v := b.Get([]byte(*key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet [%s|%s] key=%s", *char, *bucket, *key)
			return nil
		}
		//log.Printf("GOT boltBucketGet [%s|%s] key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = &v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGet [%s|%s] key=%s err='%v'", *char, *bucket, *key, err)
		return nil, err
	}
	return
} // end func boltBucketGetBytes

func boltGetAllKeysVals(db *bolt.DB, char *string, bucket *string) (keyvals map[*string]*[]byte, err error) {
	if char == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s bucket=nil", char)
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
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=nil")
	}
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s db=nil", *char)
	}
	if bucket == nil || *bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeys char=%s bucket=nil", char)
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

func readLastNLinesFromFile(filePath string, n int, oldestFirst bool) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	mmappedData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer mmappedData.Unmap()

	lines := make([]string, 0, n)
	lineStart := int64(size - 1)
	lineEnd := lineStart
	log.Printf("readLastNLinesFromFile file='%s' size=%d ls=%d le=%d", filePath, size, lineStart, lineEnd)
	if mmappedData[size-1] != '\n' {
		log.Printf("ERROR readLastNLinesFromFile: EOF != '\n' file='%s'", filePath)
		os.Exit(1)
	}
	startindex := size - 2
	var skipped int64
	for i := startindex; i >= 0; i-- { // scan backwards
		if mmappedData[i] == '\n' {
			//log.Printf("mmappedData ls=%d le=%d i=%d", lineStart, lineEnd, i)
			line := string(mmappedData[lineStart:lineEnd])
			if line == "" {
				log.Printf("extracted line='%s' ls=%d le=%d i=%d skipped=%d", line, lineStart, lineEnd, i, skipped)
				os.Exit(1)
			}
			lines = append(lines, line)
			if len(lines) >= n {
				break
			}
			lineEnd -= skipped
			skipped = 0
		} else {
			skipped++
		}
		lineStart--
	}
	if oldestFirst {
		// Reverse the order of lines. if reversed: start check from oldest else will check from latest.
		reverseStrings(lines)
	}
	return lines, nil
} // end func readLastNLinesFromFile

func reverseStrings(lines []string) {
	for i := 0; i < len(lines)/2; i++ {
		j := len(lines) - i - 1
		lines[i], lines[j] = lines[j], lines[i]
	}
}
