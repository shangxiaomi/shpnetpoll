// +build linux freebsd dragonfly darwin

package reuseport

import (
	"net"
)

// TCPSocket calls tcpReusablePort.
func TCPSocket(proto, addr string, reusePort bool) (int, net.Addr, error) {
	return tcpReusablePort(proto, addr, reusePort)
}
