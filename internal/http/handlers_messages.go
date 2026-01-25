package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/core"
)

type sendMessageRequest struct {
	From     string   `json:"from"`
	To       []string `json:"to"`
	Body     string   `json:"body"`
	ThreadID string   `json:"thread_id"`
	ID       string   `json:"id"`
}

type sendMessageResponse struct {
	MessageID string `json:"message_id"`
	Cursor    uint64 `json:"cursor"`
}

type inboxResponse struct {
	Messages []core.Message `json:"messages"`
	Cursor   uint64         `json:"cursor"`
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
	msgID := req.ID
	if msgID == "" {
		msgID = uuid.NewString()
	}
	msg := core.Message{
		ID:        msgID,
		ThreadID:  req.ThreadID,
		From:      req.From,
		To:        req.To,
		Body:      req.Body,
		CreatedAt: time.Now().UTC(),
	}
	cursor, err := s.store.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: msg})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if s.bus != nil {
		for _, agent := range msg.To {
			s.bus.Broadcast(agent, map[string]any{
				"type":       string(core.EventMessageCreated),
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
	cursor := uint64(0)
	if v := r.URL.Query().Get("since_cursor"); v != "" {
		var parsed uint64
		_, _ = fmt.Sscanf(v, "%d", &parsed)
		cursor = parsed
	}
	msgs, err := s.store.InboxSince(agent, cursor)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lastCursor := cursor
	if len(msgs) > 0 {
		lastCursor = msgs[len(msgs)-1].Cursor
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inboxResponse{Messages: msgs, Cursor: lastCursor})
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
	_, err := s.store.AppendEvent(core.Event{Type: evType, Message: core.Message{ID: msgID}})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if s.bus != nil {
		s.bus.Broadcast("", map[string]any{
			"type":       string(evType),
			"message_id": msgID,
		})
	}
	w.WriteHeader(http.StatusOK)
}
