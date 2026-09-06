//go:build !windows

package discovery

import (
	"net"

	"golang.org/x/sys/unix"
)

func setMulticastSocketOptionsFd(fd uintptr, ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	if !ip.Equal(net.IPv4zero) {
		var addr [4]byte
		copy(addr[:], ip4)
		err := unix.SetsockoptInet4Addr(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_IF, addr)
		if err != nil {
			return err
		}
	}

	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP, 1)
}

func setSOReuseAddr(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
}
