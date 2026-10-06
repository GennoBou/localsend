package discovery

import (
	"context"
	"net"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func BenchmarkStartMulticastListenerAllocation(b *testing.B) {
	myDevice := protocol.Device{
		Alias:       "BenchmarkDevice",
		Version:     "2.0",
		DeviceModel: "BenchModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "bench_fingerprint_12345",
		Port:        53317,
		Protocol:    "http",
	}

	onDiscover := func(dev protocol.Device) {}
	onAnnounce := func(dev protocol.Device) {}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		// StartMulticastListener initializes interfaces and preallocates listeners slice
		_ = StartMulticastListener(ctx, myDevice, onDiscover, onAnnounce)
		cancel()
	}
}

func BenchmarkListenerSliceAppendUnallocated(b *testing.B) {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		interfaces = make([]net.Interface, 10)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var listeners []*net.UDPConn
		for range interfaces {
			listeners = append(listeners, nil)
		}
		listeners = append(listeners, nil) // wildcard conn
		_ = listeners
	}
}

func BenchmarkListenerSliceAppendPreallocated(b *testing.B) {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		interfaces = make([]net.Interface, 10)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		listeners := make([]*net.UDPConn, 0, len(interfaces)+1)
		for range interfaces {
			listeners = append(listeners, nil)
		}
		listeners = append(listeners, nil) // wildcard conn
		_ = listeners
	}
}
