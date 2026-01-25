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
	ID        string   `json:"id,omitempty"`
	ThreadID  string   `json:"thread_id,omitempty"`
	Project   string   `json:"project,omitempty"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at,omitempty"`
	Cursor    uint64   `json:"cursor,omitempty"`
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
