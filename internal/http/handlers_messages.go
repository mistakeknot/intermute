package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
)

type sendMessageRequest struct {
	ID          string   `json:"id"`
	ThreadID    string   `json:"thread_id"`
	Project     string   `json:"project"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	CC          []string `json:"cc,omitempty"`
	BCC         []string `json:"bcc,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Body        string   `json:"body"`
	Importance  string   `json:"importance,omitempty"`
	AckRequired bool     `json:"ack_required,omitempty"`
}

type sendMessageResponse struct {
	MessageID string `json:"message_id"`
	Cursor    uint64 `json:"cursor"`
}

type apiMessage struct {
	ID          string   `json:"id"`
	ThreadID    string   `json:"thread_id"`
	Project     string   `json:"project"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	CC          []string `json:"cc,omitempty"`
	BCC         []string `json:"bcc,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Body        string   `json:"body"`
	Importance  string   `json:"importance,omitempty"`
	AckRequired bool     `json:"ack_required,omitempty"`
	CreatedAt   string   `json:"created_at"`
	Cursor      uint64   `json:"cursor"`
}

type inboxResponse struct {
	Messages []apiMessage `json:"messages"`
	Cursor   uint64       `json:"cursor"`
}

func (s *Service) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.From) == "" || len(req.To) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		if strings.TrimSpace(req.Project) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	msgID := req.ID
	if msgID == "" {
		msgID = uuid.NewString()
	}
	project := strings.TrimSpace(req.Project)
	msg := core.Message{
		ID:          msgID,
		ThreadID:    req.ThreadID,
		Project:     project,
		From:        req.From,
		To:          req.To,
		CC:          req.CC,
		BCC:         req.BCC,
		Subject:     req.Subject,
		Body:        req.Body,
		Importance:  req.Importance,
		AckRequired: req.AckRequired,
		CreatedAt:   time.Now().UTC(),
	}
	cursor, err := s.store.AppendEvent(core.Event{Type: core.EventMessageCreated, Project: project, Message: msg})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if s.bus != nil {
		for _, agent := range msg.To {
			s.bus.Broadcast(project, agent, map[string]any{
				"type":       string(core.EventMessageCreated),
				"project":    project,
				"message_id": msgID,
				"cursor":     cursor,
				"agent":      agent,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sendMessageResponse{MessageID: msgID, Cursor: cursor})
}

func (s *Service) handleInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	agent := strings.TrimPrefix(r.URL.Path, "/api/inbox/")
	agent = strings.Trim(agent, "/")
	if agent == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}
	cursor := uint64(0)
	if v := r.URL.Query().Get("since_cursor"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			cursor = parsed
		}
	}
	msgs, err := s.store.InboxSince(project, agent, cursor)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lastCursor := cursor
	if len(msgs) > 0 {
		lastCursor = msgs[len(msgs)-1].Cursor
	}
	apiMsgs := make([]apiMessage, 0, len(msgs))
	for _, m := range msgs {
		apiMsgs = append(apiMsgs, apiMessage{
			ID:          m.ID,
			ThreadID:    m.ThreadID,
			Project:     m.Project,
			From:        m.From,
			To:          m.To,
			CC:          m.CC,
			BCC:         m.BCC,
			Subject:     m.Subject,
			Body:        m.Body,
			Importance:  m.Importance,
			AckRequired: m.AckRequired,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339Nano),
			Cursor:      m.Cursor,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inboxResponse{Messages: apiMsgs, Cursor: lastCursor})
}

type messageActionRequest struct {
	Agent string `json:"agent"` // Agent performing the action
}

func (s *Service) handleMessageAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	msgID := parts[0]
	action := parts[1]
	var evType core.EventType
	switch action {
	case "ack":
		evType = core.EventMessageAck
	case "read":
		evType = core.EventMessageRead
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}

	// Parse request body to get agent ID (if provided)
	var req messageActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	agentID := req.Agent
	if agentID == "" {
		agentID = r.URL.Query().Get("agent")
	}

	// Update per-recipient tracking if agent ID is provided
	if agentID != "" {
		switch action {
		case "read":
			_ = s.store.MarkRead(project, msgID, agentID)
		case "ack":
			_ = s.store.MarkAck(project, msgID, agentID)
		}
	}

	_, err := s.store.AppendEvent(core.Event{Type: evType, Agent: agentID, Project: project, Message: core.Message{ID: msgID, Project: project}})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if s.bus != nil {
		s.bus.Broadcast(project, "", map[string]any{
			"type":       string(evType),
			"project":    project,
			"message_id": msgID,
		})
	}
	w.WriteHeader(http.StatusOK)
}
