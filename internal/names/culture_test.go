package names

import "testing"

func TestGenerate(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := Generate()
		if name == "" {
			t.Fatal("generated empty name")
		}
		seen[name] = true
	}
	// Should generate variety (at least 10 unique names in 100 tries)
	if len(seen) < 10 {
		t.Fatalf("expected variety, got only %d unique names", len(seen))
	}
}

func TestGenerateExamples(t *testing.T) {
	t.Log("Sample generated names:")
	for i := 0; i < 10; i++ {
		t.Logf("  %s", Generate())
	}
}
