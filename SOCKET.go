package history

import (
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	//"encoding/json"
	"strconv"
)

const (
	CR   = "\r"
	LF   = "\n"
	CRLF = CR + LF
)

var (
	ListenTCP  = "[::1]:49119" // launches a tcp port with a telnet interface
	SocketPath = "./socket.sock"
)

func (his *HISTORY) startSocket(tcpListen string) {
	// Remove the socket file if it already exists
	os.Remove(SocketPath)

	listenerS, err := net.Listen("unix", SocketPath)
	if err != nil {
		log.Printf("Error creating socket err='%v'", err)
		os.Exit(1)
	}
	log.Printf("UnixSocket: %s", SocketPath)

	listenerT, err := net.Listen("tcp", tcpListen)
	if err != nil {
		log.Printf("Error creating tcpListen err='%v'", err)
		os.Exit(1)
	}
	log.Printf("ListenTCP: %s", tcpListen)

	go func(listener net.Listener) {
		defer listener.Close()
		for {
			conn, err := listenerS.Accept()
			if err != nil {
				log.Printf("Error accepting socket err='%v'", err)
				continue
			}
			go his.handleSocketConn(conn, true)
		}

	}(listenerS)
	go func(listener net.Listener) {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting tcp err='%v'", err)
				continue
			}
			go his.handleSocketConn(conn, false)
		}
	}(listenerT)
} // end func startSocket

func (his *HISTORY) handleSocketConn(conn net.Conn, socket bool) {
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
		/*
			case "DUP": // search
				hobj, err := ConvertStringToHistoryObject(ARGS)
				if err != nil {
					log.Printf("ERROR handleConn conn='%#v' ConvertStringToHistoryObject err='%v'", conn, err)
					tp.PrintfLine("400 HOBJ DUP ERR")
					continue forever
				}
				// receives a line containing the HistoryObject values as text
				log.Printf("handleConn DUP '%#v'", hobj)
				// TODO add sequence to IndexQuery() //
		*/
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
				log.Printf("SOCKET handleConnn added=%d", added)
				tadded = 0
			}

		}
	}
	log.Printf("handleConn LEFT: %#v", conn)
} // end func handleConn

// holds connection to historyServer
type RemoteConn struct {
	conn net.Conn
	tp   *textproto.Conn
}

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
