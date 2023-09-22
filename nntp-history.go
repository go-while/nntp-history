package history

import (
	"bufio"
	"fmt"
	"github.com/edsrzf/mmap-go"
	"github.com/go-while/go-utils"
	//"hash/fnv"
	"log"
	"os"
	//"strings"
	bolt "go.etcd.io/bbolt"
	"sync"
	"time"
)

const (
	HashFNV32  uint8 = 1
	HashFNV32a uint8 = 2
	HashFNV64  uint8 = 3
	HashFNV64a uint8 = 4
	//DoCheckHashDupOnly uint8 = 1
)

var (
	HashType                   = HashFNV32 // can not be changed once db has been created!
	Bolt_SYNC_EVERY      int64 = 5         // seconds
	DEBUG                bool  = true
	DEBUG0               bool  = false
	DEBUG1               bool  = false
	History              HISTORY
	HISTORY_WRITER_LOCK  = make(chan struct{}, 1)
	HISTORY_INDEX_LOCK   = make(chan struct{}, 1)
	HISTORY_INDEX_LOCK16 = make(chan struct{}, 16)
	HISTORY_WRITER_CHAN  chan *HistoryObject
	HEXCHARS             = [16]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}
)

type HISTORY struct {
	mux sync.Mutex
	//rmux   sync.RWMutex
	Offset int64
	HF     string // = "history/history.dat"
	//HF_hidx    string // = "history/history.HIndex"
	HF_hash    string // = "history/history.HHash"
	IndexChan  chan *HistoryIndex
	IndexChans [16]chan *HistoryIndex
	files_IDX  [16]*os.File
	files_PAK  [16]*os.File
	mmaps_IDX  [16]*mmap.MMap
	mmaps_PAK  [16]*mmap.MMap
	charsMap   map[string]int
	useHashDB  bool
}

type HistoryObject struct {
	MessageIDHash *string
	StorageToken  *string // "F" = flatstorage | "M" = mongodb | "" = not stored
	Char          string
	Arrival       int64
	Expires       int64
	Date          int64
	ResponseChan  chan bool // receives a true/false isDUP or closed channel on error
}

func (his *HISTORY) History_Boot(history_dir string, useHashDB bool, readq int, writeq int, boltOpts *bolt.Options, bsync int64) {
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
	if readq <= 0 {
		readq = 1
	}
	if writeq <= 0 {
		writeq = 1
	}
	if history_dir == "" {
		history_dir = "history"
	} else {
		if history_dir[len(history_dir)-1] == '/' {
			history_dir = history_dir[:len(history_dir)-1] // remove final slash
		}
	}
	if bsync > 0 {
		Bolt_SYNC_EVERY = bsync
	}
	his.HF = history_dir + "/" + "history.dat"
	HISTORY_WRITER_CHAN = make(chan *HistoryObject, writeq)
	his.HF_hash = his.HF + ".hash"
	//his.HF_hidx = his.HF + ".hidx" // unused
	his.IndexChan = make(chan *HistoryIndex, boltDBs)
	if useHashDB {
		his.useHashDB = true
		his.charsMap = make(map[string]int, boltDBs)
		his.History_DBZinit(boltOpts)
	}
	go his.History_Writer()
	log.Printf("History_Boot: file='%s'", his.HF)
} // emd func History_Boot

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
	start := utils.UnixTimeMilliSec()
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
	log.Printf("Booted HashDB ms=%d", utils.UnixTimeMilliSec()-start)
} // end func wait4HashDB

func (his *HISTORY) History_Writer() {
	if !LOCKfunc(HISTORY_WRITER_LOCK, "History_Writer") {
		return
	}
	defer UNLOCKfunc(HISTORY_WRITER_LOCK, "History_Writer")
	his.wait4HashDB()

	fh, err := os.OpenFile(his.HF, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer fh.Close()
	dw := bufio.NewWriterSize(fh, 4*1024)
	var wbt int
	fileInfo, err := fh.Stat()
	if err != nil {
		log.Printf("ERROR History_Writer fh open Stat err='%v'", err)
		os.Exit(1)
	}
	his.Offset = fileInfo.Size()
	flush := true
	boottime := utils.UnixTimeSec()
	if his.Offset == 0 {
		header := fmt.Sprintf("|history.dat|%d\n|{hash}|arrival-expires-msgdate|storage\n", boottime)
		if err := writeHistoryLine(dw, &header, &his.Offset, flush, &wbt); err != nil {
			log.Printf("ERROR History_Writer create header err='%v'", err)
			return
		}
	}
	var wroteLines uint64
	flush = false // will flush when bufio gets full, default 4K
	log.Printf("History_Writer opened fp='%s' filesize=%d", his.HF, his.Offset)
	var indexRetChan chan bool
	if History.IndexChan != nil {
		indexRetChan = make(chan bool, 1)
	}
	storageToken := "?"
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
					History.IndexChan <- nil // stops history_dbz
					//close(History.IndexChan) // stops history_dbz // dont close channel as clients may still send requests
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
			ST := &storageToken
			if hobj.StorageToken != nil && *hobj.StorageToken != "" {
				ST = hobj.StorageToken
			}
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
							History.IndexChan <- nil // stops history_dbz
							//close(History.IndexChan) // stops history_dbz // dont close channel as clients may still send requests
						}
						break forever
					}
					if hobj.ResponseChan != nil {
						hobj.ResponseChan <- isDup
					}
					if isDup {
						// DUPLICATE entry
						logf(DEBUG0, "History_Writer Index DUPLICATE hash='%s'", *hobj.MessageIDHash)
						continue forever
					}
				} // end select
			}

			/*
				if hobj.Do != nil && *hobj.Do == DoCheckHashDupOnly {
					continue forever
				}
			*/

			// fake inn2 format
			// whs := fmt.Sprintf("[%s]\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *hobj.StorageToken)
			// not inn2 format
			flush := true
			whs := fmt.Sprintf("{%s}\t%d~%s~%d\t%s\n", *hobj.MessageIDHash, hobj.Arrival, expiresStr, hobj.Date, *ST)
			if err := writeHistoryLine(dw, &whs, &his.Offset, flush, &wbt); err != nil {
				log.Printf("ERROR History_Writer writeHistoryLine err='%v'", err)
				break forever
			}
			wroteLines++
			/*
				if wb, err := dw.WriteString(whs); err != nil {
					log.Printf("ERROR History_Writer WriteString err='%v'", err)
					break forever
				} else {
					//log.Printf("History_Writer whs=%d wrote=%d msgidhash='%s'", len(whs), wb, *hobj.MessageIDHash)
					wbt += wb
					if err := dw.Flush(); err != nil {
						log.Printf("ERROR History_Writer WriteString err='%v'", err)
						break forever
					}
					/,*
						if History.IndexChan != nil {
							History.IndexChan <- &HistoryIndex{Hash: hobj.MessageIDHash, Offset: his.Offset}
						}
					*,/
					his.Offset += int64(wb)
				}*/
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
		/*
			if History.IndexChan != nil {
				History.IndexChan <- &HistoryIndex{Hash: hobj.MessageIDHash, Offset: his.Offset}
			}
		*/

	}
	return nil
} // end func writeHistoryLine

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
	/*
		// Create a scanner to read lines as strings
		scanner := bufio.NewScanner(file)

		if scanner.Scan() {
			line := scanner.Text()
			if len(line) > 0 {
				if line[0] != '{' || line[len(line)-1] != '}' {
					return nil, fmt.Errorf("Error FseekHistoryMessageHash BAD line @offset=%d line='%s'", offset, line)
				}
				hash := line[1 : len(line)-1]
				if len(hash) == 64 {
					return &hash, nil
				}
			}
		}

		if scanner.Err() != nil {
			return nil, scanner.Err()
		}
	*/

	// Create a buffered reader for efficient reading
	reader := bufio.NewReader(file)

	var result string

	for {
		// Read a single byte
		char, err := reader.ReadByte()
		if err != nil {
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
		if result[0] != '{' {
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
