package history

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

// Sharding configuration constants
const (
	SHARD_SINGLE_DB  = 0 // 1 DB with 4096 tables (current default)
	SHARD_FULL_SPLIT = 1 // 4096 separate DBs
	SHARD_16_256     = 2 // 16 DBs with 256 tables each
	SHARD_64_64      = 3 // 64 DBs with 64 tables each
	SHARD_128_32     = 4 // 128 DBs with 32 tables each
	SHARD_512_8      = 5 // 512 DBs with 8 tables each
)

// SQLite3ShardedDB manages multiple SQLite databases for sharding
type SQLite3ShardedDB struct {
	//mux         sync.RWMutex
	//ctr         sync.RWMutex
	DBPools     []*SQLite3DB // Array of database pools
	shardMode   int
	numDBs      int
	tablesPerDB int
	baseDir     string
	maxOpen     int
	timeout     int64
}

// ShardConfig defines the sharding configuration
type ShardConfig struct {
	Mode         int    // Sharding mode (0-5)
	BaseDir      string // Base directory for database files
	MaxOpenPerDB int    // Max connections per database
	Timeout      int64  // Connection timeout
}

// GetShardConfig returns the configuration for a given shard mode
func GetShardConfig(mode int) (numDBs, tablesPerDB int, description string) {
	switch mode {
	case SHARD_SINGLE_DB:
		return 1, 4096, "1 DB with 4096 tables"
	case SHARD_FULL_SPLIT:
		return 4096, 1, "4096 separate DBs"
	case SHARD_16_256:
		return 16, 256, "16 DBs with 256 tables each"
	case SHARD_64_64:
		return 64, 64, "64 DBs with 64 tables each"
	case SHARD_128_32:
		return 128, 32, "128 DBs with 32 tables each"
	case SHARD_512_8:
		return 512, 8, "512 DBs with 8 tables each"
	default:
		return 1, 4096, "1 DB with 4096 tables (default)"
	}
}

// NewSQLite3ShardedDB creates a new sharded SQLite3 database system
func NewSQLite3ShardedDB(config *ShardConfig, createTables bool) (*SQLite3ShardedDB, error) {
	numDBs, tablesPerDB, description := GetShardConfig(config.Mode)

	log.Printf("Initializing SQLite3 sharded system: %s", description)

	if config.MaxOpenPerDB <= 0 {
		config.MaxOpenPerDB = 8 // SQLite works better with fewer connections
	}
	if config.Timeout < 5 {
		config.Timeout = 5
	}

	s := &SQLite3ShardedDB{
		shardMode:   config.Mode,
		numDBs:      numDBs,
		tablesPerDB: tablesPerDB,
		baseDir:     config.BaseDir,
		maxOpen:     config.MaxOpenPerDB,
		timeout:     config.Timeout,
		DBPools:     make([]*SQLite3DB, numDBs),
	}

	// Initialize database pools
	for i := 0; i < numDBs; i++ {
		var dbPath string
		switch numDBs {
		case 1:
			// Single database mode
			dbPath = filepath.Join(config.BaseDir, "hashdb.sqlite3")
		case 4096:
			// Full split mode: use 3-char hex names
			dbPath = filepath.Join(config.BaseDir, fmt.Sprintf("hashdb_%s.sqlite3",
				s.getDBNameFromIndex(i)))
		default:
			// Hybrid modes: use simple numbering
			dbPath = filepath.Join(config.BaseDir, fmt.Sprintf("hashdb_%03d.sqlite3", i))
		}

		opts := &SQLite3Opts{
			dbPath:   dbPath,
			params:   buildOptimizedParams(config.Mode),
			maxOpen:  config.MaxOpenPerDB,
			initOpen: 1,
			timeout:  config.Timeout,
		}

		pool, err := NewSQLite3Pool(opts, false) // Don't create tables yet
		if err != nil {
			return nil, fmt.Errorf("failed to create database pool %d: %v", i, err)
		}
		s.DBPools[i] = pool
	}

	if createTables {
		if err := s.CreateAllTables(); err != nil {
			return nil, err
		}
	}

	log.Printf("SQLite3 sharded system initialized successfully: %d databases, %d tables per DB",
		numDBs, tablesPerDB)
	return s, nil
}

// getDBNameFromIndex converts index to 3-character hex name for 4096 DB mode
func (s *SQLite3ShardedDB) getDBNameFromIndex(index int) string {
	if index < 0 || index >= 4096 {
		return "000"
	}
	return fmt.Sprintf("%03x", index)
}

// hexToInt converts a hex string to int
func hexToInt(hexStr string) (int, error) {
	val, err := strconv.ParseInt(hexStr, 16, 64)
	return int(val), err
}

// getDBIndexFromHash determines which database to use based on hash
func (s *SQLite3ShardedDB) getDBIndexFromHash(hash string) int {
	if len(hash) < 3 {
		return 0
	}

	switch s.shardMode {
	case SHARD_SINGLE_DB:
		return 0 // Always use the single database
	case SHARD_FULL_SPLIT:
		// Use first 3 characters to determine DB (4096 options)
		prefix := hash[:3]
		if val, err := hexToInt(prefix); err == nil {
			return val % s.numDBs
		}
		return 0
	case SHARD_16_256:
		// Use first character (16 options)
		if val, err := hexToInt(string(hash[0])); err == nil {
			return val % s.numDBs
		}
		return 0
	case SHARD_64_64:
		// Use first 1.5 characters (64 options)
		prefix := hash[:2]
		if val, err := hexToInt(prefix); err == nil {
			return (val / 4) % s.numDBs
		}
		return 0
	case SHARD_128_32:
		// Use first 2 characters, divide by 2 (128 options)
		prefix := hash[:2]
		if val, err := hexToInt(prefix); err == nil {
			return (val / 2) % s.numDBs
		}
		return 0
	case SHARD_512_8:
		// Use first 2.25 characters (512 options)
		prefix := hash[:3]
		if val, err := hexToInt(prefix); err == nil {
			return (val / 8) % s.numDBs
		}
		return 0
	default:
		return 0
	}
}

// getTableNameFromHash determines which table to use within a database
func (s *SQLite3ShardedDB) getTableNameFromHash(hash string) string {
	if len(hash) < 3 {
		return "s000"
	}

	switch s.shardMode {
	case SHARD_SINGLE_DB:
		// Use first 3 characters for table name (4096 tables)
		return fmt.Sprintf("s%s", hash[:3])
	case SHARD_FULL_SPLIT:
		// Each DB has only one table
		return "shash"
	case SHARD_16_256:
		// Use characters 2-3 for table name within DB (256 tables per DB)
		if len(hash) >= 3 {
			return fmt.Sprintf("s%s", hash[1:3])
		}
		return "s00"
	case SHARD_64_64:
		// Use bits of character 2-3 for table name (64 tables per DB)
		if len(hash) >= 3 {
			prefix := hash[1:3]
			if val, err := hexToInt(prefix); err == nil {
				return fmt.Sprintf("s%02x", val%64)
			}
		}
		return "s00"
	case SHARD_128_32:
		// Use lower bits for table name (32 tables per DB)
		if len(hash) >= 3 {
			prefix := hash[1:3]
			if val, err := hexToInt(prefix); err == nil {
				return fmt.Sprintf("s%02x", val%32)
			}
		}
		return "s00"
	case SHARD_512_8:
		// Use lower bits for table name (8 tables per DB)
		if len(hash) >= 3 {
			prefix := hash[2:3]
			if val, err := hexToInt(prefix); err == nil {
				return fmt.Sprintf("s%01x", val%8)
			}
		}
		return "s0"
	default:
		return "s000"
	}
}

// CreateAllTables creates all required tables across all databases
func (s *SQLite3ShardedDB) CreateAllTables() error {
	log.Printf("Creating tables for sharding mode %d (%d databases, %d tables per DB)",
		s.shardMode, s.numDBs, s.tablesPerDB)

	for dbIndex := 0; dbIndex < s.numDBs; dbIndex++ {
		if err := s.createTablesForDB(dbIndex); err != nil {
			return fmt.Errorf("failed to create tables for database %d: %v", dbIndex, err)
		}
	}

	log.Printf("Successfully created all tables across %d databases", s.numDBs)
	return nil
}

// createTablesForDB creates tables for a specific database
func (s *SQLite3ShardedDB) createTablesForDB(dbIndex int) error {
	db, err := s.DBPools[dbIndex].GetDB(true)
	if err != nil {
		return err
	}
	defer s.DBPools[dbIndex].ReturnDB(db)

	if s.shardMode == SHARD_FULL_SPLIT {
		// Each DB has only one table
		tableName := "shash"
		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				h CHAR(7) NOT NULL PRIMARY KEY,
				o TEXT
			) WITHOUT ROWID;
		`, tableName)

		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to create table %s: %v", tableName, err)
		}

		// Create index
		indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_h ON %s(h);", tableName, tableName)
		_, err = db.Exec(indexQuery)
		if err != nil {
			log.Printf("WARN SQLite3 index creation failed for table %s: %v", tableName, err)
		}
	} else {
		// Create multiple tables per database
		tableNames := s.getTableNamesForDB()
		for _, tableName := range tableNames {
			query := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					h CHAR(7) NOT NULL PRIMARY KEY,
					o TEXT
				) WITHOUT ROWID;
			`, tableName)

			_, err := db.Exec(query)
			if err != nil {
				return fmt.Errorf("failed to create table %s: %v", tableName, err)
			}

			// Create index
			indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_h ON %s(h);", tableName, tableName)
			_, err = db.Exec(indexQuery)
			if err != nil {
				log.Printf("WARN SQLite3 index creation failed for table %s: %v", tableName, err)
			}
		}
	}

	return nil
}

// getTableNamesForDB returns all table names for a specific database
func (s *SQLite3ShardedDB) getTableNamesForDB() []string {
	var tableNames []string

	switch s.shardMode {
	case SHARD_SINGLE_DB:
		// All 4096 tables in one database
		cs := "0123456789abcdef"
		for _, c1 := range cs {
			for _, c2 := range cs {
				for _, c3 := range cs {
					tableNames = append(tableNames, fmt.Sprintf("s%s%s%s", string(c1), string(c2), string(c3)))
				}
			}
		}
	case SHARD_16_256:
		// 256 tables per database
		cs := "0123456789abcdef"
		for _, c1 := range cs {
			for _, c2 := range cs {
				tableNames = append(tableNames, fmt.Sprintf("s%s%s", string(c1), string(c2)))
			}
		}
	case SHARD_64_64:
		// 64 tables per database
		for i := 0; i < 64; i++ {
			tableNames = append(tableNames, fmt.Sprintf("s%02x", i))
		}
	case SHARD_128_32:
		// 32 tables per database
		for i := 0; i < 32; i++ {
			tableNames = append(tableNames, fmt.Sprintf("s%02x", i))
		}
	case SHARD_512_8:
		// 8 tables per database
		for i := 0; i < 8; i++ {
			tableNames = append(tableNames, fmt.Sprintf("s%01x", i))
		}
	}

	return tableNames
}

// GetDBAndTable returns the appropriate database connection and table name for a hash
func (s *SQLite3ShardedDB) GetDBAndTable(hash string) (*sql.DB, string, int, error) {
	dbIndex := s.getDBIndexFromHash(hash)
	tableName := s.getTableNameFromHash(hash)

	db, err := s.DBPools[dbIndex].GetDB(false)
	if err != nil {
		return nil, "", dbIndex, err
	}

	return db, tableName, dbIndex, nil
}

// ReturnDB returns a database connection to its pool
func (s *SQLite3ShardedDB) ReturnDB(db *sql.DB, dbIndex int) {
	if dbIndex >= 0 && dbIndex < len(s.DBPools) {
		s.DBPools[dbIndex].ReturnDB(db)
	}
}

// Close closes all database connections
func (s *SQLite3ShardedDB) Close() error {
	for _, pool := range s.DBPools {
		if pool != nil {
			pool.ClosePool()
		}
	}
	return nil
}

// GetStats returns statistics for all databases
func (s *SQLite3ShardedDB) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"shard_mode":             s.shardMode,
		"num_databases":          s.numDBs,
		"tables_per_db":          s.tablesPerDB,
		"total_tables":           s.numDBs * s.tablesPerDB,
		"max_connections_per_db": s.maxOpen,
	}

	// Add per-database statistics
	dbStats := make([]map[string]interface{}, s.numDBs)
	for i, pool := range s.DBPools {
		if pool != nil {
			dbStats[i] = map[string]interface{}{
				"db_index": i,
				"is_open":  pool.isOpen,
				"db_path":  pool.dbPath,
			}
		}
	}
	stats["databases"] = dbStats

	return stats
}

// getAdaptiveCacheSize returns optimal cache_size for each sharding mode
func getAdaptiveCacheSize(mode int) int {
	switch mode {
	case SHARD_SINGLE_DB:
		return 100000 // 400 MB - full cache for single DB
	case SHARD_FULL_SPLIT:
		return 500 // 2 MB per DB - Total: 4096 × 2 MB = 8 GB (still high but better)
	case SHARD_16_256:
		return 2000 // 8 MB per DB - Total: 16 × 8 MB = 128 MB
	case SHARD_64_64:
		return 1000 // 4 MB per DB - Total: 64 × 4 MB = 256 MB
	case SHARD_128_32:
		return 500 // 2 MB per DB - Total: 128 × 2 MB = 256 MB
	case SHARD_512_8:
		return 250 // 1 MB per DB - Total: 512 × 1 MB = 512 MB
	default:
		return 100000 // Default to single DB cache size
	}
}

// buildOptimizedParams creates SQLite parameters with adaptive cache sizing
func buildOptimizedParams(mode int) string {
	cacheSize := getAdaptiveCacheSize(mode)
	return fmt.Sprintf("?cache=shared&mode=rwc&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=%d&_temp_store=memory&_mmap_size=268435456", cacheSize)
}

// GetMemoryInfo returns memory usage information for the current sharding configuration
func (s *SQLite3ShardedDB) GetMemoryInfo() map[string]interface{} {
	cacheSize := getAdaptiveCacheSize(s.shardMode)
	cacheMBPerDB := (cacheSize * 4096) / (1024 * 1024) // pages * 4KB / 1MB
	totalCacheMB := cacheMBPerDB * s.numDBs
	connectionsMB := s.numDBs * s.maxOpen * 1 / 10 // ~0.1 MB per connection

	var mode string
	switch s.shardMode {
	case SHARD_SINGLE_DB:
		mode = "Single DB"
	case SHARD_FULL_SPLIT:
		mode = "Full Split"
	case SHARD_16_256:
		mode = "16×256"
	case SHARD_64_64:
		mode = "64×64"
	case SHARD_128_32:
		mode = "128×32"
	case SHARD_512_8:
		mode = "512×8"
	default:
		mode = "Unknown"
	}

	return map[string]interface{}{
		"mode":               mode,
		"shard_mode":         s.shardMode,
		"num_databases":      s.numDBs,
		"tables_per_db":      s.tablesPerDB,
		"cache_size_pages":   cacheSize,
		"cache_mb_per_db":    cacheMBPerDB,
		"total_cache_mb":     totalCacheMB,
		"connections_mb":     connectionsMB,
		"estimated_total_mb": totalCacheMB + connectionsMB + 5, // +5 MB for Go runtime
	}
}
