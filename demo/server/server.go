package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"time"
)

func main() {
	port := "8848"
	fd := initListen(port)

	defer func() {
		err := unix.Close(fd)
		if err != nil {
			panic(fmt.Errorf("close fd error, err:%v", err))
		}
	}()

	for {
		netFd, sockaddr, err := unix.Accept(fd)
		if err != nil {
			log.Println("accept error, err%v ", err)
			time.Sleep(1 * time.Second)
			continue
		}
		go func() {
			log.Println("accept success %v %v", netFd, sockaddr)
			buf := make([]byte, 1024)
			time.Sleep(1 * time.Second)
			read, err := unix.Read(netFd, buf)
			if err != nil {
				log.Printf("receive error, err%v\n", err)
			} else {
				log.Printf("received msg:[%s] %d\n", string(buf), read)
			}
		}()
	}
}

// initListen 给定端口号，监听指定端口，并且返回此fd
func initListen(port string) int {

	// 获取Tcp地址
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(fmt.Errorf("ResolveTCPAddr, err:%v", err))
	}

	// 获取对应的Socket表示
	sa, family := &unix.SockaddrInet4{Port: addr.Port}, unix.AF_INET
	// 获取Socket套接字
	fd, err := sysSocket(family, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	if err != nil {
		panic(fmt.Errorf("get unix.Socket error, err:%v", err))
	}
	// 将fd绑定到套接字上
	err = unix.Bind(fd, sa)
	if err != nil {
		panic(fmt.Errorf("bind error, err:%v", err))
	}
	// 监听套接字fd
	err = unix.Listen(fd, 100000)
	if err != nil {
		panic(fmt.Errorf("listen error, err:%v", err))
	}
	log.Println("server begin")
	return fd
}

// sysSocket get fd,
func sysSocket(family, sotype, proto int) (int, error) {
	return unix.Socket(family, sotype|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, proto)
}
