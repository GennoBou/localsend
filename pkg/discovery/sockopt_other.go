//go:build !windows

package discovery

import (
	"context"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func setMulticastSocketOptions(conn *net.UDPConn, ip net.IP) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var socketErr error
	err = rawConn.Control(func(fd uintptr) {
		ip4 := ip.To4()
		if ip4 == nil {
			return
		}

		if !ip.Equal(net.IPv4zero) {
			var addr [4]byte
			copy(addr[:], ip4)
			socketErr = unix.SetsockoptInet4Addr(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_IF, addr)
			if socketErr != nil {
				return
			}
		}

		socketErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP, 1)
	})

	if err != nil {
		return err
	}
	return socketErr
}

func startWindowsMulticastListener(ctx context.Context, addr *net.UDPAddr, interfaces []net.Interface, handleConn func(*net.UDPConn)) error {
	return fmt.Errorf("windows multicast listener is only supported on windows")
}

func setReuseAddr(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
}
