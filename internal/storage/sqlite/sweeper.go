package sqlite

import (
	"context"
	"log"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

// Broadcaster is the interface for emitting events to WebSocket clients.
type Broadcaster interface {
	Broadcast(project, agent string, event any)
}

// Sweeper runs a background goroutine that periodically cleans up expired
// reservations held by inactive agents.
type Sweeper struct {
	store    *Store
	bus      Broadcaster
	interval time.Duration
	grace    time.Duration // heartbeat grace period
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewSweeper creates a new Sweeper. Call Start() to begin sweeping.
func NewSweeper(store *Store, bus Broadcaster, interval, grace time.Duration) *Sweeper {
	return &Sweeper{
		store:    store,
		bus:      bus,
		interval: interval,
		grace:    grace,
		done:     make(chan struct{}),
	}
}

// Start launches the background sweep goroutine.
func (sw *Sweeper) Start(ctx context.Context) {
	ctx, sw.cancel = context.WithCancel(ctx)

	go func() {
		defer close(sw.done)

		// Startup sweep: only clean reservations expired >5min ago
		sw.runSweep(ctx, time.Now().UTC().Add(-5*time.Minute))

		ticker := time.NewTicker(sw.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sw.runSweep(ctx, time.Now().UTC())
			}
		}
	}()
}

// Stop cancels the sweep goroutine and waits for it to finish.
func (sw *Sweeper) Stop() {
	if sw.cancel != nil {
		sw.cancel()
	}
	<-sw.done
}

func (sw *Sweeper) runSweep(ctx context.Context, expiredBefore time.Time) {
	heartbeatAfter := time.Now().UTC().Add(-sw.grace)

	deleted, err := sw.store.SweepExpired(ctx, expiredBefore, heartbeatAfter)
	if err != nil {
		log.Printf("sweeper: %v", err)
		return
	}

	if len(deleted) == 0 {
		return
	}

	log.Printf("sweeper: cleaned %d expired reservation(s)", len(deleted))

	if sw.bus != nil {
		for _, r := range deleted {
			sw.bus.Broadcast(r.Project, "", map[string]any{
				"type":           string(core.EventReservationExpired),
				"project":        r.Project,
				"reservation_id": r.ID,
				"agent_id":       r.AgentID,
				"path_pattern":   r.PathPattern,
			})
		}
	}
}
