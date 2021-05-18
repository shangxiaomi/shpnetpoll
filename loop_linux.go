package shpnetpoll

import "shpnetpoll/internal/netpoll"

func (el *eventloop) handleEvent(fd int, ev uint32) error {
	if c, ok := el.connections[fd]; ok {
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
		if ev&netpoll.OutEvents != 0 {
			if err := el.loopWrite(c); err != nil {
				return err
			}
		}
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
		return nil
	}
	return el.loopAccept(fd)
}
