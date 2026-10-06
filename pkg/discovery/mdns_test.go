package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/protocol"
	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
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

func TestRegisterMDNSService_MulticastConnError(t *testing.T) {
	origConnFunc := createMulticastConnV4Func
	defer func() { createMulticastConnV4Func = origConnFunc }()

	expectedErr := fmt.Errorf("simulated multicast conn error")
	createMulticastConnV4Func = func() (*ipv4.PacketConn, *net.UDPConn, error) {
		return nil, nil, expectedErr
	}

	ctx := context.Background()
	dev := protocol.Device{
		Alias:       "TestDevice",
		Fingerprint: "1234567890",
	}

	closer, err := RegisterMDNSService(ctx, dev)
	if err == nil {
		if closer != nil {
			closer.Close()
		}
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("expected error containing %q, got %q", expectedErr.Error(), err.Error())
	}
}

func TestRegisterMDNSService_ServerCreationError(t *testing.T) {
	origServerFunc := newMDNSServerFunc
	defer func() { newMDNSServerFunc = origServerFunc }()

	expectedErr := fmt.Errorf("simulated server creation error")
	newMDNSServerFunc = func(connV4 *ipv4.PacketConn, connV6 *ipv6.PacketConn, options ...mdns.ServerOption) (*mdns.Conn, error) {
		return nil, expectedErr
	}

	ctx := context.Background()
	dev := protocol.Device{
		Alias:       "TestDevice",
		Fingerprint: "1234567890",
	}

	closer, err := RegisterMDNSService(ctx, dev)
	if err == nil {
		if closer != nil {
			closer.Close()
		}
		t.Fatal("expected error when mdns.NewServer fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create mDNS advertising server") {
		t.Errorf("expected server creation error message, got: %v", err)
	}
}

func TestStartMDNSListener_ErrorPaths(t *testing.T) {
	t.Run("MulticastConnError", func(t *testing.T) {
		origConnFunc := createMulticastConnV4Func
		defer func() { createMulticastConnV4Func = origConnFunc }()

		expectedErr := fmt.Errorf("simulated multicast conn error")
		createMulticastConnV4Func = func() (*ipv4.PacketConn, *net.UDPConn, error) {
			return nil, nil, expectedErr
		}

		err := StartMDNSListener(context.Background(), func(protocol.Device) {})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Errorf("expected %q, got %q", expectedErr.Error(), err.Error())
		}
	})

	t.Run("ServerCreationError", func(t *testing.T) {
		origServerFunc := newMDNSServerFunc
		defer func() { newMDNSServerFunc = origServerFunc }()

		expectedErr := fmt.Errorf("simulated server creation error")
		newMDNSServerFunc = func(connV4 *ipv4.PacketConn, connV6 *ipv6.PacketConn, options ...mdns.ServerOption) (*mdns.Conn, error) {
			return nil, expectedErr
		}

		err := StartMDNSListener(context.Background(), func(protocol.Device) {})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create mDNS browsing server") {
			t.Errorf("expected server creation error message, got: %v", err)
		}
	})
}

func TestRegisterMDNSService_InstanceAndHostNameLogic(t *testing.T) {
	// Test short fingerprint (< 8 chars) and long alias (> 63 chars)
	longAlias := "ThisIsAVeryLongDeviceAliasThatExceedsSixtyThreeCharactersLimitInMDNSInstanceName"
	shortFingerprintDev := protocol.Device{
		Alias:       longAlias,
		Fingerprint: "short",
		Port:        53322,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closer, err := RegisterMDNSService(ctx, shortFingerprintDev)
	if err != nil {
		t.Fatalf("failed to register service with short fingerprint and long alias: %v", err)
	}
	defer closer.Close()

	// Cancel context to test context-cancellation cleanup goroutine
	cancel()
}

func TestMDNSCloser_Idempotency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dev := protocol.Device{
		Alias:       "CloseTestDevice",
		Fingerprint: "1234567890",
		Port:        53323,
	}

	closer, err := RegisterMDNSService(ctx, dev)
	if err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	// First Close should succeed (or return whatever underlying error, usually nil)
	err1 := closer.Close()
	// Second Close should return same err (nil) and not panic
	err2 := closer.Close()

	if err1 != err2 {
		t.Errorf("expected second close error %v to match first close error %v", err2, err1)
	}
}
