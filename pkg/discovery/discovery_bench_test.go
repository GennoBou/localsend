package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

var benchDevice = protocol.Device{
	Alias:       "BenchmarkDevice",
	Version:     "2.0",
	DeviceModel: "BenchModel",
	DeviceType:  protocol.DeviceTypeDesktop,
	Fingerprint: "benchfingerprint1234567890abcdef",
	Port:        53317,
	Protocol:    "https",
	Download:    true,
}

// BenchmarkScanHostRepeatedMarshal measures performance when json.Marshal is called on every iteration (old way).
func BenchmarkScanHostRepeatedMarshal(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate a subnet scan loop over 250 targets x 2 protocols
		for j := 0; j < 250; j++ {
			for _, proto := range []string{"https", "http"} {
				url := fmt.Sprintf("%s://192.168.1.%d:53317/api/localsend/v2/register", proto, j)
				reqBody, _ := json.Marshal(benchDevice)
				req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(reqBody)))
				_ = req
			}
		}
	}
}

// BenchmarkScanHostPreMarshaled measures performance when json.Marshal is done once outside the loop (new way).
func BenchmarkScanHostPreMarshaled(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reqBody, _ := json.Marshal(benchDevice)
		// Simulate a subnet scan loop over 250 targets x 2 protocols
		for j := 0; j < 250; j++ {
			for _, proto := range []string{"https", "http"} {
				url := fmt.Sprintf("%s://192.168.1.%d:53317/api/localsend/v2/register", proto, j)
				req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
				_ = req
			}
		}
	}
}
