package history

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-while/go-utils"
)

type SQL struct {
	mux     sync.RWMutex
	ctr     sync.RWMutex // counter
	DBs     chan *DBconn
	dsn     string
	maxOpen int
	isOpen  int
	timeout int64
} // end func SQLhandler

func (his *HISTORY) hashDB_Init(driver string) {
	switch driver {
	case "mysql":
		// Initialize MySQL RocksDB connection pool
		opts := &DBopts{
			username: "nntp_history",   // You can make these configurable
			password: "password",       // You can make these configurable
			hostname: "localhost:3306", // You can make these configurable
			dbname:   "nntp_history",   // You can make these configurable
			params:   "?charset=utf8mb4&parseTime=True&loc=Local",
			maxopen:  64, // You can make this configurable
			initopen: 16, // You can make this configurable
			tcpmode:  "tcp",
			timeout:  30,
		}

		var err error
		his.MySQLPool, err = NewSQLpool(opts, true) // true = create tables
		if err != nil {
			log.Fatalf("ERROR hashDB_Init failed to initialize MySQL pool: %v", err)
		}
		log.Printf("MySQL RocksDB pool initialized successfully")

	//case "sqlite3":
	//	// pass
	default:
		log.Fatalf("hashDB_Init driver %s not implemented", driver)
	}

	// Initialize charsMap and indexChans
	his.charsMap = make(map[string]int, NumCacheDBs)
	for i, char := range ROOTDBS {
		his.charsMap[char] = i
		his.indexChans[i] = make(chan *HistoryIndex, 16)
	}

	// Start workers
	for i, char := range ROOTDBS {
		// dont move this up into the first for loop or it drops race conditions for nothing...
		go his.hashDB_Worker(char, his.indexChans[i])
	}
	go his.hashDB_Index()
	//his.ReplayHisDat() // TODO!
} // end func hashDB_Init

type DBopts struct {
	username string
	password string
	hostname string
	dbname   string
	params   string
	maxopen  int
	initopen int
	tcpmode  string
	timeout  int64
}

type DBconn struct {
	db      *sql.DB
	timeout int64
}

/*
createTables := true

	shortHashDBpool, err := history.NewSQLpool(&history.DBopts{
	    username: "xxx",
	    password: "xxx",
	    hostname: "xxx",
	    dbname: "xxx",
	    maxopen: "128",
	    initopen: "16",
	    tcpmode: "tcp4",
	    timeout: 55,
	}, createTables)
*/
func NewSQLpool(opts *DBopts, createTables bool) (*SQL, error) {
	if opts.maxopen <= 0 {
		opts.maxopen = 1
	}
	if createTables && opts.initopen <= 0 {
		opts.initopen = 1
	}
	s := &SQL{}
	if opts.timeout < 5 {
		opts.timeout = 5
		if opts.params == "" {
			opts.params = fmt.Sprintf("?timeout=%ds", opts.timeout+5)
		} else {
			opts.params = opts.params + fmt.Sprintf("&timeout=%ds", opts.timeout+5)
		}
	}
	s.timeout = opts.timeout
	switch opts.tcpmode {
	case "tcp":
		// pass
	case "tcp4":
		// pass
	case "tcp6":
		// pass
	default:
		opts.tcpmode = "tcp"
	}
	s.maxOpen = opts.maxopen
	s.DBs = make(chan *DBconn, s.maxOpen)
	s.dsn = fmt.Sprintf("%s:%s@%s(%s)/%s%s", opts.username, opts.password, opts.tcpmode, opts.hostname, opts.dbname, opts.params)
	for i := 0; i < opts.initopen; i++ {
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
	err := db.QueryRow("SELECT o FROM s"+key[:3]+" WHERE h = ? LIMIT 1", key[3:]).Scan(&offsetsStr)
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
	// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
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
} // end func NewConn

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
				// Create table s[0-f][0-f][0-f] with 7-char shortened hash key
				query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `s%s%s%s` (`h` char(7) NOT NULL, `o` LONGTEXT NULL, PRIMARY KEY (`h`)) ENGINE=RocksDB DEFAULT CHARSET=latin1 COLLATE=latin1_bin;", string(c1), string(c2), string(c3))
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
			if dbconn.timeout > utils.UnixTimeSec() {
				db = dbconn.db
				return
			}
			// dbconn is timeout
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
	s.DBs <- &DBconn{db: db, timeout: utils.UnixTimeSec() + s.timeout}
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
		dbconn := <-s.DBs
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

	} // end for
} // end func ClosePool

func (s *SQL) SetMaxOpen(maxopen int) {
	if maxopen < 0 {
		maxopen = 0
	}
	s.ctr.Lock()
	s.maxOpen = maxopen
	s.ctr.Unlock()
	log.Printf("sql.SetMaxOpen=%d", maxopen)
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
