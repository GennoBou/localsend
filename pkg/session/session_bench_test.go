package session

import (
	"fmt"
	"testing"

	"github.com/GennoBou/localsend/pkg/protocol"
)

func BenchmarkIsCompleted_Incomplete(b *testing.B) {
	for _, fileCount := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("Files_%d", fileCount), func(b *testing.B) {
			filesMetadata := make(map[string]protocol.FileMetadata, fileCount)
			for i := 0; i < fileCount; i++ {
				id := fmt.Sprintf("file_%d", i)
				filesMetadata[id] = protocol.FileMetadata{
					ID:       id,
					FileName: fmt.Sprintf("file_%d.dat", i),
					Size:     1024 * 1024,
				}
			}

			sess := NewUploadSession("127.0.0.1", true, filesMetadata)

			// Update all except the last file to complete
			for i := 0; i < fileCount-1; i++ {
				id := fmt.Sprintf("file_%d", i)
				sess.UpdateProgress(id, 1024*1024)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = sess.IsCompleted()
			}
		})
	}
}

func BenchmarkIsCompleted_Completed(b *testing.B) {
	for _, fileCount := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("Files_%d", fileCount), func(b *testing.B) {
			filesMetadata := make(map[string]protocol.FileMetadata, fileCount)
			for i := 0; i < fileCount; i++ {
				id := fmt.Sprintf("file_%d", i)
				filesMetadata[id] = protocol.FileMetadata{
					ID:       id,
					FileName: fmt.Sprintf("file_%d.dat", i),
					Size:     1024 * 1024,
				}
			}

			sess := NewUploadSession("127.0.0.1", true, filesMetadata)

			for i := 0; i < fileCount; i++ {
				id := fmt.Sprintf("file_%d", i)
				sess.UpdateProgress(id, 1024*1024)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = sess.IsCompleted()
			}
		})
	}
}
