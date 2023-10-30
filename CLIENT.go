package history

import (
	"fmt"
	"log"
	"net"
	"net/textproto"
	"strings"
	"time"
)

const (
	MinRetryWaiter = 100
)

var (
	BootHisCli           bool
	DefaultHistoryServer = "[::1]:49119" // localhost:49119
	// set only once before boot
	TCPchanQ           = 128
	DefaultDialTimeout = 5   // seconds
	DefaultRetryWaiter = 500 // milliseconds
	DefaultDialRetries = -1  // try N times and fail or <= 0 enables infinite retry
)

// holds connection to historyServer
type RemoteConn struct {
	conn net.Conn
	tp   *textproto.Conn
}

func (his *HISTORY) BootHistoryClient(historyServer string) {
	// one can launch as many clients to historyServer as needed
	his.mux.Lock()
	if his.TCPchan != nil {
		return
	}
	his.TCPchan = make(chan *HistoryObject, TCPchanQ)
	his.mux.Unlock()

	if historyServer == "" {
		// can be 'historyServer:port' or path to 'unix.socket'
		//historyServer = DefaultHistoryServer
		historyServer = DefaultSocketPath
	}
	log.Printf("...connecting to historyServer='%s'", historyServer)
	dead := make(chan struct{}, 1)
	failed := 0
	if DefaultRetryWaiter <= MinRetryWaiter {
		DefaultRetryWaiter = MinRetryWaiter
	}

forever:
	for {
		rconn := his.NewRConn(historyServer)
		if rconn == nil {
			if DefaultDialRetries > 0 {
				if failed >= DefaultDialRetries {
					break forever
				}
				failed++
			}
			time.Sleep(time.Duration(DefaultRetryWaiter) * time.Millisecond)
			continue forever
		}
		failed = 0
		go his.handleRConn(dead, rconn.conn, rconn.tp)
		<-dead // blocking wait for handleRConn to quit
	} // end forever
} // end func BootHistoryClient

func (his *HISTORY) NewRConn(historyServer string) *RemoteConn {
	mode := "tcp"
	if strings.HasSuffix(historyServer, ".socket") {
		mode = "unix"
	}
	conn, err := net.DialTimeout(mode, historyServer, time.Duration(DefaultDialTimeout)*time.Second)
	if err != nil {
		log.Printf("Error NewConn Dial err='%v'", err)
		return nil
	}
	tp := textproto.NewConn(conn)
	line, err := tp.ReadLine()
	if err != nil {
		log.Printf("Error NewConn err='%v'", err)
		return nil
	}
	if line != "200 history" {
		log.Printf("Error in NewConn response")
		return nil
	}
	log.Printf("Connected to historyServer='%s'", historyServer)
	return &RemoteConn{conn: conn, tp: tp}
} // end func NewRConn

func (his *HISTORY) handleRConn(dead chan struct{}, conn net.Conn, tp *textproto.Conn) {
	defer conn.Close()
forever:
	for {
		select {
		case hobj := <-his.TCPchan: // receives a HistoryObject from another world
			//log.Printf("handleRConn TCPchan received hobj='%#v'", hobj)
			if hobj == nil {
				// received nil pointer, closing rconn
				break forever
			}
			// send add command to HistoryServer
			hobjStr := ConvertHistoryObjectToString(hobj)
			err := tp.PrintfLine("ADD %s %s", CRC(hobjStr), hobjStr)
			if err != nil {
				break forever
			}
			// reads reply
			isDup, message, err := tp.Reader.ReadCodeLine(CaseAdded)
			if err != nil && isDup <= 0 {
				log.Printf("ERROR handleRConn ReadCodeLine err='%v' code=%d msg=%s", err, isDup, message)
			}
			// returns reply up to ResponseChan
			if hobj.ResponseChan != nil {
				hobj.ResponseChan <- isDup
			}
		}
	}
	dead <- struct{}{}
	log.Printf("handleRConn closed '%#v", conn)
} // end func handleRemote

func ConvertHistoryObjectToString(obj *HistoryObject) string {
	return fmt.Sprintf("%s %s %d %d %d", obj.MessageIDHash, obj.StorageToken, obj.Arrival, obj.Expires, obj.Date)
}
