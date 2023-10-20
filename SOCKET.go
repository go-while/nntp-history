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
	CR                = "\r"
	LF                = "\n"
	CRLF              = CR + LF
	DefaultSocketPath = "./history.socket"
	// default launches a tcp port with a telnet interface @ localhost:49119
	DefaultServerTCPAddr = "[::]:49119"
)

var (
	ACL        AccessControlList
	DefaultACL map[string]bool // can be set before booting
)

func (his *HISTORY) startServer(tcpListen string, socketPath string) {
	if BootHisCli {
		return
	}
	if his.useHashDB {
		his.Wait4HashDB()
	}
	// socket listener
	go func() {
		if socketPath == "" {
			return
		}
		os.Remove(socketPath)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Printf("ERROR HistoryServer  creating socket err='%v'", err)
			os.Exit(1)
		}
		log.Printf("HistoryServer UnixSocket: %s", socketPath)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("ERROR HistoryServer accepting socket err='%v'", err)
				return
			}
			go his.handleSocketConn(conn, "", true)
		}
	}()

	// tcp listener
	go func() {
		if tcpListen == "" {
			return
		}
		ACL.SetupACL()
		listener, err := net.Listen("tcp", tcpListen)
		if err != nil {
			log.Printf("ERROR HistoryServer creating tcpListen err='%v'", err)
			os.Exit(1)
		}
		log.Printf("HistoryServer ListenTCP: %s", tcpListen)
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			raddr := getRemoteIP(conn)
			if err != nil {
				log.Printf("ERROR HistoryServer  accepting tcp err='%v'", err)
				return
			}
			if !checkACL(conn) {
				log.Printf("HistoryServer !ACL: '%s'", raddr)
				conn.Close()
				continue
			}
			log.Printf("HistoryServer newC: '%s'", raddr)
			go his.handleSocketConn(conn, raddr, false)
		}
	}()
} // end func startServer

func (his *HISTORY) handleSocketConn(conn net.Conn, raddr string, socket bool) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	if !socket {
		// send welcome banner to incoming tcp connection
		err := tp.PrintfLine("200 history")
		if err != nil {
			return
		}
	}
	//parts := []string{} 77
	//ARGS := []string{}
	indexRetChan := make(chan int, 1)
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
		//ARGS := parts[1:]
		//log.Printf("CONN '%#v' read CMD='%s' line='%s' parts=%d", conn, CMD, line, len(parts))
		// Process the received message here.
		switch CMD {
		case "CPU": // start/stop cpu profiling
			his.mux.Lock()
			if his.CPUfile != nil {
				his.stopCPUProfile(his.CPUfile)
				tp.PrintfLine("200 OK stopCPUProfile")
				his.CPUfile = nil
			} else {
				CPUfile, err := his.startCPUProfile()
				if err != nil || CPUfile == nil {
					log.Printf("ERROR SOCKET CMD startCPUProfile err='%v'", err)
					tp.PrintfLine("400 ERR startCPUProfile")
				} else {
					his.CPUfile = CPUfile
					tp.PrintfLine("200 OK startCPUProfile")
				}
			}
			his.mux.Unlock()
		case "STOP":
			his.CLOSE_HISTORY()
			tp.PrintfLine("502 CLOSE_HISTORY")
			break forever
		case "QUIT":
			tp.PrintfLine("205 CIAO")
			break forever
		case "ADD":
			if len(parts) != 8 {
				tp.PrintfLine("401 PART ERR")
				continue forever
			}
			//receives add request from client
			// ("ADD %s %s", CRC(hobjStr), hobjStr)
			if parts[1] != CRC(strings.Join(parts[2:], " ")) {
				tp.PrintfLine("402 CRC ERR")
				continue forever
			}
			hobj, err := ConvertStringToHistoryObject(parts[2:])
			if hobj == nil || err != nil {
				tp.PrintfLine("403 HOBJ ERR")
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

			retval := his.L1Cache.LockL1Cache(hobj.MessageIDHash, hobj.Char, CaseLock, his.useHashDB) // checks and locks hash for processing
			switch retval {
			case CasePass:
				//history.History.Sync_upcounter("L1CACHE_Lock")
				//locked++
				// pass
			default:
				if hobj.ResponseChan != nil {
					hobj.ResponseChan <- retval
				}
				continue forever
			}

			if his.useHashDB {
				isDup, err := his.IndexQuery(hobj.MessageIDHash, indexRetChan, FlagSearch)
				if err != nil {
					log.Printf("FALSE IndexQuery hash=%s", hobj.MessageIDHash)
					break forever
				}
				switch isDup {
				case CasePass:
					// pass
				case CaseDupes:
					// we locked the hash but IndexQuery replied with Duplicate
					// set L1 cache to Dupe and expire
					his.DoCacheEvict(hobj.Char, hobj.MessageIDHash, 0, EmptyStr)
					//dupes++
					if hobj.ResponseChan != nil {
						hobj.ResponseChan <- isDup
					}
					continue forever
				case CaseRetry:
					// we locked the hash but IndexQuery replied with Retry
					// set L1 cache to Retry and expire
					his.DoCacheEvict(hobj.Char, hobj.MessageIDHash, 0, EmptyStr)
					//retry++
					if hobj.ResponseChan != nil {
						hobj.ResponseChan <- isDup
					}
					continue forever
				default:
					log.Printf("ERROR handleSocketConn in response from IndexQuery unknown switch isDup=%d", isDup)
					break forever
				}
			}

			isDup := his.AddHistory(hobj, true)
			if hobj.ResponseChan != nil {
				hobj.ResponseChan <- isDup
			}
			continue forever
		} // end select
	} // end forever
	log.Printf("handleConn LEFT: %#v", conn)
} // end func handleConn

func ConvertStringToHistoryObject(parts []string) (*HistoryObject, error) {
	//log.Printf("ConvertStringToHistoryObject parts='%#v'=%d", parts, len(parts))
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
	return ACL.IsAllowed(getRemoteIP(conn))
}

type AccessControlList struct {
	mux sync.RWMutex
	acl map[string]bool
}

func (a *AccessControlList) SetupACL() {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a.acl != nil {
		return
	}
	if DefaultACL != nil {
		a.acl = DefaultACL
		return
	}
	a.acl = make(map[string]bool)
}

func (a *AccessControlList) IsAllowed(ip string) bool {
	a.mux.RLock()
	retval := a.acl[ip]
	a.mux.RUnlock()
	return retval
}

func (a *AccessControlList) SetACL(ip string, val bool) {
	a.mux.Lock()
	defer a.mux.Unlock()
	if !val { // unset
		delete(a.acl, ip)
		return
	}
	a.acl[ip] = val
}
