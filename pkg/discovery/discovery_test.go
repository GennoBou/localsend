package discovery

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestSendAnnounce(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp4", protocol.MulticastAddr)
	if err != nil {
		t.Fatalf("failed to resolve multicast address: %v", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		t.Skipf("skipping test: unable to listen on multicast UDP address %s: %v", protocol.MulticastAddr, err)
		return
	}
	defer conn.Close()

	tests := []struct {
		name     string
		device   protocol.Device
		announce bool
	}{
		{
			name: "announce true",
			device: protocol.Device{
				Alias:       "TestSendDevice1",
				Version:     "2.0",
				DeviceModel: "TestModel1",
				DeviceType:  protocol.DeviceTypeDesktop,
				Fingerprint: "fingerprint-send-1",
				Port:        53318,
				Protocol:    "https",
				Download:    true,
			},
			announce: true,
		},
		{
			name: "announce false",
			device: protocol.Device{
				Alias:       "TestSendDevice2",
				Version:     "2.0",
				DeviceModel: "TestModel2",
				DeviceType:  protocol.DeviceTypeMobile,
				Fingerprint: "fingerprint-send-2",
				Port:        53319,
				Protocol:    "http",
				Download:    false,
			},
			announce: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SendAnnounce(tt.device, tt.announce)
			if err != nil {
				t.Fatalf("SendAnnounce failed: %v", err)
			}

			buf := make([]byte, 65535)
			var receivedMsg protocol.AnnounceMessage
			found := false

			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, _, err := conn.ReadFrom(buf)
				if err != nil {
					continue
				}

				var msg protocol.AnnounceMessage
				if err := json.Unmarshal(buf[:n], &msg); err == nil {
					if msg.Fingerprint == tt.device.Fingerprint {
						receivedMsg = msg
						found = true
						break
					}
				}
			}

			if !found {
				t.Fatalf("did not receive announce packet for fingerprint %s", tt.device.Fingerprint)
			}

			if receivedMsg.Alias != tt.device.Alias {
				t.Errorf("expected alias %s, got %s", tt.device.Alias, receivedMsg.Alias)
			}
			if receivedMsg.Version != tt.device.Version {
				t.Errorf("expected version %s, got %s", tt.device.Version, receivedMsg.Version)
			}
			if receivedMsg.DeviceModel != tt.device.DeviceModel {
				t.Errorf("expected device model %s, got %s", tt.device.DeviceModel, receivedMsg.DeviceModel)
			}
			if receivedMsg.DeviceType != tt.device.DeviceType {
				t.Errorf("expected device type %s, got %s", tt.device.DeviceType, receivedMsg.DeviceType)
			}
			if receivedMsg.Port != tt.device.Port {
				t.Errorf("expected port %d, got %d", tt.device.Port, receivedMsg.Port)
			}
			if receivedMsg.Protocol != tt.device.Protocol {
				t.Errorf("expected protocol %s, got %s", tt.device.Protocol, receivedMsg.Protocol)
			}
			if receivedMsg.Download != tt.device.Download {
				t.Errorf("expected download %v, got %v", tt.device.Download, receivedMsg.Download)
			}
			if receivedMsg.Announce != tt.announce {
				t.Errorf("expected announce %v, got %v", tt.announce, receivedMsg.Announce)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
		{"127.0.0.1", false},
		{"::1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		res := isPrivateIP(ip)
		if res != tt.expected {
			t.Errorf("isPrivateIP(%s) = %v; want %v", tt.ip, res, tt.expected)
		}
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips, err := GetLocalIPs()
	if err != nil {
		t.Fatalf("GetLocalIPs failed: %v", err)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if !isPrivateIP(ip) {
			t.Errorf("GetLocalIPs returned non-private IP: %s", ipStr)
		}
	}
}
