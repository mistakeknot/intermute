package httpapi

import (
	"time"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/livetransport"
	"github.com/mistakeknot/intermute/internal/storage"
)

type Service struct {
	store        storage.Store
	bus          Broadcaster
	bcastRL      *rateLimiter
	liveDelivery livetransport.LiveDelivery
	liveLimiter  *rateLimiter
}

type Broadcaster interface {
	Broadcast(project, agent string, event any)
}

const (
	broadcastRateLimit  = 10
	broadcastRateWindow = time.Minute
	liveRateLimit       = 10
	liveRateWindow      = time.Minute
)

func NewService(store storage.Store) *Service {
	return &Service{
		store:        store,
		bcastRL:      newRateLimiter(broadcastRateLimit, broadcastRateWindow),
		liveDelivery: noopLiveDelivery{},
		liveLimiter:  newRateLimiter(liveRateLimit, liveRateWindow),
	}
}

func (s *Service) WithBroadcaster(b Broadcaster) *Service {
	s.bus = b
	return s
}

func (s *Service) WithLiveDelivery(d livetransport.LiveDelivery) *Service {
	if d == nil {
		s.liveDelivery = noopLiveDelivery{}
		return s
	}
	s.liveDelivery = d
	return s
}

// noopLiveDelivery is a no-op implementation used when the service is
// constructed without a real Injector (tests, async-only deployments).
// Deliver returns nil — a silent success — so that test harnesses exercising
// the transport=both path against an at-prompt recipient do not spuriously
// degrade to deferred delivery.  Main wiring always installs a real
// livetransport.Injector via WithLiveDelivery.
type noopLiveDelivery struct{}

func (noopLiveDelivery) Deliver(_ *core.WindowTarget, _ string) error { return nil }

func (noopLiveDelivery) ValidateTarget(_ *core.WindowTarget) error { return nil }

// broadcastExceeded is a convenience wrapper over bcastRL that preserves
// the existing broadcast-handler signature (sender exceeded limit?).
func (s *Service) broadcastExceeded(project, sender string) bool {
	return !s.bcastRL.allow(project + ":" + sender)
}

// liveAllow wraps liveLimiter with the (sender, recipient) key composition.
func (s *Service) liveAllow(sender, recipient string) bool {
	return s.liveLimiter.allow(sender + ":" + recipient)
}
