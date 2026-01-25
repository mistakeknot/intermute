package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type Mode string

const (
	ModeLocalhost Mode = "localhost"
	ModeAPIKey    Mode = "api_key"
)

type Info struct {
	Mode      Mode
	Project   string
	Localhost bool
}

type contextKey struct{}

func FromContext(ctx context.Context) (Info, bool) {
	v, ok := ctx.Value(contextKey{}).(Info)
	return v, ok
}

func Middleware(ring *Keyring) func(http.Handler) http.Handler {
	if ring == nil {
		ring = defaultKeyring()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ring.AllowLocalhostWithoutAuth && isLocalRequest(r) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, Info{Mode: ModeLocalhost, Localhost: true})))
				return
			}
			project, ok := authorize(r, ring)
			if !ok {
				writeUnauthorized(w)
				return
			}
			info := Info{Mode: ModeAPIKey, Project: project, Localhost: false}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, info)))
		})
	}
}

func authorize(r *http.Request, ring *Keyring) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	key := strings.TrimSpace(parts[1])
	if key == "" {
		return "", false
	}
	return ring.ProjectForKey(key)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

func isLocalRequest(r *http.Request) bool {
	if ip := forwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.IsLoopback()
		}
		if strings.EqualFold(ip, "localhost") {
			return true
		}
	}
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

func forwardedFor(v string) string {
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ",")
	return strings.TrimSpace(parts[0])
}
