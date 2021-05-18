// +build linux freebsd dragonfly darwin

package netpoll

import (
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

// Dup is the wrapper for dupCloseOnExec.
func Dup(fd int) (int, string, error) {
	return dupCloseOnExec(fd)
}

// tryDupCloexec indicates whether F_DUPFD_CLOEXEC should be used.
// If the kernel doesn't support it, this is set to 0.
var tryDupCloexec = int32(1)

// dupCloseOnExec dups fd and marks it close-on-exec.
func dupCloseOnExec(fd int) (int, string, error) {
	if atomic.LoadInt32(&tryDupCloexec) == 1 {
		r, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
		if err == nil {
			return r, "", nil
		}
		switch err.(syscall.Errno) {
		case unix.EINVAL, unix.ENOSYS:
			// Old kernel, or js/wasm (which returns
			// ENOSYS). Fall back to the portable way from
			// now on.
			atomic.StoreInt32(&tryDupCloexec, 0)
		default:
			return -1, "fcntl", err
		}
	}
	return dupCloseOnExecOld(fd)
}

// dupCloseOnExecOld is the traditional way to dup an fd and
// set its O_CLOEXEC bit, using two system calls.
func dupCloseOnExecOld(fd int) (int, string, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	newFD, err := syscall.Dup(fd)
	if err != nil {
		return -1, "dup", err
	}
	syscall.CloseOnExec(newFD)
	return newFD, "", nil
}
