package history

import (
	"fmt"
	"log"
	"net"
	"net/textproto"
	"time"
)

// holds connection to historyServer
type RemoteConn struct {
	conn net.Conn
	tp   *textproto.Conn
}

func (his *HISTORY) BootHistoryClient(historyServer string) {
	if historyServer == "" {
		historyServer = ListenTCP
	}
	his.mux.Lock()
	if his.TCPchan != nil {
		return
	}
	his.TCPchan = make(chan *HistoryObject, 128)
	log.Printf("BootHistoryClient historyServer='%s'", historyServer)
	dead := make(chan struct{}, 1)
forever:
	for {
		rconn := his.NewRConn(historyServer)
		if rconn == nil {
			time.Sleep(3 * time.Second)
			continue forever
		}
		go his.handleRConn(dead, rconn.conn, rconn.tp)
		<-dead // locking wait for handleRemote to quit
		time.Sleep(100 * time.Millisecond)
	}
} // end func BootHistoryClient

func (his *HISTORY) NewRConn(historyServer string) *RemoteConn {
	conn, err := net.Dial("tcp", historyServer)
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
			err := tp.PrintfLine("ADD %s", ConvertHistoryObjectToString(hobj))
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
	return fmt.Sprintf("%s %s %s %d %d %d", obj.MessageIDHash, obj.StorageToken, obj.Char, obj.Arrival, obj.Expires, obj.Date)
}
