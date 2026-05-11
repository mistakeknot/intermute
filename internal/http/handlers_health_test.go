package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(_ context.Context) error { return s.err }

func TestHealthHandlerReturnsOKWhenDBHealthy(t *testing.T) {
	h := newHealthHandler(stubPinger{err: nil})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestHealthHandlerReturns503WhenDBFails(t *testing.T) {
	h := newHealthHandler(stubPinger{err: errors.New("database is locked")})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if body["status"] != "degraded" {
		t.Errorf("status = %q, want %q", body["status"], "degraded")
	}
	if body["error"] != "database is locked" {
		t.Errorf("error = %q, want %q", body["error"], "database is locked")
	}
}

func TestHealthHandlerDegradesGracefullyWithoutPinger(t *testing.T) {
	// Pinger=nil is the back-compat path for tests/embedded setups that
	// don't bother wiring a real store. Should keep returning legacy ok.
	h := newHealthHandler(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (nil pinger should degrade to legacy ok)", w.Code)
	}
}

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	h := newHealthHandler(stubPinger{err: nil})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/health", nil)
	h(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
