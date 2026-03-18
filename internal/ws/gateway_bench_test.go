package ws

import (
	"fmt"
	"testing"

	"nhooyr.io/websocket"
)

// populateHub adds n fake connections spread across projects/agents.
// Connections are distributed as: 1 project, 1 agent, n connections.
func populateHub(h *Hub, n int) {
	for i := 0; i < n; i++ {
		// new(websocket.Conn) creates a distinct pointer — snapshot only
		// reads map keys, never dereferences the conn.
		h.add("proj", "agent", new(websocket.Conn))
	}
}

// populateHubSpread adds n connections spread across multiple projects/agents.
func populateHubSpread(h *Hub, n int) {
	projects := 5
	agents := 4
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("proj-%d", i%projects)
		a := fmt.Sprintf("agent-%d", i%agents)
		h.add(p, a, new(websocket.Conn))
	}
}

func BenchmarkSnapshot10(b *testing.B) {
	h := NewHub()
	populateHub(h, 10)
	b.ResetTimer()
	for b.Loop() {
		buf := h.snapshot("proj", "agent")
		h.putSnapshot(buf)
	}
}

func BenchmarkSnapshot100(b *testing.B) {
	h := NewHub()
	populateHub(h, 100)
	b.ResetTimer()
	for b.Loop() {
		buf := h.snapshot("proj", "agent")
		h.putSnapshot(buf)
	}
}

func BenchmarkSnapshot1000(b *testing.B) {
	h := NewHub()
	populateHub(h, 1000)
	b.ResetTimer()
	for b.Loop() {
		buf := h.snapshot("proj", "agent")
		h.putSnapshot(buf)
	}
}

func BenchmarkSnapshotWildcard100(b *testing.B) {
	h := NewHub()
	populateHubSpread(h, 100)
	b.ResetTimer()
	for b.Loop() {
		buf := h.snapshot("", "")
		h.putSnapshot(buf)
	}
}

func BenchmarkSnapshotWildcard1000(b *testing.B) {
	h := NewHub()
	populateHubSpread(h, 1000)
	b.ResetTimer()
	for b.Loop() {
		buf := h.snapshot("", "")
		h.putSnapshot(buf)
	}
}
