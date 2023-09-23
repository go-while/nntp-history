package history

import (
	"bufio"
	"fmt"
	//"github.com/edsrzf/mmap-go"
	"github.com/go-while/go-utils"
	"io"
	//"hash/fnv"
	"log"
	"os"
	//"strings"
	bolt "go.etcd.io/bbolt"
	"sync"
	"time"
)

const (
	HashShort      uint8 = 11
	HashFNV32      uint8 = 22
	HashFNV32a     uint8 = 33
	HashFNV64      uint8 = 44
	HashFNV64a     uint8 = 55
	DefaultHashLen int   = 5
)

var (
	Bolt_SYNC_EVERY      int64 = 15 // seconds
	DEBUG                bool  = true
	DEBUG0               bool  = false
	DEBUG1               bool  = false
	History              HISTORY
	HISTORY_WRITER_LOCK  = make(chan struct{}, 1)
	HISTORY_INDEX_LOCK   = make(chan struct{}, 1)
	HISTORY_INDEX_LOCK16 = make(chan struct{}, 16)
	HISTORY_WRITER_CHAN  chan *HistoryObject
	HEXCHARS                    = [16]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}
	eofhash              string = "EOF"
)

type HISTORY struct {
	mux sync.Mutex
	boltChanInit      chan struct{}
	//rmux   sync.RWMutex
	Offset int64
	HF     string // = "history/history.dat"
	HF_hash    string // = "history/history.Hash"
	IndexChan  chan *HistoryIndex
	IndexChans [16]chan *HistoryIndex
	charsMap   map[string]int
	useHashDB  bool
	hashtype   uint8
	//shorthash  bool
	hashlen    int
}

type HistoryObject struct {
	MessageIDHash *string
	StorageToken  *string // "F" = flatstorage | "M" = mongodb | "X" = deleted
	Char          string
	Arrival       int64
	Expires       int64
	Date          int64
	ResponseChan  chan int // receives a 0,1,2 if not|duplicate|retrylater
}

/* builds the history.dat header */
type HistorySettings struct {
	HashType uint8
	HashLen int
}

func (his *HISTORY) History_Boot(history_dir string, hashdb_dir string, useHashDB bool, readq int, writeq int, boltOpts *bolt.Options, bolt_SYNC_EVERY int64, hashalgo uint8, hashlen int) {
	his.mux.Lock()
	defer his.mux.Unlock()
	if HISTORY_WRITER_CHAN != nil {
		log.Printf("ERROR History already booted#1!")
		return
	}
	if useHashDB && his.IndexChan != nil {
		log.Printf("ERROR History already booted#2!")
		return
	}
	if history_dir == "" {
		history_dir = "history"
	} else {
		if history_dir[len(history_dir)-1] == '/' {
			history_dir = history_dir[:len(history_dir)-1] // remove final slash
		}
	}
	his.HF = history_dir + "/" + "history.dat"

	if hashdb_dir == "" {
		his.HF_hash = his.HF + ".hash"
	} else {
		if hashdb_dir[len(hashdb_dir)-1] == '/' {
			hashdb_dir = hashdb_dir[:len(hashdb_dir)-1] // remove final slash
		}
		his.HF_hash = hashdb_dir + "/" + ".hash" // + ".a-f0-9"
	}
	if !utils.DirExists(history_dir) && !utils.Mkdir(history_dir) {
		log.Printf("Error creating history_dir='%s'", history_dir)
		os.Exit(1)
	}
	if useHashDB && !utils.DirExists(hashdb_dir) && !utils.Mkdir(hashdb_dir) {
		log.Printf("Error creating hashdb_dir='%s'", hashdb_dir)
		os.Exit(1)
	}
	if readq <= 0 {
		readq = 1
	}
	if writeq <= 0 {
		writeq = 1
	}
	if bolt_SYNC_EVERY > 0 {
		Bolt_SYNC_EVERY = bolt_SYNC_EVERY
	}
	switch hashalgo {
	case HashShort:
		his.hashtype = HashShort
	case HashFNV32:
		his.hashtype = HashFNV32
	case HashFNV32a:
		his.hashtype = HashFNV32a
	case HashFNV64:
		his.hashtype = HashFNV64
	case HashFNV64a:
		his.hashtype = HashFNV64a
	default:
		log.Printf("ERROR History_Boot unknown hashalgo")
		return
	}
	//his.shorthash, his.hashlen = shorthash, hashlen
	his.hashlen = hashlen
	if his.hashlen < DefaultHashLen {
		log.Printf("ERROR History_Boot hashlen=%d < DefaultHashLen=%d", DefaultHashLen)
		os.Exit(1)
	}
	history_settings := &HistorySettings{ HashType: his.hashtype, HashLen: his.hashlen }
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
	dw := bufio.NewWriterSize(fh, 32*1024)
	if new {
		// create new history.dat
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
		// load header settings from history.dat
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
		his.hashtype = history_settings.HashType
		switch his.hashtype {
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
			log.Printf("ERROR History_Boot gobDecodeHeader Unknown HashType=%d'", his.hashtype)
			os.Exit(1)
		}
		his.hashlen = history_settings.HashLen
		log.Printf("Loaded History Settings: '%#v'", history_settings)
	}
	if useHashDB {
		his.IndexChan = make(chan *HistoryIndex, readq)
		his.useHashDB = true
		his.charsMap = make(map[string]int, boltDBs)
		his.History_DBZinit(boltOpts)
	}
	HISTORY_WRITER_CHAN = make(chan *HistoryObject, writeq)
	go his.History_Writer(fh, dw)
	log.Printf("History_Boot: file='%s'", his.HF)
} // end func History_Boot

func LOCKfunc(achan chan struct{}, src string) bool {
	select {
	case achan <- struct{}{}:
		logf(DEBUG1, "LOCKfunc src='%s' OK", src)
		return true
	default:
		log.Printf("ERROR LOCKfunc src='%s' already running=%d", src, len(achan))
	}
	return false
} // end LOCKfunc

func UNLOCKfunc(achan chan struct{}, src string) {
	logf(DEBUG1, "UNLOCKfunc src='%s' unlocking", src)
	<-achan
	logf(DEBUG1, "UNLOCKfunc src='%s' unlocked", src)
} // end func UNLOCKfunc

func (his *HISTORY) wait4HashDB() {
	now := utils.UnixTimeSec()
	//start := utils.UnixTimeMilliSec()
	if his.useHashDB {
		for {
			time.Sleep(10 * time.Millisecond)
			if len(BoltHashOpen) == 16 {
				break
			}
			took := utils.UnixTimeSec() - now
			if took >= 15 {
				log.Printf("Wait booting HashDB ms=%d", utils.UnixTimeSec()-now)
				now = utils.UnixTimeSec()
			}
		}
	}
	//log.Printf("Booted HashDB ms=%d", utils.UnixTimeMilliSec()-start)
} // end func wait4HashDB

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
	var wbt int
	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("ERROR History_Writer fh open Stat err='%v'", err)
		os.Exit(1)
	}
	his.Offset = fileInfo.Size()
	flush := true
	/*
	boottime := utils.UnixTimeSec()
	if his.Offset == 0 {
		header := fmt.Sprintf("|history.dat|CD=%d|HT=%d\n|{hash}|arrival~expires~msgdate|storage\n", boottime, his.hashtype)
		if err := writeHistoryLine(dw, &header, &his.Offset, flush, &wbt); err != nil {
			log.Printf("ERROR History_Writer create header err='%v'", err)
			return
		}
	}*/
	var wroteLines uint64
	flush = false // will flush when bufio gets full
	log.Printf("History_Writer opened fp='%s' filesize=%d", his.HF, his.Offset)
	var indexRetChan chan int
	if History.IndexChan != nil {
		indexRetChan = make(chan int, 1)
	}
	//storageToken := "?"
forever:
	for {
		if HISTORY_WRITER_CHAN == nil {
			log.Printf("History_Writer HISTORY_WRITER_CHAN=nil")
			return
		}
		select {
		case hobj, ok := <-HISTORY_WRITER_CHAN: // recevies a HistoryObject struct
			if !ok || hobj == nil {
				// receiving a nil object stops history_writer
				if History.IndexChan != nil {
					History.IndexChan <- nil // stops history_dbz // dont close IndexChan as clients may still send requests
				}
				break forever
			}
			if hobj.Arrival == 0 {
				hobj.Arrival = utils.UnixTimeSec()
			}
			if hobj.MessageIDHash == nil {
				log.Printf("ERROR History_Writer hobj.MessageIDHash=nil")
				continue forever
			}
			if hobj.StorageToken == nil {
				log.Printf("ERROR History_Writer hobj.StorageToken=nil")
				continue forever
			}
			//ST := &storageToken
			//if hobj.StorageToken != nil && *hobj.StorageToken != "" {
			//	ST = hobj.StorageToken
			//}
			expiresStr := "-"
			if hobj.Expires >= 0 {
				expiresStr = fmt.Sprintf("%d", hobj.Expires)
			}

			if History.IndexChan != nil {
				History.IndexChan <- &HistoryIndex{Hash: hobj.MessageIDHash, Offset: his.Offset, IndexRetChan: indexRetChan}
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
					if isDup > 0 {
						if isDup == 2 { // got EOF retry from dupecheck. flush history file so next check may hit
							if err := dw.Flush(); err != nil {
								log.Printf("ERROR History_Writer dw.Flush err='%v'", err)
								break forever
							}
						}
						// DUPLICATE entry
						logf(DEBUG0, "History_Writer Index DUPLICATE hash='%s'", *hobj.MessageIDHash)
						continue forever
					}
				} // end select
			}

			// DONT! fake inn2 format... we use a lowercased hash and { as indicator, not < or [.
			// whs := fmt.Sprintf("[%s]\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			// not inn2 format
			whs := fmt.Sprintf("{%s}\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			if err := writeHistoryLine(dw, &whs, &his.Offset, flush, &wbt); err != nil {
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
	log.Printf("History_Writer closed fp='%s' wbt=%d offset=%d wroteLines=%d", his.HF, wbt, his.Offset, wroteLines)
} // end func History_Writer

func writeHistoryLine(dw *bufio.Writer, line *string, offset *int64, flush bool, wbt *int) error {
	if wb, err := dw.WriteString(*line); err != nil {
		log.Printf("ERROR History_Writer WriteString err='%v'", err)
		return err
	} else {
		//log.Printf("History_Writer whs=%d wrote=%d msgidhash='%s'", len(whs), wb, *hobj.MessageIDHash)
		if wbt != nil {
			*wbt += wb
		}
		if offset != nil {
			*offset += int64(wb)
		}
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

func (his *HISTORY) FseekHistoryMessageHash(offset int64) (*string, error) {
	file, err := os.OpenFile(his.HF, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Seek to the specified offset
	_, seekErr := file.Seek(offset, 0)
	if seekErr != nil {
		log.Printf("ERROR FseekHistoryMessageHash seekErr='%v' fp='%s'", seekErr, his.HF)
		return nil, seekErr
	}

	// Create a buffered reader for efficient reading
	reader := bufio.NewReader(file)

	var result string

	for {
		// Read a single byte
		char, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				log.Printf("WARN FseekHistoryMessageHash EOF offset=%d", offset)
				// EOF Reached end of history file! entry not yet flushed: asume a hit or return 436 retry later?
				return &eofhash, nil
			}
			return nil, err
		}
		// Check if the character is a '\t'
		if char == '\t' {
			break
		}
		// Append the character to the result string
		result += string(char)
	}

	if len(result) > 0 {
		if result[0] != '{' || result[len(result)-1] != '}' {
			return nil, fmt.Errorf("Error FseekHistoryMessageHash BAD line @offset=%d", offset)
		}
		hash := result[1 : len(result)-1]
		if len(hash) >= 32 { // at least md5
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

	/*
	// Seek to the specified offset
	_, seekErr := file.Seek(0, 1024)
	if seekErr != nil {
		return nil, seekErr
	}
	*/
	// Create a buffered reader for efficient reading
	reader := bufio.NewReader(file)

	var result []byte

	for {
		// Read a single byte
		char, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		// Check if the character is a '\n'
		if char == '\n' {
			break
		}
		// Append the character to the result string
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

	// Create a buffered reader for efficient reading
	reader := bufio.NewReader(file)

	var result string

	for {
		// Read a single byte
		char, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// EOF Reached end of history file! entry not yet flushed
				return nil, nil
			}
			return nil, err
		}
		// Check if the character is a '\n'
		if char == '\n' {
			break
		}
		// Append the character to the result string
		result += string(char)
	}
	if len(result) > 0 {
		if offset > 0 && result[0] != '{' {
			return nil, fmt.Errorf("Error FseekHistoryLine line[0]!='{' offset=%d", offset)
		}
	}
	return &result, nil
} // end func FseekHistoryLine

func logf(debug bool, format string, a ...any) {
	if debug {
		log.Printf(format, a...)
	}
} // end logf
