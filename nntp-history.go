package history

import (
	"bufio"
	"fmt"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"io"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	ALWAYS = true
	// DefExpiresStr use 10 digits as spare so we can update it later without breaking offsets
	DefExpiresStr string = "----------" // never expires
	CaseLock             = 0xFF         // internal cache state. reply with CaseRetry while CaseLock
	CasePass             = 0xF1         // is a reply to L1Lock and IndexQuery
	CaseDupes            = 0x1C         // is a reply and cache state
	CaseRetry            = 0x2C         // is a reply to if CaseLock or CaseWrite or if history.dat returns EOF
	CaseAdded            = 0x3C         // is a reply to WriterChan:responseChan
	CaseWrite            = 0x4C         // internal cache state. is not a reply. reply with CaseRetry while CaseWrite is happening
	//CaseAddDupes = 0xC2
	//CaseAddRetry = 0xC3
)

var (
	TESTHASH0   string = "0f05e27ca579892a63a256dacd657f5615fab04bf81e85f53ee52103e3a4fae8"
	TESTHASH1   string = "f0d784ae13ce7cf1f3ab076027a6265861eb003ad80069cdfb1549dd1b8032e8"
	TESTHASH2   string = "f0d784ae1747092974d02bd3359f044a91ed4fd0a39dc9a1feffe646e6c7ce09"
	TESTHASH           = TESTHASH2
	TESTCACKEY         = "f0d784ae1"
	TESTKEY            = "784ae1"
	TESTBUK            = "0d"
	TESTDB             = "f"
	ALLBUCKETS  []string
	BUFIOBUFFER = 4 * 1024 // a history line with sha256 is 102 bytes long including LF or 38 bytes of payload + hashLen
	History     HISTORY
	DEBUG       bool   = true
	DEBUG0      bool   = false
	DEBUG1      bool   = false
	DEBUG2      bool   = false
	DEBUG9      bool   = false
	LOCKHISTORY        = make(chan struct{}, 1)
	HEXCHARS           = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}
	eofhash     string = "EOF"
)

// History_Boot initializes the history component, configuring its settings and preparing it for operation.
// It sets up the necessary directories for history and hash databases, and opens the history data file.
// The function also manages the communication channels for reading and writing historical data.
// If the `useHashDB` parameter is set to true, it initializes the history database (HashDB) and starts worker routines.
// Parameters:
//   - history_dir: The directory where history data will be stored.
//   - hashdb_dir: The directory where the history database (HashDB) will be stored.
//   - useHashDB: If true, enables the use of the history database (HashDB).
//   - boltOpts: Bolt database options for configuring the HashDB.
//   - keyalgo: The hash algorithm used for indexing historical data.
//   - keylen: The length of the hash values used for indexing.
func (his *HISTORY) History_Boot(history_dir string, hashdb_dir string, useHashDB bool, boltOpts *bolt.Options, keyalgo int, keylen int) {
	his.mux.Lock()
	defer his.mux.Unlock()
	if useHashDB {
		his.useHashDB = true
	}
	go his.startSocket()
	switch BUCKETSperDB {
	// BUCKETSperDB defines startindex cutFirst for cutHashlen!
	case 16:
		his.cutFirst = 2
		for _, c1 := range HEXCHARS {
			ALLBUCKETS = append(ALLBUCKETS, c1)
		}
	case 256:
		his.cutFirst = 3
		for _, c1 := range HEXCHARS {
			for _, c2 := range HEXCHARS {
				ALLBUCKETS = append(ALLBUCKETS, c1+c2)
			}
		}
	case 4096:
		his.cutFirst = 4
		for _, c1 := range HEXCHARS {
			for _, c2 := range HEXCHARS {
				for _, c3 := range HEXCHARS {
					ALLBUCKETS = append(ALLBUCKETS, c1+c2+c3)
				}
			}
		}
	default:
		log.Printf("ERROR History_Boot BUCKETSperDB invalid")
		os.Exit(1)
	}
	rand.Seed(time.Now().UnixNano())
	if his.WriterChan != nil {
		log.Printf("ERROR History already booted")
		return
	}

	if NumQueueWriteChan <= 0 {
		NumQueueWriteChan = 1
	} else if NumQueueWriteChan > 1000000 {
		NumQueueWriteChan = 1000000
	}

	if BatchFlushEvery <= 500 { // milliseconds
		BatchFlushEvery = 500
	} else if BatchFlushEvery > 15000 {
		BatchFlushEvery = 15000
	}

	/*
		if BatchFlushEvery*2 > DefaultCacheExpires*1000 {
			DefaultCacheExpires = BatchFlushEvery * 2 / 1000
		}
	*/

	if BatchFlushEvery*2 > DefaultCacheExpires*1000 {
		DefaultCacheExpires = BatchFlushEvery * 2 / 1000
		DefaultCacheExtend = DefaultCacheExpires
	}

	if DefaultCachePurge <= 0 { // seconds
		DefaultCachePurge = 1
	}

	if DefaultCacheExpires <= 0 { // seconds
		DefaultCacheExpires = 1
	}

	// boltDB_Index receives a HistoryIndex struct and passes it down to boltDB_Worker['0-9a-f']
	if IndexParallel <= 0 {
		IndexParallel = 1
	} else if IndexParallel > 16 {
		IndexParallel = 16 // hardcoded limit to 16
	}
	his.indexPar = IndexParallel

	if CharBucketBatchSize < 16 {
		CharBucketBatchSize = 16
	} else if CharBucketBatchSize > 65536 {
		// we don't need more before year 2050, he said in 2023.
		CharBucketBatchSize = 65536
	}
	his.wCBBS = CharBucketBatchSize
	log.Printf("CharBucketBatchSize=%d his.wCBBS=%d", CharBucketBatchSize, his.wCBBS)

	linSlashS := "/"
	winSlashS := "\\" // escaped
	linSlashB := byte('/')
	winSlashB := byte('\\') // escaped

	useSlash := linSlashS

	if history_dir == "" {
		history_dir = "history"
	} else {
		delslash := false // detect windows if history_dir ends with a winSlash
		if history_dir[len(history_dir)-1] == winSlashB {
			his.win, delslash, useSlash = true, true, winSlashS
		} else if history_dir[len(history_dir)-1] == linSlashB {
			delslash = true
		}
		if delslash {
			history_dir = history_dir[:len(history_dir)-1] // remove final slash
		}
	}
	his.hisDat = history_dir + useSlash + "history.dat"

	if hashdb_dir == "" {
		his.hisDatDB = his.hisDat + ".hash"
	} else {
		delslash := false // detect windows if hashdb_dir ends with a winSlash
		if hashdb_dir[len(hashdb_dir)-1] == winSlashB {
			his.win, delslash, useSlash = true, true, winSlashS
		} else if hashdb_dir[len(hashdb_dir)-1] == linSlashB {
			delslash = true
		}
		if delslash {
			hashdb_dir = hashdb_dir[:len(hashdb_dir)-1] // remove final slash
		}
		his.hisDatDB = hashdb_dir + useSlash + "history.dat.hash" // + ".a-f0-9"
	}
	if !utils.DirExists(history_dir) && !utils.Mkdir(history_dir) {
		log.Printf("ERROR creating history_dir='%s'", history_dir)
		os.Exit(1)
	}
	if useHashDB && !utils.DirExists(hashdb_dir) && !utils.Mkdir(hashdb_dir) {
		log.Printf("ERROR creating hashdb_dir='%s'", hashdb_dir)
		os.Exit(1)
	}

	switch keyalgo {
	case HashShort:
		his.keyalgo = HashShort
	case HashFNV32:
		his.keyalgo = HashFNV32
	case HashFNV32a:
		his.keyalgo = HashFNV32a
	case HashFNV64:
		his.keyalgo = HashFNV64
	case HashFNV64a:
		his.keyalgo = HashFNV64a
	default:
		log.Printf("ERROR History_Boot unknown keyalgo")
		return
	}
	his.keylen = keylen
	if his.keylen < MinKeyLen {
		log.Printf("ERROR History_Boot keylen=%d < MinKeyLen=%d", keylen, MinKeyLen)
		os.Exit(1)
	}

	// default history settings
	history_settings := &HistorySettings{KeyAlgo: his.keyalgo, KeyLen: his.keylen}

	// opens history.dat
	var fh *os.File
	new := false
	if !utils.FileExists(his.hisDat) {
		new = true
	}
	fh, err := os.OpenFile(his.hisDat, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR History_Boot os.OpenFile err='%v'", err)
		os.Exit(1)
	}
	dw := bufio.NewWriterSize(fh, BUFIOBUFFER)
	var headerdata []byte
	if new {
		// create history.dat
		_, err := gobEncodeHeader(&headerdata, history_settings)
		if err != nil {
			log.Printf("ERROR History_Boot gobEncodeHeader err='%v'", err)
			os.Exit(1)
		}
		if err := writeHistoryHeader(dw, headerdata, &his.Offset, true); err != nil {
			log.Printf("ERROR History_Boot writeHistoryHeader err='%v'", err)
			os.Exit(1)
		}
	} else {
		var header []byte
		// read history.dat header history_settings
		if b, err := his.FseekHistoryHeader(&header); b == 0 || err != nil {
			log.Printf("ERROR History_Boot header FseekHistoryLine err='%v' header='%v'", err, header)
			os.Exit(1)
		}
		logf(DEBUG0, "History_Boot history.dat headerBytes='%v'", header)

		if err := gobDecodeHeader(&header, history_settings); err != nil {
			log.Printf("ERROR History_Boot gobDecodeHeader err='%v'", err)
			os.Exit(1)
		}
		switch history_settings.KeyAlgo {
		case HashShort:
			// pass
		case HashFNV32:
			// pass
		case HashFNV32a:
			// pass
		case HashFNV64:
			// pass
		case HashFNV64a:
			// pass
		default:
			log.Printf("ERROR History_Boot gobDecodeHeader Unknown history_settings.KeyAlgo=%d'", history_settings.KeyAlgo)
			os.Exit(1)
		}
		his.keyalgo = history_settings.KeyAlgo
		his.keylen = history_settings.KeyLen
		//logf(DEBUG2, "Loaded History Settings: '%#v'", history_settings)
	}
	his.Counter = make(map[string]uint64)

	his.L1Cache.L1CACHE_Boot(his)
	his.CacheEvictThread()

	if his.useHashDB {
		his.boltDB_Init(boltOpts)
	}

	log.Printf("History: new=%t hisDat='%s' NumQueueWriteChan=%d DefaultCacheExpires=%d\n settings='#%v'", new, his.hisDat, NumQueueWriteChan, DefaultCacheExpires, history_settings)
	his.WriterChan = make(chan *HistoryObject, NumQueueWriteChan)
	go his.history_Writer(fh, dw)
} // end func History_Boot

func (his *HISTORY) AddHistory(hobj *HistoryObject, useL1Cache bool) int {
	if hobj == nil {
		log.Printf("ERROR AddHistory hobj=nil")
		return -999
	}
	if his.WriterChan == nil {
		log.Printf("ERROR AddHistory his.WriterChan=nil")
		return -999
	}

	his.WriterChan <- hobj // blocks if channel is full

	if (his.useHashDB || useL1Cache) && hobj.ResponseChan != nil {
		// wait for reponse from ResponseChan
		select {
		case isDup, ok := <-hobj.ResponseChan:
			if !ok {
				// error: responseChan got closed
				log.Printf("ERROR AddHistory responseChan closed! hash='%#v'", hobj.MessageIDHash)
				return -999
			} else {
				return isDup
			}
		} // end select
	} // end responseChan
	return -999
} // end func AddHistory

func (his *HISTORY) Wait4HashDB() {
	if his.useHashDB {
		now := utils.UnixTimeSec()
		for {
			time.Sleep(10 * time.Millisecond)
			if len(BoltHashOpen) == intBoltDBs {
				//logf(DEBUG2, "Booted HashDB")
				return
			}
			took := utils.UnixTimeSec() - now
			if took >= 15 {
				log.Printf("Wait booting HashDB")
				now = utils.UnixTimeSec()
			}
		}
		log.Printf("Wait4HashDB OK his.batchQueues.BootCh=%d", len(his.batchQueues.BootCh))
		time.Sleep(time.Second * 5)
	}
} // end func Wait4HashDB

// history_Writer writes historical data to the specified file and manages the communication with the history database (HashDB).
// It listens to incoming HistoryObject structs on th* WriterChan channel, processes them, and writes formatted data to the file.
// If an index channel (IndexChan) is provided, it also interacts with the history database for duplicate checks.
// The function periodically flushes the data to the file to ensure data integrity.
func (his *HISTORY) history_Writer(fh *os.File, dw *bufio.Writer) {
	if fh == nil || dw == nil {
		log.Printf("ERROR history_Writer fh=nil || dw=nil")
		return
	}
	if !LOCKfunc(LOCKHISTORY, "history_Writer") {
		return
	}
	defer fh.Close()
	defer UNLOCKfunc(LOCKHISTORY, "history_Writer")
	his.Wait4HashDB()
	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("ERROR history_Writer fh open Stat err='%v'", err)
		os.Exit(1)
	}
	his.Offset = fileInfo.Size()
	//logf(DEBUG2, "history_Writer opened fp='%s' filesize=%d", his.hisDat, his.Offset)
	flush := false // false: will flush when bufio gets full
	var wbt uint64
	var wroteLines uint64
	var indexRetChan chan int
	if History.IndexChan != nil {
		indexRetChan = make(chan int, 1)
	}
	buffered := 0
forever:
	for {
		if his.WriterChan == nil {
			log.Printf("history_Writer WriterChan=nil")
			return
		}
		select {
		case hobj, ok := <-his.WriterChan: // receives a HistoryObject struct
			if !ok || hobj == nil {
				// receiving a nil object stops history_writer
				if History.IndexChan != nil {
					History.IndexChan <- nil // stops history_dbz // dont close IndexChan as clients may still send requests
				}
				break forever
			}
			if hobj.MessageIDHash == "" {
				log.Printf("ERROR history_Writer hobj.MessageIDHash=nil")
				break forever
			}
			if hobj.StorageToken == "" {
				log.Printf("ERROR history_Writer hobj.StorageToken=nil")
				break forever
			}
			if hobj.Arrival == 0 {
				hobj.Arrival = utils.UnixTimeSec()
			}
			if History.IndexChan != nil {
				History.IndexChan <- &HistoryIndex{Hash: hobj.MessageIDHash, Char: hobj.Char, Offset: his.Offset, IndexRetChan: indexRetChan}
				select {
				case isDup, ok := <-indexRetChan:
					if !ok {
						log.Printf("ERROR history_Writer indexRetChan closed! error in boltDB_Worker")
						if hobj.ResponseChan != nil {
							close(hobj.ResponseChan) // close responseChan to client
						}
						if History.IndexChan != nil {
							History.IndexChan <- nil // stops history_dbz // dont close IndexChan as clients may still send requests
						}
						break forever
					}
					if hobj.ResponseChan != nil {
						hobj.ResponseChan <- isDup
					}
					switch isDup {
					case CaseDupes:
						continue forever
					case CaseRetry:
						// got EOF retry from dupecheck. flush history file so next check may hit
						if err := dw.Flush(); err != nil {
							log.Printf("ERROR history_Writer dw.Flush err='%v'", err)
							break forever
						}
						log.Printf("INFO history_Writer CaseRetry EOF flushed hisDat hash='%s' offset=%d", hobj.MessageIDHash, his.Offset)
						continue forever
					case CaseAdded:
						// pass
					default:
						log.Printf("ERROR history_Writer unknown Switch after indexRetChan: isDup=%d=%x", err, isDup, isDup)
						break forever
					}

					/*
						if isDup > 0 {
							if isDup == 2 { // got EOF retry from dupecheck. flush history file so next check may hit
								if err := dw.Flush(); err != nil {
									log.Printf("ERROR history_Writer dw.Flush err='%v'", err)
									break forever
								}
								//log.Printf("EOF forced flush OK")
							}
							// DUPLICATE entry
							//logf(DEBUG0, "history_Writer Index DUPLICATE hash='%s'", *hobj.MessageIDHash)
							continue forever
						}
					*/
				} // end select
			} // end if History.IndexChan != nil {
			// DONT! fake inn2 format... we use a lowercased hash and { as indicator, not < or [.
			// whs := fmt.Sprintf("[%s]\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			// not inn2 format
			//whs := fmt.Sprintf("{%s}\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			if err := his.writeHistoryLine(dw, hobj, flush, &wbt, &buffered); err != nil {
				log.Printf("ERROR history_Writer writeHistoryLine err='%v'", err)
				break forever
			}
			wroteLines++
		} // end select
	} // end for
	if err := dw.Flush(); err != nil {
		log.Printf("ERROR history_Writer dw.Flush() err='%v'", err)
	}
	if err := fh.Close(); err != nil {
		log.Printf("ERROR history_Writer fh.Close err='%v'", err)
	}
	logf(ALWAYS, "history_Writer closed fp='%s' wbt=%d offset=%d wroteLines=%d", his.hisDat, wbt, his.Offset, wroteLines)
} // end func history_Writer

func (his *HISTORY) writeHistoryLine(dw *bufio.Writer, hobj *HistoryObject, flush bool, wbt *uint64, bufferedptr *int) error {
	expiresStr := DefExpiresStr
	if hobj.Expires > 0 {
		expiresStr = fmt.Sprintf("%010d", hobj.Expires) // leftpad zeros to 10 digit
	}
	//if hobj.MessageIDHash == TESTHASH {
	//	log.Printf("writeHistoryLine TESTHASH='%s' offset=%d", hobj.MessageIDHash, his.Offset)
	//}
	line := fmt.Sprintf("{%s}\t%010d~%s~%010d\t%s\n", hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, hobj.StorageToken) // leftpad zeros to 10 digit
	ll := len(line)
	// check and flush only complete lines
	if cerr := checkBufioWriteBuffer(dw, ll, bufferedptr); cerr != nil {
		log.Printf("ERROR writeHistoryLine checkWriteBuffer cerr='%v'", cerr)
		return cerr
	}
	if wb, err := dw.WriteString(line); err != nil {
		log.Printf("ERROR history_Writer WriteString err='%v'", err)
		return err
	} else {
		//logf(DEBUG2, "history_Writer ll=%d wb=%d hash='%s' bufferedptr=%d", len(line), wb, hobj.MessageIDHash, *bufferedptr)
		if wbt != nil {
			*wbt += uint64(wb)
		}
		his.Offset += int64(wb)
		if flush {
			if err := dw.Flush(); err != nil {
				log.Printf("ERROR history_Writer WriteString err='%v'", err)
				return err
			}
			*bufferedptr = 0
		}
	}
	return nil
} // end func writeHistoryLine

func checkBufioWriteBuffer(dw *bufio.Writer, lenline int, bufferedptr *int) error {
	buffered := dw.Buffered()
	if buffered+lenline >= BUFIOBUFFER {
		//logf(DEBUG2, "checkWriteBuffer dw.Flush(): (buffered=%d + lenline=%d) > bufmax=%d bufferedptr=%d", buffered, lenline, BUFIOBUFFER, *bufferedptr)
		if err := dw.Flush(); err != nil {
			log.Printf("ERROR checkWriteBuffer Flush err='%v'", err)
			return err
		}
		*bufferedptr = 0
	}
	*bufferedptr += lenline
	return nil
} // end func checkBufioWriteBuffer

func writeHistoryHeader(dw *bufio.Writer, data []byte, offset *int64, flush bool) error {
	if dw == nil {
		return fmt.Errorf("ERROR writeHistoryHeader dw=nil")
	}
	if len(data) != 254 { // ZEROPADLEN
		return fmt.Errorf("ERROR writeHistoryHeader data=nil")
	}
	if offset == nil {
		return fmt.Errorf("ERROR writeHistoryHeader offset!=0")
	}
	data = append(data, '\n')
	if wb, err := dw.Write(data); err != nil {
		log.Printf("ERROR writeHistoryHeader Write err='%v'", err)
		return err
	} else {
		if flush {
			if err := dw.Flush(); err != nil {
				log.Printf("ERROR writeHistoryHeader Flush err='%v'", err)
				return err
			}
		}
		if offset != nil {
			*offset += int64(wb)
		}
	}
	return nil
} // end func writeHistoryHeader

// FseekHistoryMessageHash seeks to a specified offset in the history file and extracts a message-ID hash.
// It reads characters from the file until a tab character ('\t') is encountered, extracting the hash enclosed in curly braces.
// If a valid hash is found, it returns the hash as a string without curly braces.
// If the end of the file (EOF) is reached, it returns a special EOF marker.
func (his *HISTORY) FseekHistoryMessageHash(file *os.File, offset int64, char string, bucket string, rethash *string) error {
	if offset <= 0 || rethash == nil {
		return fmt.Errorf("ERROR FseekHistoryMessageHash io nil")
	}
	if file == nil {
		var err error
		file, err = os.OpenFile(his.hisDat, os.O_RDONLY, 0666)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	//var hash string
	//logf(offset == 1019995695, "FSEEK [%s|%s] check L2 offset=%d", char, bucket, offset)
	his.L2Cache.GetHashFromOffset(offset, rethash)
	if *rethash != "" {
		return nil
	}
	//if hash := his.L2Cache.GetHashFromOffset(offset, &hash); hash != "" {
	//	//logf(hash == TESTHASH0, "FSEEK [%s|%s] L2Cache.GetHashFromOffset=%d => hash='%s' return hash", char, bucket, offset, hash)
	//	return hash, nil
	//}
	//logf(offset == 1019995695, "FSEEK [%s|%s] notfound L2 offset=%d", char, bucket, offset)

	// Seek to the specified offset
	_, seekErr := file.Seek(offset, 0)
	if seekErr != nil {
		log.Printf("ERROR FseekHistoryMessageHash seekErr='%v' fp='%s'", seekErr, his.hisDat)
		return seekErr
	}
	//logf(offset == 1019995695, "FSEEK [%s|%s] seeking offset=%d", char, bucket, offset)

	reader := bufio.NewReaderSize(file, 67) // {sha256}\t
	//logf(offset == 1019995695, "FSEEK [%s|%s] reading offset=%d", char, bucket, offset)

	// Read until the first tab character
	result, err := reader.ReadString('\t')
	if err != nil {
		log.Printf("ERROR FSEEK err='%v'", err)
		if err == io.EOF {
			go his.Sync_upcounter("FSEEK_EOF")
			*rethash = eofhash
			return nil
		}
		return err
	}
	go his.Sync_upcounter("FSEEK")
	result = strings.TrimSuffix(result, "\t")
	//logf(offset == 1019995695, "FSEEK [%s|%s] result=%d='%s' offset=%d", char, bucket, len(result), result, offset)

	if len(result) > 0 {
		if result[0] != '{' || result[len(result)-1] != '}' {
			return fmt.Errorf("ERROR FseekHistoryMessageHash BAD line @offset=%d result='%s'", offset, result)
		}
		if len(result[1:len(result)-1]) >= 32 { // at least md5
			//logf(hash == TESTHASH, "FSEEK [%s|%s] @offset=%d => hash='%s'", char, bucket, offset, hash)
			*rethash = result[1 : len(result)-1]
			his.L2Cache.SetOffsetHash(offset, *rethash, FlagExpires)
			return nil
		}
	}
	return nil
} // end func FseekHistoryMessageHash

func (his *HISTORY) FseekHistoryHeader(output *[]byte) (int, error) {
	if output == nil {
		log.Printf("ERROR FseekHistoryHeader output=nil")
	}
	file, err := os.OpenFile(his.hisDat, os.O_RDONLY, 0666)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		char, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		if char == '\n' {
			break
		}
		*output = append(*output, char)
	}
	return len(*output), nil
} // end func FseekHistoryHeader

func (his *HISTORY) FseekHistoryLine(offset int64) (string, error) {
	file, err := os.OpenFile(his.hisDat, os.O_RDONLY, 0666)
	if err != nil {
		return "", err
	}
	defer file.Close()
	// Seek to the specified offset
	_, seekErr := file.Seek(offset, 0)
	if seekErr != nil {
		return "", seekErr
	}
	reader := bufio.NewReader(file)
	// Read until the next newline character
	result, err := reader.ReadString('\n')
	if err != nil {
		//if err == io.EOF {
		//	return &eofhash, nil
		//}
		return "", err
	}
	//result := strings.Split(line, "\t")[0]
	if len(result) > 0 {
		if offset > 0 && result[0] != '{' {
			return "", fmt.Errorf("ERROR FseekHistoryLine line[0]!='{' offset=%d", offset)
		}
	}
	result = strings.TrimSuffix(result, "\n")
	return result, nil
} // end func FseekHistoryLine

func (his *HISTORY) SET_DEBUG(debug int) {
	if debug < 0 {
		return
	}
	switch debug {
	case 0:
		DEBUG = true
		DEBUG0 = true
	case 1:
		DEBUG0 = true
		DEBUG1 = true
	case 2:
		DEBUG0 = true
		DEBUG1 = true
		DEBUG2 = true
	case 9:
		DEBUG9 = true
	}
} // end func SET_DEBUG

func (his *HISTORY) CLOSE_HISTORY() {
	log.Printf("CLOSE_HISTORY aquire lock")
	his.mux.Lock()
	defer his.mux.Unlock()
	log.Printf("CLOSE_HISTORY aquired lock ok")
	defer log.Printf("CLOSE_HISTORY DONE")
	if his.WriterChan == nil {
		log.Printf("ERROR CLOSE_HISTORY his.WriterChan=nil")
		return
	}
	log.Printf("CLOSE_HISTORY: his.WriterChan <- nil")
	his.WriterChan <- nil // closes workers
	for {

		lock1, v1 := len(LOCKHISTORY) > 0, len(LOCKHISTORY)
		lock2, v2 := len(HISTORY_INDEX_LOCK) > 0, len(HISTORY_INDEX_LOCK)
		lock3, v3 := len(HISTORY_INDEX_LOCK16) > 0, len(HISTORY_INDEX_LOCK16)
		lock4, v4 := his.GetBoltHashOpen() > 0, his.GetBoltHashOpen()
		lock5, v5 := false, 0
		if his.useHashDB {
			lock5, v5 = len(his.batchQueues.BootCh) > 0, len(his.batchQueues.BootCh)
		}

		batchQ, batchLOCKS := 0, 0
		if his.useHashDB {
			for _, char := range HEXCHARS {
				for _, bucket := range ALLBUCKETS {
					batchQ += len(his.batchQueues.Maps[char][bucket])
					batchLOCKS += len(his.BatchLocks[char].bl[bucket].ch)
				}
			}
		}
		batchQueued := batchQ > 0
		batchLocked := batchLOCKS > 0
		if !lock1 && !lock2 && !lock2 && !lock3 && !lock4 && !lock5 && !batchQueued && !batchLocked {
			break
		}
		// if batchQ < 256: it's most likely remaining 'nil' pointers which should be returned on next BatchFlushEvery
		// if v5 == 256: all batchQueues are still running
		//if batchQ > 256 && v5 == 256 {
		//if batchLOCKS > 0 {
		log.Printf("WAIT CLOSE_HISTORY: lock1=%t=%d lock2=%t=%d lock3=%t=%d lock4=%t=%d lock5=%t=%d batchQueued=%t=%d batchLocked=%t=%d", lock1, v1, lock2, v2, lock3, v3, lock4, v4, lock5, v5, batchQueued, batchQ, batchLocked, batchLOCKS)
		//}
		time.Sleep(time.Second)
	}
	his.WriterChan = nil
} // end func CLOSE_HISTORY

func LOCKfunc(achan chan struct{}, src string) bool {
	select {
	case achan <- struct{}{}:
		return true
	default:
		log.Printf("ERROR LOCKfunc src='%s' already running=%d", src, len(achan))
	}
	return false
} // end LOCKfunc

func UNLOCKfunc(achan chan struct{}, src string) {
	<-achan
} // end func UNLOCKfunc

func logf(debug bool, format string, a ...any) {
	if debug {
		log.Printf(format, a...)
	}
} // end logf

func PrintMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc: %v MiB, TotalAlloc: %v MiB, Sys: %v MiB, NumGC: %v\n",
		m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024, m.NumGC)
}

func PrintMemoryStatsEvery(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		PrintMemoryStats()
	}
}

func IsPow2(n int) bool {
	// result 0 is pow^2
	res := n & (n - 1)
	if res != 0 {
		return false
	}
	return true
} // end func isPow2
