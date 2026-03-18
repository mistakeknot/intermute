package glob

import "testing"

func BenchmarkPatternsOverlapLiteral(b *testing.B) {
	for b.Loop() {
		_, _ = PatternsOverlap("src/main.go", "src/main.go")
	}
}

func BenchmarkPatternsOverlapWildcard(b *testing.B) {
	for b.Loop() {
		_, _ = PatternsOverlap("src/*.go", "src/main.go")
	}
}

func BenchmarkPatternsOverlapComplex(b *testing.B) {
	for b.Loop() {
		_, _ = PatternsOverlap("internal/*/handler/*.go", "internal/*/handler/auth.go")
	}
}

func BenchmarkPatternsOverlapNoMatch(b *testing.B) {
	for b.Loop() {
		_, _ = PatternsOverlap("src/main.go", "lib/util.go")
	}
}

func BenchmarkPatternsOverlapCharClass(b *testing.B) {
	for b.Loop() {
		_, _ = PatternsOverlap("src/[a-z]*.go", "src/[A-Z]*.go")
	}
}

func BenchmarkValidateComplexitySimple(b *testing.B) {
	for b.Loop() {
		_ = ValidateComplexity("src/*.go")
	}
}

func BenchmarkValidateComplexityDeep(b *testing.B) {
	for b.Loop() {
		_ = ValidateComplexity("a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/*.go")
	}
}
