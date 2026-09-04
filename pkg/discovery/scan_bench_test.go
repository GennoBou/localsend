package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func scanHostBaseline(ctx context.Context, client *http.Client, myDevice protocol.Device, ip string, port int, onDiscover func(protocol.Device)) {
	protocols := []string{"https", "http"}
	for _, proto := range protocols {
		url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/register", proto, ip, port)

		reqBody, err := json.Marshal(myDevice)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

func scanHostOptimized(ctx context.Context, client *http.Client, myDevice protocol.Device, ip string, port int, onDiscover func(protocol.Device)) {
	reqBody, err := json.Marshal(myDevice)
	if err != nil {
		return
	}

	protocols := []string{"https", "http"}
	for _, proto := range protocols {
		url := fmt.Sprintf("%s://%s:%d/api/localsend/v2/register", proto, ip, port)

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("connection refused")
}

func BenchmarkScanHostBaseline(b *testing.B) {
	client := &http.Client{Transport: &mockTransport{}}
	ctx := context.Background()
	dev := protocol.Device{
		Alias:       "TestDevice",
		Version:     "2.0",
		DeviceModel: "TestModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "1234567890abcdef",
		Port:        53317,
		Protocol:    "https",
		Download:    true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		scanHostBaseline(ctx, client, dev, "192.168.1.100", 53317, func(d protocol.Device) {})
	}
}

func BenchmarkScanHostOptimized(b *testing.B) {
	client := &http.Client{Transport: &mockTransport{}}
	ctx := context.Background()
	dev := protocol.Device{
		Alias:       "TestDevice",
		Version:     "2.0",
		DeviceModel: "TestModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "1234567890abcdef",
		Port:        53317,
		Protocol:    "https",
		Download:    true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		scanHostOptimized(ctx, client, dev, "192.168.1.100", 53317, func(d protocol.Device) {})
	}
}
