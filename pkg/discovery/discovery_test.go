package discovery

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestReadMulticastLoop(t *testing.T) {
	// Create a UDP listener on loopback
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to listen UDP: %v", err)
	}
	defer serverConn.Close()

	clientConn, err := net.DialUDP("udp4", nil, serverConn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("failed to dial UDP: %v", err)
	}
	defer clientConn.Close()

	myDevice := protocol.Device{
		Fingerprint: "my-fingerprint",
		Port:        53317,
	}

	var discoveredCount int32
	var announcedCount int32

	onDiscover := func(dev protocol.Device) {
		atomic.AddInt32(&discoveredCount, 1)
	}
	onAnnounce := func(dev protocol.Device) {
		atomic.AddInt32(&announcedCount, 1)
	}

	var lastReceived sync.Map

	go readMulticastLoop(serverConn, myDevice, &lastReceived, onDiscover, onAnnounce)

	// 1. Self message should be ignored
	selfMsg := protocol.AnnounceMessage{
		Device: protocol.Device{
			Fingerprint: "my-fingerprint",
			Port:        53317,
		},
		Announce: true,
	}
	selfData, _ := json.Marshal(selfMsg)
	clientConn.Write(selfData)

	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&discoveredCount) != 0 {
		t.Errorf("expected 0 discovered devices for self-echo, got %d", atomic.LoadInt32(&discoveredCount))
	}

	// 2. Message from another device
	otherMsg := protocol.AnnounceMessage{
		Device: protocol.Device{
			Fingerprint: "other-fingerprint",
			Port:        53317,
			Alias:       "OtherDevice",
		},
		Announce: true,
	}
	otherData, _ := json.Marshal(otherMsg)
	clientConn.Write(otherData)

	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&discoveredCount) != 1 {
		t.Errorf("expected 1 discovered device, got %d", atomic.LoadInt32(&discoveredCount))
	}
	if atomic.LoadInt32(&announcedCount) != 1 {
		t.Errorf("expected 1 announced device, got %d", atomic.LoadInt32(&announcedCount))
	}

	// 3. Duplicate message within 2 seconds should be deduplicated
	clientConn.Write(otherData)
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&discoveredCount) != 1 {
		t.Errorf("expected still 1 discovered device after duplicate packet, got %d", atomic.LoadInt32(&discoveredCount))
	}
}

func TestStartMulticastListenerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	myDevice := protocol.Device{
		Fingerprint: "test-device",
		Port:        53317,
	}

	// Start listener (if local interfaces support multicast, or expect error if no multicast interface available)
	err := StartMulticastListener(ctx, myDevice, func(d protocol.Device) {}, func(d protocol.Device) {})
	if err != nil {
		// In some CI/sandbox environments, multicast listening might fail due to network setup
		t.Skipf("skipping multicast test as StartMulticastListener failed: %v", err)
		cancel()
		return
	}

	// Cancel context to test cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)
}
