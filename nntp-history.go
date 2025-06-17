package history

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-while/go-utils"
)

const (
	HashShort = 0x0B // 11
	//KeyIndex   = 0
	KeyLen      = 7    // Fixed key length for MySQL 3-level hex structure (7 chars after 3-char table prefix)
	NumCacheDBs = 4096 // Changed from 16 to 4096 for 3-level hex (16^3 = 4096)
	ALWAYS      = true
	// DefExpiresStr use 10 digits as spare so we can update it later without breaking offsets
	DefExpiresStr string = "----------" // never expires
	CaseLock             = 0xFF         // internal cache state. reply with CaseRetry while CaseLock
	CasePass             = 0xF1         // is a reply to L1Lock and IndexQuery
	CaseDupes            = 0x1C         // is a reply and cache state
	CaseRetry            = 0x2C         // is a reply to if CaseLock or CaseWrite or if history.dat returns EOF
	CaseAdded            = 0x3C         // is a reply to WriterChan:responseChan
	CaseWrite            = 0x4C         // internal cache state. is not a reply. reply with CaseRetry while CaseWrite is happening
	CaseError            = 0xE1         // some things drop this error
	ZEROPADLEN           = 0xFFF        // zeropads the header
	//CaseAddDupes = 0xC2
	//CaseAddRetry = 0xC3
)

var (
	ForcedReplay         bool
	NoReplayHisDat       bool
	UseHashDB            bool  = true                             // controls whether to use hash database for duplicate detection
	BatchFlushEvery      int64 = 5120                             // milliseconds
	HISTORY_INDEX_LOCK         = make(chan struct{}, 1)           // main lock
	HISTORY_INDEX_LOCK16       = make(chan struct{}, NumCacheDBs) // sub locks
	BootVerbose                = true
	//TESTHASH0                  = "0f05e27ca579892a63a256dacd657f5615fab04bf81e85f53ee52103e3a4fae8"
	//TESTHASH1                  = "f0d784ae13ce7cf1f3ab076027a6265861eb003ad80069cdfb1549dd1b8032e8"
	//TESTHASH2                  = "f0d784ae1747092974d02bd3359f044a91ed4fd0a39dc9a1feffe646e6c7ce09"
	TESTHASH = ""
	//TESTCACKEY                 = "f0d784ae1"
	//TESTKEY                    = "784ae1"
	//TESTBUK                    = "0d"
	//TESTDB                     = "f"
	//TESTOFFSET                 = 123456
	ROOTDBS []string
	//ROOTBUCKETS          []string
	//SUBBUCKETS           []string
	BUFLINES    = 10
	BUFIOBUFFER = 102 * BUFLINES // a history line with sha256 is 102 bytes long including LF or 38 bytes of payload + hashLen
	History     HISTORY
	DEBUG       bool = true
	DEBUG0      bool = false
	DEBUG1      bool = false
	DEBUG2      bool = false
	DEBUG9      bool = false
	LOCKHISTORY      = make(chan struct{}, 1)
	HEXCHARS         = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}
	eofhash          = "EOF"
)

// BootHistory initializes the history component, configuring its settings and preparing it for operation.
// It sets up the necessary directories for history and hash databases, and opens the history data file.
// The function also manages the communication channels for reading and writing historical data.
// If the `useHashDB` parameter is set to true, it initializes the history database (HashDB) and starts worker routines.
// Parameters:
//   - history_dir: The directory where history data will be stored.
//   - keylen: The length of the hash values used for indexing.
func (his *HISTORY) BootHistory(history_dir string, keylen int) {
	his.mux.Lock()
	defer his.mux.Unlock()
	if CPUProfile { // PROFILE.go
		CPUfile, err := his.startCPUProfile()
		if err != nil {
			os.Exit(1)
		}
		his.CPUfile = CPUfile
	}
	if his.WriterChan != nil {
		log.Printf("ERROR History already booted")
		return
	}
	rand.Seed(time.Now().UnixNano())
	his.Counter = make(map[string]uint64)

	go his.startServer(DefaultServerTCPAddr, DefaultSocketPath)

	if NumQueueWriteChan <= 0 {
		NumQueueWriteChan = 1
	}

	if BatchFlushEvery <= 2500 { // milliseconds
		BatchFlushEvery = 2500
	}

	if DefaultCachePurge <= 0 { // seconds
		DefaultCachePurge = 1
	}

	if DefaultCacheExpires <= 0 { // seconds
		DefaultCacheExpires = 1
	}

	if DefaultEvictsCapacity <= 0 {
		DefaultEvictsCapacity = 1
	}
	his.cEvCap = DefaultEvictsCapacity

	// hashDB_Index receives a HistoryIndex struct and passes it down to hashDB_Worker['0-9a-f']
	if IndexParallel <= 0 {
		IndexParallel = 1
	} else if IndexParallel > NumCacheDBs {
		IndexParallel = NumCacheDBs
	}
	his.indexPar = IndexParallel

	if history_dir == "" {
		history_dir = "history"
	}
	his.DIR = history_dir
	if !utils.DirExists(his.DIR) && !utils.Mkdir(his.DIR+"/hashdb") {
		os.Exit(1)
	}
	his.hisDat = his.DIR + "/history.dat"

	if !utils.DirExists(history_dir) && !utils.Mkdir(history_dir) {
		log.Printf("ERROR creating history_dir='%s'", history_dir)
		os.Exit(1)
	}

	// default history settings
	his.keyalgo = HashShort
	his.keylen = keylen
	if his.keylen != KeyLen {
		log.Printf("ERROR BootHistory keylen=%d != KeyLen=%d (fixed for MySQL 3-level hex structure)", keylen, KeyLen)
		os.Exit(1)
	}
	history_settings := &HistorySettings{Ka: his.keyalgo, Kl: his.keylen}
	// opens history.dat
	var fh *os.File
	new := false
	if !utils.FileExists(his.hisDat) {
		new = true
	}
	fh, err := os.OpenFile(his.hisDat, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR BootHistory os.OpenFile err='%v'", err)
		os.Exit(1)
	}
	dw := bufio.NewWriterSize(fh, BUFIOBUFFER)
	var headerdata []byte
	if new {
		// create history.dat
		_, err := gobEncodeHeader(&headerdata, history_settings)
		if err != nil {
			log.Printf("ERROR BootHistory gobEncodeHeader err='%v'", err)
			os.Exit(1)
		}
		if err := writeHistoryHeader(dw, headerdata, &his.Offset, true); err != nil {
			log.Printf("ERROR BootHistory writeHistoryHeader err='%v'", err)
			os.Exit(1)
		}

	} else {
		var header []byte
		// read history.dat header history_settings
		if b, err := his.FseekHistoryHeader(&header); b == 0 || err != nil {
			log.Printf("ERROR BootHistory header FseekHistoryLine err='%v' header='%v'", err, header)
			os.Exit(1)
		}
		logf(DEBUG0, "BootHistory history.dat headerBytes='%v'", header)

		if err := gobDecodeHeader(header, history_settings); err != nil {
			log.Printf("ERROR BootHistory gobDecodeHeader err='%v'", err)
			os.Exit(1)
		}
		if history_settings.Kl != his.keylen {
			log.Printf("ERROR BootHistory history_settings.Kl != his.keylen")
			os.Exit(1)
		}
		switch history_settings.Ka { // KeyAlgo
		case HashShort:
			// pass
		default:
			log.Printf("ERROR BootHistory gobDecodeHeader Unknown history_settings.KeyAlgo=%d'", history_settings.Ka)
			os.Exit(1)
		}
		his.keyalgo = history_settings.Ka
		his.keylen = history_settings.Kl
		//logf(DEBUG2, "Loaded History Settings: '%#v'", history_settings)
	}

	switch NumCacheDBs {
	case 16:
		his.cutChar = 1
		his.CutCharRO = his.cutChar
		ROOTDBS = generateCombinations(HEXCHARS, 1, []string{}, []string{})
	case 256:
		his.cutChar = 2
		his.CutCharRO = his.cutChar
		ROOTDBS = generateCombinations(HEXCHARS, 2, []string{}, []string{})
	case 4096:
		his.cutChar = 3
		his.CutCharRO = his.cutChar
		ROOTDBS = generateCombinations(HEXCHARS, 3, []string{}, []string{})
	default:
		log.Printf("ERROR BootHistory NumCacheDBs invalid=%d", NumCacheDBs)
		os.Exit(1)
	}
	//his.CutCharRO = his.cutChar

	if UseHashDB {
		his.hashDB_Init("mysql")
		log.Printf("hashDB init done")
	} else {
		log.Printf("hashDB disabled - initializing L1 cache for lightweight duplicate detection")
		his.L1.BootL1Cache(his)
		log.Printf("L1 cache init done")
	}

	//his.CacheEvictThread(NumCacheEvictThreads) // hardcoded

	logf(BootVerbose, "\n--> BootHistory: new=%t\n hisDat='%s'\n NumQueueWriteChan=%d DefaultCacheExpires=%d\n settings='%#v'", new, his.hisDat, NumQueueWriteChan, DefaultCacheExpires, history_settings)
	his.WriterChan = make(chan *HistoryObject, NumQueueWriteChan)
	go his.history_Writer(fh, dw)
} // end func BootHistory

func (his *HISTORY) AddHistory(hobj *HistoryObject, useL1Cache bool) int {
	if hobj == nil {
		log.Printf("ERROR AddHistory hobj=nil")
		return -999
	}
	if his.WriterChan == nil {
		log.Printf("ERROR AddHistory his.WriterChan=nil")
		return -999
	}

	//logf(DEBUG, "AddHistory hobj='%#v' before chan CHlen=%d CHcap=%d", hobj, len(his.WriterChan), cap(his.WriterChan))

	// sends it to history_Writer()
	// blocks if channel is full
	his.WriterChan <- hobj

	//logf(DEBUG, "AddHistory hobj='%#v' in chan CHlen=%d CHcap=%d", hobj, len(his.WriterChan), cap(his.WriterChan))

	// wait for response from ResponseChan
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
} // end func AddHistory

// history_Writer writes historical data to the specified file and manages the communication with the history database (HashDB).
// It listens to incoming HistoryObject structs on th* WriterChan channel, processes them, and writes formatted data to the file.
// If an index channel (IndexChan) is provided, it also interacts with the history database for duplicate checks.
// The function periodically flushes the data to the file to ensure data integrity.
func (his *HISTORY) history_Writer(fh *os.File, dw *bufio.Writer) {
	log.Printf("start history_Writer")
	if fh == nil || dw == nil {
		log.Printf("ERROR history_Writer fh=nil || dw=nil")
		return
	}
	if !LOCKfunc(LOCKHISTORY, "history_Writer") {
		return
	}
	defer fh.Close()
	defer UNLOCKfunc(LOCKHISTORY, "history_Writer")
	//log.Printf("start history_Writer Wait4HashDB")
	//his.Wait4HashDB()
	log.Printf("started history_Writer OK")
	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("ERROR history_Writer fh open Stat err='%v'", err)
		os.Exit(1)
	}
	his.Offset = fileInfo.Size()
	logf(DEBUG, "history_Writer opened fp='%s' filesize=%d", his.hisDat, his.Offset)
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
				hobj.Arrival = time.Now().Unix()
			}

			// Check for duplicates using available method (hashDB or L1 cache)
			var isDup int
			if UseHashDB && his.IndexChan != nil {
				// Use hash database for duplicate detection
				his.IndexChan <- &HistoryIndex{Hash: hobj.MessageIDHash, Char: hobj.Char, Offset: his.Offset, IndexRetChan: indexRetChan}
				select {
				case isDup_result, ok := <-indexRetChan:
					if !ok {
						log.Printf("ERROR history_Writer indexRetChan closed! error in hashDB_Worker")
						if hobj.ResponseChan != nil {
							close(hobj.ResponseChan) // close responseChan to client
						}
						if his.IndexChan != nil {
							his.IndexChan <- nil // stops history_dbz // dont close IndexChan as clients may still send requests
						}
						break forever
					}
					isDup = isDup_result
				} // end select
			} else {
				// Use L1 cache for lightweight duplicate detection
				result, err := his.IndexQuery(hobj.MessageIDHash, nil, his.Offset)
				if err != nil {
					log.Printf("ERROR history_Writer IndexQuery err='%v'", err)
					if hobj.ResponseChan != nil {
						hobj.ResponseChan <- CaseError
						close(hobj.ResponseChan)
					}
					break forever
				}
				isDup = result
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
				log.Printf("ERROR history_Writer unknown Switch after indexRetChan: isDup=%d=%x", isDup, isDup)
				break forever
			} // end switch isDup

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
	//logf(DEBUG, "writeHistoryLine='%s'", line)
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
	//logf(DEBUG, "wroteHistoryLine='%s'", line)
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
	if len(data) != ZEROPADLEN { // ZEROPADLEN
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
		*offset += int64(wb)
	}
	return nil
} // end func writeHistoryHeader

// FseekHistoryMessageHash seeks to a specified offset in the history file and extracts a message-ID hash.
// It reads characters from the file until a tab character ('\t') is encountered, extracting the hash enclosed in curly braces.
// If a valid hash is found, it returns the hash as a string without curly braces.
// If the end of the file (EOF) is reached, it returns a special EOF marker.
func (his *HISTORY) FseekHistoryMessageHash(file *os.File, offset int64, char string, rethash *string) error {
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

	// Seek to the specified offset
	_, seekErr := file.Seek(offset, 0)
	if seekErr != nil {
		log.Printf("ERROR FseekHistoryMessageHash seekErr='%v' fp='%s'", seekErr, his.hisDat)
		return seekErr
	}

	reader := bufio.NewReaderSize(file, 67) // {sha256}\t

	// Read until the first tab character
	result, err := reader.ReadString('\t')
	if err != nil {
		if err == io.EOF {
			//go his.Sync_upcounter("FSEEK_EOF")
			*rethash = eofhash
			return nil
		}
		log.Printf("ERROR FseekHistoryMessageHash err='%v'", err)
		return err
	}
	go his.Sync_upcounter("FSEEK")
	result = strings.TrimSuffix(result, "\t")

	if len(result) > 0 {
		if result[0] != '{' || result[len(result)-1] != '}' {
			return fmt.Errorf("ERROR FseekHistoryMessageHash BAD line @offset=%d result='%s'", offset, result)
		}
		if len(result[1:len(result)-1]) == 64 { // sha256
			*rethash = result[1 : len(result)-1]
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
			return "", fmt.Errorf("ERROR FseekHistoryLine line[0]!='{' offset=%d line='%s'", offset, result)
		}
	}
	result = strings.TrimSuffix(result, "\n")
	return result, nil
} // end func FseekHistoryLine

func (his *HISTORY) IndexQuery(hash string, indexRetChan chan int, offset int64) (int, error) {
	if len(hash) < 64 {
		return -999, fmt.Errorf("ERROR IndexQuery hash=nil")
	}

	// If hash database is available, use it
	if his.IndexChan != nil {
		if indexRetChan == nil {
			// for frequent access betters supply a indexRetChan
			indexRetChan = make(chan int, 1)
		}
		//logf(hash == TESTHASH0, "IndexQuery hash='%s' indexRetChan='%#v' offset=%d his.IndexChan=%d/%d", hash, indexRetChan, offset, len(his.IndexChan), cap(his.IndexChan))
		if offset > 0 {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: offset, IndexRetChan: indexRetChan}
		} else {
			his.IndexChan <- &HistoryIndex{Hash: hash, Offset: -1, IndexRetChan: indexRetChan}
		}
		select {
		case isDup, ok := <-indexRetChan:
			if !ok {
				return -999, fmt.Errorf("ERROR IndexQuery indexRetChan closed! error in hashDB_Worker")
			}
			return isDup, nil
		} // end select
	}

	// Fallback to L1 cache when hash database is not available
	if !UseHashDB {
		if offset > 0 {
			// Insert mode: add hash to L1 cache
			his.L1.Set(hash, "", CaseAdded, true, his)
			return CaseAdded, nil
		} else {
			// Query mode: check L1 cache for duplicates
			result := his.L1.LockL1Cache(hash, CaseDupes, his)
			if result == CasePass {
				// Not found in cache, it's new
				return CasePass, nil
			} else {
				// Found in cache, it's a duplicate
				return CaseDupes, nil
			}
		}
	}

	return -999, fmt.Errorf("ERROR IndexQuery - no hash database or L1 cache available")
} // end func IndexQuery

// hashDB_Index listens to incoming HistoryIndex structs on the IndexChan channel
// and distributes them to corresponding hashDB_Worker goroutines.
func (his *HISTORY) hashDB_Index() {
	if !LOCKfunc(HISTORY_INDEX_LOCK, "hashDB_Index") {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK, "hashDB_Index")
	//his.Wait4HashDB()
	//logf(DEBUG2, "Boot hashDB_Index")
	if DEBUG2 {
		defer log.Printf("Quit hashDB_Index")
	}
	waitchan := make(chan struct{}, his.indexPar)
	for p := 1; p <= his.indexPar; p++ {
		waitchan <- struct{}{}
		go func(p int, waitchan chan struct{}) {
			char := ""
		forever:
			for {
				select {
				case hi, ok := <-his.IndexChan: // receives a HistoryIndex struct and passes it down to '0-9a-f' workers
					if !ok {
						//logf(DEBUG2, "Stopping hashDB_Index IndexChan closed")
						break forever
					}
					if hi == nil || len(hi.Hash) < 64 { // allow at least sha256
						switch his.indexPar {
						case 1:
							close(his.IndexChan)
						default:
							his.IndexChan <- nil
						}
						logf(DEBUG2, "Stopping hashDB_Index IndexChan p=%d/%d received nil pointer", p, his.indexPar)
						break forever
					}
					if hi.Offset == 0 {
						log.Printf("ERROR hashDB_Index offset=0") // must: Offset -1 to checkonly OR Offset > 0 adds to hashDB
						break forever
					}

					// gets first N char of hash: hash must be lowercase!
					// hex.EncodeToString returns a lowercased string of a hashsum
					char = strings.ToLower(string(hi.Hash[:his.cutChar]))

					//logf(hi.Hash == TESTHASH0, "hashDB_Index hash='%s' hi.Offset=%d C1=%s chan=%d/%d",
					//	hi.Hash, hi.Offset, C1, len(his.indexChans[his.charsMap[C1]]), cap(his.indexChans[his.charsMap[C1]]))

					// sends object to hashDB_Worker char
					his.indexChans[his.charsMap[char]] <- hi
				} // end select
			} // end for
			<-waitchan
		}(p, waitchan) // end go func
	} // end for p
	for {
		time.Sleep(time.Second / 10)
		if len(waitchan) == 0 {
			break
		}
	}
	for _, achan := range his.indexChans {
		// passing nils to indexChans will stop hashDB_Worker
		achan <- nil
	}
} // end func hashDB_Index

func (his *HISTORY) hashDB_Worker(char string, i int, indexchan chan *HistoryIndex) {
	if !LOCKfunc(HISTORY_INDEX_LOCK16, "hashDB_Worker "+char) {
		return
	}
	defer UNLOCKfunc(HISTORY_INDEX_LOCK16, "hashDB_Worker "+char)

	logf(DEBUG2, "Boot hashDB_Worker [%s]", char)
	defer logf(DEBUG2, "Quit hashDB_Worker [%s]", char)

forever:
	for {
		select {
		case hi, ok := <-indexchan:
			if !ok {
				logf(DEBUG2, "hashDB_Worker [%s] indexchan closed", char)
				break forever
			}
			if hi == nil {
				logf(DEBUG2, "hashDB_Worker [%s] received nil pointer", char)
				break forever
			}
			if hi.IndexRetChan == nil {
				log.Printf("ERROR hashDB_Worker [%s] hi.IndexRetChan=nil", char)
				continue forever
			}

			// Extract the key from the hash (first 3 chars for table, next 7 chars for key)
			if len(hi.Hash) < 10 { // need at least 10 chars: 3 for table + 7 for key
				log.Printf("ERROR hashDB_Worker [%s] hash too short: %s", char, hi.Hash)
				hi.IndexRetChan <- CaseError
				continue forever
			}

			// Use first 10 chars: first 3 for table selection, next 7 as key
			fullKey := hi.Hash[:10]

			if hi.Offset == -1 {
				// Query mode: check if hash exists
				offsets, err := his.MySQLPool.GetOffsets(fullKey, nil)
				if err != nil {
					log.Printf("ERROR hashDB_Worker [%s] GetOffsets fullKey='%s' err='%v'", char, fullKey, err)
					hi.IndexRetChan <- CaseRetry
					continue forever
				}

				if len(offsets) > 1 {
					// Multiple offsets: need to check history.dat file at each offset to find exact match
					found := false
					for _, offset := range offsets {
						var hashFromFile string
						err := his.FseekHistoryMessageHash(nil, offset, char, &hashFromFile)
						if err != nil {
							log.Printf("ERROR hashDB_Worker [%s] FseekHistoryMessageHash offset=%d err='%v'", char, offset, err)
							continue
						}
						if hashFromFile == hi.Hash {
							// Found exact match in history.dat
							found = true
							break
						}
					}
					if found {
						hi.IndexRetChan <- CaseDupes
						go his.Sync_upcounter("duplicates")
					} else {
						// Full hash not found, it's new
						hi.IndexRetChan <- CasePass
					}
				} else if len(offsets) == 1 {
					// Single offset: need to verify it's the same full hash
					var hashFromFile string
					err := his.FseekHistoryMessageHash(nil, offsets[0], char, &hashFromFile)
					if err != nil {
						log.Printf("ERROR hashDB_Worker [%s] FseekHistoryMessageHash offset=%d err='%v'", char, offsets[0], err)
						hi.IndexRetChan <- CaseRetry
						continue forever
					}
					if hashFromFile == hi.Hash {
						// Exact match found
						hi.IndexRetChan <- CaseDupes
						go his.Sync_upcounter("duplicates")
					} else {
						// Different full hash, it's new
						hi.IndexRetChan <- CasePass
					}
				} else {
					// Hash doesn't exist, it's new
					hi.IndexRetChan <- CasePass
				}
			} else if hi.Offset > 0 {
				// Insert mode: add hash with offset
				err := his.MySQLPool.InsertOffset(fullKey, hi.Offset, nil)
				if err != nil {
					log.Printf("ERROR hashDB_Worker [%s] InsertOffset fullKey='%s' offset=%d err='%v'", char, fullKey, hi.Offset, err)
					hi.IndexRetChan <- CaseRetry
					continue forever
				}

				hi.IndexRetChan <- CaseAdded
				go his.Sync_upcounter("inserted")
			} else {
				log.Printf("ERROR hashDB_Worker [%s] invalid offset: %d", char, hi.Offset)
				hi.IndexRetChan <- CaseError
			}
		}
	}
} // end func hashDB_Worker

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
		lock5, v5 := false, 0
		//if his.useHashDB {
		//	lock5, v5 = len(his.batchQueues.BootCh) > 0, len(his.batchQueues.BootCh)
		//}

		batchQ, batchLOCKS := 0, 0
		//if his.useHashDB {
		/*
			for _, char := range ROOTDBS {
				for _, bucket := range ROOTBUCKETS {
					batchQ += len(his.batchQueues.Maps[char][bucket])
					batchLOCKS += len(his.BatchLocks[char].bl[bucket].ch)
				}
			}
		*/
		//}
		batchQueued := batchQ > 0
		batchLocked := batchLOCKS > 0
		if !lock1 && !lock2 && !lock3 && !lock5 && !batchQueued && !batchLocked {
			break
		}
		// if batchQ < NumCacheDBs*RootBuckets: it's most likely remaining 'nil' pointers which should be returned on next BatchFlushEvery
		// if v5 >= NumCacheDBs*RootBuckets: all batchQueues are still running
		log.Printf("WAIT CLOSE_HISTORY: lock1=%t=%d lock2=%t=%d lock3=%t=%d lock5=%t=%d batchQueued=%t=%d batchLocked=%t=%d", lock1, v1, lock2, v2, lock3, v3, lock5, v5, batchQueued, batchQ, batchLocked, batchLOCKS)
		time.Sleep(time.Second)
	}
	//his.WriterChan = nil
	if his.CPUfile != nil {
		his.stopCPUProfile(his.CPUfile)
	}
	/*
		 *
		if his.MEMfile != nil {
			his.stopMEMProfile(his.CPUfile)
		}
		*
	*/
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

func generateCombinations(hexChars []string, length int, currentCombination []string, combinations []string) []string {
	if len(currentCombination) == length {
		// Print or use the current combination here
		combination := strings.Join(currentCombination, "")
		combinations = append(combinations, combination)
		//fmt.Println(combination)
		return combinations
	}

	for _, char := range hexChars {
		newCombination := append(currentCombination, char)
		combinations = generateCombinations(hexChars, length, newCombination, combinations)
	}
	return combinations
}

func (his *HISTORY) BatchTicker(char string, ticker chan struct{}) {
	//isleep := 32
	isleep := BatchFlushEvery // / int64(RootBUCKETSperDB)
	if isleep <= 4 {
		isleep = 4
	}
	log.Printf("BatchTicker [%s] isleep=%d", char, isleep)
	for {
		ticker <- struct{}{}
		time.Sleep(time.Duration(isleep) * time.Millisecond)
	}
}

// InitializeDatabase initializes either MySQL or SQLite3 database backend
func (his *HISTORY) InitializeDatabase(useMySQL bool) error {
	return his.InitializeDatabaseWithSharding(useMySQL, SHARD_SINGLE_DB)
}

// InitializeDatabaseWithSharding initializes database backend with specific sharding mode
func (his *HISTORY) InitializeDatabaseWithSharding(useMySQL bool, shardMode int) error {
	if useMySQL {
		// Initialize MySQL RocksDB
		his.hashDB_Init("mysql")
		log.Printf("Initialized MySQL RocksDB backend")
	} else {
		// Initialize SQLite3 with specified sharding mode
		err := his.InitSQLite3WithSharding(shardMode)
		if err != nil {
			return fmt.Errorf("failed to initialize SQLite3 with sharding mode %d: %v", shardMode, err)
		}

		// Set sharding configuration in HISTORY struct
		numDBs, tablesPerDB, description := GetShardConfig(shardMode)
		his.ShardMode = shardMode
		his.ShardDBs = numDBs
		his.ShardTables = tablesPerDB

		log.Printf("Initialized SQLite3 backend: %s", description)
	}
	return nil
}

// UsingSQLite3 returns true if SQLite3 backend is being used
func (his *HISTORY) UsingSQLite3() bool {
	return his.SQLite3Pool != nil
}

// GetShardedDB returns the underlying SQLite3ShardedDB if available
func (his *HISTORY) GetShardedDB() (*SQLite3ShardedDB, bool) {
	if his.SQLite3Pool == nil {
		return nil, false
	}
	if shardedDB, ok := his.SQLite3Pool.(*SQLite3ShardedDB); ok {
		return shardedDB, true
	}
	return nil, false
}

// Convenience functions for initializing specific sharding modes

// InitializeSingleDB initializes with 1 database and 4096 tables
func (his *HISTORY) InitializeSingleDB() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_SINGLE_DB)
}

// InitializeFullSplit initializes with 4096 separate databases
func (his *HISTORY) InitializeFullSplit() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_FULL_SPLIT)
}

// Initialize16DB256Tables initializes with 16 databases, 256 tables each
func (his *HISTORY) Initialize16DB256Tables() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_16_256)
}

// Initialize64DB64Tables initializes with 64 databases, 64 tables each
func (his *HISTORY) Initialize64DB64Tables() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_64_64)
}

// Initialize128DB32Tables initializes with 128 databases, 32 tables each
func (his *HISTORY) Initialize128DB32Tables() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_128_32)
}

// Initialize512DB8Tables initializes with 512 databases, 8 tables each
func (his *HISTORY) Initialize512DB8Tables() error {
	return his.InitializeDatabaseWithSharding(false, SHARD_512_8)
}

// GetShardingInfo returns current sharding configuration
func (his *HISTORY) GetShardingInfo() (mode, numDBs, tablesPerDB int, description string) {
	numDBs, tablesPerDB, description = GetShardConfig(his.ShardMode)
	return his.ShardMode, numDBs, tablesPerDB, description
}


// SetBackend configures the backend type for the history system.
// This method should be called before BootHistory to set the desired backend.
// Parameters:
//   - backend: The backend type string ("sqlite3", "rocksdb", or "none")
func (his *HISTORY) SetBackend(backend string) {
	his.mux.Lock()
	defer his.mux.Unlock()

	switch backend {
	case "sqlite3":
		his.BackendType = BACKEND_SQLITE3
	case "rocksdb":
		his.BackendType = BACKEND_ROCKSDB
	case "none":
		his.BackendType = BACKEND_NONE
	default:
		log.Printf("WARN: Unknown backend '%s', defaulting to 'none'", backend)
		his.BackendType = BACKEND_NONE
	}

	logf(DEBUG, "Backend set to: %s", backend)
}
