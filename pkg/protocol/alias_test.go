package protocol

import (
	"strings"
	"testing"
)

func TestGenerateRandomAlias(t *testing.T) {
	adjMap := make(map[string]bool, len(adjectives))
	for _, adj := range adjectives {
		adjMap[adj] = true
	}

	fruitMap := make(map[string]bool, len(fruits))
	for _, fruit := range fruits {
		fruitMap[fruit] = true
	}

	const iterations = 100
	generatedAliases := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		alias := GenerateRandomAlias()

		if alias == "" {
			t.Fatalf("iteration %d: GenerateRandomAlias returned an empty string", i)
		}

		parts := strings.Split(alias, " ")
		if len(parts) != 2 {
			t.Fatalf("iteration %d: expected alias %q to consist of 2 space-separated words, got %d parts", i, alias, len(parts))
		}

		adj, fruit := parts[0], parts[1]

		if !adjMap[adj] {
			t.Errorf("iteration %d: adjective %q in alias %q is not in predefined adjectives list", i, adj, alias)
		}

		if !fruitMap[fruit] {
			t.Errorf("iteration %d: fruit %q in alias %q is not in predefined fruits list", i, fruit, alias)
		}

		generatedAliases[alias] = true
	}

	if len(generatedAliases) <= 1 {
		t.Errorf("expected multiple distinct aliases across %d runs, got %d distinct alias(es)", iterations, len(generatedAliases))
	}
}
