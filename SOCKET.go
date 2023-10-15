package history

import (
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
)

const (
	CR   = "\r"
	LF   = "\n"
	CRLF = CR + LF
)

var (
	SocketPath = "./socket.sock"
)

func (his *HISTORY) startSocket() {
	// Remove the socket file if it already exists
	os.Remove(SocketPath)
	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		log.Printf("Error creating socket err='%v'", err)
		return
	}
	defer listener.Close()
	log.Printf("Listening on Unix socket: %s", SocketPath)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting socket err='%v'", err)
			continue
		}
		go his.handleSocket(conn)
	}
}

func (his *HISTORY) handleSocket(conn net.Conn) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			log.Printf("Error reading from socket err='%v'", err)
			return
		}
		parts := strings.SplitN(line, " ", 1)
		log.Printf("Socket read line='%s' parts='%v'", line, parts)
		// Process the received message here.
		// Send a response.
		response := "Response to: " + line
		err = tp.PrintfLine(response)
		//_, err = conn.Write([]byte(response + CRLF))
		if err != nil {
			log.Printf("Error writing to socket err='%v'", err)
			return
		}
	}
}
