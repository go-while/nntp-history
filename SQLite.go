package history

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-while/go-utils"
	_ "github.com/mattn/go-sqlite3"
)

type SQLite3DB struct {
	//mux     sync.RWMutex
	ctr     sync.RWMutex // counter
	DBs     chan *SQLite3Conn
	dsn     string
	maxOpen int
	isOpen  int
	timeout int64
	dbPath  string
}

type SQLite3Conn struct {
	db      *sql.DB
	timeout int64
}

type SQLite3Opts struct {
	dbPath   string
	params   string
	maxOpen  int
	initOpen int
	timeout  int64
}

func NewSQLite3Pool(opts *SQLite3Opts, createTables bool) (*SQLite3DB, error) {
	if opts.maxOpen <= 0 {
		opts.maxOpen = 8 // SQLite works better with fewer connections
	}
	if createTables && opts.initOpen <= 0 {
		opts.initOpen = 1
	}

	s := &SQLite3DB{dbPath: opts.dbPath}
	if opts.timeout < 5 {
		opts.timeout = 5
	}
	s.timeout = opts.timeout
	s.maxOpen = opts.maxOpen
	s.DBs = make(chan *SQLite3Conn, s.maxOpen)

	// Build DSN with optimizations
	if opts.params == "" {
		opts.params = "?cache=shared&mode=rwc&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=100000&_temp_store=memory&_mmap_size=268435456"
	}
	s.dsn = opts.dbPath + opts.params

	// Initialize connection pool
	for i := 0; i < opts.initOpen; i++ {
		db, err := s.GetDB(false)
		if err != nil {
			return nil, err
		}
		s.isOpen++
		s.ReturnDB(db)
	}

	if createTables {
		if err := s.CreateTables(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *SQLite3DB) NewConn() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", s.dsn)
	if err != nil {
		log.Printf("ERROR SQLite3 NewConn 'open' failed err='%v'", err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		log.Printf("ERROR SQLite3 NewConn 'ping' failed err='%v'", err)
		return nil, err
	}

	// SQLite3-specific optimizations for RocksDB-like performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",        // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",      // Balanced durability/performance
		"PRAGMA cache_size=100000",       // Large cache for better performance
		"PRAGMA temp_store=memory",       // Store temp tables in memory
		"PRAGMA mmap_size=268435456",     // 256MB memory mapping
		"PRAGMA page_size=4096",          // Optimal page size
		"PRAGMA auto_vacuum=INCREMENTAL", // Prevent database bloat
		"PRAGMA busy_timeout=30000",      // 30 second busy timeout
		"PRAGMA wal_autocheckpoint=1000", // Checkpoint every 1000 pages
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("WARN SQLite3 NewConn pragma failed: %s, err='%v'", pragma, err)
		}
	}

	return db, nil
}

func (s *SQLite3DB) CreateTables() error {
	db, err := s.GetDB(true)
	if err != nil {
		return err
	}
	defer s.ReturnDB(db)

	// Create tables for all 3-character combinations (4096 tables)
	cs := "0123456789abcdef"
	for _, c1 := range cs {
		for _, c2 := range cs {
			for _, c3 := range cs {
				tableName := fmt.Sprintf("s%s%s%s", string(c1), string(c2), string(c3))
				query := fmt.Sprintf(`
					CREATE TABLE IF NOT EXISTS %s (
						h CHAR(7) NOT NULL PRIMARY KEY,
						o TEXT
					) WITHOUT ROWID;
				`, tableName)

				_, err := db.Exec(query)
				if err != nil {
					log.Printf("ERROR SQLite3 CreateTables query='%s' err='%v'", query, err)
					return err
				}

				// Create index for better performance
				indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_h ON %s(h);", tableName, tableName)
				_, err = db.Exec(indexQuery)
				if err != nil {
					log.Printf("WARN SQLite3 CreateTables index failed for table %s: %v", tableName, err)
				}
			}
		}
	}

	log.Printf("SQLite3 created 4096 tables successfully")
	return nil
}

func (s *SQLite3DB) InsertOffset(key string, offset int64, db *sql.DB) error {
	if db == nil {
		adb, err := s.GetDB(true)
		if err != nil {
			return err
		}
		db = adb
		defer s.ReturnDB(db)
	}

	tableName := fmt.Sprintf("s%s", key[:3])
	hashKey := key[3:]

	// Use UPSERT (INSERT OR REPLACE) with concatenation
	query := fmt.Sprintf(`
		INSERT INTO %s (h, o) VALUES (?, ?)
		ON CONFLICT(h) DO UPDATE SET o = COALESCE(o, '') || ?
	`, tableName)

	offsetStr := fmt.Sprintf("%d,", offset)
	_, err := db.Exec(query, hashKey, offsetStr, offsetStr)
	if err != nil {
		log.Printf("ERROR SQLite3 InsertOffset table=%s key=%s offset=%d err='%v'", tableName, hashKey, offset, err)
		return err
	}
	return nil
}

func (s *SQLite3DB) GetOffsets(key string, db *sql.DB) ([]int64, error) {
	if db == nil {
		adb, err := s.GetDB(true)
		if err != nil {
			return nil, err
		}
		db = adb
		defer s.ReturnDB(db)
	}

	tableName := fmt.Sprintf("s%s", key[:3])
	hashKey := key[3:]

	var offsetsStr string
	query := fmt.Sprintf("SELECT o FROM %s WHERE h = ? LIMIT 1", tableName)
	err := db.QueryRow(query, hashKey).Scan(&offsetsStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		log.Printf("ERROR SQLite3 GetOffsets table=%s key=%s err='%v'", tableName, hashKey, err)
		return nil, err
	}

	var offsets []int64
	if len(offsetsStr) > 0 {
		offs := strings.Split(offsetsStr, ",")
		for _, offsetStr := range offs {
			if offsetStr != "" {
				offset := utils.Str2int64(offsetStr)
				if offset > 0 {
					offsets = append(offsets, offset)
				}
			}
		}
	}
	return offsets, nil
}

func (s *SQLite3DB) GetDB(wait bool) (db *sql.DB, err error) {
	if wait {
		s.ctr.RLock()
		if s.isOpen > 0 {
			s.ctr.RUnlock()
			dbconn := <-s.DBs
			if dbconn.timeout > utils.UnixTimeSec() {
				db = dbconn.db
				return
			}
			// dbconn is timeout, check if still valid
			if dbconn.db != nil {
				err = dbconn.db.Ping()
				if err != nil {
					log.Printf("WARN SQLite3 GetDB 'ping' failed err='%v'", err)
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
	}

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
	}
	return
}

func (s *SQLite3DB) ReturnDB(db *sql.DB) {
	s.DBs <- &SQLite3Conn{db: db, timeout: utils.UnixTimeSec() + s.timeout}
}

func (s *SQLite3DB) CloseDB(db *sql.DB) {
	db.Close()
	s.ctr.Lock()
	s.isOpen--
	s.ctr.Unlock()
}

func (s *SQLite3DB) ClosePool() {
	log.Printf("SQLite3 ClosePool")
	defer log.Printf("SQLite3 ClosePool returned")

	close(s.DBs)
	for dbconn := range s.DBs {
		if dbconn.db != nil {
			dbconn.db.Close()
		}
		s.ctr.Lock()
		s.isOpen--
		s.ctr.Unlock()
	}
}

// Initialize SQLite3 for history system
func (his *HISTORY) InitSQLite3() error {
	return his.InitSQLite3WithSharding(SHARD_SINGLE_DB)
}

// Initialize SQLite3 with specific sharding mode
func (his *HISTORY) InitSQLite3WithSharding(shardMode int) error {
	if shardMode == SHARD_SINGLE_DB {
		// Use original single-database implementation for backward compatibility
		opts := &SQLite3Opts{
			dbPath:   his.DIR + "/hashdb.sqlite3",
			params:   "?cache=shared&mode=rwc&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=100000&_temp_store=memory&_mmap_size=268435456",
			maxOpen:  8, // SQLite works better with fewer connections
			initOpen: 2,
			timeout:  30,
		}

		pool, err := NewSQLite3Pool(opts, true)
		if err != nil {
			return fmt.Errorf("failed to initialize SQLite3 pool: %v", err)
		}

		// Store as interface{} to avoid type issues with build tags
		his.SQLite3Pool = pool

		log.Printf("SQLite3 single-database pool initialized successfully at %s", opts.dbPath)
	} else {
		// Use sharded database implementation
		config := &ShardConfig{
			Mode:         shardMode,
			BaseDir:      his.DIR,
			MaxOpenPerDB: 8, // SQLite works better with fewer connections per DB
			Timeout:      30,
		}

		shardedDB, err := NewSQLite3ShardedDB(config, true)
		if err != nil {
			return fmt.Errorf("failed to initialize SQLite3 sharded system: %v", err)
		}

		// Store as interface{} to maintain compatibility
		his.SQLite3Pool = shardedDB

		numDBs, tablesPerDB, description := GetShardConfig(shardMode)
		log.Printf("SQLite3 sharded system initialized: %s (%d DBs, %d tables per DB)",
			description, numDBs, tablesPerDB)
	}
	return nil
}

// GetSQLite3Pool returns the SQLite3 pool if available
func (his *HISTORY) GetSQLite3Pool() *SQLite3DB {
	if his.SQLite3Pool == nil {
		return nil
	}
	if pool, ok := his.SQLite3Pool.(*SQLite3DB); ok {
		return pool
	}
	return nil
}

// Optimize SQLite3 database periodically
func (s *SQLite3DB) OptimizeDB() {
	db, err := s.GetDB(true)
	if err != nil {
		log.Printf("ERROR SQLite3 OptimizeDB failed to get connection: %v", err)
		return
	}
	defer s.ReturnDB(db)

	optimizations := []string{
		"PRAGMA optimize",
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"PRAGMA incremental_vacuum",
	}

	for _, opt := range optimizations {
		if _, err := db.Exec(opt); err != nil {
			log.Printf("WARN SQLite3 OptimizeDB failed: %s, err='%v'", opt, err)
		}
	}
}

// Start periodic optimization
func (s *SQLite3DB) StartOptimizer() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			s.OptimizeDB()
		}
	}()
}
