// +build linux freebsd dragonfly darwin

package shpnetpoll

import (
	"os"

	"golang.org/x/sys/unix"
	"shpnetpoll/errors"
	"shpnetpoll/internal/netpoll"
)

func (svr *server) acceptNewConnection(fd int) error {
	// 建立连接，产生新的fd
	nfd, sa, err := unix.Accept(fd)
	if err != nil {
		if err == unix.EAGAIN {
			return nil
		}
		return errors.ErrAcceptSocket
	}
	if err = os.NewSyscallError("fcntl nonblock", unix.SetNonblock(nfd, true)); err != nil {
		return err
	}

	netAddr := netpoll.SockaddrToTCPOrUnixAddr(sa)
	// 从负载均衡获取eventLoop
	el := svr.lb.next(netAddr)
	c := newTCPConn(nfd, el, sa, netAddr)

	// 注册异步的任务
	err = el.poller.Trigger(func() (err error) {
		// 在这里将连接的读事件注册到epoll中
		// 这里就是要执行的异步事件
		if err = el.poller.AddRead(nfd); err != nil {
			_ = unix.Close(nfd)
			c.releaseTCP()
			return
		}
		el.connections[nfd] = c
		// TODO ???????循环开始是啥意思？？？？
		err = el.loopOpen(c)
		return
	})
	if err != nil {
		_ = unix.Close(nfd)
		c.releaseTCP()
	}
	return nil
}
