package protocol

import (
	"slices"
	"strings"
	"testing"
)

func TestGenerateRandomAlias(t *testing.T) {
	t.Run("valid format and components", func(t *testing.T) {
		const iterations = 100
		for i := 0; i < iterations; i++ {
			alias := GenerateRandomAlias()
			if alias == "" {
				t.Fatalf("iteration %d: expected non-empty alias", i)
			}

			parts := strings.Split(alias, " ")
			if len(parts) != 2 {
				t.Fatalf("iteration %d: expected alias to consist of 2 space-separated words, got %q (parts count=%d)", i, alias, len(parts))
			}

			adj, fruit := parts[0], parts[1]

			if !slices.Contains(adjectives, adj) {
				t.Errorf("iteration %d: adjective %q not found in adjectives slice", i, adj)
			}

			if !slices.Contains(fruits, fruit) {
				t.Errorf("iteration %d: fruit %q not found in fruits slice", i, fruit)
			}
		}
	})

	t.Run("randomness across multiple calls", func(t *testing.T) {
		generated := make(map[string]bool)
		const iterations = 50

		for i := 0; i < iterations; i++ {
			generated[GenerateRandomAlias()] = true
		}

		// With 20 adjectives and 20 fruits, there are 400 possible combinations.
		// Generating 50 aliases should yield multiple distinct aliases.
		if len(generated) <= 1 {
			t.Errorf("expected multiple unique aliases across %d calls, got %d unique alias(es)", iterations, len(generated))
		}
	})
}
