package main

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"time"
)

const (
	maxSize = 8 * 1024
	timeOut = 3 * time.Second
)

type sock struct {
	id      int
	num     uint64
	mainCon net.Conn
	con     net.Conn
	c       chan []byte
}

func (s *sock) proxy() {
	defer safeClose(s.con)
	defer func() {
		s.bye(8)
		time.Sleep(timeOut)
		safeClose(s.con)
		select {
		case s.c <- make([]byte, 1):
		case <-time.After(timeOut * 2):
		}
	}()

	b := <-s.c
	d, err := key.Open(nil, nonce, b, nil)
	if err != nil {
		return
	}
	s.con, err = net.Dial("tcp", string(d))
	if err != nil {
		return
	}

	go s.getFromChan()

	if len(b) > 0 && b[0] == 0x16 {
		head := make([]byte, 5)
		f := s.keyWrite
		for {
			if _, err := io.ReadFull(s.con, head); err != nil {
				break
			}
			b := make([]byte, binary.BigEndian.Uint16(head[3:])+5)
			if _, err := io.ReadFull(s.con, b[5:]); err != nil {
				break
			}
			copy(b, head)
			if head[0] == 0x17 {
				if len(b) > 13 && binary.BigEndian.Uint64(b[5:]) != 1 {
					s.bye(9)
					f = s.plainWrite
				} else {
					s.bye(10)
					f = s.plain64Write
				}
			}
			if err := f(b); err != nil {
				break
			}
		}
	} else {
		for {
			b := make([]byte, maxSize)
			n, err := s.con.Read(b)
			if err != nil {
				break
			}
			b = b[:n]
			_, err = s.mainCon.Write(s.hat(key.Seal(nil, nonce, b, nil)))
			if err != nil {
				break
			}
		}
	}
}

func (s *sock) keyWrite(b []byte) error {
	_, err := s.mainCon.Write(s.hat(key.Seal(nil, nonce, b, nil)))
	return err
}

func (s *sock) plainWrite(b []byte) error {
	_, err := s.mainCon.Write(s.hat(b[5:]))
	return err
}

func (s *sock) plain64Write(b []byte) error {
	_, err := s.mainCon.Write(s.hat(b[13:]))
	return err
}

func (s *sock) bye(n int) {
	bye := make([]byte, n)
	rand.Read(bye)
	s.mainCon.Write(s.hat(bye))
}

func (s *sock) getFromChan() {
	defer safeClose(s.con)
	defer time.Sleep(timeOut)

	b := <-s.c
	b, err := key.Open(nil, nonce, b, nil)
	if err != nil {
		return
	}
	_, err = s.con.Write(b)
	if err != nil {
		return
	}
	if len(b) > 0 && b[0] == 0x16 {
		f := s.keyRead
		for {
			b := <-s.c
			if len(b) <= 10 {
				switch len(b) {
				case 9:
					f = s.plainRead
				case 10:
					f = s.plain64Read
				default:
					return
				}
				continue
			}
			if err := f(b); err != nil {
				return
			}
		}
	} else {
		for {
			b := <-s.c
			if len(b) <= 10 {
				return
			}
			b, err := key.Open(nil, nonce, b, nil)
			if err != nil {
				return
			}
			_, err = s.con.Write(b)
			if err != nil {
				return
			}
		}
	}
}

func (s *sock) keyRead(b []byte) error {
	d, err := key.Open(nil, nonce, b, nil)
	if err != nil {
		return err
	}
	_, err = s.con.Write(d)
	return err
}

func (s *sock) plainRead(b []byte) error {
	head := []byte{0x17, 3, 3, 0, 0}
	binary.BigEndian.PutUint16(head[3:], uint16(len(b)))
	s.con.Write(head)
	_, err := s.con.Write(b)
	return err
}

func (s *sock) plain64Read(b []byte) error {
	s.num++
	head := []byte{0x17, 3, 3, 0, 0}
	binary.BigEndian.PutUint16(head[3:], uint16(len(b)+8))
	num := make([]byte, 8)
	binary.BigEndian.PutUint64(num, s.num)
	s.con.Write(head[:])
	s.con.Write(num[:])
	_, err := s.con.Write(b)
	return err
}

func (s *sock) hat(p []byte) []byte {
	b := make([]byte, len(p)+7)
	copy(b, []byte{0x17, 3, 3})
	n := len(p) + 2
	b[3], b[4] = byte(n>>8), byte(n)
	b[5], b[6] = byte(s.id>>8), byte(s.id)
	copy(b[7:], p)
	return b
}
