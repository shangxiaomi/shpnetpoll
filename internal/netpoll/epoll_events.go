// +build linux

package netpoll

import "golang.org/x/sys/unix"

const (
	// InitEvents represents the initial length of poller event-list.
	InitEvents = 128
	// AsyncTasks is the maximum number of asynchronous tasks that the event-loop will process at one time.
	AsyncTasks = 64
	// ErrEvents represents exceptional events that are not read/write, like socket being closed,
	// reading/writing from/to a closed socket, etc.
	ErrEvents = unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLRDHUP
	// OutEvents combines EPOLLOUT event and some exceptional events.
	OutEvents = ErrEvents | unix.EPOLLOUT
	// InEvents combines EPOLLIN/EPOLLPRI events and some exceptional events.
	InEvents = ErrEvents | unix.EPOLLIN | unix.EPOLLPRI
)

type eventList struct {
	size   int
	events []unix.EpollEvent
}

func newEventList(size int) *eventList {
	return &eventList{size, make([]unix.EpollEvent, size)}
}

func (el *eventList) expand() {
	el.size <<= 1
	el.events = make([]unix.EpollEvent, el.size)
}

func (el *eventList) shrink() {
	el.size >>= 1
	el.events = make([]unix.EpollEvent, el.size)
}
