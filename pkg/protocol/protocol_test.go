package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDevice_JSONSerialization(t *testing.T) {
	dev := Device{
		Alias:       "Test Device",
		Version:     "2.0",
		DeviceModel: "GoModel",
		DeviceType:  DeviceTypeDesktop,
		Fingerprint: "fp123456",
		Port:        53317,
		Protocol:    "https",
		Download:    true,
		IP:          "192.168.1.100", // IP must be ignored in JSON
	}

	data, err := json.Marshal(dev)
	if err != nil {
		t.Fatalf("failed to marshal Device: %v", err)
	}

	jsonStr := string(data)
	if jsonStr == "" {
		t.Fatal("marshaled JSON is empty")
	}

	// Verify that IP field is not present in serialized JSON
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal JSON into map: %v", err)
	}

	if _, exists := rawMap["IP"]; exists {
		t.Errorf("expected IP field to be omitted from JSON (json:\"-\"), but found in JSON")
	}
	if _, exists := rawMap["ip"]; exists {
		t.Errorf("expected ip field to be omitted from JSON (json:\"-\"), but found in JSON")
	}

	// Unmarshal back to Device struct
	var unmarshaledDev Device
	if err := json.Unmarshal(data, &unmarshaledDev); err != nil {
		t.Fatalf("failed to unmarshal JSON into Device: %v", err)
	}

	if unmarshaledDev.Alias != dev.Alias {
		t.Errorf("Alias = %q, want %q", unmarshaledDev.Alias, dev.Alias)
	}
	if unmarshaledDev.Version != dev.Version {
		t.Errorf("Version = %q, want %q", unmarshaledDev.Version, dev.Version)
	}
	if unmarshaledDev.DeviceModel != dev.DeviceModel {
		t.Errorf("DeviceModel = %q, want %q", unmarshaledDev.DeviceModel, dev.DeviceModel)
	}
	if unmarshaledDev.DeviceType != dev.DeviceType {
		t.Errorf("DeviceType = %q, want %q", unmarshaledDev.DeviceType, dev.DeviceType)
	}
	if unmarshaledDev.Fingerprint != dev.Fingerprint {
		t.Errorf("Fingerprint = %q, want %q", unmarshaledDev.Fingerprint, dev.Fingerprint)
	}
	if unmarshaledDev.Port != dev.Port {
		t.Errorf("Port = %d, want %d", unmarshaledDev.Port, dev.Port)
	}
	if unmarshaledDev.Protocol != dev.Protocol {
		t.Errorf("Protocol = %q, want %q", unmarshaledDev.Protocol, dev.Protocol)
	}
	if unmarshaledDev.Download != dev.Download {
		t.Errorf("Download = %v, want %v", unmarshaledDev.Download, dev.Download)
	}
	if unmarshaledDev.IP != "" {
		t.Errorf("IP = %q, want empty string after JSON unmarshal", unmarshaledDev.IP)
	}
}

func TestFileMetadata_JSONSerialization(t *testing.T) {
	t.Run("with omitempty fields populated", func(t *testing.T) {
		meta := FileMetadata{
			ID:       "file-1",
			FileName: "test.txt",
			Size:     1024,
			FileType: "text/plain",
			Sha256:   "dummy-sha256",
			Preview:  "preview-text",
			Metadata: map[string]string{"modified": "2026-09-06"},
		}

		data, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("failed to marshal FileMetadata: %v", err)
		}

		var unmarshaled FileMetadata
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal FileMetadata: %v", err)
		}

		if unmarshaled.Sha256 != meta.Sha256 {
			t.Errorf("Sha256 = %q, want %q", unmarshaled.Sha256, meta.Sha256)
		}
		if unmarshaled.Preview != meta.Preview {
			t.Errorf("Preview = %q, want %q", unmarshaled.Preview, meta.Preview)
		}
		if unmarshaled.Metadata["modified"] != "2026-09-06" {
			t.Errorf("Metadata['modified'] = %q, want '2026-09-06'", unmarshaled.Metadata["modified"])
		}
	})

	t.Run("with omitempty fields omitted", func(t *testing.T) {
		meta := FileMetadata{
			ID:       "file-2",
			FileName: "empty.bin",
			Size:     0,
			FileType: "application/octet-stream",
		}

		data, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("failed to marshal FileMetadata: %v", err)
		}

		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err != nil {
			t.Fatalf("failed to unmarshal JSON map: %v", err)
		}

		if _, exists := rawMap["sha256"]; exists {
			t.Errorf("expected sha256 to be omitted when empty")
		}
		if _, exists := rawMap["preview"]; exists {
			t.Errorf("expected preview to be omitted when empty")
		}
		if _, exists := rawMap["metadata"]; exists {
			t.Errorf("expected metadata to be omitted when empty")
		}
	})
}

func TestProtocolRequestsAndResponses_JSON(t *testing.T) {
	t.Run("AnnounceMessage", func(t *testing.T) {
		msg := AnnounceMessage{
			Device: Device{
				Alias:       "Announcer",
				Version:     "2.0",
				DeviceModel: "Model",
				DeviceType:  DeviceTypeMobile,
				Fingerprint: "fp-announce",
				Port:        53317,
				Protocol:    "https",
			},
			Announce: true,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("failed to marshal AnnounceMessage: %v", err)
		}

		var unmarshaled AnnounceMessage
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal AnnounceMessage: %v", err)
		}

		if !unmarshaled.Announce {
			t.Errorf("expected Announce = true, got %v", unmarshaled.Announce)
		}
		if unmarshaled.Alias != msg.Alias {
			t.Errorf("Alias = %q, want %q", unmarshaled.Alias, msg.Alias)
		}
	})

	t.Run("PrepareUploadRequest and Response", func(t *testing.T) {
		req := PrepareUploadRequest{
			Info: Device{Alias: "Sender", Version: "2.0"},
			Files: map[string]FileMetadata{
				"f1": {ID: "f1", FileName: "doc.pdf", Size: 200},
			},
		}

		reqData, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("failed to marshal PrepareUploadRequest: %v", err)
		}

		var unmarshaledReq PrepareUploadRequest
		if err := json.Unmarshal(reqData, &unmarshaledReq); err != nil {
			t.Fatalf("failed to unmarshal PrepareUploadRequest: %v", err)
		}

		if unmarshaledReq.Info.Alias != "Sender" {
			t.Errorf("Info.Alias = %q, want 'Sender'", unmarshaledReq.Info.Alias)
		}
		if len(unmarshaledReq.Files) != 1 {
			t.Errorf("Files count = %d, want 1", len(unmarshaledReq.Files))
		}

		resp := PrepareUploadResponse{
			SessionID: "session-123",
			Files: map[string]string{
				"f1": "token-xyz",
			},
		}

		respData, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal PrepareUploadResponse: %v", err)
		}

		var unmarshaledResp PrepareUploadResponse
		if err := json.Unmarshal(respData, &unmarshaledResp); err != nil {
			t.Fatalf("failed to unmarshal PrepareUploadResponse: %v", err)
		}

		if unmarshaledResp.SessionID != "session-123" {
			t.Errorf("SessionID = %q, want 'session-123'", unmarshaledResp.SessionID)
		}
		if unmarshaledResp.Files["f1"] != "token-xyz" {
			t.Errorf("Files['f1'] = %q, want 'token-xyz'", unmarshaledResp.Files["f1"])
		}
	})

	t.Run("PrepareDownloadResponse", func(t *testing.T) {
		resp := PrepareDownloadResponse{
			Info:      Device{Alias: "Downloader", Version: "2.0"},
			SessionID: "dl-session",
			Files: map[string]FileMetadata{
				"f1": {ID: "f1", FileName: "dl.txt", Size: 50},
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal PrepareDownloadResponse: %v", err)
		}

		var unmarshaled PrepareDownloadResponse
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal PrepareDownloadResponse: %v", err)
		}

		if unmarshaled.SessionID != "dl-session" {
			t.Errorf("SessionID = %q, want 'dl-session'", unmarshaled.SessionID)
		}
	})

	t.Run("InfoResponse", func(t *testing.T) {
		info := InfoResponse{
			Alias:       "InfoAlias",
			Version:     "2.0",
			DeviceModel: "InfoModel",
			DeviceType:  DeviceTypeDesktop,
			Fingerprint: "info-fp",
			Download:    true,
		}

		data, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("failed to marshal InfoResponse: %v", err)
		}

		var unmarshaled InfoResponse
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal InfoResponse: %v", err)
		}

		if unmarshaled.Alias != info.Alias {
			t.Errorf("Alias = %q, want %q", unmarshaled.Alias, info.Alias)
		}
		if unmarshaled.Download != info.Download {
			t.Errorf("Download = %v, want %v", unmarshaled.Download, info.Download)
		}
	})
}

func TestErrorDefinitions(t *testing.T) {
	errTests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrRejected", ErrRejected, "transfer rejected by receiver"},
		{"ErrInvalidPIN", ErrInvalidPIN, "invalid or missing PIN"},
		{"ErrSessionBusy", ErrSessionBusy, "receiver is busy with another session"},
		{"ErrDeviceNotFound", ErrDeviceNotFound, "target device not found"},
		{"ErrTransferCanceled", ErrTransferCanceled, "transfer canceled"},
		{"ErrChecksumMismatch", ErrChecksumMismatch, "checksum mismatch"},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatalf("error %s is nil", tt.name)
			}
			if tt.err.Error() != tt.expected {
				t.Errorf("%s message = %q, want %q", tt.name, tt.err.Error(), tt.expected)
			}
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, %s) expected true", tt.name, tt.name)
			}
		})
	}
}
