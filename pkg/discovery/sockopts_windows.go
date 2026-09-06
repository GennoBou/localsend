//go:build windows

package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"syscall"
	"unsafe"
)

func setReuseAddr(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

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
			socketErr = syscall.Setsockopt(
				syscall.Handle(fd),
				syscall.IPPROTO_IP,
				syscall.IP_MULTICAST_IF,
				(*byte)(unsafe.Pointer(&addr[0])),
				4,
			)
			if socketErr != nil {
				return
			}
		}

		socketErr = syscall.SetsockoptInt(
			syscall.Handle(fd),
			syscall.IPPROTO_IP,
			syscall.IP_MULTICAST_LOOP,
			1,
		)
	})

	if err != nil {
		return err
	}
	return socketErr
}

func startWindowsMulticastListener(ctx context.Context, addr *net.UDPAddr, interfaces []net.Interface, handleConn func(*net.UDPConn)) error {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = setReuseAddr(fd)
			})
		},
	}

	packetConn, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf("0.0.0.0:%d", addr.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on 0.0.0.0:%d: %w", addr.Port, err)
	}
	udpConn := packetConn.(*net.UDPConn)

	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("failed to get raw conn: %w", err)
	}

	var ips []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil && isPrivateIP(ipNet.IP) {
				ips = append(ips, ipNet.IP)
			}
		}
	}

	var setoptErr error
	err = rawConn.Control(func(fd uintptr) {
		for _, ip := range ips {
			mreq := syscall.IPMreq{}
			copy(mreq.Multiaddr[:], addr.IP.To4())
			copy(mreq.Interface[:], ip.To4())
			err := syscall.SetsockoptIPMreq(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, &mreq)
			if err != nil {
				log.Printf("IP_ADD_MEMBERSHIP failed for interface %s: %v", ip, err)
				setoptErr = err
			} else {
				setoptErr = nil
			}
		}
	})

	if err != nil {
		udpConn.Close()
		return fmt.Errorf("raw conn control failed: %w", err)
	}

	if setoptErr != nil {
		udpConn.Close()
		return fmt.Errorf("failed to join multicast group on any interface: %w", setoptErr)
	}

	go func() {
		<-ctx.Done()
		_ = udpConn.Close()
	}()

	go handleConn(udpConn)

	return nil
}
