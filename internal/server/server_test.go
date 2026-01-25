package server

import "testing"

func TestServerStarts(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error without addr")
	}
}
