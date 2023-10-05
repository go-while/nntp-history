package history

import (
	"bufio"
	//"encoding/gob"
	"fmt"
	//"github.com/edsrzf/mmap-go"
	"github.com/go-while/go-utils"
	bolt "go.etcd.io/bbolt"
	"io"
	"log"
	"os"
	"strings"
	//"github.com/dgraph-io/badger"
	//"strconv"
	"time"
)

const (
	TESTHASH1 string = "76d4b3a84c3c72a08a5b4c433f864a29c441a8806a70c02256026ac54a5b726a" // i=651695
	TESTHASH2 string = "76d4b3a80f26e7941e6f96da3c76852f249677f53723b7432b3063d56861eafa" // i=659591
	// DefExpiresStr use 10 digits as spare so we can update it later without breaking offsets
	DefExpiresStr string = "----------" // never expires
	//DefExpiresStr       string = "-" // never expires
	DefaultCacheExpires int64 = 9 // seconds
	DefaultCachePurge   int64 = 3 // seconds
	CaseLock     = 0xF0
	CasePass     = 0xF1
	CaseDupes    = 0xB1
	CaseRetry    = 0xB2
	CaseAdded    = 0xC1
	//CaseAddDupes = 0xC2
	//CaseAddRetry = 0xC3
)

var (
	History             HISTORY
	DEBUG               bool   = true
	DEBUG0              bool   = false
	DEBUG1              bool   = false
	DEBUG2              bool   = false
	DEBUG9              bool   = false
	HISTORY_WRITER_LOCK        = make(chan struct{}, 1)
	QueueWriteChan      int    = BoltDBs
	BATCHFLUSH          int64  = 3000
	HEXCHARS                   = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}
	eofhash             string = "EOF"
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
	//gob.RegisterName("GOBOFFSETS", GOBOFFSETS{})
	his.mux.Lock()
	defer his.mux.Unlock()
	if his.WriterChan != nil {
		log.Printf("ERROR History already booted")
		return
	}

	if QueueWriteChan <= 0 {
		QueueWriteChan = 1
	} else if QueueWriteChan > 1000000 {
		QueueWriteChan = 1000000
	}

	if BATCHFLUSH == 0 {
		BATCHFLUSH = 3000
	} else if BATCHFLUSH < 10 {
		BATCHFLUSH = 10
	} else if BATCHFLUSH > 5000 {
		BATCHFLUSH = 5000
	}

	if BATCHSIZE < 16 {
		BATCHFLUSH = 16
	} else if BATCHFLUSH > 65536 {
		BATCHFLUSH = 65536
	}

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
	his.HF = history_dir + useSlash + "history.dat"

	if hashdb_dir == "" {
		his.HF_hash = his.HF + ".hash"
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
		his.HF_hash = hashdb_dir + useSlash + "history.dat.hash" // + ".a-f0-9"
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
	if !utils.FileExists(his.HF) {
		new = true
	}
	fh, err := os.OpenFile(his.HF, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERROR History_Boot os.OpenFile err='%v'", err)
		os.Exit(1)
	}
	dw := bufio.NewWriterSize(fh, 4*1024)
	if new {
		// create history.dat
		data, err := gobEncodeHeader(history_settings)
		if err != nil {
			log.Printf("ERROR History_Boot gobEncodeHeader err='%v'", err)
			os.Exit(1)
		}
		if err := writeHistoryHeader(dw, data, &his.Offset, true); err != nil {
			log.Printf("ERROR History_Boot writeHistoryHeader err='%v'", err)
			os.Exit(1)
		}
	} else {
		// read history.dat header history_settings
		header, err := his.FseekHistoryHeader()
		if err != nil || header == nil {
			log.Printf("ERROR History_Boot header FseekHistoryLine err='%v' header='%v'", err, header)
			os.Exit(1)
		}
		logf(DEBUG0, "History_Boot history.dat headerBytes='%v'", header)
		history_settings, err = gobDecodeHeader(*header)
		if err != nil {
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
		logf(DEBUG2, "Loaded History Settings: '%#v'", history_settings)
	}

	his.L1Cache.L1CACHE_Boot()

	HashDBQueues := ""
	if useHashDB {
		his.L2Cache.L2CACHE_Boot()
		his.L3Cache.L3CACHE_Boot()
		his.useHashDB = true
		his.BatchQueues = &BQ{}
		his.BatchQueues.Booted = make(chan struct{}, 16*16)                           // char [0-9a-f] * bucket [0-9a-f]
		his.BatchQueues.Maps = make(map[string]map[string]chan *BatchOffset, BoltDBs) // maps char : bucket => chan
		for _, char := range HEXCHARS {
			his.BatchQueues.Maps[char] = make(map[string]chan *BatchOffset, BoltDBs) // maps bucket => chan
		}
		his.IndexChan = make(chan *HistoryIndex, QueueIndexChan)
		his.charsMap = make(map[string]int, BoltDBs)
		his.History_DBZinit(boltOpts)
		HashDBQueues = fmt.Sprintf("QueueIndexChan=%d QueueIndexChans=%d", QueueIndexChan, QueueIndexChans)
	}
	his.Counter = make(map[string]uint64)
	log.Printf("History: new=%t\n  HF='%s' DB='%s.[0-9a-f]'\n  KeyAlgo=%d KeyLen=%d QueueWriteChan=%d\n  HashDBQueues:{%s}", new, his.HF, his.HF_hash, his.keyalgo, his.keylen, QueueWriteChan, HashDBQueues)
	his.WriterChan = make(chan *HistoryObject, QueueWriteChan)
	go his.History_Writer(fh, dw)
} // end func History_Boot

func (his *HISTORY) wait4HashDB() {
	now := utils.UnixTimeSec()
	if his.useHashDB {
		for {
			time.Sleep(10 * time.Millisecond)
			if len(BoltHashOpen) == BoltDBs {
				break
			}
			took := utils.UnixTimeSec() - now
			if took >= 15 {
				log.Printf("Wait booting HashDB")
				now = utils.UnixTimeSec()
			}
		}
	}
	//log.Printf("Booted HashDB")
} // end func wait4HashDB

// History_Writer writes historical data to the specified file and manages the communication with the history database (HashDB).
// It listens to incoming HistoryObject structs on th* WriterChan channel, processes them, and writes formatted data to the file.
// If an index channel (IndexChan) is provided, it also interacts with the history database for duplicate checks.
// The function periodically flushes the data to the file to ensure data integrity.
func (his *HISTORY) History_Writer(fh *os.File, dw *bufio.Writer) {
	if fh == nil || dw == nil {
		log.Printf("ERROR History_Writer fh=nil || dw=nil")
		return
	}
	if !LOCKfunc(HISTORY_WRITER_LOCK, "History_Writer") {
		return
	}
	defer fh.Close()
	defer UNLOCKfunc(HISTORY_WRITER_LOCK, "History_Writer")
	his.wait4HashDB()
	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("ERROR History_Writer fh open Stat err='%v'", err)
		os.Exit(1)
	}
	his.Offset = fileInfo.Size()
	logf(DEBUG2, "History_Writer opened fp='%s' filesize=%d", his.HF, his.Offset)
	flush := false // false: will flush when bufio gets full
	var wbt uint64
	var wroteLines uint64
	var indexRetChan chan int
	if History.IndexChan != nil {
		indexRetChan = make(chan int, 1)
	}
forever:
	for {
		if his.WriterChan == nil {
			log.Printf("History_Writer WriterChan=nil")
			return
		}
		select {
		case hobj, ok := <-his.WriterChan: // recevies a HistoryObject struct
			if !ok || hobj == nil {
				// receiving a nil object stops history_writer
				if History.IndexChan != nil {
					History.IndexChan <- nil // stops history_dbz // dont close IndexChan as clients may still send requests
				}
				break forever
			}
			if hobj.MessageIDHash == nil {
				log.Printf("ERROR History_Writer hobj.MessageIDHash=nil")
				break forever
			}
			if hobj.StorageToken == nil {
				log.Printf("ERROR History_Writer hobj.StorageToken=nil")
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
						log.Printf("ERROR History_Writer indexRetChan closed! error in History_DBZ_Worker")
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
								log.Printf("ERROR History_Writer dw.Flush err='%v'", err)
								break forever
							}
							continue forever
						//case CasePass:
						//	is not a possible response here
						case CaseAdded:
							// pass
						default:
							log.Printf("ERROR History_Writer unknown Switch after indexRetChan: isDup=%d=%x=%#v", err, isDup, isDup, isDup)
							break forever
					}

					/*
					if isDup > 0 {
						if isDup == 2 { // got EOF retry from dupecheck. flush history file so next check may hit
							if err := dw.Flush(); err != nil {
								log.Printf("ERROR History_Writer dw.Flush err='%v'", err)
								break forever
							}
							//log.Printf("EOF forced flush OK")
						}
						// DUPLICATE entry
						//logf(DEBUG0, "History_Writer Index DUPLICATE hash='%s'", *hobj.MessageIDHash)
						continue forever
					}
					*/
				} // end select
			} // end if History.IndexChan != nil {
			// DONT! fake inn2 format... we use a lowercased hash and { as indicator, not < or [.
			// whs := fmt.Sprintf("[%s]\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			// not inn2 format
			//whs := fmt.Sprintf("{%s}\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			if err := his.writeHistoryLine(dw, hobj, flush, &wbt); err != nil {
				log.Printf("ERROR History_Writer writeHistoryLine err='%v'", err)
				break forever
			}
			wroteLines++
		} // end select
	} // end for
	if err := dw.Flush(); err != nil {
		log.Printf("ERROR History_Writer dw.Flush() err='%v'", err)
	}
	if err := fh.Close(); err != nil {
		log.Printf("ERROR History_Writer fh.Close err='%v'", err)
	}
	logf(DEBUG1, "History_Writer closed fp='%s' wbt=%d offset=%d wroteLines=%d", his.HF, wbt, his.Offset, wroteLines)
} // end func History_Writer

func (his *HISTORY) writeHistoryLine(dw *bufio.Writer, hobj *HistoryObject, flush bool, wbt *uint64) error {
	expiresStr := DefExpiresStr
	if hobj.Expires > 0 {
		expiresStr = fmt.Sprintf("%010d", hobj.Expires) // leftpad zeros to 10 digit
	}
	line := fmt.Sprintf("{%s}\t%010d~%s~%010d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken) // leftpad zeros to 10 digit
	if wb, err := dw.WriteString(line); err != nil {
		log.Printf("ERROR History_Writer WriteString err='%v'", err)
		return err
	} else {
		logf(DEBUG2, "History_Writer ll=%d wb=%d hash='%s'", len(line), wb, *hobj.MessageIDHash)
		if wbt != nil {
			*wbt += uint64(wb)
		}
		his.Offset += int64(wb)
		if flush {
			if err := dw.Flush(); err != nil {
				log.Printf("ERROR History_Writer WriteString err='%v'", err)
				return err
			}
		}
	}
	return nil
} // end func writeHistoryLine

func writeHistoryHeader(dw *bufio.Writer, data *[]byte, offset *int64, flush bool) error {
	if dw == nil {
		return fmt.Errorf("ERROR writeHistoryHeader dw=nil")
	}
	if data == nil {
		return fmt.Errorf("ERROR writeHistoryHeader data=nil")
	}
	if offset == nil {
		return fmt.Errorf("ERROR writeHistoryHeader offset=nil")
	}
	*data = append(*data, '\n')
	if wb, err := dw.Write(*data); err != nil {
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
func (his *HISTORY) FseekHistoryMessageHash(file *os.File, offset *int64) (*string, error) {
	if offset == nil {
		return nil, fmt.Errorf("ERROR FseekHistoryMessageHash offset=nil")
	}
	if file == nil {
		var err error
		file, err = os.OpenFile(his.HF, os.O_RDONLY, 0666)
		if err != nil {
			return nil, err
		}
		defer file.Close()
	}

	if hash := his.L2Cache.GetHashFromOffset(offset); hash != nil {
		his.Sync_upcounter("L2CACHE_Get")
		return hash, nil
	}

	// Seek to the specified offset
	_, seekErr := file.Seek(*offset, 0)
	if seekErr != nil {
		log.Printf("ERROR FseekHistoryMessageHash seekErr='%v' fp='%s'", seekErr, his.HF)
		return nil, seekErr
	}

	reader := bufio.NewReaderSize(file, 67)

	// Read until the first tab character
	result, err := reader.ReadString('\t')
	if err != nil {
		if err == io.EOF {
			his.Sync_upcounter("FSEEK_EOF")
			return &eofhash, nil
		}
		return nil, err
	}
	his.Sync_upcounter("FSEEK")
	//log.Printf("FseekHistoryMessageHash offset=%d", *offset)

	//result := strings.Split(line, "\t")[0]
	result = strings.TrimSuffix(result, "\t")
	if len(result) > 0 {
		if result[0] != '{' || result[len(result)-1] != '}' {
			return nil, fmt.Errorf("ERROR FseekHistoryMessageHash BAD line @offset=%d result='%s'", *offset, result)
		}
		hash := result[1 : len(result)-1]
		if len(hash) >= 32 { // at least md5
			//logf(DEBUG2, "FseekHistoryMessageHash offset=%d hash='%s'", *offset, hash)
			his.L2Cache.SetOffsetHash(offset, &hash)
			return &hash, nil
		}
	}
	return nil, nil
} // end func FseekHistoryMessageHash

func (his *HISTORY) FseekHistoryHeader() (*[]byte, error) {
	file, err := os.OpenFile(his.HF, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var result []byte
	for {
		char, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if char == '\n' {
			break
		}
		result = append(result, char)
	}
	return &result, nil
} // end func FseekHistoryHeader

func (his *HISTORY) FseekHistoryLine(offset int64) (*string, error) {
	file, err := os.OpenFile(his.HF, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// Seek to the specified offset
	_, seekErr := file.Seek(offset, 0)
	if seekErr != nil {
		return nil, seekErr
	}
	reader := bufio.NewReader(file)
	// Read until the next newline character
	result, err := reader.ReadString('\n')
	if err != nil {
		//if err == io.EOF {
		//	return &eofhash, nil
		//}
		return nil, err
	}
	//result := strings.Split(line, "\t")[0]
	if len(result) > 0 {
		if offset > 0 && result[0] != '{' {
			return nil, fmt.Errorf("ERROR FseekHistoryLine line[0]!='{' offset=%d", offset)
		}
	}
	result = strings.TrimSuffix(result, "\n")
	return &result, nil
} // end func FseekHistoryLine

func (his *HISTORY) SET_DEBUG(debug int) {
	switch debug {
	case 0:
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
}

func (his *HISTORY) CLOSE_HISTORY() {
	his.mux.Lock()
	defer his.mux.Unlock()
	defer log.Printf("CLOSE_HISTORY DONE")
	if his.WriterChan == nil {
		return
	}
	his.WriterChan <- nil // closes workers
	for {
		time.Sleep(time.Second)
		lock1, v1 := len(HISTORY_WRITER_LOCK) > 0, len(HISTORY_WRITER_LOCK)
		lock2, v2 := len(HISTORY_INDEX_LOCK) > 0, len(HISTORY_INDEX_LOCK)
		lock3, v3 := len(HISTORY_INDEX_LOCK16) > 0, len(HISTORY_INDEX_LOCK16)
		lock4, v4 := his.GetBoltHashOpen() > 0, his.GetBoltHashOpen()
		lock5, v5 := len(his.BatchQueues.Booted) > 0, len(his.BatchQueues.Booted)
		batchQ := 0
		for _, char := range HEXCHARS {
			for _, bucket := range HEXCHARS {
				batchQ += len(his.BatchQueues.Maps[char][bucket])
			}
		}
		lockBatch := batchQ > 0
		if !lock1 && !lock2 && !lock2 && !lock3 && !lock4 && !lock5 && !lockBatch {
			break
		}
		// if batchQ < 256: it's most likely remaining 'nil' pointers which should be returned on next BATCHFLUSH
		// if v5 == 256: all batchQueues are still running
		if batchQ > 256 && v5 == 256 {
			log.Printf("WAIT CLOSE_HISTORY: lock1=%t=%d lock2=%t=%d lock3=%t=%d lock4=%t=%d lock5=%t=%d lockBatch=%t=%d", lock1, v1, lock2, v2, lock3, v3, lock4, v4, lock5, v5, lockBatch, batchQ)
		}
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
