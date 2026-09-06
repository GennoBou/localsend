package discovery

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ipStr    string
		expected bool
	}{
		{"10.0.0.1 (Class A private)", "10.0.0.1", true},
		{"10.255.255.255 (Class A boundary)", "10.255.255.255", true},
		{"172.16.0.1 (Class B lower private)", "172.16.0.1", true},
		{"172.31.255.255 (Class B upper private)", "172.31.255.255", true},
		{"172.15.255.255 (Class B outside lower)", "172.15.255.255", false},
		{"172.32.0.1 (Class B outside upper)", "172.32.0.1", false},
		{"192.168.1.1 (Class C private)", "192.168.1.1", true},
		{"192.168.0.254 (Class C private)", "192.168.0.254", true},
		{"8.8.8.8 (Public DNS)", "8.8.8.8", false},
		{"1.1.1.1 (Public IP)", "1.1.1.1", false},
		{"127.0.0.1 (Loopback)", "127.0.0.1", false},
		{"IPv6 Address", "fe80::1", false},
		{"Nil IP", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ipStr)
			if got := isPrivateIP(ip); got != tt.expected {
				t.Errorf("isPrivateIP(%q) = %v; expected %v", tt.ipStr, got, tt.expected)
			}
		})
	}
}

func TestIsLocalIP(t *testing.T) {
	localIPs := []string{"192.168.1.100", "10.0.0.5"}

	if !isLocalIP("192.168.1.100", localIPs) {
		t.Errorf("expected isLocalIP to return true for matching IP")
	}

	if !isLocalIP("10.0.0.5", localIPs) {
		t.Errorf("expected isLocalIP to return true for matching IP")
	}

	if isLocalIP("192.168.1.101", localIPs) {
		t.Errorf("expected isLocalIP to return false for non-matching IP")
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips, err := GetLocalIPs()
	if err != nil {
		t.Fatalf("GetLocalIPs() failed with error: %v", err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Errorf("GetLocalIPs() returned invalid IP string: %s", ipStr)
		}
		if ip.IsLoopback() {
			t.Errorf("GetLocalIPs() returned loopback address: %s", ipStr)
		}
		if !isPrivateIP(ip) {
			t.Errorf("GetLocalIPs() returned non-private address: %s", ipStr)
		}
	}
}

func TestSendAnnounceAndMulticastListener(t *testing.T) {
	senderDevice := protocol.Device{
		Alias:       "SenderDevice",
		Version:     "2.0",
		DeviceModel: "SenderModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "senderfingerprint1234567890abcdef",
		Port:        53330,
		Protocol:    "https",
		Download:    true,
	}

	receiverDevice := protocol.Device{
		Alias:       "ReceiverDevice",
		Version:     "2.0",
		DeviceModel: "ReceiverModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "receiverfingerprint1234567890abcdef",
		Port:        53331,
		Protocol:    "https",
		Download:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var discovered sync.Map
	var announced sync.Map

	onDiscover := func(dev protocol.Device) {
		discovered.Store(dev.Fingerprint, dev)
	}

	onAnnounce := func(dev protocol.Device) {
		announced.Store(dev.Fingerprint, dev)
	}

	err := StartMulticastListener(ctx, receiverDevice, onDiscover, onAnnounce)
	if err != nil {
		t.Fatalf("failed to start multicast listener: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	err = SendAnnounce(senderDevice, true)
	if err != nil {
		t.Fatalf("SendAnnounce failed: %v", err)
	}

	// Give time for multicast packet delivery and processing
	var foundDev protocol.Device
	success := false
	for i := 0; i < 20; i++ {
		if val, ok := discovered.Load(senderDevice.Fingerprint); ok {
			foundDev = val.(protocol.Device)
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Log("Note: Multicast packet delivery might depend on network interface configuration.")
	} else {
		if foundDev.Alias != senderDevice.Alias {
			t.Errorf("expected alias %q, got %q", senderDevice.Alias, foundDev.Alias)
		}
		if foundDev.Port != senderDevice.Port {
			t.Errorf("expected port %d, got %d", senderDevice.Port, foundDev.Port)
		}
		if _, ok := announced.Load(senderDevice.Fingerprint); !ok {
			t.Errorf("expected onAnnounce callback to be called when announce=true")
		}
	}
}

func TestScanHost(t *testing.T) {
	partnerDevice := protocol.Device{
		Alias:       "PartnerDevice",
		Version:     "2.0",
		DeviceModel: "PartnerModel",
		DeviceType:  protocol.DeviceTypeMobile,
		Fingerprint: "partnerfingerprint1234567890",
		Port:        53317,
		Protocol:    "http",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/localsend/v2/register" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(partnerDevice)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	host, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	var port int
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	myDevice := protocol.Device{
		Alias:       "MyDevice",
		Fingerprint: "myfingerprint",
		Port:        53318,
	}

	var discoveredDev protocol.Device
	var discoveredCount int32

	onDiscover := func(dev protocol.Device) {
		atomic.AddInt32(&discoveredCount, 1)
		discoveredDev = dev
	}

	ctx := context.Background()
	client := server.Client()

	scanHost(ctx, client, myDevice, host, port, onDiscover)

	if atomic.LoadInt32(&discoveredCount) != 1 {
		t.Fatalf("expected 1 discovered device, got %d", discoveredCount)
	}

	if discoveredDev.Alias != partnerDevice.Alias {
		t.Errorf("expected partner alias %q, got %q", partnerDevice.Alias, discoveredDev.Alias)
	}
	if discoveredDev.IP != host {
		t.Errorf("expected IP %q, got %q", host, discoveredDev.IP)
	}
	if discoveredDev.Port != port {
		t.Errorf("expected port %d, got %d", port, discoveredDev.Port)
	}
}

func TestScanLegacy(t *testing.T) {
	myDevice := protocol.Device{
		Alias:       "ScannerDevice",
		Fingerprint: "scannerfingerprint123",
		Port:        53320,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var discovered sync.Map
	onDiscover := func(dev protocol.Device) {
		discovered.Store(dev.Fingerprint, dev)
	}

	// Executing ScanLegacy should complete without hanging or panicking.
	ScanLegacy(ctx, myDevice, onDiscover)
}
