package history

import (
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	CR        = "\r"
	LF        = "\n"
	CRLF      = CR + LF
	ListenTCP = "[::1]:49119" // default launches a tcp port with a telnet interface @ localhost:49119
)

var (
	acl        ACL
	SocketPath = "./socket.sock"
)

func (his *HISTORY) startSocket(tcpListen string) {

	// socket listener
	go func() {
		os.Remove(SocketPath)
		listener, err := net.Listen("unix", SocketPath)
		if err != nil {
			log.Printf("Error creating socket err='%v'", err)
			os.Exit(1)
		}
		log.Printf("UnixSocket: %s", SocketPath)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting socket err='%v'", err)
				continue
			}
			go his.handleSocketConn(conn, "", true)
		}
	}()

	// tcp listener
	go func() {
		acl.SetupACL()
		listener, err := net.Listen("tcp", tcpListen)
		if err != nil {
			log.Printf("Error creating tcpListen err='%v'", err)
			os.Exit(1)
		}
		log.Printf("ListenTCP: %s", tcpListen)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			raddr := getRemoteIP(conn)
			if !checkACL(conn) {
				log.Printf("HistoryServer !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			if err != nil {
				log.Printf("Error accepting tcp err='%v'", err)
				continue
			}
			log.Printf("HistoryServer newC: '%s'", raddr)
			go his.handleSocketConn(conn, raddr, false)
		}
	}()
} // end func startSocket

func (his *HISTORY) handleSocketConn(conn net.Conn, raddr string, socket bool) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	if !socket {
		// send welcome banner to tcp connection
		tp.PrintfLine("200 history")
	}
	//parts := []string{} 77
	//ARGS := []string{}
	//responseChan := make(chan int, 1)
	var added, tadded uint64
forever:
	for {
		line, err := tp.ReadLine()
		if err != nil {
			log.Printf("Error handleConn err='%v'", err)
			break forever
		}
		parts := strings.Split(line, " ")
		if len(parts) == 0 {
			break forever
		}
		CMD := strings.ToUpper(parts[0])
		ARGS := parts[1:]
		//log.Printf("CONN '%#v' read CMD='%s'", conn, CMD)
		// Process the received message here.
		// Send a response.
		switch CMD {
		case "QUIT":
			tp.PrintfLine("205 CIAO")
			break forever
		case "ADD":
			hobj, err := ConvertStringToHistoryObject(ARGS)
			if hobj == nil || err != nil {
				tp.PrintfLine("400 HOBJ ADD ERR")
				log.Printf("400 HOBJ ADD ERR")
				continue forever
			}
			// ADD command receives a line containing the HistoryObject values as text
			//log.Printf("handleConn ADD '%#v'", hobj)
			/* TODO add sequence to AddHistory() */
			tp.PrintfLine("%03d AOK", CaseAdded)
			added++
			tadded++
			if tadded >= 10000 {
				log.Printf("HistoryServer handleConnn: raddr='%s' added=%d", raddr, added)
				tadded = 0
			}
		}
	}
	log.Printf("handleConn LEFT: %#v", conn)
} // end func handleConn

func ConvertStringToHistoryObject(parts []string) (*HistoryObject, error) {
	if len(parts) != 6 {
		return nil, fmt.Errorf("Invalid input string format")
	}
	arrival, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, err
	}
	expires, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return nil, err
	}
	date, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return nil, err
	}
	obj := &HistoryObject{
		MessageIDHash: parts[0],
		StorageToken:  parts[1],
		Char:          parts[2],
		Arrival:       arrival,
		Expires:       expires,
		Date:          date,
	}
	return obj, nil
}

func getRemoteIP(conn net.Conn) string {
	remoteAddr := conn.RemoteAddr()
	if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
		return fmt.Sprintf("%s", tcpAddr.IP)
	}
	return "x"
}

func checkACL(conn net.Conn) bool {
	return acl.IsAllowed(getRemoteIP(conn))
}

type ACL struct {
	mux sync.RWMutex
	acl map[string]bool
}

func (a *ACL) SetupACL() {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a.acl != nil {
		return
	}
	a.acl = make(map[string]bool)
	if DEBUG {
		a.acl["127.0.0.1"] = true
		a.acl["::1"] = true
	}
}

func (a *ACL) IsAllowed(ip string) bool {
	a.mux.RLock()
	retval := a.acl[ip]
	a.mux.RUnlock()
	return retval
}

func (a *ACL) SetACL(ip string, val bool) {
	a.mux.Lock()
	defer a.mux.Unlock()
	if !val { // unset
		delete(a.acl, ip)
		return
	}
	a.acl[ip] = val
}
