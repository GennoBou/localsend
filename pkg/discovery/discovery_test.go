package discovery

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// 10.0.0.0/8
		{"10.0.0.1 private", "10.0.0.1", true},
		{"10.255.255.255 private", "10.255.255.255", true},

		// 172.16.0.0/12
		{"172.16.0.1 private", "172.16.0.1", true},
		{"172.31.255.255 private", "172.31.255.255", true},
		{"172.15.255.255 non-private", "172.15.255.255", false},
		{"172.32.0.0 non-private", "172.32.0.0", false},

		// 192.168.0.0/16
		{"192.168.0.1 private", "192.168.0.1", true},
		{"192.168.255.255 private", "192.168.255.255", true},
		{"192.169.0.1 non-private", "192.169.0.1", false},

		// Public IPv4
		{"8.8.8.8 public", "8.8.8.8", false},
		{"1.1.1.1 public", "1.1.1.1", false},

		// Loopback IPv4
		{"127.0.0.1 loopback", "127.0.0.1", false},

		// IPv6
		{"IPv6 loopback", "::1", false},
		{"IPv6 documentation", "2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := net.ParseIP(tt.ip)
			if parsed == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			result := isPrivateIP(parsed)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v; expected %v", tt.ip, result, tt.expected)
			}
		})
	}

	t.Run("nil IP", func(t *testing.T) {
		if isPrivateIP(nil) {
			t.Errorf("isPrivateIP(nil) expected false, got true")
		}
	})
}

func TestIsLocalIP(t *testing.T) {
	localIPs := []string{"192.168.1.10", "10.0.0.5"}

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"Match first IP", "192.168.1.10", true},
		{"Match second IP", "10.0.0.5", true},
		{"No match", "192.168.1.20", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isLocalIP(tt.ip, localIPs)
			if res != tt.expected {
				t.Errorf("isLocalIP(%q, %v) = %v; expected %v", tt.ip, localIPs, res, tt.expected)
			}
		})
	}

	t.Run("Empty localIPs slice", func(t *testing.T) {
		if isLocalIP("192.168.1.10", nil) {
			t.Errorf("isLocalIP with nil slice expected false")
		}
	})
}

func TestGetLocalIPs(t *testing.T) {
	ips, err := GetLocalIPs()
	if err != nil {
		t.Fatalf("GetLocalIPs failed: %v", err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Errorf("GetLocalIPs returned invalid IP: %s", ipStr)
			continue
		}
		if ip.IsLoopback() {
			t.Errorf("GetLocalIPs returned loopback IP: %s", ipStr)
		}
		if !isPrivateIP(ip) {
			t.Errorf("GetLocalIPs returned non-private IP: %s", ipStr)
		}
	}
}

func TestMulticastListenerAndAnnounce(t *testing.T) {
	deviceA := protocol.Device{
		Alias:       "DeviceA",
		Version:     "2.0",
		DeviceModel: "TestModelA",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "fingerprint_device_a_12345678",
		Port:        53322,
		Protocol:    "http",
	}

	deviceB := protocol.Device{
		Alias:       "DeviceB",
		Version:     "2.0",
		DeviceModel: "TestModelB",
		DeviceType:  protocol.DeviceTypeMobile,
		Fingerprint: "fingerprint_device_b_87654321",
		Port:        53323,
		Protocol:    "http",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	discoveredDevices := make(map[string]protocol.Device)
	announcedDevices := make(map[string]protocol.Device)

	onDiscover := func(dev protocol.Device) {
		mu.Lock()
		defer mu.Unlock()
		discoveredDevices[dev.Fingerprint] = dev
	}

	onAnnounce := func(dev protocol.Device) {
		mu.Lock()
		defer mu.Unlock()
		announcedDevices[dev.Fingerprint] = dev
	}

	// Start listener representing device A
	err := StartMulticastListener(ctx, deviceA, onDiscover, onAnnounce)
	if err != nil {
		t.Fatalf("StartMulticastListener failed: %v", err)
	}

	// Give the listener time to start
	time.Sleep(200 * time.Millisecond)

	// Send announcement from Device B (announce = true)
	err = SendAnnounce(deviceB, true)
	if err != nil {
		t.Fatalf("SendAnnounce failed: %v", err)
	}

	// Wait for packet processing
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	discB, foundDisc := discoveredDevices[deviceB.Fingerprint]
	annB, foundAnn := announcedDevices[deviceB.Fingerprint]
	_, foundSelf := discoveredDevices[deviceA.Fingerprint]
	mu.Unlock()

	if !foundDisc {
		t.Errorf("expected DeviceB to be discovered via multicast")
	} else if discB.Alias != deviceB.Alias {
		t.Errorf("expected discovered alias %q, got %q", deviceB.Alias, discB.Alias)
	}

	if !foundAnn {
		t.Errorf("expected DeviceB to trigger onAnnounce callback")
	} else if annB.Fingerprint != deviceB.Fingerprint {
		t.Errorf("expected announced fingerprint %q, got %q", deviceB.Fingerprint, annB.Fingerprint)
	}

	if foundSelf {
		t.Errorf("listener should ignore self-echo packets from same fingerprint and port")
	}

	// Send announce from Device A (self-echo check)
	err = SendAnnounce(deviceA, true)
	if err != nil {
		t.Fatalf("SendAnnounce for DeviceA failed: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	_, foundSelf = discoveredDevices[deviceA.Fingerprint]
	mu.Unlock()

	if foundSelf {
		t.Errorf("listener failed to ignore self-echo packet for DeviceA")
	}
}

func TestScanHost(t *testing.T) {
	expectedDevice := protocol.Device{
		Alias:       "RemoteTargetDevice",
		Version:     "2.0",
		DeviceModel: "TargetModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "remote_fingerprint_99999",
		Port:        53317,
		Protocol:    "http",
	}

	myDevice := protocol.Device{
		Alias:       "ScanningDevice",
		Version:     "2.0",
		Fingerprint: "scanner_fingerprint_00000",
		Port:        53317,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/localsend/v2/register" {
			var reqDev protocol.Device
			if err := json.NewDecoder(r.Body).Decode(&reqDev); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expectedDevice)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var discovered protocol.Device
	var discoveredMu sync.Mutex
	onDiscover := func(dev protocol.Device) {
		discoveredMu.Lock()
		defer discoveredMu.Unlock()
		discovered = dev
	}

	client := server.Client()
	scanHost(ctx, client, myDevice, host, port, onDiscover)

	discoveredMu.Lock()
	defer discoveredMu.Unlock()

	if discovered.Fingerprint != expectedDevice.Fingerprint {
		t.Errorf("scanHost failed: expected fingerprint %q, got %q", expectedDevice.Fingerprint, discovered.Fingerprint)
	}
	if discovered.Alias != expectedDevice.Alias {
		t.Errorf("scanHost failed: expected alias %q, got %q", expectedDevice.Alias, discovered.Alias)
	}
	if discovered.IP != host {
		t.Errorf("scanHost failed: expected IP %q, got %q", host, discovered.IP)
	}
	if discovered.Port != port {
		t.Errorf("scanHost failed: expected port %d, got %d", port, discovered.Port)
	}
}

func TestScanLegacy(t *testing.T) {
	myDevice := protocol.Device{
		Alias:       "LegacyScanner",
		Version:     "2.0",
		Fingerprint: "scanner_fingerprint_11111",
		Port:        53317,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var discovered []protocol.Device
	var mu sync.Mutex

	onDiscover := func(dev protocol.Device) {
		mu.Lock()
		defer mu.Unlock()
		discovered = append(discovered, dev)
	}

	// ScanLegacy runs without panic or error even if no target hosts respond
	ScanLegacy(ctx, myDevice, onDiscover)
}
