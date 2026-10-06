package discovery

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestMDNSServiceDiscovery(t *testing.T) {
	// 1. Define test device
	testDevice := protocol.Device{
		Alias:       "TestMdnsGoDevice",
		Version:     "2.0",
		DeviceModel: "GoTestModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "testfingerprint1234567890abcdef",
		Port:        53320, // Use a different port to prevent conflict
		Protocol:    "https",
		Download:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Start advertising the test device using new wrapper in MDNS mode
	advertiserCloser, err := StartAdvertising(ctx, testDevice, DiscoveryModeMDNS, true)
	if err != nil {
		t.Fatalf("failed to register mDNS service: %v", err)
	}
	defer advertiserCloser.Close()

	// Give the server a moment to spin up and bind
	time.Sleep(500 * time.Millisecond)

	var discoveredDevice protocol.Device
	var once sync.Once
	discoverChan := make(chan struct{})

	onDiscover := func(dev protocol.Device) {
		// Verify if it is our test device using the unique fingerprint
		if dev.Fingerprint == testDevice.Fingerprint {
			once.Do(func() {
				discoveredDevice = dev
				close(discoverChan)
			})
		}
	}

	// 3. Start browsing using new wrapper in MDNS mode
	err = StartListeners(ctx, testDevice, DiscoveryModeMDNS, onDiscover, func(dev protocol.Device) {})
	if err != nil {
		t.Fatalf("failed to start mDNS listener: %v", err)
	}

	// 4. Wait for discovery or timeout
	select {
	case <-discoverChan:
		// Validation of discovered device attributes
		if discoveredDevice.Alias != testDevice.Alias {
			t.Errorf("expected alias %q, got %q", testDevice.Alias, discoveredDevice.Alias)
		}
		if discoveredDevice.Port != testDevice.Port {
			t.Errorf("expected port %d, got %d", testDevice.Port, discoveredDevice.Port)
		}
		if discoveredDevice.DeviceModel != testDevice.DeviceModel {
			t.Errorf("expected device model %q, got %q", testDevice.DeviceModel, discoveredDevice.DeviceModel)
		}
		if discoveredDevice.DeviceType != testDevice.DeviceType {
			t.Errorf("expected device type %q, got %q", testDevice.DeviceType, discoveredDevice.DeviceType)
		}
		if discoveredDevice.Protocol != testDevice.Protocol {
			t.Errorf("expected protocol %q, got %q", testDevice.Protocol, discoveredDevice.Protocol)
		}
		if discoveredDevice.Download != testDevice.Download {
			t.Errorf("expected download %v, got %v", testDevice.Download, discoveredDevice.Download)
		}
		if discoveredDevice.IP == "" {
			t.Errorf("expected non-empty resolved IP address")
		}
		t.Logf("Successfully discovered test mDNS device at %s:%d", discoveredDevice.IP, discoveredDevice.Port)

	case <-ctx.Done():
		t.Fatal("timed out waiting for mDNS service discovery")
	}
}

func TestDiscoveryModeInitializations(t *testing.T) {
	testDevice := protocol.Device{
		Alias:       "TestMdnsGoDevice",
		Version:     "2.0",
		DeviceModel: "GoTestModel",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "testfingerprint1234567890abcdef",
		Port:        53321,
		Protocol:    "https",
		Download:    true,
	}

	modes := []DiscoveryMode{DiscoveryModeHybrid, DiscoveryModeMulticast, DiscoveryModeMDNS}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Test Listener Startup
			err := StartListeners(ctx, testDevice, mode, func(dev protocol.Device) {}, func(dev protocol.Device) {})
			if err != nil {
				t.Fatalf("failed to start listeners for mode %s: %v", mode, err)
			}

			// Test Advertiser Startup
			advertiser, err := StartAdvertising(ctx, testDevice, mode, false)
			if err != nil {
				t.Fatalf("failed to start advertising for mode %s: %v", mode, err)
			}
			advertiser.Close()
		})
	}
}

func TestMDNSCloser_Close(t *testing.T) {
	t.Run("normal close and idempotency", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dev := protocol.Device{
			Alias:       "TestCloserDevice",
			Fingerprint: "fingerprint12345678",
			Port:        53322,
		}

		closer, err := RegisterMDNSService(ctx, dev)
		if err != nil {
			t.Fatalf("failed to register mDNS service: %v", err)
		}

		// First Close call closes server and rawConn.
		// Since server.Close() closes the underlying socket, rawConn.Close() may return net.ErrClosed.
		err1 := closer.Close()
		if err1 != nil && !errors.Is(err1, net.ErrClosed) {
			t.Fatalf("unexpected error on first Close(): %v", err1)
		}

		// Subsequent Close call should be idempotent and return nil without panic
		err2 := closer.Close()
		if err2 != nil {
			t.Fatalf("expected nil error on second Close(), got %v", err2)
		}
	})

	t.Run("concurrent close", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dev := protocol.Device{
			Alias:       "TestConcurrentCloseDevice",
			Fingerprint: "fingerprint87654321",
			Port:        53323,
		}

		closer, err := RegisterMDNSService(ctx, dev)
		if err != nil {
			t.Fatalf("failed to register mDNS service: %v", err)
		}

		const goroutines = 10
		var wg sync.WaitGroup
		wg.Add(goroutines)

		errCh := make(chan error, goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				errCh <- closer.Close()
			}()
		}
		wg.Wait()
		close(errCh)

		for err := range errCh {
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("unexpected error from concurrent Close(): %v", err)
			}
		}
	})
}

func TestAdvertisingCloser_Close(t *testing.T) {
	t.Run("nil mdnsCloser", func(t *testing.T) {
		adv := &AdvertisingCloser{
			mdnsCloser: nil,
		}

		if err := adv.Close(); err != nil {
			t.Fatalf("expected nil error when closing AdvertisingCloser with nil mdnsCloser, got: %v", err)
		}
	})

	t.Run("non-nil mdnsCloser and idempotency", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dev := protocol.Device{
			Alias:       "TestAdvCloserDevice",
			Fingerprint: "fingerprint11223344",
			Port:        53324,
		}

		advCloser, err := StartAdvertising(ctx, dev, DiscoveryModeMDNS, false)
		if err != nil {
			t.Fatalf("failed to start advertising: %v", err)
		}

		err1 := advCloser.Close()
		if err1 != nil && !errors.Is(err1, net.ErrClosed) {
			t.Fatalf("unexpected error on first AdvertisingCloser.Close(): %v", err1)
		}

		err2 := advCloser.Close()
		if err2 != nil {
			t.Fatalf("expected nil error on second AdvertisingCloser.Close(), got %v", err2)
		}
	})
}
