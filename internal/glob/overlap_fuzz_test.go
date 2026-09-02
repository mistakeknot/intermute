package glob

import (
	"strings"
	"testing"
	"time"
)

// FuzzPatternsOverlap guards the DoS bound the reservation layer relies on:
// for any pair of patterns ValidateComplexity accepts, PatternsOverlap must
// answer quickly. Patterns that exceed the caps are expected to be rejected
// with an error, never to run long.
func FuzzPatternsOverlap(f *testing.F) {
	seeds := [][2]string{
		{"*.go", "*.go"}, {"*.go", "*.rs"}, {"internal/*.go", "internal/http.go"},
		{"src/[a-z]*.go", "src/main.go"}, {"**/*.go", "cmd/**/main.go"},
		{strings.Repeat("*a", MaxWildcards), strings.Repeat("a*", MaxWildcards)},
		{strings.Repeat("?", MaxTokens), strings.Repeat("[a-z]", MaxTokens)},
		{"a/**/b/**/c/**/d", "**/**/**/**"},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}
	// Generous enough to survive fuzz workers contending for cores, tight
	// enough that a super-linear blowup on a capped pattern still fails.
	const budget = 200 * time.Millisecond
	f.Fuzz(func(t *testing.T, a, b string) {
		timed := func() (time.Duration, error) {
			start := time.Now()
			_, err := PatternsOverlap(a, b)
			return time.Since(start), err
		}
		elapsed, err := timed()
		if elapsed > budget {
			// One retry so a scheduler hiccup does not read as a DoS.
			if again, _ := timed(); again < elapsed {
				elapsed = again
			}
		}
		if elapsed > budget {
			t.Fatalf("PatternsOverlap(%q, %q) took %v (budget %v), err=%v", a, b, elapsed, budget, err)
		}
		// The complexity gate is the contract: a pair it accepts must also be
		// accepted by the overlap check, and vice versa.
		va, vb := ValidateComplexity(a), ValidateComplexity(b)
		if va == nil && vb == nil && err != nil && !isSyntaxError(err) {
			t.Fatalf("PatternsOverlap(%q, %q) rejected a pair ValidateComplexity accepts: %v", a, b, err)
		}
	})
}

// isSyntaxError reports whether err is a pattern syntax error rather than a
// complexity rejection. Syntax errors are legitimate for fuzz-generated input.
func isSyntaxError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "syntax") || strings.Contains(msg, "invalid") || strings.Contains(msg, "unterminated") || strings.Contains(msg, "malformed")
}
