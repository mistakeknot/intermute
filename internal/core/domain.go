package core

import (
	"errors"
	"time"
)

// ErrConcurrentModification is returned when an optimistic locking conflict occurs
var ErrConcurrentModification = errors.New("concurrent modification: entity was updated by another process")

// ErrNotFound is returned when a requested entity does not exist
var ErrNotFound = errors.New("not found")

// ErrActiveSessionConflict is returned when a session_id is already in use by an active agent.
var ErrActiveSessionConflict = errors.New("active session conflict: session_id is in use by an agent with a recent heartbeat")

// SessionStaleThreshold is the duration after which an agent's heartbeat is considered stale,
// allowing session_id reuse.
const SessionStaleThreshold = 5 * time.Minute

// Domain event types for Autarch domain entities
const (
	// Spec events
	EventSpecCreated  EventType = "spec.created"
	EventSpecUpdated  EventType = "spec.updated"
	EventSpecArchived EventType = "spec.archived"

	// Epic events
	EventEpicCreated EventType = "epic.created"
	EventEpicUpdated EventType = "epic.updated"

	// Story events
	EventStoryCreated EventType = "story.created"
	EventStoryUpdated EventType = "story.updated"

	// Task events
	EventTaskCreated   EventType = "task.created"
	EventTaskAssigned  EventType = "task.assigned"
	EventTaskCompleted EventType = "task.completed"

	// Insight events
	EventInsightCreated EventType = "insight.created"
	EventInsightLinked  EventType = "insight.linked"

	// Session events
	EventSessionStarted EventType = "session.started"
	EventSessionStopped EventType = "session.stopped"

	// Reservation events
	EventReservationExpired EventType = "reservation.expired"
)

// SpecStatus represents the status of a specification
type SpecStatus string

const (
	SpecStatusDraft     SpecStatus = "draft"
	SpecStatusResearch  SpecStatus = "research"
	SpecStatusValidated SpecStatus = "validated"
	SpecStatusArchived  SpecStatus = "archived"
)

// Spec represents a product specification (PRD)
type Spec struct {
	ID        string     `json:"id"`
	Project   string     `json:"project"`
	Title     string     `json:"title"`
	Vision    string     `json:"vision,omitempty"`
	Users     string     `json:"users,omitempty"`
	Problem   string     `json:"problem,omitempty"`
	Status    SpecStatus `json:"status"`
	Version   int64      `json:"version,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// EpicStatus represents the status of an epic
type EpicStatus string

const (
	EpicStatusOpen       EpicStatus = "open"
	EpicStatusInProgress EpicStatus = "in_progress"
	EpicStatusDone       EpicStatus = "done"
)

// Epic represents a large feature or initiative
type Epic struct {
	ID          string     `json:"id"`
	Project     string     `json:"project"`
	SpecID      string     `json:"spec_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      EpicStatus `json:"status"`
	Version     int64      `json:"version,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// StoryStatus represents the status of a story
type StoryStatus string

const (
	StoryStatusTodo       StoryStatus = "todo"
	StoryStatusInProgress StoryStatus = "in_progress"
	StoryStatusReview     StoryStatus = "review"
	StoryStatusDone       StoryStatus = "done"
)

// Story represents a user story within an epic
type Story struct {
	ID                 string      `json:"id"`
	Project            string      `json:"project"`
	EpicID             string      `json:"epic_id"`
	Title              string      `json:"title"`
	AcceptanceCriteria []string    `json:"acceptance_criteria,omitempty"`
	Status             StoryStatus `json:"status"`
	Version            int64       `json:"version,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending TaskStatus = "pending"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusBlocked TaskStatus = "blocked"
	TaskStatusDone    TaskStatus = "done"
)

// Task represents an execution unit assigned to an agent
type Task struct {
	ID        string     `json:"id"`
	Project   string     `json:"project"`
	StoryID   string     `json:"story_id,omitempty"`
	Title     string     `json:"title"`
	Agent     string     `json:"agent,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	Status    TaskStatus `json:"status"`
	Version   int64      `json:"version,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Insight represents a research insight from Pollard
type Insight struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	SpecID    string    `json:"spec_id,omitempty"`
	Source    string    `json:"source"`
	Category  string    `json:"category"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	URL       string    `json:"url,omitempty"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionStatus represents the status of an agent session
type SessionStatus string

const (
	SessionStatusRunning SessionStatus = "running"
	SessionStatusIdle    SessionStatus = "idle"
	SessionStatusError   SessionStatus = "error"
)

// Session represents an agent session (tmux session)
type Session struct {
	ID        string        `json:"id"`
	Project   string        `json:"project"`
	Name      string        `json:"name"`
	Agent     string        `json:"agent"`
	TaskID    string        `json:"task_id,omitempty"`
	Status    SessionStatus `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// DomainEvent wraps a domain entity change for event sourcing
type DomainEvent struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Project   string    `json:"project"`
	EntityID  string    `json:"entity_id"`
	Data      any       `json:"data,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Cursor    uint64    `json:"cursor"`
}

// CUJ (Critical User Journey) events
const (
	EventCUJCreated   EventType = "cuj.created"
	EventCUJValidated EventType = "cuj.validated"
	EventCUJUpdated   EventType = "cuj.updated"
	EventCUJArchived  EventType = "cuj.archived"
)

// CUJStatus represents the status of a Critical User Journey
type CUJStatus string

const (
	CUJStatusDraft     CUJStatus = "draft"
	CUJStatusValidated CUJStatus = "validated"
	CUJStatusArchived  CUJStatus = "archived"
)

// CUJPriority represents the priority level of a CUJ
type CUJPriority string

const (
	CUJPriorityHigh   CUJPriority = "high"
	CUJPriorityMedium CUJPriority = "medium"
	CUJPriorityLow    CUJPriority = "low"
)

// CriticalUserJourney represents a first-class CUJ entity
type CriticalUserJourney struct {
	ID              string      `json:"id"`
	SpecID          string      `json:"spec_id"`
	Project         string      `json:"project"`
	Title           string      `json:"title"`
	Persona         string      `json:"persona,omitempty"`
	Priority        CUJPriority `json:"priority"`
	EntryPoint      string      `json:"entry_point,omitempty"`
	ExitPoint       string      `json:"exit_point,omitempty"`
	Steps           []CUJStep   `json:"steps,omitempty"`
	SuccessCriteria []string    `json:"success_criteria,omitempty"`
	ErrorRecovery   []string    `json:"error_recovery,omitempty"`
	Status          CUJStatus   `json:"status"`
	Version         int64       `json:"version,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// CUJStep represents a single step in a Critical User Journey
type CUJStep struct {
	Order        int      `json:"order"`
	Action       string   `json:"action"`
	Expected     string   `json:"expected"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// CUJFeatureLink represents a many-to-many link between CUJs and features
type CUJFeatureLink struct {
	CUJID     string    `json:"cuj_id"`
	FeatureID string    `json:"feature_id"`
	Project   string    `json:"project"`
	LinkedAt  time.Time `json:"linked_at"`
}
