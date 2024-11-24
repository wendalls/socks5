package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		return
	}
	defer safeClose(l)
	for {
		con, err := l.Accept()
		if err != nil {
			return
		}
		go handle(con)
	}
}

func handle(con net.Conn) {
	defer safeClose(con)
	onTime := make(chan struct{}, 1)
	go func() {
		select {
		case <-onTime:
		case <-time.After(time.Second * 3):
			safeClose(con)
		}
	}()
	buf := make([]byte, 256)
	if _, err := io.ReadFull(con, buf[:2]); err != nil {
		return
	}
	if _, err := io.ReadFull(con, buf[:buf[1]]); err != nil {
		return
	}
	if n, err := con.Write([]byte{5, 0}); n != 2 || err != nil {
		return
	}

	if _, err := io.ReadFull(con, buf[:4]); err != nil {
		return
	}
	cmd, atyp := buf[1], buf[3]

	var addr string
	switch atyp {
	case 1:
		_, err := io.ReadFull(con, buf[:4])
		if err != nil {
			return
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case 3:
		_, err := io.ReadFull(con, buf[:1])
		if err != nil {
			return
		}
		addrLen := int(buf[0])

		_, err = io.ReadFull(con, buf[:addrLen])
		if err != nil {
			return
		}
		addr = string(buf[:addrLen])
		if addr == "localhost" {
			return
		}
	default:
		return
	}

	_, err := io.ReadFull(con, buf[:2])
	if err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[:2])
	destAddrPort := fmt.Sprintf("%s:%d", addr, port)

	msg := []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	if cmd == 1 {
		dest, err := net.Dial("tcp", destAddrPort)
		if err != nil {
			return
		}
		defer safeClose(dest)
		onTime <- struct{}{}
		con.(*net.TCPConn).SetWriteBuffer(256 * 1024)
		n, err := con.Write(msg)
		if err != nil || n != 10 {
			return
		}
		go func() {
			b := make([]byte, 1024*8)
			for {
				n, err := dest.Read(b)
				if err != nil {
					break
				}
				_, err = con.Write(b[:n])
				if err != nil {
					break
				}
			}
			time.Sleep(3 * time.Second)
			safeClose(dest)
			safeClose(con)
		}()
		b := make([]byte, 1024*8)
		for {
			n, err := con.Read(b)
			if err != nil {
				break
			}
			_, err = dest.Write(b[:n])
			if err != nil {
				break
			}
		}
		time.Sleep(3 * time.Second)
		safeClose(dest)
		safeClose(con)
	}
}

func safeClose(c interface{}) {
	if a, ok := c.(interface{ io.Closer }); ok {
		a.Close()
	}
}
