package protocol

import (
	"os"
	"runtime"
)

// GetDefaultDevice builds and returns default Device information from the current system environment.
func GetDefaultDevice(port int, protocolStr string, download bool) Device {
	alias := GenerateRandomAlias()

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "Unknown"
	}

	var model string
	var devType DeviceType

	switch runtime.GOOS {
	case "windows":
		model = "Windows"
		devType = DeviceTypeDesktop
	case "darwin":
		model = "macOS"
		devType = DeviceTypeDesktop
	case "linux":
		model = "Linux"
		devType = DeviceTypeDesktop
	default:
		model = "Headless OS"
		devType = DeviceTypeHeadless
	}

	if hostname != "" && hostname != "Unknown" {
		model = model + " (" + hostname + ")"
	}

	return Device{
		Alias:       alias,
		Version:     ProtocolVersion,
		DeviceModel: model,
		DeviceType:  devType,
		Port:        port,
		Protocol:    protocolStr,
		Download:    download,
	}
}
