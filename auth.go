package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
)

var (
	whiteList   sync.Map
	blackList   sync.Map
	keyString   string
	key         cipher.AEAD
	nonceString string
	nonce       []byte
)

const serverHello = "160303007a020000760303a2587d57a753572d3b95ac7c9bc44975a068879e4763c2e012e7e017254960fd20369f84e54b9f6f46f0658686dd17709c8ee48baf73190e79062424f623ccd8bf130200002e002b0002030400330024001d00204adaffc0524666b04c6955e302bcaea49651e9c0d513ea6a5d620f001c2c2210" +
	"140303000101" +
	"170303002e0f09d6084d368da58fb278194b671c8c8ed798183a0c61f53b77a1a203cb732b596d40a43aa77b81db811db79f17"

func parse() {
	blockBytes := make([]byte, 32)
	copy(blockBytes, keyString)
	var block, _ = aes.NewCipher(blockBytes)
	key, _ = cipher.NewGCM(block)
	nonce = make([]byte, 12)
	copy(nonce, nonceString)
}

func auth(con net.Conn) error {
	b := make([]byte, 561)
	_, err := io.ReadFull(con, b)
	if err != nil {
		return err
	}

	srvHello, _ := hex.DecodeString(serverHello)
	rand.Read(srvHello[11:43])
	rand.Read(srvHello[44:76]) //session id
	rand.Read(srvHello[95:127])
	rand.Read(srvHello[138:])
	_, err = con.Write(srvHello)
	if err != nil {
		return err
	}

	b = make([]byte, 80)
	_, err = io.ReadFull(con, b)
	if err != nil {
		return err
	}

	d, err := key.Open(nil, nonce, b[11:80], nil)
	if err != nil {
		return err
	}

	if string(d) != string(srvHello[11:64]) {
		return fmt.Errorf("string")
	}
	return nil
}

func safeClose(c interface{}) {
	if a, ok := c.(interface{ io.Closer }); ok {
		a.Close()
	}
}
