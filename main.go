package main

import (
	"flag"
	"io"
	"net"
	"sync"
	"time"
)

func main() {
	flag.StringVar(&keyString, "key", "initial", "cipher key")
	flag.StringVar(&nonceString, "nonce", "initial", "cipher nonce")
	flag.Parse()
	parse()

	l, err := net.Listen("tcp", ":443")
	if err != nil {
		panic(err)
	}
	for {
		con, err := l.Accept()
		if err != nil {
			continue
		}
		go handle(con)
	}
}

type mux struct {
	lock    sync.Mutex
	con     net.Conn
	chanMap []chan []byte
}

func handle(con net.Conn) {
	defer safeClose(con)

	onTime := make(chan struct{}, 1)

	ip := make([]byte, 4)
	ip = con.RemoteAddr().(*net.TCPAddr).IP.To4()
	n := int(ip[0])<<16 | int(ip[1])<<8 | int(ip[2])
	if _, ok := whiteList.Load(n); !ok {
		if _, ok = blackList.Load(n); ok {
			return
		} else {
			go func() {
				select {
				case <-time.After(timeOut):
					blackList.Store(n, struct{}{})
				case <-onTime:
					whiteList.Store(n, struct{}{})
				}
			}()
		}
	}

	err := auth(con)
	if err != nil {
		return
	}
	onTime <- struct{}{}
	mx := &mux{con: con}

	err = con.(*net.TCPConn).SetWriteBuffer(1 * 1024 * 1024)
	if err != nil {
		return
	}

	head := make([]byte, 7)
	for {
		if _, err := io.ReadFull(mx.con, head); err != nil || head[0] != 0x17 {
			break
		}
		n := int(head[3])<<8 | int(head[4]) - 2
		if n < 8 {
			break
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(mx.con, b); err != nil {
			break
		}
		id := int(head[5])<<8 | int(head[6])
		if id == 65535 {
			id = int(b[0])<<8 | int(b[1])
			b = b[2:]
			mx.lock.Lock()
			for id+1 > len(mx.chanMap) {
				mx.chanMap = append(mx.chanMap, make(chan []byte))
			}
			mx.chanMap[id] = make(chan []byte, 256)
			s := &sock{
				id:      id,
				mainCon: mx.con,
				c:       mx.chanMap[id],
			}
			mx.lock.Unlock()
			go s.proxy()
		}
		if id < len(mx.chanMap) {
			select {
			case mx.chanMap[id] <- b:
			default:
			}
		}
	}

	for _, c := range mx.chanMap {
		select {
		case <-c:
		default:
		}
		select {
		case c <- make([]byte, 1):
		default:
		}
	}
}
