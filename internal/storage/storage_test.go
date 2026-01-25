package storage

import "testing"

func TestAppendEventReturnsCursor(t *testing.T) {
	st := NewInMemory()
	cursor, err := st.AppendEvent(Event{Type: "message.created"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cursor == 0 {
		t.Fatalf("expected non-zero cursor")
	}
}
