package history
/*
import (
	"fmt"
	"log"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"sync"
	"time"
)

type SQLiteDB struct {
	dbmux sync.Mutex
	DB *sql.DB
	locked bool
}

type WorkerData struct {
	table string
	data map[string][]int64 // key, offsets
}

type SQLite struct {
	mux sync.RWMutex
	MaxOpen int
	DBs map[string]*SQLiteDB // table
	DIR string // /history/hashdb/
	DSN string
	InputCh chan *OffsetData
	WorksCh chan *WorkerData // batchmap: key, offsets
} // end func SQLite

func NewSQLite(maxopen int, params string, dir string) *SQLite {
	if maxopen <= 0 {
		log.Printf("NewSQLitehandler maxopen = 0")
		return nil
	}
	s := &SQLite{}
	s.MaxOpen = maxopen
	s.DBs = make(map[string]*SQLiteDB)
	s.DIR = dir
	s.InputCh = make(chan *OffsetData, s.MaxOpen)
	s.WorksCh = make(chan *WorkerData, s.MaxOpen)
	go s.MainWorker()
	for id := 1; id <= s.MaxOpen; id++ {
		go s.DBWorker(id)
	}
	return s
} // end func NewSQLite

func (s *SQLite) MainWorker() {
	batchmap := make(map[string]map[string][]int64, 1024) // table, key, offsets
	timeout := time.NewTimer(1*time.Second)
	forever:
	for {
		select {
			case <- timeout.C:
				log.Printf("MainWorker alive")
				timeout = time.NewTimer(1*time.Second)
				if len(batchmap) == 0 {
					continue forever
				}
				for table, keyoff := range batchmap {
					s.WorksCh <- &WorkerData {
						table: table,
						data: keyoff,
					}
				}
				batchmap = make(map[string]map[string][]int64, 1024)

			case od := <- s.InputCh: // OffsetData
				table := od.Shorthash[:3]
				key := od.Shorthash[3:]
				log.Printf("SQLite MainWorker recv od='%#v'", od)
				if batchmap[table] == nil {
					batchmap[table] = make(map[string][]int64, 256)
				}
				batchmap[table][key] = append(batchmap[table][key], od.Offset)
				if len(batchmap[table]) >= cap(s.WorksCh)/2 {
					s.WorksCh <- &WorkerData{ table: table, data: batchmap[table] }
					batchmap[table] = nil
				}
		}// end select
	}
} // end func SQLite MainWorker

func (s *SQLite) CreateTables(table string, db *sql.DB) (error) {
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `h_%s` (`hash` char(5) NOT NULL primary key, `offs` TEXT NULL)", table)
	_, err := db.Exec(query);
	if err != nil {
		log.Printf("ERROR SQLite CreateTables query err='%v'", err)
		return nil
	}
	return nil
} // end func CreateTables

func (s *SQLite) INSERT(table string, key string, offset int64, db *sql.DB) (error) {
	query := fmt.Sprintf("INSERT INTO `h_%s` (`hash`,`offs`) VALUES (`%s`,`%d,`) ON DUPLICATE KEY UPDATE offs=CONCAT(offs, '%d,')", table, key, offset, offset)
	log.Printf("HashDB_InsertHash2OffsetSQLite query='%s'", query)
	_, err := db.Exec(query);
	if err != nil {
		log.Printf("ERROR SQLite INSERT err='%v'", err)
		return nil
	}
	// TODO cacheEvict
	return nil
} // end func HashDB_InsertHash2OffsetSQLite

func (s *SQLite) DBWorker(id int) {
	log.Printf("Boot SQLite DBWorker [%d]", id)
	defer log.Printf("QUIT SQLite DBWorker [%d]", id)
	var queue []*WorkerData
	var work *WorkerData
	timeout := time.NewTimer(1*time.Second)
	forever:
	for {
		select {
			case <- timeout.C:
				timeout = time.NewTimer(1*time.Second)
				if len(queue) > 0 {
					work = queue[0]
					queue[0] = nil
					queue = queue[1:]
				}
				// pass
			case work = <- s.WorksCh: // *WorkerData
				if len(queue) > 0 {
					queue = append(queue, work)
					work = queue[0]
					queue[0] = nil
					queue = queue[1:]
				}
				log.Printf("DBWorker [%d] work='%#v'", id, work)
				// pass
		} // end select

		s.mux.Lock()
		if s.DBs[work.table] == nil {
			s.DBs[work.table] = &SQLiteDB{}
		}
		s.mux.Unlock()

		s.DBs[work.table].dbmux.Lock()
		if s.DBs[work.table].locked {
			s.DBs[work.table].dbmux.Unlock()
			queue = append(queue, work)
			continue forever
		}
		s.DBs[work.table].locked = true
		s.DBs[work.table].dbmux.Unlock()

		switch s.DBs[work.table].DB {
			case nil:
				// is not open, open this DB
				//dsn := "file:"+work.table+".db"
				db, err := sql.Open("sqlite3", "file:"+work.table+".db")
				if err != nil {
					log.Printf("ERROR SQLite DBWorker [%d] 'open' failed err='%v'", id, err)
					return
				}
				if err := s.CreateTables(work.table, db); err != nil {
					return
				}
				s.DBs[work.table].DB = db
			default:
				// db is open
		} // end switch

		for key, offs := range work.data {
			for _, offset := range offs {
				err := s.INSERT(work.table, key, offset, s.DBs[work.table].DB)
				if err != nil {
					return
				}
			}
		}
	} // end forever
} // end func SQLite DBWorker
*/
