package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:8848")
	if err != nil {
		panic(fmt.Errorf("dial error, err%v", err))
	}
	log.Println("conn success")
	defer func() {
		err = conn.Close()
		if err != nil {
			panic(fmt.Errorf("close error, err%v", err))
		}
	}()
	_, err = conn.Write([]byte("你好，世界"))
	if err != nil {
		log.Printf("[error] write error, %v\n", err)
	} else {
		log.Printf("msg send success, msg:[%s]\n", "你好世界")
	}
}

// sysSocket get fd,
func sysSocket(family, sotype, proto int) (int, error) {
	return unix.Socket(family, sotype|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, proto)
}
