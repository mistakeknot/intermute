package sqlite

import (
	"context"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/storage"
)

// Compile-time interface check.
var _ storage.DomainStore = (*ResilientStore)(nil)

// ResilientStore wraps every method of *Store with CircuitBreaker + RetryOnDBLock
// to provide resilience against transient SQLite errors (database-is-locked,
// connection failures, etc.).
type ResilientStore struct {
	inner *Store
	cb    *CircuitBreaker
}

// NewResilient creates a ResilientStore with default circuit breaker settings
// (threshold=5, resetTimeout=30s).
func NewResilient(inner *Store) *ResilientStore {
	return &ResilientStore{inner: inner, cb: NewCircuitBreaker(5, 30*time.Second)}
}

// NewResilientWithBreaker creates a ResilientStore with a custom circuit breaker.
func NewResilientWithBreaker(inner *Store, cb *CircuitBreaker) *ResilientStore {
	return &ResilientStore{inner: inner, cb: cb}
}

// CircuitBreakerState returns the current state of the circuit breaker as a string.
func (r *ResilientStore) CircuitBreakerState() string {
	return r.cb.State().String()
}

// ---------------------------------------------------------------------------
// Store interface methods
// ---------------------------------------------------------------------------

func (r *ResilientStore) AppendEvent(ctx context.Context, ev storage.Event) (uint64, error) {
	var result uint64
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.AppendEvent(ctx, ev)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) InboxSince(ctx context.Context, project, agent string, cursor uint64, limit int) ([]core.Message, error) {
	var result []core.Message
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.InboxSince(ctx, project, agent, cursor, limit)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ThreadMessages(ctx context.Context, project, threadID string, cursor uint64) ([]core.Message, error) {
	var result []core.Message
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ThreadMessages(ctx, project, threadID, cursor)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListThreads(ctx context.Context, project, agent string, cursor uint64, limit int) ([]storage.ThreadSummary, error) {
	var result []storage.ThreadSummary
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListThreads(ctx, project, agent, cursor, limit)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) RegisterAgent(ctx context.Context, agent core.Agent) (core.Agent, error) {
	var result core.Agent
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.RegisterAgent(ctx, agent)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) Heartbeat(ctx context.Context, project, agentID string) (core.Agent, error) {
	var result core.Agent
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.Heartbeat(ctx, project, agentID)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateAgentMetadata(ctx context.Context, agentID string, meta map[string]string) (core.Agent, error) {
	var result core.Agent
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateAgentMetadata(ctx, agentID, meta)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListAgents(ctx context.Context, project string) ([]core.Agent, error) {
	var result []core.Agent
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListAgents(ctx, project)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) MarkRead(ctx context.Context, project, messageID, agentID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.MarkRead(ctx, project, messageID, agentID)
		})
	})
}

func (r *ResilientStore) MarkAck(ctx context.Context, project, messageID, agentID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.MarkAck(ctx, project, messageID, agentID)
		})
	})
}

func (r *ResilientStore) RecipientStatus(ctx context.Context, project, messageID string) (map[string]*core.RecipientStatus, error) {
	var result map[string]*core.RecipientStatus
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.RecipientStatus(ctx, project, messageID)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) InboxCounts(ctx context.Context, project, agentID string) (int, int, error) {
	var total, unread int
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			total, unread, innerErr = r.inner.InboxCounts(ctx, project, agentID)
			return innerErr
		})
	})
	return total, unread, err
}

func (r *ResilientStore) Reserve(ctx context.Context, res core.Reservation) (*core.Reservation, error) {
	var result *core.Reservation
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.Reserve(ctx, res)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetReservation(ctx context.Context, id string) (*core.Reservation, error) {
	var result *core.Reservation
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetReservation(ctx, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ReleaseReservation(ctx context.Context, id, agentID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.ReleaseReservation(ctx, id, agentID)
		})
	})
}

func (r *ResilientStore) ActiveReservations(ctx context.Context, project string) ([]core.Reservation, error) {
	var result []core.Reservation
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ActiveReservations(ctx, project)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) AgentReservations(ctx context.Context, agentID string) ([]core.Reservation, error) {
	var result []core.Reservation
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.AgentReservations(ctx, agentID)
			return innerErr
		})
	})
	return result, err
}

// ---------------------------------------------------------------------------
// DomainStore interface methods
// ---------------------------------------------------------------------------

// Spec operations

func (r *ResilientStore) CreateSpec(ctx context.Context, spec core.Spec) (core.Spec, error) {
	var result core.Spec
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateSpec(ctx, spec)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetSpec(ctx context.Context, project, id string) (core.Spec, error) {
	var result core.Spec
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetSpec(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListSpecs(ctx context.Context, project, status string) ([]core.Spec, error) {
	var result []core.Spec
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListSpecs(ctx, project, status)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateSpec(ctx context.Context, spec core.Spec) (core.Spec, error) {
	var result core.Spec
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateSpec(ctx, spec)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteSpec(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteSpec(ctx, project, id)
		})
	})
}

// Epic operations

func (r *ResilientStore) CreateEpic(ctx context.Context, epic core.Epic) (core.Epic, error) {
	var result core.Epic
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateEpic(ctx, epic)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetEpic(ctx context.Context, project, id string) (core.Epic, error) {
	var result core.Epic
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetEpic(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListEpics(ctx context.Context, project, specID string) ([]core.Epic, error) {
	var result []core.Epic
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListEpics(ctx, project, specID)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateEpic(ctx context.Context, epic core.Epic) (core.Epic, error) {
	var result core.Epic
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateEpic(ctx, epic)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteEpic(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteEpic(ctx, project, id)
		})
	})
}

// Story operations

func (r *ResilientStore) CreateStory(ctx context.Context, story core.Story) (core.Story, error) {
	var result core.Story
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateStory(ctx, story)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetStory(ctx context.Context, project, id string) (core.Story, error) {
	var result core.Story
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetStory(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListStories(ctx context.Context, project, epicID string) ([]core.Story, error) {
	var result []core.Story
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListStories(ctx, project, epicID)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateStory(ctx context.Context, story core.Story) (core.Story, error) {
	var result core.Story
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateStory(ctx, story)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteStory(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteStory(ctx, project, id)
		})
	})
}

// Task operations

func (r *ResilientStore) CreateTask(ctx context.Context, task core.Task) (core.Task, error) {
	var result core.Task
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateTask(ctx, task)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetTask(ctx context.Context, project, id string) (core.Task, error) {
	var result core.Task
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetTask(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListTasks(ctx context.Context, project, status, agent string) ([]core.Task, error) {
	var result []core.Task
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListTasks(ctx, project, status, agent)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateTask(ctx context.Context, task core.Task) (core.Task, error) {
	var result core.Task
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateTask(ctx, task)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteTask(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteTask(ctx, project, id)
		})
	})
}

// Insight operations

func (r *ResilientStore) CreateInsight(ctx context.Context, insight core.Insight) (core.Insight, error) {
	var result core.Insight
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateInsight(ctx, insight)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetInsight(ctx context.Context, project, id string) (core.Insight, error) {
	var result core.Insight
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetInsight(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListInsights(ctx context.Context, project, specID, category string) ([]core.Insight, error) {
	var result []core.Insight
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListInsights(ctx, project, specID, category)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) LinkInsightToSpec(ctx context.Context, project, insightID, specID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.LinkInsightToSpec(ctx, project, insightID, specID)
		})
	})
}

func (r *ResilientStore) DeleteInsight(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteInsight(ctx, project, id)
		})
	})
}

// Session operations

func (r *ResilientStore) CreateSession(ctx context.Context, session core.Session) (core.Session, error) {
	var result core.Session
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateSession(ctx, session)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetSession(ctx context.Context, project, id string) (core.Session, error) {
	var result core.Session
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetSession(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListSessions(ctx context.Context, project, status string) ([]core.Session, error) {
	var result []core.Session
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListSessions(ctx, project, status)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateSession(ctx context.Context, session core.Session) (core.Session, error) {
	var result core.Session
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateSession(ctx, session)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteSession(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteSession(ctx, project, id)
		})
	})
}

// CUJ (Critical User Journey) operations

func (r *ResilientStore) CreateCUJ(ctx context.Context, cuj core.CriticalUserJourney) (core.CriticalUserJourney, error) {
	var result core.CriticalUserJourney
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CreateCUJ(ctx, cuj)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) GetCUJ(ctx context.Context, project, id string) (core.CriticalUserJourney, error) {
	var result core.CriticalUserJourney
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetCUJ(ctx, project, id)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) ListCUJs(ctx context.Context, project, specID string) ([]core.CriticalUserJourney, error) {
	var result []core.CriticalUserJourney
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.ListCUJs(ctx, project, specID)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) UpdateCUJ(ctx context.Context, cuj core.CriticalUserJourney) (core.CriticalUserJourney, error) {
	var result core.CriticalUserJourney
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.UpdateCUJ(ctx, cuj)
			return innerErr
		})
	})
	return result, err
}

func (r *ResilientStore) DeleteCUJ(ctx context.Context, project, id string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.DeleteCUJ(ctx, project, id)
		})
	})
}

func (r *ResilientStore) LinkCUJToFeature(ctx context.Context, project, cujID, featureID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.LinkCUJToFeature(ctx, project, cujID, featureID)
		})
	})
}

func (r *ResilientStore) UnlinkCUJFromFeature(ctx context.Context, project, cujID, featureID string) error {
	return r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			return r.inner.UnlinkCUJFromFeature(ctx, project, cujID, featureID)
		})
	})
}

func (r *ResilientStore) GetCUJFeatureLinks(ctx context.Context, project, cujID string) ([]core.CUJFeatureLink, error) {
	var result []core.CUJFeatureLink
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.GetCUJFeatureLinks(ctx, project, cujID)
			return innerErr
		})
	})
	return result, err
}

// ---------------------------------------------------------------------------
// Concrete *Store methods (not part of interfaces)
// ---------------------------------------------------------------------------

// CheckConflicts wraps the Store's conflict check with CB+retry (F4 sprint).
func (r *ResilientStore) CheckConflicts(ctx context.Context, project, pathPattern string, exclusive bool) ([]core.ConflictDetail, error) {
	var result []core.ConflictDetail
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.CheckConflicts(ctx, project, pathPattern, exclusive)
			return innerErr
		})
	})
	return result, err
}

// SweepExpired wraps the Store's expiration sweep with CB+retry (F3 sprint).
func (r *ResilientStore) SweepExpired(ctx context.Context, expiredBefore time.Time, heartbeatAfter time.Time) ([]core.Reservation, error) {
	var result []core.Reservation
	err := r.cb.Execute(func() error {
		return RetryOnDBLock(func() error {
			var innerErr error
			result, innerErr = r.inner.SweepExpired(ctx, expiredBefore, heartbeatAfter)
			return innerErr
		})
	})
	return result, err
}

// Close delegates directly to the inner store without CB or retry.
func (r *ResilientStore) Close() error {
	return r.inner.Close()
}
