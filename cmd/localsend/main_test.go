package main

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/GennoBou/localsend/pkg/client"
)

func BenchmarkProgressFunc_Unoptimized(b *testing.B) {
	// Suppress stdout output during benchmark
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	go io.Copy(io.Discard, r)
	defer func() {
		w.Close()
		os.Stdout = oldStdout
	}()

	numFiles := 100
	files := make([]client.SendFileSource, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = client.SendFileSource{
			ID:       fmt.Sprintf("file-id-%d", i),
			FileName: fmt.Sprintf("test-file-%d.txt", i),
			Size:     1024 * 1024 * 100, // 100MB
		}
	}

	progressFunc := func(fileID string, sentBytes int64) {
		var fileMeta client.SendFileSource
		for _, f := range files {
			if f.ID == fileID {
				fileMeta = f
				break
			}
		}
		percent := float64(sentBytes) / float64(fileMeta.Size) * 100
		fmt.Printf("\rProgress for %s: %.1f%% (%d/%d bytes)", fileMeta.FileName, percent, sentBytes, fileMeta.Size)
	}

	targetFileID := "file-id-99"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progressFunc(targetFileID, int64(i*1024))
	}
}

func BenchmarkProgressFunc_Optimized(b *testing.B) {
	// Suppress stdout output during benchmark
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	go io.Copy(io.Discard, r)
	defer func() {
		w.Close()
		os.Stdout = oldStdout
	}()

	numFiles := 100
	files := make([]client.SendFileSource, numFiles)
	fileMap := make(map[string]client.SendFileSource, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = client.SendFileSource{
			ID:       fmt.Sprintf("file-id-%d", i),
			FileName: fmt.Sprintf("test-file-%d.txt", i),
			Size:     1024 * 1024 * 100, // 100MB
		}
		fileMap[files[i].ID] = files[i]
	}

	var lastUpdate time.Time
	progressFunc := func(fileID string, sentBytes int64) {
		fileMeta, ok := fileMap[fileID]
		if !ok {
			return
		}
		now := time.Now()
		if now.Sub(lastUpdate) < 100*time.Millisecond && sentBytes < fileMeta.Size {
			return
		}
		lastUpdate = now
		percent := float64(sentBytes) / float64(fileMeta.Size) * 100
		fmt.Printf("\rProgress for %s: %.1f%% (%d/%d bytes)", fileMeta.FileName, percent, sentBytes, fileMeta.Size)
	}

	targetFileID := "file-id-99"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progressFunc(targetFileID, int64(i*1024))
	}
}
