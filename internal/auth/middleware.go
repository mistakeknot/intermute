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
	AgentID   string
	Localhost bool
}

type contextKey struct{}

func FromContext(ctx context.Context) (Info, bool) {
	v, ok := ctx.Value(contextKey{}).(Info)
	return v, ok
}

// TokenLookup resolves a registration token to its bound agent ID.
// Returns ("", error) if the token is not found.
type TokenLookup func(ctx context.Context, token string) (agentID string, err error)

func Middleware(ring *Keyring, lookupToken ...TokenLookup) func(http.Handler) http.Handler {
	if ring == nil {
		ring = defaultKeyring()
	}
	var lookup TokenLookup
	if len(lookupToken) > 0 {
		lookup = lookupToken[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentID := strings.TrimSpace(r.Header.Get("X-Agent-ID"))
			agentToken := strings.TrimSpace(r.Header.Get("X-Agent-Token"))

			// Token-bound identity verification (applies to both code paths)
			if lookup != nil && agentToken != "" {
				if err := verifyAgentToken(r.Context(), lookup, agentID, agentToken); err != nil {
					writeForbidden(w, err.Error())
					return
				}
			}

			if ring.AllowLocalhostWithoutAuth && isLocalRequest(r) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, Info{Mode: ModeLocalhost, AgentID: agentID, Localhost: true})))
				return
			}
			project, ok := authorize(r, ring)
			if !ok {
				writeUnauthorized(w)
				return
			}
			info := Info{Mode: ModeAPIKey, Project: project, AgentID: agentID, Localhost: false}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, info)))
		})
	}
}

// verifyAgentToken checks that X-Agent-Token is bound to X-Agent-ID.
func verifyAgentToken(ctx context.Context, lookup TokenLookup, agentID, token string) error {
	registered, err := lookup(ctx, token)
	if err != nil {
		// Token not found — let it pass (could be pre-upgrade agent)
		return nil
	}
	if agentID == "" {
		// Token is bound but X-Agent-ID omitted — reject (header-omission bypass)
		return &identityError{msg: "agent token is bound but X-Agent-ID header is missing"}
	}
	if registered != agentID {
		return &identityError{msg: "agent identity mismatch"}
	}
	return nil
}

type identityError struct{ msg string }

func (e *identityError) Error() string { return e.msg }

func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	host = strings.TrimSpace(host)
	remoteIsLoopback := strings.EqualFold(host, "localhost")
	if !remoteIsLoopback {
		if parsed := net.ParseIP(host); parsed != nil {
			remoteIsLoopback = parsed.IsLoopback()
		}
	}
	if !remoteIsLoopback {
		return false
	}
	if ip := forwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.IsLoopback()
		}
		return strings.EqualFold(ip, "localhost")
	}
	return true
}

func forwardedFor(v string) string {
	if v == "" {
		return ""
	}
	parts := strings.Split(v, ",")
	return strings.TrimSpace(parts[0])
}
