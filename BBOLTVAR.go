package history

import (
	"arena"
	"bufio"
	// "google.golang.org/protobuf/encoding/prototext"
	// "google.golang.org/protobuf/encoding/protowire"
	//"bytes"
	"fmt"
	//"github.com/go-while/go-utils"
	//"golang.org/x/exp/mmap"
	"github.com/edsrzf/mmap-go"
	bolt "go.etcd.io/bbolt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	ForcedReplay bool
)

type AHASH struct {
	hash           string
	buffer         []byte
	missing_hashes []string
	missingoffsets map[string]int64
	//mmappedData    mmap.MMap
}

func reverseBytes(data []byte) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

func (his *HISTORY) ReplayHisDat() {
	var bufferSize int64 = 102
	mem := arena.NewArena()
	//defer mem.Free()
	memhash := arena.New[AHASH](mem)
	memhash.buffer = make([]byte, bufferSize)

	if NoReplayHisDat {
		return
	}
	if !his.useHashDB {
		return
	}
	if his.hisDat == "" {
		log.Printf("return ReplayHisDat his.hisDat empty")
		os.Exit(1)
	}
	his.Wait4HashDB()
	indexRetChan := make(chan int, 1)

	file, err := os.Open(his.hisDat)
	if err != nil {
		log.Printf("ERROR ReplayHisDat os.Open err='%v'", err)
		os.Exit(1)
	}
	//defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		log.Printf("ERROR ReplayHisDat file.Stat err='%v'", err)
		os.Exit(1)
	}
	size := stat.Size()
	mmappedData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		log.Printf("ERROR ReplayHisDat mmap.Map err='%v'", err)
		os.Exit(1)
	}
	//defer memhash.mmappedData.Unmap()
	lineStart := int64(size - 1)
	lineEnd := lineStart
	log.Printf("ReplayHisDat file='%s' size=%d ls=%d:le=%d", his.hisDat, size, lineStart, lineEnd)
	if mmappedData[size-1] != '\n' {
		log.Printf("ERROR ReplayHisDat: EOF != '\n' file='%s'", his.hisDat)
		os.Exit(1)
	}
	replaytestmax := ReplayDistance
	if int64(BoltDB_MaxBatchSize) > 0 && int64(BoltDB_MaxBatchSize*2) > ReplayDistance {
		replaytestmax = int64(BoltDB_MaxBatchSize * 2)
	}
	if replaytestmax < DefaultReplayDistance {
		replaytestmax = DefaultReplayDistance
	}
	var checked, ok, missed, skippedBytes, field1End, nlm int64
	startindex := size - 2
	memhash.missing_hashes = []string{}
	memhash.missingoffsets = make(map[string]int64)
replay: // backwards: from latest hash
	for offset := startindex; offset >= 0; offset-- { // scan hisDat backwards
		skippedBytes++
		//logf(DEBUG, "A skippedBytes=%03d @offset=%d buffer=%d='%s'", skippedBytes, offset+1, len(memhash.buffer), string(memhash.buffer))
		if mmappedData[offset] != '\n' {
			lineStart--
			continue replay
		}
		if skippedBytes != bufferSize {
			log.Printf("ERROR ReplayHisDat @offset=%d skipped=%d len(buffer)=%d/%d ls=%d:le=%d", offset+1, skippedBytes, len(memhash.buffer), bufferSize, lineStart, lineEnd)
			os.Exit(1)
		}
		//line := mmappedData[lineStart:lineEnd]
		//memhash.buffer = append(memhash.buffer, line)
		copy(memhash.buffer, mmappedData[lineStart:lineEnd])
		if memhash.buffer[0] != '{' {
			log.Printf("ERROR ReplayHisDat buf[0]!='}' @offset=%d skipped=%d len(buffer)=%d/%d ls=%d:le=%d", offset+1, skippedBytes, len(memhash.buffer), bufferSize, lineStart, lineEnd)
			os.Exit(1)
		}
	getFirstField:
		for i, c := range memhash.buffer {
			if c != '\t' {
				continue getFirstField
			}
			//logf(DEBUG2, "ReplayHisDat getFirstField: i=%d hash", i)
			field1End = int64(i) - 1 // accounts for tab
			break
		}
		if memhash.buffer[field1End] != '}' {
			log.Printf("ERROR ReplayHisDat buf[f1e=%d]!='}'='%s' @offset=%d skipped=%d len(buffer)=%d/%d='%s' ls=%d:le=%d", field1End, string(memhash.buffer[field1End]), offset+1, skippedBytes, len(memhash.buffer), bufferSize, string(memhash.buffer), lineStart, lineEnd)
			os.Exit(1)
		}

		switch len(memhash.buffer[1:field1End]) {
		case 64:
			// sha256 pass
		case 40:
			// sha1 pass
		case 32:
			// md5 pass
		case 128:
			// sha512 pass
		default:
			log.Printf("ERROR ReplayHisDat BAD HASHLEN @offset=%d buf='%s' ls=%d:le=%d", offset+1, string(memhash.buffer), lineStart, lineEnd)
			os.Exit(1)
		}
		checked++ // only if line has valid format in first field
		memhash.hash = string(memhash.buffer[1:field1End])
		//logf(DEBUG2, "ReplayHisDat B skippedBytes=%03d field1End=%d hash='%s'@offset=%d",
		//	skippedBytes, field1End, memhash.hash, offset+1)
		isDup, err := his.IndexQuery(memhash.hash, indexRetChan, -1)
		if err != nil {
			log.Printf("ERROR ReplayHisDat IndexQuery hash='%s' @offset=%d err='%v'", memhash.hash, offset, err)
			os.Exit(1)
		}
		switch isDup {
		case CasePass:
			// hash from history.dat is missing in hashdb
			missed++
			// whenever we hit a missing hash
			//  update nlm and add value of `ok` to it
			// adding `ok` to it resets the distance calculation
			nlm = missed + ok // + checked // adding checked to nlm extends the replay a lot
			memhash.missing_hashes = append(memhash.missing_hashes, memhash.hash)
			memhash.missingoffsets[memhash.hash] = offset
			distance := ok - nlm
			//log.Printf("WARN ReplayHisDat NOT!FOUND hash='%s' @offset=%d (checked=%d missed=%d ok=%d nlm=%d dist=%d/%d)", memhash, offset, checked, missed, ok, nlm, distance, replaytestmax)
			log.Printf("WARN ReplayHisDat NOT!FOUND hash='%s' @offset=%d (checked=%d missed=%d ok=%d nlm=%d dist=%d/%d)",
				memhash.hash, offset+1, checked, missed, ok, nlm, distance, replaytestmax)
			/*
				if DEBUG2 {
					if missed < 10 {
						time.Sleep(time.Second / 100)
					} else if missed == 10 {
						time.Sleep(time.Second * 3)
					}
				}
			*/

		case CaseDupes:
			ok++
			distance := ok - nlm
			if distance > replaytestmax {
				break replay
			}
			//log.Printf("INFO ReplayHisDat CaseDupes hash='%s' @offset=%d (checked=%d missed=%d ok=%d nlm=%d dist=%d/%d)", memhash, offset, checked, missed, ok, nlm, distance, replaytestmax)
			log.Printf("INFO ReplayHisDat CaseDupes @offset=%d (checked=%d missed=%d ok=%d nlm=%d dist=%d/%d)", offset+1, checked, missed, ok, nlm, distance, replaytestmax)
			/*
				if DEBUG2 {
					if ok < 10 {
						time.Sleep(time.Second / 100)
					} else if ok == 10 {
						time.Sleep(time.Second * 5)
					}
				}
			*/

		default:
			log.Printf("ERROR ReplayHisDat bad response from IndexQuery isDup=%x", isDup)
			os.Exit(1)
		} // end switch isDup
		lineStart--
		lineEnd -= skippedBytes
		skippedBytes = 0
		clear(memhash.buffer)
	} // end for replay
	log.Printf("LOOPEND ReplayHisDat checked=%d ok=%d missed=%d", checked, ok, missed)

	if len(memhash.missing_hashes) > 0 || ForcedReplay {
		log.Printf("WARN ReplayHisDat missing=%d", len(memhash.missing_hashes))
		// missing is ordered from latest backward
		// reverse order to have oldestFirst
		reverseStrings(memhash.missing_hashes)
		mmappedData.Unmap()
		mem.Free()
		//mem.Dispose()
		//mem = nil
		file.Close()
		time.Sleep(1 * time.Second)
		runtime.GC()
		//CPUBURN()
		return
	}
	return
	/*
	 * TODO loop over missing and pass hash to indexQuery with offset
	 *
	 */

	added := 0
	for _, hash := range memhash.missing_hashes {
		offset := memhash.missingoffsets[hash]
		isDup, err := his.IndexQuery(hash, indexRetChan, offset)
		if err != nil {
			log.Printf("ERROR ReplayHisDat ADD IndexQuery hash='%s' @offset=%d err='%v'", hash, offset, err)
			os.Exit(1)
		}
		switch isDup {
		case CasePass:
			// hash from history.dat is missing in hashdb
			added++
			log.Printf("INFO ReplayHisDat added missing hash='%s' @offset=%d added=%d/%d", hash, offset, added, missed)
		default:
			log.Printf("ERROR ReplayHisDat bad response from ADD hash='%s' to IndexQuery isDup=%x", hash, isDup)
			os.Exit(1)
		}
	}

	/*
			oldestFirst := true
			if quickreplay {
				oldestFirst = false
				testmax = QuickReplayTestMax
			}
			lines, err := readLastNLinesFromFile(his.hisDat, testmax, oldestFirst)
			lenlines := len(lines)
			if lenlines == 0 {
				return
			}
			log.Printf("ReplayHisDat quickreplay=%t testmax=%d lines=%d ", quickreplay, testmax, lenlines)
			if err != nil {
				log.Printf("ERROR ReplayHisDat err='%#v'", err)
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
					log.Printf("ERROR ReplayHisDat i=%d hash empty line='%s'", i, line)
					os.Exit(1)
				}
				logf(DEBUG, "ReplayHisDat: replay hash='%s' i=%d", hash, i)
				isDup, err := his.IndexQuery(&hash, indexRetChan, -1)
				if err != nil {
					log.Printf("ERROR ReplayHisDat IndexQuery hash='%s' err='%v'", hash, err)
					os.Exit(1)
				}
				switch isDup {
				case CaseDupes:
					ok++
					if !oldestFirst && ok == testmax {
						break replay
					}
				default:
					log.Printf("Error ReplayHisDat bad response from IndexQuery isDup=%x", isDup)
					os.Exit(1)
				}
			}
	*/
	log.Printf("ReplayHisDat ok=%d", ok)
} // end func ReplayHisDat

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
	var offset, added, passed, skippedLines, dupes, retry, did, total int64
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
					skippedLines++
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
				skippedLines++
			}
			continue
		}
		//log.Printf("RebuildHashDB hash='%s' @offset=%d", hash, offset)
		// pass hash:offset to IndexChan

		isDup, err := his.IndexQuery(hash, indexRetChan, offset)
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
	log.Printf("RebuildHashDB: his.hisDat='%s' added=%d passed=%d skippedLines=%d dupes=%d retry=%d", his.hisDat, added, passed, skippedLines, dupes, retry)
	return err
} // end func RebuildHashDB

func boltCreateBucket(db *bolt.DB, char string, bucket string) (retbool bool, err error) {
	if db == nil {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s db=nil", char)
	}
	if char == "" {
		return false, fmt.Errorf("ERROR boltCreateBucket char=nil")
	}
	if bucket == "" {
		return false, fmt.Errorf("ERROR boltCreateBucket char=%s bucket=nil", char)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(bucket)) // _ == bb == *bbolt.Bucket
		//_, err := tx.CreateBucketIfNotExists([]byte(*bucket)) // _ == bb == *bbolt.Bucket
		if err != nil {
			return err
		}
		//logf(DEBUG2, "OK boltCreateBucket char=%s bucket='%s'", *char, *bucket)
		retbool = true
		return nil
	}); err != nil {
		if err != bolt.ErrBucketExists {
			log.Printf("ERROR boltCreateBucket char=%s bucket='%s' err='%v'", char, bucket, err)
		}
		return false, err
	}
	return
} // end func boltCreateBucket

func boltBucketPutString(db *bolt.DB, char string, bucket string, key string, val string) (err error) {
	if db == nil {
		return fmt.Errorf("ERROR boltBucketPut char=%s db=nil", char)
	}
	if char == "" {
		return fmt.Errorf("ERROR boltBucketPut char=nil")
	}
	if bucket == "" {
		return fmt.Errorf("ERROR boltBucketPut char=%s bucket=nil", char)
	}
	if key == "" {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] key=nil", char, bucket)
	}
	if val == "" {
		return fmt.Errorf("ERROR boltBucketPut [%s|%s] val=nil", char, bucket)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		err := b.Put([]byte(key), []byte(val))
		return err
	}); err != nil {
		log.Printf("ERROR boltBucketPut [%s|%s] key=%s val=%s err='%v'", char, bucket, key, val, err)
		return err
	}
	return
} // end func boltBucketPutString

func boltBucketGetBytes(db *bolt.DB, char string, bucket string, key string) (retval []byte, err error) {
	if db == nil {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s db=nil", char)
	}
	if char == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet char=nil")
	}
	if bucket == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet char=%s bucket=nil", char)
	}
	if key == "" {
		return nil, fmt.Errorf("ERROR boltBucketGet [%s|%s] key=nil", char, bucket)
	}
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(key))
		if v == nil {
			log.Printf("NOTFOUND boltBucketGet [%s|%s] key=%s", char, bucket, key)
			return nil
		}
		//log.Printf("GOT boltBucketGet [%s|%s] key=%s val='%s'", *char, *bucket, *key, string(v))
		retval = v
		return nil
	}); err != nil {
		log.Printf("ERROR boltBucketGet [%s|%s] key=%s err='%v'", char, bucket, key, err)
		return nil, err
	}
	return
} // end func boltBucketGetBytes

func boltGetAllKeysVals(db *bolt.DB, char string, bucket string) (keyvals map[string][]byte, err error) {
	if db == nil {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s db=nil", char)
	}
	if char == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=nil")
	}
	if bucket == "" {
		return nil, fmt.Errorf("ERROR boltGetAllKeysVals char=%s bucket=nil", char)
	}
	keyvals = make(map[string][]byte)
	if err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(bucket))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			key := string(k)
			keyvals[key] = v
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

func CPUBURN() {
	log.Printf("INFINITE WAIT CPUBURN!")
	go func() {
		time.Sleep(time.Second)
		runtime.GC()
	}()
	j := 1
	for {
		IsPow2(j)
		j++
	}
}
