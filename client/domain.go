// Package client provides a Go client for the Intermute coordination server.
// This file contains domain entity types and CRUD methods for specs, epics,
// stories, tasks, insights, and sessions.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SpecStatus represents the status of a specification
type SpecStatus string

const (
	SpecStatusDraft     SpecStatus = "draft"
	SpecStatusResearch  SpecStatus = "research"
	SpecStatusValidated SpecStatus = "validated"
	SpecStatusArchived  SpecStatus = "archived"
)

// EpicStatus represents the status of an epic
type EpicStatus string

const (
	EpicStatusOpen       EpicStatus = "open"
	EpicStatusInProgress EpicStatus = "in_progress"
	EpicStatusDone       EpicStatus = "done"
)

// StoryStatus represents the status of a story
type StoryStatus string

const (
	StoryStatusTodo       StoryStatus = "todo"
	StoryStatusInProgress StoryStatus = "in_progress"
	StoryStatusReview     StoryStatus = "review"
	StoryStatusDone       StoryStatus = "done"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending TaskStatus = "pending"
	TaskStatusRunning TaskStatus = "running"
	TaskStatusBlocked TaskStatus = "blocked"
	TaskStatusDone    TaskStatus = "done"
)

// SessionStatus represents the status of an agent session
type SessionStatus string

const (
	SessionStatusRunning SessionStatus = "running"
	SessionStatusIdle    SessionStatus = "idle"
	SessionStatusError   SessionStatus = "error"
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
	Version   int64      `json:"version,omitempty"` // For optimistic locking
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

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
	Type      string    `json:"type"`
	Project   string    `json:"project"`
	EntityID  string    `json:"entity_id"`
	Data      any       `json:"data,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

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

// CUJFeatureLink represents a link between a CUJ and a feature
type CUJFeatureLink struct {
	CUJID     string    `json:"cuj_id"`
	FeatureID string    `json:"feature_id"`
	Project   string    `json:"project"`
	LinkedAt  time.Time `json:"linked_at"`
}

// ErrConflict is returned when optimistic locking fails
var ErrConflict = fmt.Errorf("concurrent modification conflict")

// --- Spec Operations ---

// CreateSpec creates a new specification
func (c *Client) CreateSpec(ctx context.Context, spec Spec) (Spec, error) {
	if spec.Project == "" {
		spec.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/specs", spec)
	if err != nil {
		return Spec{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Spec{}, ErrConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return Spec{}, fmt.Errorf("create spec failed: %d", resp.StatusCode)
	}
	var out Spec
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Spec{}, err
	}
	return out, nil
}

// GetSpec retrieves a specification by ID
func (c *Client) GetSpec(ctx context.Context, id string) (Spec, error) {
	endpoint := "/api/specs/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Spec{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Spec{}, fmt.Errorf("spec not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Spec{}, fmt.Errorf("get spec failed: %d", resp.StatusCode)
	}
	var out Spec
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Spec{}, err
	}
	return out, nil
}

// ListSpecs lists specifications with optional filters
func (c *Client) ListSpecs(ctx context.Context, status string) ([]Spec, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if status != "" {
		values.Set("status", status)
	}
	endpoint := "/api/specs"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list specs failed: %d", resp.StatusCode)
	}
	var out []Spec
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateSpec updates a specification
func (c *Client) UpdateSpec(ctx context.Context, spec Spec) (Spec, error) {
	if spec.Project == "" {
		spec.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/specs/"+url.PathEscape(spec.ID), spec)
	if err != nil {
		return Spec{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Spec{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return Spec{}, fmt.Errorf("update spec failed: %d", resp.StatusCode)
	}
	var out Spec
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Spec{}, err
	}
	return out, nil
}

// DeleteSpec deletes a specification
func (c *Client) DeleteSpec(ctx context.Context, id string) error {
	endpoint := "/api/specs/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete spec failed: %d", resp.StatusCode)
	}
	return nil
}

// --- Epic Operations ---

// CreateEpic creates a new epic
func (c *Client) CreateEpic(ctx context.Context, epic Epic) (Epic, error) {
	if epic.Project == "" {
		epic.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/epics", epic)
	if err != nil {
		return Epic{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Epic{}, ErrConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return Epic{}, fmt.Errorf("create epic failed: %d", resp.StatusCode)
	}
	var out Epic
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Epic{}, err
	}
	return out, nil
}

// GetEpic retrieves an epic by ID
func (c *Client) GetEpic(ctx context.Context, id string) (Epic, error) {
	endpoint := "/api/epics/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Epic{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Epic{}, fmt.Errorf("epic not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Epic{}, fmt.Errorf("get epic failed: %d", resp.StatusCode)
	}
	var out Epic
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Epic{}, err
	}
	return out, nil
}

// ListEpics lists epics with optional spec filter
func (c *Client) ListEpics(ctx context.Context, specID string) ([]Epic, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if specID != "" {
		values.Set("spec", specID)
	}
	endpoint := "/api/epics"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list epics failed: %d", resp.StatusCode)
	}
	var out []Epic
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateEpic updates an epic
func (c *Client) UpdateEpic(ctx context.Context, epic Epic) (Epic, error) {
	if epic.Project == "" {
		epic.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/epics/"+url.PathEscape(epic.ID), epic)
	if err != nil {
		return Epic{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Epic{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return Epic{}, fmt.Errorf("update epic failed: %d", resp.StatusCode)
	}
	var out Epic
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Epic{}, err
	}
	return out, nil
}

// DeleteEpic deletes an epic
func (c *Client) DeleteEpic(ctx context.Context, id string) error {
	endpoint := "/api/epics/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete epic failed: %d", resp.StatusCode)
	}
	return nil
}

// --- Story Operations ---

// CreateStory creates a new story
func (c *Client) CreateStory(ctx context.Context, story Story) (Story, error) {
	if story.Project == "" {
		story.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/stories", story)
	if err != nil {
		return Story{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Story{}, ErrConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return Story{}, fmt.Errorf("create story failed: %d", resp.StatusCode)
	}
	var out Story
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Story{}, err
	}
	return out, nil
}

// GetStory retrieves a story by ID
func (c *Client) GetStory(ctx context.Context, id string) (Story, error) {
	endpoint := "/api/stories/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Story{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Story{}, fmt.Errorf("story not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Story{}, fmt.Errorf("get story failed: %d", resp.StatusCode)
	}
	var out Story
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Story{}, err
	}
	return out, nil
}

// ListStories lists stories with optional epic filter
func (c *Client) ListStories(ctx context.Context, epicID string) ([]Story, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if epicID != "" {
		values.Set("epic", epicID)
	}
	endpoint := "/api/stories"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list stories failed: %d", resp.StatusCode)
	}
	var out []Story
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateStory updates a story
func (c *Client) UpdateStory(ctx context.Context, story Story) (Story, error) {
	if story.Project == "" {
		story.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/stories/"+url.PathEscape(story.ID), story)
	if err != nil {
		return Story{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Story{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return Story{}, fmt.Errorf("update story failed: %d", resp.StatusCode)
	}
	var out Story
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Story{}, err
	}
	return out, nil
}

// DeleteStory deletes a story
func (c *Client) DeleteStory(ctx context.Context, id string) error {
	endpoint := "/api/stories/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete story failed: %d", resp.StatusCode)
	}
	return nil
}

// --- Task Operations ---

// CreateTask creates a new task
func (c *Client) CreateTask(ctx context.Context, task Task) (Task, error) {
	if task.Project == "" {
		task.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/tasks", task)
	if err != nil {
		return Task{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Task{}, ErrConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return Task{}, fmt.Errorf("create task failed: %d", resp.StatusCode)
	}
	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Task{}, err
	}
	return out, nil
}

// GetTask retrieves a task by ID
func (c *Client) GetTask(ctx context.Context, id string) (Task, error) {
	endpoint := "/api/tasks/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Task{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Task{}, fmt.Errorf("task not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Task{}, fmt.Errorf("get task failed: %d", resp.StatusCode)
	}
	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Task{}, err
	}
	return out, nil
}

// ListTasks lists tasks with optional filters
func (c *Client) ListTasks(ctx context.Context, status, agent string) ([]Task, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if status != "" {
		values.Set("status", status)
	}
	if agent != "" {
		values.Set("agent", agent)
	}
	endpoint := "/api/tasks"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list tasks failed: %d", resp.StatusCode)
	}
	var out []Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateTask updates a task
func (c *Client) UpdateTask(ctx context.Context, task Task) (Task, error) {
	if task.Project == "" {
		task.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/tasks/"+url.PathEscape(task.ID), task)
	if err != nil {
		return Task{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return Task{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return Task{}, fmt.Errorf("update task failed: %d", resp.StatusCode)
	}
	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Task{}, err
	}
	return out, nil
}

// AssignTask assigns a task to an agent
func (c *Client) AssignTask(ctx context.Context, taskID, agent string) (Task, error) {
	endpoint := "/api/tasks/" + url.PathEscape(taskID) + "/assign"
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.postJSON(ctx, endpoint, map[string]string{"agent": agent})
	if err != nil {
		return Task{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Task{}, fmt.Errorf("assign task failed: %d", resp.StatusCode)
	}
	var out Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Task{}, err
	}
	return out, nil
}

// DeleteTask deletes a task
func (c *Client) DeleteTask(ctx context.Context, id string) error {
	endpoint := "/api/tasks/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete task failed: %d", resp.StatusCode)
	}
	return nil
}

// --- Insight Operations ---

// CreateInsight creates a new insight
func (c *Client) CreateInsight(ctx context.Context, insight Insight) (Insight, error) {
	if insight.Project == "" {
		insight.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/insights", insight)
	if err != nil {
		return Insight{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return Insight{}, fmt.Errorf("create insight failed: %d", resp.StatusCode)
	}
	var out Insight
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Insight{}, err
	}
	return out, nil
}

// GetInsight retrieves an insight by ID
func (c *Client) GetInsight(ctx context.Context, id string) (Insight, error) {
	endpoint := "/api/insights/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Insight{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Insight{}, fmt.Errorf("insight not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Insight{}, fmt.Errorf("get insight failed: %d", resp.StatusCode)
	}
	var out Insight
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Insight{}, err
	}
	return out, nil
}

// ListInsights lists insights with optional filters
func (c *Client) ListInsights(ctx context.Context, specID, category string) ([]Insight, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if specID != "" {
		values.Set("spec", specID)
	}
	if category != "" {
		values.Set("category", category)
	}
	endpoint := "/api/insights"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list insights failed: %d", resp.StatusCode)
	}
	var out []Insight
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// LinkInsightToSpec links an insight to a specification
func (c *Client) LinkInsightToSpec(ctx context.Context, insightID, specID string) error {
	endpoint := "/api/insights/" + url.PathEscape(insightID) + "/link"
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.postJSON(ctx, endpoint, map[string]string{"spec_id": specID})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("link insight failed: %d", resp.StatusCode)
	}
	return nil
}

// DeleteInsight deletes an insight
func (c *Client) DeleteInsight(ctx context.Context, id string) error {
	endpoint := "/api/insights/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete insight failed: %d", resp.StatusCode)
	}
	return nil
}

// --- Session Operations ---

// CreateSession creates a new agent session
func (c *Client) CreateSession(ctx context.Context, session Session) (Session, error) {
	if session.Project == "" {
		session.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/sessions", session)
	if err != nil {
		return Session{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return Session{}, fmt.Errorf("create session failed: %d", resp.StatusCode)
	}
	var out Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Session{}, err
	}
	return out, nil
}

// GetSession retrieves a session by ID
func (c *Client) GetSession(ctx context.Context, id string) (Session, error) {
	endpoint := "/api/sessions/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return Session{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Session{}, fmt.Errorf("session not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return Session{}, fmt.Errorf("get session failed: %d", resp.StatusCode)
	}
	var out Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Session{}, err
	}
	return out, nil
}

// ListSessions lists sessions with optional status filter
func (c *Client) ListSessions(ctx context.Context, status string) ([]Session, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if status != "" {
		values.Set("status", status)
	}
	endpoint := "/api/sessions"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list sessions failed: %d", resp.StatusCode)
	}
	var out []Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateSession updates a session
func (c *Client) UpdateSession(ctx context.Context, session Session) (Session, error) {
	if session.Project == "" {
		session.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/sessions/"+url.PathEscape(session.ID), session)
	if err != nil {
		return Session{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Session{}, fmt.Errorf("update session failed: %d", resp.StatusCode)
	}
	var out Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Session{}, err
	}
	return out, nil
}

// DeleteSession deletes a session
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	endpoint := "/api/sessions/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete session failed: %d", resp.StatusCode)
	}
	return nil
}

// --- CUJ Operations ---

// CreateCUJ creates a new Critical User Journey
func (c *Client) CreateCUJ(ctx context.Context, cuj CriticalUserJourney) (CriticalUserJourney, error) {
	if cuj.Project == "" {
		cuj.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/cujs", cuj)
	if err != nil {
		return CriticalUserJourney{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return CriticalUserJourney{}, ErrConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return CriticalUserJourney{}, fmt.Errorf("create cuj failed: %d", resp.StatusCode)
	}
	var out CriticalUserJourney
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CriticalUserJourney{}, err
	}
	return out, nil
}

// GetCUJ retrieves a CUJ by ID
func (c *Client) GetCUJ(ctx context.Context, id string) (CriticalUserJourney, error) {
	endpoint := "/api/cujs/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return CriticalUserJourney{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return CriticalUserJourney{}, fmt.Errorf("cuj not found: %s", id)
	}
	if resp.StatusCode != http.StatusOK {
		return CriticalUserJourney{}, fmt.Errorf("get cuj failed: %d", resp.StatusCode)
	}
	var out CriticalUserJourney
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CriticalUserJourney{}, err
	}
	return out, nil
}

// ListCUJs lists CUJs with optional spec filter
func (c *Client) ListCUJs(ctx context.Context, specID string) ([]CriticalUserJourney, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if specID != "" {
		values.Set("spec", specID)
	}
	endpoint := "/api/cujs"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list cujs failed: %d", resp.StatusCode)
	}
	var out []CriticalUserJourney
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateCUJ updates a CUJ
func (c *Client) UpdateCUJ(ctx context.Context, cuj CriticalUserJourney) (CriticalUserJourney, error) {
	if cuj.Project == "" {
		cuj.Project = c.Project
	}
	resp, err := c.putJSON(ctx, "/api/cujs/"+url.PathEscape(cuj.ID), cuj)
	if err != nil {
		return CriticalUserJourney{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return CriticalUserJourney{}, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return CriticalUserJourney{}, fmt.Errorf("update cuj failed: %d", resp.StatusCode)
	}
	var out CriticalUserJourney
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CriticalUserJourney{}, err
	}
	return out, nil
}

// DeleteCUJ deletes a CUJ
func (c *Client) DeleteCUJ(ctx context.Context, id string) error {
	endpoint := "/api/cujs/" + url.PathEscape(id)
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.delete(ctx, endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete cuj failed: %d", resp.StatusCode)
	}
	return nil
}

// LinkCUJToFeature links a CUJ to a feature
func (c *Client) LinkCUJToFeature(ctx context.Context, cujID, featureID string) error {
	endpoint := "/api/cujs/" + url.PathEscape(cujID) + "/link"
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.postJSON(ctx, endpoint, map[string]string{"feature_id": featureID})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("link cuj failed: %d", resp.StatusCode)
	}
	return nil
}

// UnlinkCUJFromFeature removes a link between a CUJ and a feature
func (c *Client) UnlinkCUJFromFeature(ctx context.Context, cujID, featureID string) error {
	endpoint := "/api/cujs/" + url.PathEscape(cujID) + "/unlink"
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.postJSON(ctx, endpoint, map[string]string{"feature_id": featureID})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unlink cuj failed: %d", resp.StatusCode)
	}
	return nil
}

// GetCUJFeatureLinks gets all feature links for a CUJ
func (c *Client) GetCUJFeatureLinks(ctx context.Context, cujID string) ([]CUJFeatureLink, error) {
	endpoint := "/api/cujs/" + url.PathEscape(cujID) + "/links"
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get cuj links failed: %d", resp.StatusCode)
	}
	var out []CUJFeatureLink
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- HTTP helpers ---

func (c *Client) putJSON(ctx context.Context, path string, payload any) (*http.Response, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	return c.HTTP.Do(req)
}
