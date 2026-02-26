package httpapi

import (
	"sync"
	"time"

	"github.com/mistakeknot/intermute/internal/storage"
)

type Service struct {
	store   storage.Store
	bus     Broadcaster
	bcastRL broadcastLimiter
}

type Broadcaster interface {
	Broadcast(project, agent string, event any)
}

func NewService(store storage.Store) *Service {
	return &Service{
		store:   store,
		bcastRL: broadcastLimiter{buckets: make(map[string]*rlBucket)},
	}
}

func (s *Service) WithBroadcaster(b Broadcaster) *Service {
	s.bus = b
	return s
}

// broadcastLimiter is a simple per-sender rate limiter for broadcast messages.
// 10 broadcasts per minute per (project, sender) pair.
type broadcastLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rlBucket
}

type rlBucket struct {
	count   int
	resetAt time.Time
}

const broadcastRateLimit = 10
const broadcastRateWindow = time.Minute

// exceeded returns true if the sender has exceeded the broadcast rate limit.
func (l *broadcastLimiter) exceeded(project, sender string) bool {
	key := project + ":" + sender
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		l.buckets[key] = &rlBucket{count: 1, resetAt: now.Add(broadcastRateWindow)}
		return false
	}
	b.count++
	return b.count > broadcastRateLimit
}
