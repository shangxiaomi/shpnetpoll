// +build linux freebsd dragonfly

package netpoll

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// SetKeepAlive sets whether the operating system should send
// keep-alive messages on the connection and sets period between keep-alive's.
func SetKeepAlive(fd int, d time.Duration) error {
	if d <= 0 {
		return errors.New("invalid time duration")
	}
	secs := int(d / time.Second)
	if err := os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1)); err != nil {
		return err
	}
	if err := os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, secs)); err != nil {
		return err
	}
	return os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, secs))
}
