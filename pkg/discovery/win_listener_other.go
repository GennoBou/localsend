//go:build !windows

package discovery

import (
	"context"
	"fmt"
	"net"
)

func startWindowsMulticastListener(ctx context.Context, addr *net.UDPAddr, interfaces []net.Interface, handleConn func(*net.UDPConn)) error {
	return fmt.Errorf("windows multicast listener not supported on non-windows platform")
}
