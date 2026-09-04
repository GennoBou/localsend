//go:build windows

package discovery

import (
	"syscall"
	"unsafe"
)

func setSockoptInterface(fd uintptr, addr [4]byte) error {
	return syscall.Setsockopt(
		syscall.Handle(fd),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_IF,
		(*byte)(unsafe.Pointer(&addr[0])),
		4,
	)
}

func setSockoptLoop(fd uintptr) error {
	return syscall.SetsockoptInt(
		syscall.Handle(fd),
		syscall.IPPROTO_IP,
		syscall.IP_MULTICAST_LOOP,
		1,
	)
}

func setSockoptReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

func setSockoptIPMreq(fd uintptr, multiaddr [4]byte, interfaceAddr [4]byte) error {
	mreq := syscall.IPMreq{}
	copy(mreq.Multiaddr[:], multiaddr[:])
	copy(mreq.Interface[:], interfaceAddr[:])
	return syscall.SetsockoptIPMreq(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, &mreq)
}
