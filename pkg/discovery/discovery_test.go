package discovery

import (
	"encoding/json"
	"net"
	"sync"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestProcessMulticastPacket(t *testing.T) {
	myDevice := protocol.Device{
		Alias:       "MyDevice",
		Version:     "2.0",
		Fingerprint: "myfingerprint",
		Port:        53317,
	}

	senderDevice := protocol.Device{
		Alias:       "SenderDevice",
		Version:     "2.0",
		Fingerprint: "senderfingerprint",
		Port:        53317,
	}

	srcAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.50"),
		Port: 53317,
	}

	t.Run("valid packet calls onDiscover and onAnnounce when announce=true", func(t *testing.T) {
		var lastReceived sync.Map
		discovered := false
		announced := false

		msg := protocol.AnnounceMessage{
			Device:   senderDevice,
			Announce: true,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("failed to marshal msg: %v", err)
		}

		processMulticastPacket(data, srcAddr, myDevice, &lastReceived,
			func(dev protocol.Device) {
				discovered = true
				if dev.IP != "192.168.1.50" {
					t.Errorf("expected IP 192.168.1.50, got %s", dev.IP)
				}
				if dev.Fingerprint != senderDevice.Fingerprint {
					t.Errorf("expected fingerprint %s, got %s", senderDevice.Fingerprint, dev.Fingerprint)
				}
			},
			func(dev protocol.Device) {
				announced = true
			},
		)

		if !discovered {
			t.Error("expected onDiscover to be called")
		}
		if !announced {
			t.Error("expected onAnnounce to be called")
		}
	})

	t.Run("self-echo packet from same fingerprint and port is ignored", func(t *testing.T) {
		var lastReceived sync.Map
		discovered := false

		msg := protocol.AnnounceMessage{
			Device:   myDevice,
			Announce: true,
		}
		data, _ := json.Marshal(msg)

		processMulticastPacket(data, srcAddr, myDevice, &lastReceived,
			func(dev protocol.Device) {
				discovered = true
			},
			func(dev protocol.Device) {},
		)

		if discovered {
			t.Error("expected self-echo packet to be ignored")
		}
	})

	t.Run("duplicate packet within 2 seconds is ignored", func(t *testing.T) {
		var lastReceived sync.Map
		discoverCount := 0

		msg := protocol.AnnounceMessage{
			Device:   senderDevice,
			Announce: false,
		}
		data, _ := json.Marshal(msg)

		onDiscover := func(dev protocol.Device) {
			discoverCount++
		}

		processMulticastPacket(data, srcAddr, myDevice, &lastReceived, onDiscover, func(dev protocol.Device) {})
		processMulticastPacket(data, srcAddr, myDevice, &lastReceived, onDiscover, func(dev protocol.Device) {})

		if discoverCount != 1 {
			t.Errorf("expected onDiscover to be called exactly once, got %d", discoverCount)
		}
	})

	t.Run("invalid JSON is ignored", func(t *testing.T) {
		var lastReceived sync.Map
		discovered := false

		processMulticastPacket([]byte("invalid json"), srcAddr, myDevice, &lastReceived,
			func(dev protocol.Device) {
				discovered = true
			},
			func(dev protocol.Device) {},
		)

		if discovered {
			t.Error("expected invalid JSON packet to be ignored")
		}
	})
}
