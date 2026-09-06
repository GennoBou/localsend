//go:build windows

package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"syscall"
)

// startWindowsMulticastListener is a custom multicast receiver listener for Windows environments.
// It binds to 0.0.0.0:53317 with SO_REUSEADDR enabled, and joins the multicast group
// by configuring IP_ADD_MEMBERSHIP for each interface.
func startWindowsMulticastListener(ctx context.Context, addr *net.UDPAddr, interfaces []net.Interface, handleConn func(*net.UDPConn)) error {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// Set SO_REUSEADDR to allow multiple binds on Windows
				_ = setSOReuseAddr(fd)
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

	// Get active private IPv4 addresses
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
				// Log the error but continue as other interfaces might succeed
				log.Printf("IP_ADD_MEMBERSHIP failed for interface %s: %v", ip, err)
				setoptErr = err
			} else {
				// Clear error if at least one interface succeeded
				setoptErr = nil
			}
		}
	})

	if err != nil {
		udpConn.Close()
		return fmt.Errorf("raw conn control failed: %w", err)
	}

	// Return error if none of the interfaces succeeded
	if setoptErr != nil {
		udpConn.Close()
		return fmt.Errorf("failed to join multicast group on any interface: %w", setoptErr)
	}

	// Connection monitoring and cleanup
	go func() {
		<-ctx.Done()
		_ = udpConn.Close()
	}()

	go handleConn(udpConn)

	return nil
}
