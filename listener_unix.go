// +build linux freebsd dragonfly darwin

package shpnetpoll

import (
	"net"
	"os"
	"shpnetpoll/internal/reuseport"
	"sync"

	"github.com/panjf2000/gnet/errors"
	"golang.org/x/sys/unix"
	"shpnetpoll/internal/netpoll"
)

type listener struct {
	once          sync.Once
	fd            int
	lnaddr        net.Addr
	reusePort     bool
	addr, network string
}

func (ln *listener) Dup() (int, string, error) {
	return netpoll.Dup(ln.fd)
}

func (ln *listener) normalize() (err error) {
	switch ln.network {
	case "tcp", "tcp4", "tcp6":
		ln.fd, ln.lnaddr, err = reuseport.TCPSocket(ln.network, ln.addr, ln.reusePort)
		ln.network = "tcp"
	default:
		err = errors.ErrUnsupportedProtocol
	}
	return
}

func (ln *listener) close() {
	ln.once.Do(
		func() {
			if ln.fd > 0 {
				sniffErrorAndLog(os.NewSyscallError("close", unix.Close(ln.fd)))
			}
			if ln.network == "unix" {
				sniffErrorAndLog(os.RemoveAll(ln.addr))
			}
		})
}

func initListener(network, addr string, reusePort bool) (l *listener, err error) {
	l = &listener{network: network, addr: addr, reusePort: reusePort}
	err = l.normalize()
	return
}
