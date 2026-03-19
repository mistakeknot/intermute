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

func BenchmarkNormalizedOverlapLiteral(b *testing.B) {
	a, _ := NormalizePattern("src/main.go")
	bp, _ := NormalizePattern("src/main.go")
	for b.Loop() {
		_, _ = NormalizedOverlap(a, bp)
	}
}

func BenchmarkNormalizedOverlapWildcard(b *testing.B) {
	a, _ := NormalizePattern("src/*.go")
	bp, _ := NormalizePattern("src/main.go")
	for b.Loop() {
		_, _ = NormalizedOverlap(a, bp)
	}
}

func BenchmarkNormalizedOverlapComplex(b *testing.B) {
	a, _ := NormalizePattern("internal/*/handler/*.go")
	bp, _ := NormalizePattern("internal/*/handler/auth.go")
	for b.Loop() {
		_, _ = NormalizedOverlap(a, bp)
	}
}

func BenchmarkNormalizedOverlapNoMatch(b *testing.B) {
	a, _ := NormalizePattern("src/main.go")
	bp, _ := NormalizePattern("lib/util.go")
	for b.Loop() {
		_, _ = NormalizedOverlap(a, bp)
	}
}

func BenchmarkNormalizedOverlapCharClass(b *testing.B) {
	a, _ := NormalizePattern("src/[a-z]*.go")
	bp, _ := NormalizePattern("src/[A-Z]*.go")
	for b.Loop() {
		_, _ = NormalizedOverlap(a, bp)
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
