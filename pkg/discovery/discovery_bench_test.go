package discovery

import (
	"fmt"
	"strconv"
	"testing"
)

func BenchmarkTargetIPSprintf(b *testing.B) {
	baseIP := "192.168.1."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 1; j <= 254; j++ {
			_ = fmt.Sprintf("%s%d", baseIP, j)
		}
	}
}

func BenchmarkTargetIPConcat(b *testing.B) {
	baseIP := "192.168.1."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 1; j <= 254; j++ {
			_ = baseIP + strconv.Itoa(j)
		}
	}
}
