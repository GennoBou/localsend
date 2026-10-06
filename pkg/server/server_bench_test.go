package server

import (
	"crypto/tls"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/GennoBou/localsend/pkg/crypto"
	"github.com/GennoBou/localsend/pkg/protocol"
)

type repeatReader struct {
	remaining int64
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	toRead := len(p)
	if int64(toRead) > r.remaining {
		toRead = int(r.remaining)
	}
	for i := 0; i < toRead; i++ {
		p[i] = byte(i % 256)
	}
	r.remaining -= int64(toRead)
	return toRead, nil
}

func BenchmarkHandleUpload_Throughput(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "localsend-server-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var tlsCert tls.Certificate
	tlsCert, _, _, _, err = crypto.GenerateSelfSignedCert()
	if err != nil {
		b.Fatalf("failed to generate TLS cert: %v", err)
	}

	myDevice := protocol.Device{
		Alias:       "Bench Device",
		DeviceModel: "GoBench",
		DeviceType:  "desktop",
		Fingerprint: "bench-fingerprint",
		Port:        53317,
		Protocol:    "http",
		Download:    true,
	}

	s := NewServer(myDevice, tlsCert, tempDir, "", false)
	defer s.sessionMgr.Close()

	// Transfer 10MB per iteration (~320 chunks of 32KB per iteration)
	fileSize := int64(10 * 1024 * 1024)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		fileID := "bench-file-id"
		fileName := "bench-file.dat"
		meta := protocol.FileMetadata{
			ID:       fileID,
			FileName: fileName,
			Size:     fileSize,
			FileType: "application/octet-stream",
		}
		filesMeta := map[string]protocol.FileMetadata{meta.ID: meta}

		sessionObj := s.sessionMgr.CreateSession("127.0.0.1", false, filesMeta)
		token, _, _ := sessionObj.GetFileTokenAndMeta(fileID)

		query := url.Values{}
		query.Set("sessionId", sessionObj.ID)
		query.Set("fileId", fileID)
		query.Set("token", token)

		body := &repeatReader{remaining: fileSize}
		req := httptest.NewRequest("POST", "/api/localsend/v2/upload?"+query.Encode(), body)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		b.StartTimer()
		s.handleUpload(w, req)
	}
}

func BenchmarkHandleDownload_Lookup(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "localsend-server-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dummyFilePath := tempDir + "/testfile.txt"
	if err := os.WriteFile(dummyFilePath, []byte("hello world"), 0644); err != nil {
		b.Fatalf("failed to write dummy file: %v", err)
	}

	const numFiles = 1000
	sharedFiles := make([]ShareFile, numFiles)
	for i := 0; i < numFiles; i++ {
		sharedFiles[i] = ShareFile{
			ID:       "file-id-" + string(rune(i)),
			Path:     dummyFilePath,
			FileName: "file.txt",
			Size:     11,
			FileType: "text/plain",
		}
	}
	targetFileID := sharedFiles[numFiles-1].ID

	myDevice := protocol.Device{Alias: "Bench Device"}
	var tlsCert tls.Certificate
	s := NewServer(myDevice, tlsCert, tempDir, "", false)
	defer s.sessionMgr.Close()

	s.downloadMu.Lock()
	s.sharedFiles = sharedFiles
	s.sharedFilesMap = make(map[string]*ShareFile, len(sharedFiles))
	for i := range sharedFiles {
		s.sharedFilesMap[sharedFiles[i].ID] = &sharedFiles[i]
	}
	s.downloadSession = "bench-session"
	s.downloadMu.Unlock()

	query := url.Values{}
	query.Set("sessionId", s.downloadSession)
	query.Set("fileId", targetFileID)

	req := httptest.NewRequest("GET", "/api/localsend/v2/download?"+query.Encode(), nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		s.handleDownload(w, req)
	}
}
