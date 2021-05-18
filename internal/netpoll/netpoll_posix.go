// +build linux freebsd dragonfly darwin

package netpoll

import (
	"os"

	"golang.org/x/sys/unix"
)

// SetNoDelay controls whether the operating system should delay
// packet transmission in hopes of sending fewer packets (Nagle's algorithm).
//
// The default is true (no delay), meaning that data is
// sent as soon as possible after a Write.
func SetNoDelay(fd int, noDelay bool) error {
	var arg int
	if noDelay {
		arg = 1
	}
	return os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, arg))
}
