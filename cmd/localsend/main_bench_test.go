package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// BenchmarkSequentialHashing benchmarks hashing 10 files sequentially.
func BenchmarkSequentialHashing(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "hash_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	filePaths := make([]string, 10)
	dummyData := make([]byte, 10*1024*1024) // 10MB each
	for i := range dummyData {
		dummyData[i] = byte(i % 256)
	}

	for i := 0; i < 10; i++ {
		p := filepath.Join(tempDir, fmt.Sprintf("file_%d.bin", i))
		if err := os.WriteFile(p, dummyData, 0644); err != nil {
			b.Fatal(err)
		}
		filePaths[i] = p
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, fp := range filePaths {
			_, _ = computeFileSha256(fp)
		}
	}
}

// BenchmarkConcurrentHashing benchmarks hashing 10 files concurrently using WaitGroup.
func BenchmarkConcurrentHashing(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "hash_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	filePaths := make([]string, 10)
	dummyData := make([]byte, 10*1024*1024) // 10MB each
	for i := range dummyData {
		dummyData[i] = byte(i % 256)
	}

	for i := 0; i < 10; i++ {
		p := filepath.Join(tempDir, fmt.Sprintf("file_%d.bin", i))
		if err := os.WriteFile(p, dummyData, 0644); err != nil {
			b.Fatal(err)
		}
		filePaths[i] = p
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(len(filePaths))
		for _, fp := range filePaths {
			go func(filePath string) {
				defer wg.Done()
				_, _ = computeFileSha256(filePath)
			}(fp)
		}
		wg.Wait()
	}
}
