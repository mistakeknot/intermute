package sqlite

import "testing"

func NewSQLiteTest(t *testing.T) *Store {
	t.Helper()
	st, err := NewInMemory()
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	return st
}
