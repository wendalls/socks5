package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxSize = 4 * 1024
	timeOut = 3 * time.Second
)

var token = flag.String("token", "secret_token_123", "认证令牌")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxSize,
	WriteBufferSize: maxSize,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != *token {
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//log.Println("升级到WebSocket失败:", err)
		return
	}
	defer conn.Close()

	var chanMap []chan []byte
	wsChan := make(chan []byte, maxSize)
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			case msg := <-wsChan:
				err = conn.WriteMessage(websocket.BinaryMessage, msg)
				if err != nil {
					return
				}
			}
		}
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			//log.Println("读取消息失败:", err)
			break
		}
		if len(msg) < 2 {
			continue
		}
		id := int(msg[0])<<8 | int(msg[1])
		msg = msg[2:]
		if id == 65535 {
			if len(msg) < 2 {
				continue
			}
			id = int(msg[0])<<8 | int(msg[1])
			msg = msg[2:]
			for id+1 > len(chanMap) {
				chanMap = append(chanMap, make(chan []byte))
			}
			chanMap[id] = make(chan []byte, 256)
			go proxy(chanMap[id], wsChan, id)
		}
		if id < len(chanMap) {
			select {
			case chanMap[id] <- msg:
			default:
			}
		}
	}
	done <- struct{}{}
	for _, c := range chanMap {
		select {
		case <-c:
		default:
		}
		select {
		case c <- []byte{}:
		default:
		}
	}
}

func proxy(c chan []byte, wsChan chan []byte, id int) {
	msg := <-c
	//log.Println(string(msg))
	if len(msg) < 7 {
		return
	}
	var addr string
	switch msg[0] { //aType
	case 1:
		addr = fmt.Sprintf("%d.%d.%d.%d:%d", msg[1], msg[2], msg[3], msg[4], int(msg[5])<<8|int(msg[6]))
	case 3:
		addr = fmt.Sprintf("%s:%d", string(msg[1:len(msg)-2]), int(msg[len(msg)-2])<<8|int(msg[len(msg)-1]))
	default:
		return
	}

	con, err := net.Dial("tcp", addr)
	if err != nil {
		//log.Println(addr, "连接失败")
		return
	}
	defer con.Close()

	done := make(chan struct{}, 2)
	go func() {
		defer func() {
			time.Sleep(timeOut)
			done <- struct{}{}
		}()
		for {
			b := make([]byte, maxSize)
			b[0], b[1] = byte(id>>8), byte(id)
			n, err := con.Read(b[2:])
			select {
			case wsChan <- b[:n+2]:
			case <-time.After(timeOut):
				return
			case <-done:
				return
			}
			if err != nil && n == 0 {
				return
			}
		}
	}()
	defer func() {
		time.Sleep(timeOut)
		done <- struct{}{}
	}()
	for {
		select {
		case msg = <-c:
			if len(msg) == 0 {
				return
			}
			if _, err := con.Write(msg); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func main() {
	flag.Parse()

	http.HandleFunc("/ws", wsHandler)
	http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil)
}
