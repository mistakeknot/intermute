package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalhostBypass(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	mw := Middleware(ring)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := FromContext(r.Context())
		if !ok || info.Mode != ModeLocalhost {
			t.Fatalf("expected localhost auth mode")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNonLocalhostRequiresBearer(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{"secret": "proj-a"}}
	mw := Middleware(ring)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := FromContext(r.Context())
		if !ok || info.Project != "proj-a" || info.Mode != ModeAPIKey {
			t.Fatalf("expected apikey auth info")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "203.0.113.10:9999"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "203.0.113.10:9999"
	req.Header.Set("Authorization", "Bearer wrong")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong bearer, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "203.0.113.10:9999"
	req.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer, got %d", rr.Code)
	}
}

func TestXForwardedForSpoofing(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	mw := Middleware(ring)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected request to be rejected before handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "203.0.113.10:9999"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for spoofed x-forwarded-for, got %d", rr.Code)
	}
}

func TestAgentTokenMismatchBlocked(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	// Token "tok-A" is bound to agent "agent-A"
	lookup := func(_ context.Context, token string) (string, error) {
		if token == "tok-A" {
			return "agent-A", nil
		}
		return "", fmt.Errorf("not found")
	}
	mw := Middleware(ring, lookup)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be reached for mismatched token")
	}))

	// Agent B sends token belonging to Agent A → 403
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Agent-ID", "agent-B")
	req.Header.Set("X-Agent-Token", "tok-A")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAgentTokenMatchPasses(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	lookup := func(_ context.Context, token string) (string, error) {
		if token == "tok-A" {
			return "agent-A", nil
		}
		return "", fmt.Errorf("not found")
	}
	mw := Middleware(ring, lookup)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := FromContext(r.Context())
		if !ok || info.AgentID != "agent-A" {
			t.Fatalf("expected agent-A, got %s", info.AgentID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Agent-ID", "agent-A")
	req.Header.Set("X-Agent-Token", "tok-A")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAgentTokenWithoutIDBlocked(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	lookup := func(_ context.Context, token string) (string, error) {
		if token == "tok-A" {
			return "agent-A", nil
		}
		return "", fmt.Errorf("not found")
	}
	mw := Middleware(ring, lookup)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be reached for bound token without X-Agent-ID")
	}))

	// Bound token sent without X-Agent-ID → 403 (header-omission bypass prevention)
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Agent-Token", "tok-A")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestNoTokenHeaderPasses(t *testing.T) {
	ring := &Keyring{AllowLocalhostWithoutAuth: true, keyToProject: map[string]string{}}
	lookup := func(_ context.Context, token string) (string, error) {
		return "agent-A", nil
	}
	mw := Middleware(ring, lookup)

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No X-Agent-Token header → passes (backward compat)
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestLoadKeyringCreatesDevKeyWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	keysPath := filepath.Join(tmp, "intermute.keys.yaml")

	ring, err := LoadKeyring(keysPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	if _, err := os.Stat(keysPath); err != nil {
		t.Fatalf("expected keys file to be created: %v", err)
	}
	if len(ring.keyToProject) != 1 {
		t.Fatalf("expected 1 dev key, got %d", len(ring.keyToProject))
	}
}
