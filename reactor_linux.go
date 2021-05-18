package shpnetpoll

import (
	"runtime"

	"shpnetpoll/internal/netpoll"
)

func (svr *server) activateMainReactor(lockOSThread bool) {
	if lockOSThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	defer svr.signalShutdown()
	// 主Reactor 设置为mainLoop，赋值监听连接，并注册读事件
	err := svr.mainLoop.poller.Polling(func(fd int, ev uint32) error { return svr.acceptNewConnection(fd) })
	svr.logger.Infof("Main reactor is exiting due to error: %v", err)
}

func (svr *server) activateSubReactor(el *eventloop, lockOSThread bool) {
	if lockOSThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	defer func() {
		el.closeAllConns()
		svr.signalShutdown()
	}()

	// 副reactor池负责读写操作，
	// Polling函数中进行循环处理读写事件
	err := el.poller.Polling(func(fd int, ev uint32) error {
		if c, ack := el.connections[fd]; ack {
			// Don't change the ordering of processing EPOLLOUT | EPOLLRDHUP / EPOLLIN unless you're 100%
			// sure what you're doing!
			// Re-ordering can easily introduce bugs and bad side-effects, as I found out painfully in the past.

			// We should always check for the EPOLLOUT event first, as we must try to send the leftover data back to
			// client when any error occurs on a connection.
			//
			// Either an EPOLLOUT or EPOLLERR event may be fired when a connection is refused.
			// In either case loopWrite() should take care of it properly:
			// 1) writing data back,
			// 2) closing the connection.
			// 处理写事件，
			if ev&netpoll.OutEvents != 0 {
				if err := el.loopWrite(c); err != nil {
					return err
				}
			}
			// TODO 待确认，通过判断语句得出结论，写事件优先处理
			// If there is pending data in outbound buffer, then we should omit this readable event
			// and prioritize the writable events to achieve a higher performance.
			//
			// Note that the client may send massive amounts of data to server by write() under blocking mode,
			// resulting in that it won't receive any responses before the server read all data from client,
			// in which case if the socket send buffer is full, we need to let it go and continue reading the data
			// to prevent blocking forever.
			if ev&netpoll.InEvents != 0 && (ev&netpoll.OutEvents == 0 || c.outboundBuffer.IsEmpty()) {
				return el.loopRead(c)
			}
		}
		return nil
	})
	svr.logger.Infof("Event-loop(%d) is exiting normally on the signal error: %v", el.idx, err)
}
