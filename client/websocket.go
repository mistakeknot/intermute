// Package client provides a Go client for the Intermute coordination server.
// This file contains WebSocket support for real-time event subscriptions.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// EventHandler is called for each event received via WebSocket
type EventHandler func(event DomainEvent)

// WSClient manages a WebSocket connection for real-time events
type WSClient struct {
	baseURL   string
	apiKey    string
	project   string
	agentID   string
	conn      *websocket.Conn
	handlers  []EventHandler
	mu        sync.RWMutex
	done      chan struct{}
	reconnect bool
}

// WSOption configures the WebSocket client
type WSOption func(*WSClient)

// WithWSAPIKey sets the API key for WebSocket authentication
func WithWSAPIKey(key string) WSOption {
	return func(c *WSClient) {
		c.apiKey = key
	}
}

// WithWSProject sets the project scope for filtering events
func WithWSProject(project string) WSOption {
	return func(c *WSClient) {
		c.project = project
	}
}

// WithWSAgentID sets the agent ID for the WebSocket connection
func WithWSAgentID(agentID string) WSOption {
	return func(c *WSClient) {
		c.agentID = agentID
	}
}

// WithAutoReconnect enables automatic reconnection on disconnect
func WithAutoReconnect(enabled bool) WSOption {
	return func(c *WSClient) {
		c.reconnect = enabled
	}
}

// NewWSClient creates a new WebSocket client for real-time events
func NewWSClient(baseURL string, opts ...WSOption) *WSClient {
	c := &WSClient{
		baseURL:   baseURL,
		handlers:  make([]EventHandler, 0),
		done:      make(chan struct{}),
		reconnect: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// OnEvent registers an event handler
func (c *WSClient) OnEvent(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}

// Connect establishes the WebSocket connection
func (c *WSClient) Connect(ctx context.Context) error {
	wsURL, err := c.buildWSURL()
	if err != nil {
		return fmt.Errorf("build websocket url: %w", err)
	}

	opts := &websocket.DialOptions{}
	if c.apiKey != "" {
		opts.HTTPHeader = make(map[string][]string)
		opts.HTTPHeader["Authorization"] = []string{"Bearer " + c.apiKey}
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.conn = conn

	go c.readLoop(ctx)

	return nil
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	close(c.done)
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "client closing")
	}
	return nil
}

// Subscribe sends a subscription message for specific event types
func (c *WSClient) Subscribe(ctx context.Context, eventTypes ...string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	msg := map[string]any{
		"type":        "subscribe",
		"event_types": eventTypes,
	}
	if c.project != "" {
		msg["project"] = c.project
	}
	return wsjson.Write(ctx, c.conn, msg)
}

// Unsubscribe sends an unsubscription message for specific event types
func (c *WSClient) Unsubscribe(ctx context.Context, eventTypes ...string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return wsjson.Write(ctx, c.conn, map[string]any{
		"type":        "unsubscribe",
		"event_types": eventTypes,
	})
}

func (c *WSClient) buildWSURL() (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	// Convert http(s) to ws(s)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	// Build path
	if c.agentID != "" {
		u.Path = "/ws/agents/" + c.agentID
	} else {
		u.Path = "/ws/events"
	}

	// Add project filter if specified
	if c.project != "" {
		q := u.Query()
		q.Set("project", c.project)
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}

func (c *WSClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		var event DomainEvent
		err := wsjson.Read(ctx, c.conn, &event)
		if err != nil {
			// Check if we should reconnect
			if c.reconnect {
				select {
				case <-c.done:
					return
				default:
					c.handleReconnect(ctx)
					continue
				}
			}
			return
		}

		c.dispatchEvent(event)
	}
}

func (c *WSClient) dispatchEvent(event DomainEvent) {
	c.mu.RLock()
	handlers := make([]EventHandler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

func (c *WSClient) handleReconnect(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		err := c.Connect(ctx)
		if err == nil {
			return
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// EventFilter provides typed event filtering
type EventFilter struct {
	Types    []string // Event types to listen for (e.g., "spec.created")
	Project  string   // Filter by project
	EntityID string   // Filter by specific entity ID
}

// FilteredEventHandler wraps an EventHandler with filtering logic
func FilteredEventHandler(filter EventFilter, handler EventHandler) EventHandler {
	return func(event DomainEvent) {
		// Check type filter
		if len(filter.Types) > 0 {
			matched := false
			for _, t := range filter.Types {
				if event.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				return
			}
		}

		// Check project filter
		if filter.Project != "" && event.Project != filter.Project {
			return
		}

		// Check entity ID filter
		if filter.EntityID != "" && event.EntityID != filter.EntityID {
			return
		}

		handler(event)
	}
}

// EventTypes defines standard domain event type constants
var EventTypes = struct {
	// Spec events
	SpecCreated  string
	SpecUpdated  string
	SpecArchived string

	// Epic events
	EpicCreated string
	EpicUpdated string

	// Story events
	StoryCreated string
	StoryUpdated string

	// Task events
	TaskCreated   string
	TaskAssigned  string
	TaskCompleted string

	// Insight events
	InsightCreated string
	InsightLinked  string

	// Session events
	SessionStarted string
	SessionStopped string

	// CUJ events (for future use)
	CUJCreated   string
	CUJValidated string
	CUJUpdated   string
}{
	SpecCreated:    "spec.created",
	SpecUpdated:    "spec.updated",
	SpecArchived:   "spec.archived",
	EpicCreated:    "epic.created",
	EpicUpdated:    "epic.updated",
	StoryCreated:   "story.created",
	StoryUpdated:   "story.updated",
	TaskCreated:    "task.created",
	TaskAssigned:   "task.assigned",
	TaskCompleted:  "task.completed",
	InsightCreated: "insight.created",
	InsightLinked:  "insight.linked",
	SessionStarted: "session.started",
	SessionStopped: "session.stopped",
	CUJCreated:     "cuj.created",
	CUJValidated:   "cuj.validated",
	CUJUpdated:     "cuj.updated",
}

// DomainEventData provides type-safe access to event data
type DomainEventData struct {
	raw json.RawMessage
}

// AsSpec decodes the event data as a Spec
func (d DomainEventData) AsSpec() (Spec, error) {
	var s Spec
	return s, json.Unmarshal(d.raw, &s)
}

// AsEpic decodes the event data as an Epic
func (d DomainEventData) AsEpic() (Epic, error) {
	var e Epic
	return e, json.Unmarshal(d.raw, &e)
}

// AsStory decodes the event data as a Story
func (d DomainEventData) AsStory() (Story, error) {
	var s Story
	return s, json.Unmarshal(d.raw, &s)
}

// AsTask decodes the event data as a Task
func (d DomainEventData) AsTask() (Task, error) {
	var t Task
	return t, json.Unmarshal(d.raw, &t)
}

// AsInsight decodes the event data as an Insight
func (d DomainEventData) AsInsight() (Insight, error) {
	var i Insight
	return i, json.Unmarshal(d.raw, &i)
}

// AsSession decodes the event data as a Session
func (d DomainEventData) AsSession() (Session, error) {
	var s Session
	return s, json.Unmarshal(d.raw, &s)
}
