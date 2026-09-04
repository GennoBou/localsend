package main

import (
	"testing"

	"github.com/GennoBou/localsend/pkg/client"
)

// Calculate total size using a separate iteration over the files slice (Baseline)
func calculateTotalSizeRedundant(files []client.SendFileSource) int64 {
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}
	return totalSize
}

// Calculate total size while constructing / processing files directly (Optimized)
func BenchmarkTotalSizeCalculation(b *testing.B) {
	numFiles := 1000
	files := make([]client.SendFileSource, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = client.SendFileSource{
			Size: int64(i * 1024),
		}
	}

	b.Run("RedundantLoop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = calculateTotalSizeRedundant(files)
		}
	})

	b.Run("AccumulatedDuringConstruction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var totalSize int64
			for j := 0; j < numFiles; j++ {
				totalSize += files[j].Size
			}
			_ = totalSize
		}
	})
}
