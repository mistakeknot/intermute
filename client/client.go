package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	APIKey  string
	Project string
}

type Option func(*Client)

func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.APIKey = strings.TrimSpace(key)
	}
}

func WithProject(project string) Option {
	return func(c *Client) {
		c.Project = strings.TrimSpace(project)
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.HTTP = httpClient
		}
	}
}

type Agent struct {
	ID           string            `json:"agent_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Name         string            `json:"name,omitempty"`
	Project      string            `json:"project,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Status       string            `json:"status,omitempty"`
	LastSeen     string            `json:"last_seen,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
}

type ListAgentsResponse struct {
	Agents []Agent `json:"agents"`
}

type Message struct {
	ID          string   `json:"id,omitempty"`
	ThreadID    string   `json:"thread_id,omitempty"`
	Project     string   `json:"project,omitempty"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	CC          []string `json:"cc,omitempty"`
	BCC         []string `json:"bcc,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Body        string   `json:"body"`
	Importance  string   `json:"importance,omitempty"`
	AckRequired bool     `json:"ack_required,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	Cursor      uint64   `json:"cursor,omitempty"`
}

type SendResponse struct {
	MessageID string `json:"message_id"`
	Cursor    uint64 `json:"cursor"`
}

type InboxResponse struct {
	Messages []Message `json:"messages"`
	Cursor   uint64    `json:"cursor"`
}

type ThreadSummary struct {
	ThreadID     string `json:"thread_id"`
	LastCursor   uint64 `json:"last_cursor"`
	MessageCount int    `json:"message_count"`
	LastFrom     string `json:"last_from"`
	LastBody     string `json:"last_body"`
	LastAt       string `json:"last_at"`
}

type ListThreadsResponse struct {
	Threads []ThreadSummary `json:"threads"`
	Cursor  uint64          `json:"cursor"`
}

type ThreadMessagesResponse struct {
	ThreadID string    `json:"thread_id"`
	Messages []Message `json:"messages"`
	Cursor   uint64    `json:"cursor"`
}

// InboxCounts represents inbox statistics
type InboxCounts struct {
	Total  int `json:"total"`
	Unread int `json:"unread"`
}

// Reservation represents a file lock held by an agent
type Reservation struct {
	ID          string  `json:"id"`
	AgentID     string  `json:"agent_id"`
	Project     string  `json:"project"`
	PathPattern string  `json:"path_pattern"`
	Exclusive   bool    `json:"exclusive"`
	Reason      string  `json:"reason,omitempty"`
	TTLMinutes  int     `json:"ttl_minutes,omitempty"` // For requests
	CreatedAt   string  `json:"created_at,omitempty"`
	ExpiresAt   string  `json:"expires_at,omitempty"`
	ReleasedAt  *string `json:"released_at,omitempty"`
	IsActive    bool    `json:"is_active,omitempty"`
}

type ReservationsResponse struct {
	Reservations []Reservation `json:"reservations"`
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) RegisterAgent(ctx context.Context, agent Agent) (Agent, error) {
	if agent.Project == "" {
		agent.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/agents", agent)
	if err != nil {
		return Agent{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Agent{}, fmt.Errorf("register failed: %d", resp.StatusCode)
	}
	var out Agent
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Agent{}, err
	}
	return out, nil
}

func (c *Client) Heartbeat(ctx context.Context, agentID string) error {
	resp, err := c.postJSON(ctx, "/api/agents/"+url.PathEscape(agentID)+"/heartbeat", map[string]string{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListAgents(ctx context.Context, project string) ([]Agent, error) {
	values := url.Values{}
	if project != "" {
		values.Set("project", project)
	} else if c.Project != "" {
		values.Set("project", c.Project)
	}
	endpoint := "/api/agents"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list agents failed: %d", resp.StatusCode)
	}
	var out ListAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

// DiscoverAgents lists agents filtered by capability tags.
// Capabilities uses OR matching — agents with any of the given capabilities are returned.
func (c *Client) DiscoverAgents(ctx context.Context, capabilities []string) ([]Agent, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	if len(capabilities) > 0 {
		values.Set("capability", strings.Join(capabilities, ","))
	}
	endpoint := "/api/agents"
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover agents failed: %d", resp.StatusCode)
	}
	var out ListAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

func (c *Client) SendMessage(ctx context.Context, msg Message) (SendResponse, error) {
	if msg.Project == "" {
		msg.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/messages", msg)
	if err != nil {
		return SendResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SendResponse{}, fmt.Errorf("send failed: %d", resp.StatusCode)
	}
	var out SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SendResponse{}, err
	}
	return out, nil
}

func (c *Client) InboxSince(ctx context.Context, agent string, cursor uint64) (InboxResponse, error) {
	values := url.Values{}
	values.Set("since_cursor", fmt.Sprintf("%d", cursor))
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	endpoint := fmt.Sprintf("/api/inbox/%s?%s", url.PathEscape(agent), values.Encode())
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return InboxResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return InboxResponse{}, fmt.Errorf("inbox failed: %d", resp.StatusCode)
	}
	var out InboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return InboxResponse{}, err
	}
	return out, nil
}

func (c *Client) Ack(ctx context.Context, messageID string) error {
	return c.messageAction(ctx, messageID, "ack")
}

func (c *Client) Read(ctx context.Context, messageID string) error {
	return c.messageAction(ctx, messageID, "read")
}

func (c *Client) messageAction(ctx context.Context, messageID, action string) error {
	endpoint := "/api/messages/" + url.PathEscape(messageID) + "/" + action
	if c.Project != "" {
		endpoint += "?project=" + url.QueryEscape(c.Project)
	}
	resp, err := c.postJSON(ctx, endpoint, map[string]string{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s failed: %d", action, resp.StatusCode)
	}
	return nil
}

func (c *Client) ListThreads(ctx context.Context, agent string, cursor uint64) (ListThreadsResponse, error) {
	values := url.Values{}
	values.Set("agent", agent)
	values.Set("cursor", fmt.Sprintf("%d", cursor))
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	endpoint := "/api/threads?" + values.Encode()
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return ListThreadsResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ListThreadsResponse{}, fmt.Errorf("list threads failed: %d", resp.StatusCode)
	}
	var out ListThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ListThreadsResponse{}, err
	}
	return out, nil
}

func (c *Client) ThreadMessages(ctx context.Context, threadID string, cursor uint64) (ThreadMessagesResponse, error) {
	values := url.Values{}
	values.Set("cursor", fmt.Sprintf("%d", cursor))
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	endpoint := "/api/threads/" + url.PathEscape(threadID) + "?" + values.Encode()
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return ThreadMessagesResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ThreadMessagesResponse{}, fmt.Errorf("thread messages failed: %d", resp.StatusCode)
	}
	var out ThreadMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ThreadMessagesResponse{}, err
	}
	return out, nil
}

// InboxCounts returns the total and unread message counts for an agent
func (c *Client) InboxCounts(ctx context.Context, agent string) (InboxCounts, error) {
	values := url.Values{}
	if c.Project != "" {
		values.Set("project", c.Project)
	}
	endpoint := fmt.Sprintf("/api/inbox/%s/counts", url.PathEscape(agent))
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return InboxCounts{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return InboxCounts{}, fmt.Errorf("inbox counts failed: %d", resp.StatusCode)
	}
	var out InboxCounts
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return InboxCounts{}, err
	}
	return out, nil
}

// Reserve creates a new file reservation
func (c *Client) Reserve(ctx context.Context, r Reservation) (Reservation, error) {
	if r.Project == "" {
		r.Project = c.Project
	}
	resp, err := c.postJSON(ctx, "/api/reservations", r)
	if err != nil {
		return Reservation{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reservation{}, fmt.Errorf("reserve failed: %d", resp.StatusCode)
	}
	var out Reservation
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Reservation{}, err
	}
	return out, nil
}

// ReleaseReservation releases a file reservation by ID
func (c *Client) ReleaseReservation(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.BaseURL+"/api/reservations/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release failed: %d", resp.StatusCode)
	}
	return nil
}

// ActiveReservations returns all active reservations for a project
func (c *Client) ActiveReservations(ctx context.Context, project string) ([]Reservation, error) {
	if project == "" {
		project = c.Project
	}
	endpoint := "/api/reservations?project=" + url.QueryEscape(project)
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list reservations failed: %d", resp.StatusCode)
	}
	var out ReservationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Reservations, nil
}

// AgentReservations returns all reservations held by an agent
func (c *Client) AgentReservations(ctx context.Context, agentID string) ([]Reservation, error) {
	endpoint := "/api/reservations?agent=" + url.QueryEscape(agentID)
	resp, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent reservations failed: %d", resp.StatusCode)
	}
	var out ReservationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Reservations, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload any) (*http.Response, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	return c.HTTP.Do(req)
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
}
