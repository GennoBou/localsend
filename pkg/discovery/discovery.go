package discovery

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

// SendAnnounce announces own device information to the surrounding network using UDP multicast.
func SendAnnounce(myDevice protocol.Device, announce bool) error {
	addr, err := net.ResolveUDPAddr("udp4", protocol.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Send announce from all active local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}

	msg := protocol.AnnounceMessage{
		Device:   myDevice,
		Announce: announce,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal announce message: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addrVal := range addrs {
			ipNet, ok := addrVal.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil || !isPrivateIP(ipNet.IP) {
				continue
			}

			// Set local address to send from a specific interface's IP
			localAddr := &net.UDPAddr{IP: ipNet.IP, Port: 0}
			conn, err := net.DialUDP("udp4", localAddr, addr)
			if err != nil {
				continue
			}

			// Explicitly specify transmission interface and loopback to the host itself
			if serr := setMulticastSocketOptions(conn, ipNet.IP); serr != nil {
				log.Printf("setMulticastSocketOptions failed on %s: %v", ipNet.IP, serr)
			}

			_, _ = conn.Write(data)
			conn.Close()
		}
	}

	// Send an additional announce with the source IP wildcarded (nil) to improve reachability
	// to loopback communications (other processes) within the same machine.
	wildcardConn, err := net.DialUDP("udp4", nil, addr)
	if err == nil {
		if serr := setMulticastSocketOptions(wildcardConn, net.IPv4zero); serr != nil {
			log.Printf("setMulticastSocketOptions failed on wildcard: %v", serr)
		}
		_, _ = wildcardConn.Write(data)
		wildcardConn.Close()
	}

	return nil
}

// StartMulticastListener listens on the UDP multicast port to discover other LocalSend devices.
// onDiscover is called when a new device is discovered.
// onAnnounce is called to respond when a message with announce=true is received.
func StartMulticastListener(ctx context.Context, myDevice protocol.Device, onDiscover func(protocol.Device), onAnnounce func(protocol.Device)) error {
	addr, err := net.ResolveUDPAddr("udp4", protocol.MulticastAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}

	var listeners []*net.UDPConn
	var successCount int
	var lastReceived sync.Map // key: fingerprint+port, value: time.Time

	handleConn := func(conn *net.UDPConn) {
		buf := make([]byte, 65535)
		for {
			n, src, err := conn.ReadFrom(buf)
			if err != nil {
				// E.g., when the connection is closed
				return
			}

			var msg protocol.AnnounceMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}

			// Ignore self-echo (packets sent from oneself) unless it comes from a
			// different process (different port) on the same machine
			if msg.Fingerprint == myDevice.Fingerprint && msg.Port == myDevice.Port {
				continue
			}

			// Deduplication: Ignore if duplicate packets (same fingerprint and port)
			// from the same device are received within a short period (2 seconds)
			dedupKey := fmt.Sprintf("%s:%d", msg.Fingerprint, msg.Port)
			if lastTimeVal, ok := lastReceived.Load(dedupKey); ok {
				if lastTime, ok := lastTimeVal.(time.Time); ok && time.Since(lastTime) < 2*time.Second {
					continue
				}
			}
			lastReceived.Store(dedupKey, time.Now())

			udpAddr, ok := src.(*net.UDPAddr)
			if !ok {
				continue
			}

			// Store the source IP address in the device info
			msg.IP = udpAddr.IP.String()

			onDiscover(msg.Device)

			// Respond (register back) if the partner requests an announce response,
			// skipping if the partner's port is 0 (not listening)
			if msg.Announce && msg.Port > 0 {
				onAnnounce(msg.Device)
			}
		}
	}

	// Use a custom multicast listener on Windows
	if runtime.GOOS == "windows" {
		err := startWindowsMulticastListener(ctx, addr, interfaces, handleConn)
		if err != nil {
			return err
		}
		return nil
	}

	for _, iface := range interfaces {
		// Target only active and multicast-capable interfaces (excluding loopback)
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// Check for a valid private IPv4 address
		hasPrivateIPv4 := false
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil && isPrivateIP(ipNet.IP) {
				hasPrivateIPv4 = true
				break
			}
		}
		if !hasPrivateIPv4 {
			continue
		}

		// Start multicast listening on this interface
		conn, err := net.ListenMulticastUDP("udp4", &iface, addr)
		if err != nil {
			continue
		}
		listeners = append(listeners, conn)
		successCount++
		go handleConn(conn)
	}

	// Always start a listener on the wildcard (nil) interface to receive loopback packets
	// and fallback in certain environments.
	nilConn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err == nil {
		listeners = append(listeners, nilConn)
		successCount++
		go handleConn(nilConn)
	}

	// Return an error if failed to bind to any interface
	if successCount == 0 {
		return fmt.Errorf("failed to listen multicast on any interface")
	}

	go func() {
		<-ctx.Done()
		for _, conn := range listeners {
			_ = conn.Close()
		}
	}()

	return nil
}

// ScanLegacy scans the local C-class subnet in parallel using HTTP for environments where multicast is not available.
func ScanLegacy(ctx context.Context, myDevice protocol.Device, onDiscover func(protocol.Device)) {
	localIPs, err := GetLocalIPs()
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	// Semaphore channel to limit concurrent tasks to 64
	sem := make(chan struct{}, 64)

	// HTTP transport that ignores TLS certificate validation (since LocalSend uses self-signed certificates)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   500 * time.Millisecond, // Timeout for each host is 500ms
	}

	// 1. Scan conflict ports (53317-53326) against own local IPs and loopback
	// to discover other instances on the same host
	targetIPs := append(localIPs, "127.0.0.1")
	for _, ip := range targetIPs {
		for port := protocol.DefaultPort; port <= protocol.DefaultPort+9; port++ {
			// Skip scanning own IP and port to prevent self-scanning
			isSelf := myDevice.Fingerprint != "" && port == myDevice.Port && (ip == "127.0.0.1" || isLocalIP(ip, localIPs))
			if isSelf {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(targetIP string, targetPort int) {
				defer func() {
					<-sem
					wg.Done()
				}()
				scanHost(ctx, client, myDevice, targetIP, targetPort, onDiscover)
			}(ip, port)
		}
	}

	// 2. Scan the default port (53317) across the entire local C-class subnet
	for _, localIP := range localIPs {
		parts := strings.Split(localIP, ".")
		if len(parts) != 4 {
			continue
		}
		baseIP := strings.Join(parts[:3], ".") + "."

		for i := 1; i <= 254; i++ {
			targetIP := fmt.Sprintf("%s%d", baseIP, i)
			// Skip own IP since it is covered in Step 1
			if isLocalIP(targetIP, localIPs) {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(ip string) {
				defer func() {
					<-sem
					wg.Done()
				}()
				scanHost(ctx, client, myDevice, ip, protocol.DefaultPort, onDiscover)
			}(targetIP)
		}
	}

	wg.Wait()
}

// scanHost performs device discovery by sending a registration request to a specific IP and port.
func scanHost(ctx context.Context, client *http.Client, myDevice protocol.Device, ip string, port int, onDiscover func(protocol.Device)) {
	protocols := []string{"https", "http"}
	for _, proto := range protocols {
		url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/register", proto, ip, port)

		reqBody, err := json.Marshal(myDevice)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var partner protocol.Device
			err = json.NewDecoder(resp.Body).Decode(&partner)
			resp.Body.Close()
			if err == nil {
				partner.IP = ip
				partner.Protocol = proto
				partner.Port = port
				onDiscover(partner)
				break // Break the protocol loop as discovery succeeded
			}
		} else {
			resp.Body.Close()
		}
	}
}

// isLocalIP checks if the specified IP is in the list of local IPs.
func isLocalIP(ip string, localIPs []string) bool {
	for _, localIP := range localIPs {
		if ip == localIP {
			return true
		}
	}
	return false
}

// GetLocalIPs returns a list of private IPv4 addresses from active interfaces, excluding loopback.
func GetLocalIPs() ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface addresses: %w", err)
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			if isPrivateIP(ipNet.IP) {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	return ips, nil
}

// isPrivateIP checks if the given IP address is a private IPv4 address.
func isPrivateIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 192 && ip4[1] == 168)
}

