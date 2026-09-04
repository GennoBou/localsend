package protocol

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestGetDefaultDevice(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		protocolStr string
		download    bool
	}{
		{
			name:        "HTTPS with download enabled",
			port:        53317,
			protocolStr: "https",
			download:    true,
		},
		{
			name:        "HTTP with download disabled",
			port:        8080,
			protocolStr: "http",
			download:    false,
		},
	}

	hostname, err := os.Hostname()
	hasHostname := err == nil && hostname != "" && hostname != "Unknown"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := GetDefaultDevice(tt.port, tt.protocolStr, tt.download)

			if dev.Port != tt.port {
				t.Errorf("Port = %d, want %d", dev.Port, tt.port)
			}
			if dev.Protocol != tt.protocolStr {
				t.Errorf("Protocol = %q, want %q", dev.Protocol, tt.protocolStr)
			}
			if dev.Download != tt.download {
				t.Errorf("Download = %v, want %v", dev.Download, tt.download)
			}
			if dev.Version != ProtocolVersion {
				t.Errorf("Version = %q, want %q", dev.Version, ProtocolVersion)
			}
			if dev.Alias == "" {
				t.Error("Alias should not be empty")
			}

			// Validate DeviceType and DeviceModel according to runtime.GOOS
			var expectedType DeviceType
			var expectedModelPrefix string

			switch runtime.GOOS {
			case "windows":
				expectedType = DeviceTypeDesktop
				expectedModelPrefix = "Windows"
			case "darwin":
				expectedType = DeviceTypeDesktop
				expectedModelPrefix = "macOS"
			case "linux":
				expectedType = DeviceTypeDesktop
				expectedModelPrefix = "Linux"
			default:
				expectedType = DeviceTypeHeadless
				expectedModelPrefix = "Headless OS"
			}

			if dev.DeviceType != expectedType {
				t.Errorf("DeviceType = %q, want %q", dev.DeviceType, expectedType)
			}

			if !strings.HasPrefix(dev.DeviceModel, expectedModelPrefix) {
				t.Errorf("DeviceModel = %q, want prefix %q", dev.DeviceModel, expectedModelPrefix)
			}

			if hasHostname {
				expectedSuffix := "(" + hostname + ")"
				if !strings.HasSuffix(dev.DeviceModel, expectedSuffix) {
					t.Errorf("DeviceModel = %q, want suffix %q", dev.DeviceModel, expectedSuffix)
				}
			}
		})
	}
}

func TestGenerateRandomAlias(t *testing.T) {
	alias := GenerateRandomAlias()
	if alias == "" {
		t.Error("GenerateRandomAlias() returned an empty string")
	}
	parts := strings.Split(alias, " ")
	if len(parts) != 2 {
		t.Errorf("GenerateRandomAlias() = %q, expected format 'Adjective Fruit'", alias)
	}
}
