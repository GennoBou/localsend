package protocol

import "errors"

// Protocol-related constant definitions
const (
	// DefaultPort is the default port number (TCP/UDP) for the LocalSend protocol.
	DefaultPort = 53317

	// MulticastGroup is the multicast IP address for LocalSend device discovery.
	MulticastGroup = "224.0.0.167"

	// MulticastAddr is the combination of multicast address and port.
	MulticastAddr = "224.0.0.167:53317"
	// ProtocolVersion is the supported version of the LocalSend protocol.
	ProtocolVersion = "2.0"
)

// DeviceType represents the type of device.
type DeviceType string

const (
	DeviceTypeMobile   DeviceType = "mobile"
	DeviceTypeDesktop  DeviceType = "desktop"
	DeviceTypeWeb      DeviceType = "web"
	DeviceTypeHeadless DeviceType = "headless"
	DeviceTypeServer   DeviceType = "server"
)

// Device represents device information on the LocalSend network.
type Device struct {
	Alias       string     `json:"alias"`
	Version     string     `json:"version"` // Protocol version (e.g., "2.0")
	DeviceModel string     `json:"deviceModel"`
	DeviceType  DeviceType `json:"deviceType"`
	Fingerprint string     `json:"fingerprint"`
	Port        int        `json:"port"`
	Protocol    string     `json:"protocol"` // "http" or "https"
	Download    bool       `json:"download"` // Whether reverse transfer via web browser is supported
	IP          string     `json:"-"`        // For in-memory management (not included in JSON)
}

// AnnounceMessage is the device notification message sent via UDP multicast.
type AnnounceMessage struct {
	Device
	Announce bool `json:"announce"` // If true, requests other devices to respond via /register
}

// FileMetadata represents the metadata of the file to be transferred.
type FileMetadata struct {
	ID       string            `json:"id"`
	FileName string            `json:"fileName"`
	Size     int64             `json:"size"`
	FileType string            `json:"fileType"`
	Sha256   string            `json:"sha256,omitempty"`
	Preview  string            `json:"preview,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"` // E.g., modified, accessed
}

// PrepareUploadRequest is the request body for POST /api/localsend/v2/prepare-upload.
type PrepareUploadRequest struct {
	Info  Device                  `json:"info"`
	Files map[string]FileMetadata `json:"files"`
}

// PrepareUploadResponse is the response body for POST /api/localsend/v2/prepare-upload.
type PrepareUploadResponse struct {
	SessionID string            `json:"sessionId"`
	Files     map[string]string `json:"files"` // fileId -> token
}

// PrepareDownloadResponse is the response body for POST /api/localsend/v2/prepare-download.
type PrepareDownloadResponse struct {
	Info      Device                  `json:"info"`
	SessionID string                  `json:"sessionId"`
	Files     map[string]FileMetadata `json:"files"`
}

// InfoResponse is the response body for GET /api/localsend/v2/info.
type InfoResponse struct {
	Alias       string     `json:"alias"`
	Version     string     `json:"version"`
	DeviceModel string     `json:"deviceModel"`
	DeviceType  DeviceType `json:"deviceType"`
	Fingerprint string     `json:"fingerprint"`
	Download    bool       `json:"download"`
}

// Common error definitions
var (
	// ErrRejected is returned when the transfer is rejected by the receiver.
	ErrRejected = errors.New("transfer rejected by receiver")

	// ErrInvalidPIN is returned when the specified PIN is invalid or missing.
	ErrInvalidPIN = errors.New("invalid or missing PIN")

	// ErrSessionBusy is returned when the receiver is busy processing another session.
	ErrSessionBusy = errors.New("receiver is busy with another session")

	// ErrDeviceNotFound is returned when the target device is not found.
	ErrDeviceNotFound = errors.New("target device not found")

	// ErrTransferCanceled is returned when the transfer is canceled.
	ErrTransferCanceled = errors.New("transfer canceled")
)
