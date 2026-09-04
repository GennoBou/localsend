//go:build !windows

package discovery

import (
	"golang.org/x/sys/unix"
)

func setSockoptInterface(fd uintptr, addr [4]byte) error {
	return unix.SetsockoptInet4Addr(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_IF, addr)
}

func setSockoptLoop(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP, 1)
}

func setSockoptReuseAddr(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
}

func setSockoptIPMreq(fd uintptr, multiaddr [4]byte, interfaceAddr [4]byte) error {
	mreq := &unix.IPMreq{
		Multiaddr: multiaddr,
		Interface: interfaceAddr,
	}
	return unix.SetsockoptIPMreq(int(fd), unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, mreq)
}
