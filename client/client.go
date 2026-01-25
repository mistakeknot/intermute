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
}

type Agent struct {
	ID           string            `json:"agent_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Name         string            `json:"name,omitempty"`
	Project      string            `json:"project,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Status       string            `json:"status,omitempty"`
}

type Message struct {
	ID       string   `json:"id,omitempty"`
	ThreadID string   `json:"thread_id,omitempty"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	Body     string   `json:"body"`
	CreatedAt string  `json:"created_at,omitempty"`
	Cursor   uint64   `json:"cursor,omitempty"`
}

type SendResponse struct {
	MessageID string `json:"message_id"`
	Cursor    uint64 `json:"cursor"`
}

type InboxResponse struct {
	Messages []Message `json:"messages"`
	Cursor   uint64    `json:"cursor"`
}

func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) RegisterAgent(ctx context.Context, agent Agent) (Agent, error) {
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

func (c *Client) SendMessage(ctx context.Context, msg Message) (SendResponse, error) {
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
	endpoint := fmt.Sprintf("/api/inbox/%s?since_cursor=%d", url.PathEscape(agent), cursor)
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
	resp, err := c.postJSON(ctx, "/api/messages/"+url.PathEscape(messageID)+"/"+action, map[string]string{})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s failed: %d", action, resp.StatusCode)
	}
	return nil
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
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.HTTP.Do(req)
}
