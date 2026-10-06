package client

import (
	"fmt"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func BenchmarkUploadURLConstruction_Baseline(b *testing.B) {
	target := &protocol.Device{
		Protocol: "http",
		IP:       "192.168.1.100",
		Port:     53317,
	}
	sessionID := "session-12345"
	fileID := "file-67890"
	token := "token-abcde"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uploadURL := fmt.Sprintf("%s://%s:%d/api/localsend/v2/upload?sessionId=%s&fileId=%s&token=%s", target.Protocol, target.IP, target.Port, sessionID, fileID, token)
		_ = uploadURL
	}
}

func BenchmarkSendResultsAllocation_Unallocated(b *testing.B) {
	fileCount := 100
	files := make([]SendFileSource, fileCount)
	for i := 0; i < fileCount; i++ {
		files[i] = SendFileSource{ID: fmt.Sprintf("file-%d", i)}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var results []SendResult
		for _, f := range files {
			results = append(results, SendResult{FileID: f.ID, Err: nil})
		}
		_ = results
	}
}

func BenchmarkSendResultsAllocation_Preallocated(b *testing.B) {
	fileCount := 100
	files := make([]SendFileSource, fileCount)
	for i := 0; i < fileCount; i++ {
		files[i] = SendFileSource{ID: fmt.Sprintf("file-%d", i)}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results := make([]SendResult, 0, len(files))
		for _, f := range files {
			results = append(results, SendResult{FileID: f.ID, Err: nil})
		}
		_ = results
	}
}

func BenchmarkUploadURLConstruction_Optimized(b *testing.B) {
	target := &protocol.Device{
		Protocol: "http",
		IP:       "192.168.1.100",
		Port:     53317,
	}
	sessionID := "session-12345"
	fileID := "file-67890"
	token := "token-abcde"

	uploadBaseURL := fmt.Sprintf("%s://%s:%d/api/localsend/v2/upload?sessionId=%s", target.Protocol, target.IP, target.Port, sessionID)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uploadURL := uploadBaseURL + "&fileId=" + fileID + "&token=" + token
		_ = uploadURL
	}
}
