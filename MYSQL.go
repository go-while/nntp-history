package history

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-while/go-utils"
	"log"
	"os"
	"strings"
	"sync"
)

type SQL struct {
	mux     sync.RWMutex
	ctr     sync.RWMutex // counter
	DBs     chan *DBconn
	dsn     string
	maxOpen int
	isOpen  int
	Timeout int64
} // end func SQLhandler

type DBopts struct {
	Username string
	Password string
	Hostname string
	DBname   string
	params   string
	MaxOpen  int
	InitOpen int
	TCPmode  string
	Timeout  int64
}

type DBconn struct {
	db      *sql.DB
	Timeout int64
}

/*
	createTables := true
	shortHashDBpool, err := history.NewSQLpool(&history.DBopts{
	    Username: "xxx",
	    Password: "xxx",
	    Hostname: "xxx",
	    DBname: "xxx",
	    MaxOpen: 128,
	    InitOpen: 16,
	    TCPmode: "tcp4",
	    Timeout: 55,
	}, createTables)
*/

func NewSQLpool(opts *DBopts, createTables bool) (*SQL, error) {
	if opts.MaxOpen <= 0 {
		opts.MaxOpen = 1
	}
	if createTables && opts.InitOpen <= 0 {
		opts.InitOpen = 1
	}
	s := &SQL{}
	if opts.Timeout < 5 {
		opts.Timeout = 5
		if opts.params == "" {
			opts.params = fmt.Sprintf("?Timeout=%ds", opts.Timeout+5)
		} else {
			opts.params = opts.params + fmt.Sprintf("&Timeout=%ds", opts.Timeout+5)
		}
	}
	s.Timeout = opts.Timeout
	switch opts.TCPmode {
	case "tcp":
		// pass
	case "tcp4":
		// pass
	case "tcp6":
		// pass
	default:
		opts.TCPmode = "tcp"
	}
	s.maxOpen = opts.MaxOpen
	s.DBs = make(chan *DBconn, s.maxOpen)
	s.dsn = fmt.Sprintf("%s:%s@%s(%s)/%s%s", opts.Username, opts.Password, opts.TCPmode, opts.Hostname, opts.DBname, opts.params)
	for i := 0; i < opts.InitOpen; i++ {
		db, err := s.GetDB(false)
		if err != nil {
			return nil, err
		}
		s.isOpen++
		s.ReturnDB(db)
	}

	if createTables {
		if err := s.ShortHashDB_CreateTables(); err != nil {
			return nil, err
		}
	}
	return s, nil
} // end func NewSQLpool

func (s *SQL) InsertOffset(key string, offset int64, db *sql.DB) error {
	if db == nil {
		adb, err := s.GetDB(true)
		if err != nil {
			return err
		}
		db = adb
		defer s.ReturnDB(db)
	}

	query := fmt.Sprintf("INSERT INTO s%s (h,o) VALUES ('%s','%d,') ON DUPLICATE KEY UPDATE o=CONCAT(o, '%d,')", key[:3], key[3:], offset, offset)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("ERROR history InsertOffset query='%s' err='%v'", query, err)
		return err
	}
	return nil
} // end func InsertOffset

func (s *SQL) GetOffsets(key string, db *sql.DB) ([]int64, error) {
	if db == nil {
		adb, err := s.GetDB(true)
		if err != nil {
			return nil, err
		}
		db = adb
		defer s.ReturnDB(db)
	}

	var offsetsStr string
	err := db.QueryRow("SELECT o FROM s"+key[:3]+"` WHERE h = ? LIMIT 1", key[3:]).Scan(&offsetsStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		log.Printf("ERROR history GetOffsets err='%v'", err)
		return nil, err
	}
	var offsets []int64
	if len(offsetsStr) > 0 {
		offs := strings.Split(offsetsStr, ",")
		for _, offsetStr := range offs {
			offset := utils.Str2int64(offsetStr)
			if offset > 0 {
				offsets = append(offsets, offset)
			}
		}
		return offsets, nil
	}
	return nil, nil
} // end func GetOffsets

func (s *SQL) NewConn() (*sql.DB, error) {
	// [Username[:Password]@][protocol[(address)]]/DBname[?param1=value1&...&paramN=valueN]
	db, err := sql.Open("mysql", s.dsn)
	if err != nil {
		log.Printf("ERROR history NewConn 'open' failed err='%v'", err)
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		log.Printf("ERROR history NewConn 'ping' failed err='%v'", err)
		return nil, err
	}
	return db, nil
} // end func connSQL

func (s *SQL) ShortHashDB_CreateTables() error {
	db, err := s.GetDB(true)
	if err != nil {
		return err
	}
	defer s.ReturnDB(db)
	cs := "0123456789abcdef"
	for _, c1 := range cs {
		for _, c2 := range cs {
			for _, c3 := range cs {
				query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `s%s%s%s` (`h` char(7) NOT NULL, `o` LONGTEXT NULL, PRIMARY KEY `h`) ENGINE=RocksDB DEFAULT CHARSET=latin1 COLLATE=latin1_bin;", string(c1), string(c2), string(c3))
				_, err := db.Exec(query)
				if err != nil {
					log.Printf("ERROR history CreateTables query='%s' err='%v'", query, err)
					os.Exit(1)
				}
			}
		}
	}
	return nil
} // end func ShortHashDB_CreateTables

func (s *SQL) GetDB(wait bool) (db *sql.DB, err error) {
	if wait {
		s.ctr.RLock()
		if s.isOpen > 0 {
			s.ctr.RUnlock()
			dbconn := <-s.DBs
			if dbconn.Timeout > utils.UnixTimeSec() {
				db = dbconn.db
				return
			}
			// dbconn is Timeout
			if dbconn.db != nil {
				err = dbconn.db.Ping()
				if err != nil {
					log.Printf("WARN history GetDB 'ping' failed err='%v'", err)
					dbconn.db = nil
				}
			}
			if dbconn.db != nil {
				db = dbconn.db
				return
			}
			db, err = s.NewConn()
			if err != nil {
				s.ctr.Lock()
				s.isOpen--
				s.ctr.Unlock()
			}
			return
		}
		s.ctr.RUnlock()
	} // end if wait

	select {
	case dbconn := <-s.DBs:
		db = dbconn.db
		return

	default:
		s.ctr.Lock()
		defer s.ctr.Unlock()
		if s.isOpen < s.maxOpen {
			db, err = s.NewConn()
			if err != nil {
				return
			}
			s.isOpen++
		}
	} // end select
	return
} // end func GetDB

func (s *SQL) ReturnDB(db *sql.DB) {
	s.DBs <- &DBconn{db: db, Timeout: utils.UnixTimeSec() + s.Timeout}
} // end func ReturnDB

func (s *SQL) CloseDB(db *sql.DB) {
	db.Close()
	s.ctr.Lock()
	s.isOpen--
	s.ctr.Unlock()
} // end func CloseDB

func (s *SQL) ClosePool() {
	log.Printf("sql.ClosePool")
	defer log.Printf("sql.ClosePool returned")
	for {
		select {
		case dbconn := <-s.DBs:
			if dbconn.db != nil {
				dbconn.db.Close()
			}
			s.ctr.Lock()
			s.isOpen--
			isopen := s.isOpen
			s.ctr.Unlock()
			log.Printf("sql.ClosePool: open %d/%d", isopen, s.maxOpen)
			if isopen == 0 {
				return
			}
		} // end select
	} // end for
} // end func ClosePool

func (s *SQL) SetMaxOpen(MaxOpen int) {
	if MaxOpen < 0 {
		MaxOpen = 0
	}
	s.ctr.Lock()
	s.maxOpen = MaxOpen
	s.ctr.Unlock()
	log.Printf("sql.SetMaxOpen=%d", MaxOpen)
} // end func SetMaxOpen

func (s *SQL) GetIsOpen() int {
	s.ctr.RLock()
	isopen := s.isOpen
	s.ctr.RUnlock()
	return isopen
} // end func GetIsOpen

func (s *SQL) GetDSN() string {
	s.mux.RLock()
	dsn := s.dsn
	s.mux.RUnlock()
	return dsn
} // end func GetDSN
