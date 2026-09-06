//go:build windows

package discovery

import (
	"net"
	"syscall"
	"unsafe"
)

func setMulticastSocketOptionsFd(fd uintptr, ip net.IP) error {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	var socketErr error
	if !ip.Equal(net.IPv4zero) {
		var addr [4]byte
		copy(addr[:], ip4)
		socketErr = syscall.Setsockopt(
			syscall.Handle(fd),
			syscall.IPPROTO_IP,
			syscall.IP_MULTICAST_IF,
			(*byte)(unsafe.Pointer(&addr[0])),
			4,
		)
		if socketErr != nil {
			return socketErr
		}
	}

	socketErr = syscall.SetsockoptInt(
		syscall.Handle(fd),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_LOOP,
		1,
	)
	return socketErr
}

func setSOReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
