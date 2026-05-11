package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger is anything that can verify its own backing store is responsive.
// Implemented by *sqlite.Store; kept narrow so test fakes are easy.
type Pinger interface {
	Ping(ctx context.Context) error
}

// healthPingTimeout caps how long /health waits for the DB to respond.
// Short enough that monitors get a fast NACK on real wedges; long enough
// to tolerate a slow query under brief contention.
const healthPingTimeout = 500 * time.Millisecond

// newHealthHandler returns a handler that reports DB liveness, not just
// process liveness. Returns 200 with {"status":"ok"} on success; 503 with
// an error message on Ping failure or timeout. Pinger may be nil, in
// which case the handler degrades to the legacy hardcoded-ok behavior —
// useful for tests that don't need a real DB.
func newHealthHandler(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if p == nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()
		if err := p.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "degraded",
				"error":  err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
