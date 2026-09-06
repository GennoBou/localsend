package protocol

import (
	"strings"
	"testing"
)

func TestGenerateRandomAlias(t *testing.T) {
	adjMap := make(map[string]bool)
	for _, adj := range adjectives {
		adjMap[adj] = true
	}

	fruitMap := make(map[string]bool)
	for _, fruit := range fruits {
		fruitMap[fruit] = true
	}

	for i := 0; i < 100; i++ {
		alias := GenerateRandomAlias()
		if alias == "" {
			t.Fatalf("GenerateRandomAlias() returned an empty string at iteration %d", i)
		}

		parts := strings.Split(alias, " ")
		if len(parts) != 2 {
			t.Fatalf("GenerateRandomAlias() = %q; expected 2 words separated by a space, got %d parts", alias, len(parts))
		}

		adj, fruit := parts[0], parts[1]

		if !adjMap[adj] {
			t.Errorf("GenerateRandomAlias() returned unknown adjective %q in alias %q", adj, alias)
		}

		if !fruitMap[fruit] {
			t.Errorf("GenerateRandomAlias() returned unknown fruit %q in alias %q", fruit, alias)
		}
	}
}
