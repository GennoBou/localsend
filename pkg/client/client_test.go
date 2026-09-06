package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func TestClient_SendFiles_ChecksumMismatch(t *testing.T) {
	var capturedSha256 string
	sessionID := "test-session-id"
	fileID := "file-1"
	token := "file-token-1"

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			var req protocol.PrepareUploadRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if f, ok := req.Files[fileID]; ok {
				capturedSha256 = f.Sha256
			}

			resp := protocol.PrepareUploadResponse{
				SessionID: sessionID,
				Files: map[string]string{
					fileID: token,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case "/api/localsend/v2/upload":
			// Simulate 422 Unprocessable Entity
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte("Checksum mismatch"))

		case "/api/localsend/v2/cancel":
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())

	target := &protocol.Device{
		Alias:       "TestReceiver",
		Version:     "2.0",
		DeviceModel: "Test",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "",
		Port:        port,
		Protocol:    "http",
		IP:          u.Hostname(),
	}

	client, err := NewClient(protocol.Device{
		Alias:       "TestSender",
		Version:     "2.0",
		Fingerprint: "sender-fingerprint",
	}, nil, "", false, "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	testContent := "test content"
	testSha256 := "expected-sha256-hash"

	files := []SendFileSource{
		{
			ID:       fileID,
			FileName: "test.txt",
			Size:     int64(len(testContent)),
			FileType: "text/plain",
			Sha256:   testSha256,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(testContent)), nil
			},
		},
	}

	results, err := client.SendFiles(context.Background(), target, files, "", nil)
	if err == nil {
		t.Fatal("expected error due to 422 checksum mismatch, got nil")
	}

	if !errors.Is(err, protocol.ErrChecksumMismatch) {
		t.Errorf("expected ErrChecksumMismatch, got %v", err)
	}

	if capturedSha256 != testSha256 {
		t.Errorf("expected captured sha256 to be '%s', got '%s'", testSha256, capturedSha256)
	}

	if len(results) != 1 || results[0].Err == nil {
		t.Errorf("expected results to indicate failure for file %s", fileID)
	}
}

func TestClient_SendFiles_Success(t *testing.T) {
	sessionID := "test-session-2"
	fileID := "file-2"
	token := "file-token-2"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			resp := protocol.PrepareUploadResponse{
				SessionID: sessionID,
				Files: map[string]string{
					fileID: token,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case "/api/localsend/v2/upload":
			if r.URL.Query().Get("sessionId") != sessionID || r.URL.Query().Get("fileId") != fileID || r.URL.Query().Get("token") != token {
				http.Error(w, "bad parameters", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())

	target := &protocol.Device{
		Alias:       "TestReceiver",
		Version:     "2.0",
		DeviceModel: "Test",
		DeviceType:  protocol.DeviceTypeDesktop,
		Fingerprint: "",
		Port:        port,
		Protocol:    "http",
		IP:          u.Hostname(),
	}

	client, err := NewClient(protocol.Device{
		Alias:       "TestSender",
		Version:     "2.0",
		Fingerprint: "sender-fingerprint",
	}, nil, "", false, "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	testContent := "hello successful upload"

	files := []SendFileSource{
		{
			ID:       fileID,
			FileName: "success.txt",
			Size:     int64(len(testContent)),
			FileType: "text/plain",
			Sha256:   "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(testContent)), nil
			},
		},
	}

	results, err := client.SendFiles(context.Background(), target, files, "", nil)
	if err != nil {
		t.Fatalf("expected successful send, got error: %v", err)
	}

	if len(results) != 1 || results[0].Err != nil {
		t.Errorf("expected result to have no error, got %v", results[0].Err)
	}
}

func TestClient_SendText(t *testing.T) {
	sessionID := "text-session"
	var receivedFileMeta protocol.FileMetadata

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			var req protocol.PrepareUploadRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			for _, f := range req.Files {
				receivedFileMeta = f
			}
			resp := protocol.PrepareUploadResponse{
				SessionID: sessionID,
				Files: map[string]string{
					receivedFileMeta.ID: "token-text",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/localsend/v2/upload":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	target := &protocol.Device{
		Alias:      "Receiver",
		Version:    "2.0",
		DeviceType: protocol.DeviceTypeDesktop,
		Port:       port,
		Protocol:   "http",
		IP:         u.Hostname(),
	}

	client, err := NewClient(protocol.Device{Alias: "Sender"}, nil, "", false, "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	textMessage := "Hello LocalSend from Go CLI!"
	textSource := NewTextSendSource(textMessage)

	if textSource.Preview != textMessage {
		t.Errorf("expected preview %q, got %q", textMessage, textSource.Preview)
	}
	if textSource.FileType != "text/plain" {
		t.Errorf("expected FileType text/plain, got %q", textSource.FileType)
	}

	results, err := client.SendFiles(context.Background(), target, []SendFileSource{textSource}, "", nil)
	if err != nil {
		t.Fatalf("failed to send text: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Errorf("expected success result, got %v", results)
	}

	if receivedFileMeta.Preview != textMessage {
		t.Errorf("expected server to receive preview %q, got %q", textMessage, receivedFileMeta.Preview)
	}
}
