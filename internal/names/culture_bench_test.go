package names

import "testing"

func BenchmarkGenerate(b *testing.B) {
	for b.Loop() {
		_ = Generate()
	}
}
