package client

import (
	"context"
	"testing"
	"time"
)

func TestClientSendAndInbox(t *testing.T) {
	c := New("http://localhost:7338")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.SendMessage(ctx, Message{From: "a", To: []string{"b"}, Body: "hi"})
	if err == nil {
		t.Fatalf("expected failure without server")
	}
}
