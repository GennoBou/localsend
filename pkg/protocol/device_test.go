package protocol

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestGetDefaultDevice(t *testing.T) {
	port := 53317
	protocolStr := "https"
	download := true

	dev := GetDefaultDevice(port, protocolStr, download)

	// Verify passed parameters
	if dev.Port != port {
		t.Errorf("Expected Port %d, got %d", port, dev.Port)
	}
	if dev.Protocol != protocolStr {
		t.Errorf("Expected Protocol %q, got %q", protocolStr, dev.Protocol)
	}
	if dev.Download != download {
		t.Errorf("Expected Download %v, got %v", download, dev.Download)
	}

	// Verify protocol version
	if dev.Version != ProtocolVersion {
		t.Errorf("Expected Version %q, got %q", ProtocolVersion, dev.Version)
	}

	// Verify generated alias
	if dev.Alias == "" {
		t.Error("Expected non-empty Alias")
	}

	// Verify OS-dependent device model and device type
	var expectedModelPrefix string
	var expectedType DeviceType

	switch runtime.GOOS {
	case "windows":
		expectedModelPrefix = "Windows"
		expectedType = DeviceTypeDesktop
	case "darwin":
		expectedModelPrefix = "macOS"
		expectedType = DeviceTypeDesktop
	case "linux":
		expectedModelPrefix = "Linux"
		expectedType = DeviceTypeDesktop
	default:
		expectedModelPrefix = "Headless OS"
		expectedType = DeviceTypeHeadless
	}

	if dev.DeviceType != expectedType {
		t.Errorf("Expected DeviceType %q, got %q", expectedType, dev.DeviceType)
	}

	if !strings.HasPrefix(dev.DeviceModel, expectedModelPrefix) {
		t.Errorf("Expected DeviceModel to start with %q, got %q", expectedModelPrefix, dev.DeviceModel)
	}

	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "Unknown" {
		expectedSub := "(" + hostname + ")"
		if !strings.Contains(dev.DeviceModel, expectedSub) {
			t.Errorf("Expected DeviceModel %q to contain %q", dev.DeviceModel, expectedSub)
		}
	}
}

func TestGenerateRandomAlias(t *testing.T) {
	alias := GenerateRandomAlias()
	if alias == "" {
		t.Fatal("Expected non-empty alias")
	}

	parts := strings.Split(alias, " ")
	if len(parts) != 2 {
		t.Errorf("Expected alias to consist of two words (adjective fruit), got %q", alias)
	}
}
