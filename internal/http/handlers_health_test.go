package httpapi

import (
	"net/http"
	"testing"
)

func TestHealthChecksStorageLiveness(t *testing.T) {
	env := newTestEnv(t)

	resp := env.get(t, "/health")
	requireStatus(t, resp, http.StatusOK)
	okBody := decodeJSON[map[string]string](t, resp)
	if okBody["status"] != "ok" {
		t.Fatalf("expected healthy status ok, got %q", okBody["status"])
	}

	if err := env.store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	resp = env.get(t, "/health")
	requireStatus(t, resp, http.StatusServiceUnavailable)
	unhealthyBody := decodeJSON[map[string]string](t, resp)
	if unhealthyBody["status"] != "unhealthy" {
		t.Fatalf("expected unhealthy status, got %q", unhealthyBody["status"])
	}
	if unhealthyBody["error"] == "" {
		t.Fatalf("expected health failure to include error")
	}
}
