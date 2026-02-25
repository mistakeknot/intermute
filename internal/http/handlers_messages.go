package httpapi

import (
	"context"
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
	Topic       string   `json:"topic,omitempty"`
	Body        string   `json:"body"`
	Importance  string   `json:"importance,omitempty"`
	AckRequired bool     `json:"ack_required,omitempty"`
}

type sendMessageResponse struct {
	MessageID string   `json:"message_id"`
	Cursor    uint64   `json:"cursor"`
	Denied    []string `json:"denied,omitempty"`
}

type policyDeniedResponse struct {
	Error  string   `json:"error"`
	Denied []string `json:"denied"`
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
	Topic       string   `json:"topic,omitempty"`
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
	// Enforce contact policies on all recipient lists
	ctx := r.Context()
	allowedTo, deniedTo := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.To)
	allowedCC, deniedCC := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.CC)
	allowedBCC, deniedBCC := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.BCC)
	allDenied := append(append(deniedTo, deniedCC...), deniedBCC...)

	// If ALL recipients denied, return 403
	if len(allowedTo) == 0 && len(allowedCC) == 0 && len(allowedBCC) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(policyDeniedResponse{
			Error:  "policy_denied",
			Denied: allDenied,
		})
		return
	}

	msg := core.Message{
		ID:          msgID,
		ThreadID:    req.ThreadID,
		Project:     project,
		From:        req.From,
		To:          allowedTo,
		CC:          allowedCC,
		BCC:         allowedBCC,
		Subject:     req.Subject,
		Topic:       req.Topic,
		Body:        req.Body,
		Importance:  req.Importance,
		AckRequired: req.AckRequired,
		CreatedAt:   time.Now().UTC(),
	}
	cursor, err := s.store.AppendEvent(ctx, core.Event{Type: core.EventMessageCreated, Project: project, Message: msg})
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
	_ = json.NewEncoder(w).Encode(sendMessageResponse{MessageID: msgID, Cursor: cursor, Denied: allDenied})
}

// filterByPolicy checks each recipient's contact policy and returns allowed/denied lists.
func (s *Service) filterByPolicy(ctx context.Context, project, sender, threadID string, recipients []string) (allowed, denied []string) {
	for _, recipient := range recipients {
		policy, err := s.store.GetContactPolicy(ctx, recipient)
		if err != nil {
			// On error, default to open (don't block delivery on lookup failure)
			allowed = append(allowed, recipient)
			continue
		}
		switch policy {
		case core.PolicyOpen, "":
			allowed = append(allowed, recipient)
		case core.PolicyBlockAll:
			denied = append(denied, recipient)
		case core.PolicyContactsOnly:
			if s.senderAllowed(ctx, project, sender, recipient, threadID) {
				allowed = append(allowed, recipient)
			} else {
				denied = append(denied, recipient)
			}
		case core.PolicyAuto:
			if s.senderAllowedAuto(ctx, project, sender, recipient, threadID) {
				allowed = append(allowed, recipient)
			} else {
				denied = append(denied, recipient)
			}
		default:
			// Unknown policy — default open
			allowed = append(allowed, recipient)
		}
	}
	return
}

// senderAllowed checks if sender passes contacts_only policy for recipient.
func (s *Service) senderAllowed(ctx context.Context, project, sender, recipient, threadID string) bool {
	// Check explicit contact list
	if ok, err := s.store.IsContact(ctx, recipient, sender); err == nil && ok {
		return true
	}
	// Thread participant exception (but not for block_all, which is handled above)
	if threadID != "" {
		if ok, err := s.store.IsThreadParticipant(ctx, project, threadID, sender); err == nil && ok {
			return true
		}
	}
	return false
}

// senderAllowedAuto checks if sender passes auto policy for recipient.
// Auto allows: file reservation overlap OR contact list OR thread participant.
func (s *Service) senderAllowedAuto(ctx context.Context, project, sender, recipient, threadID string) bool {
	// Check file reservation overlap first (the defining feature of auto mode)
	if ok, err := s.store.HasReservationOverlap(ctx, project, recipient, sender); err == nil && ok {
		return true
	}
	// Fall through to contacts_only checks
	return s.senderAllowed(ctx, project, sender, recipient, threadID)
}

func (s *Service) handleInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Check if this is a counts request: /api/inbox/{agent}/counts
	path := strings.TrimPrefix(r.URL.Path, "/api/inbox/")
	if strings.HasSuffix(path, "/counts") {
		s.handleInboxCounts(w, r)
		return
	}
	agent := strings.Trim(path, "/")
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
	var limit int
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	msgs, err := s.store.InboxSince(r.Context(), project, agent, cursor, limit)
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
			Topic:       m.Topic,
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

type inboxCountsResponse struct {
	Total  int `json:"total"`
	Unread int `json:"unread"`
}

func (s *Service) handleInboxCounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Path: /api/inbox/{agent}/counts
	path := strings.TrimPrefix(r.URL.Path, "/api/inbox/")
	path = strings.TrimSuffix(path, "/counts")
	agent := strings.Trim(path, "/")
	if agent == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}

	total, unread, err := s.store.InboxCounts(r.Context(), project, agent)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inboxCountsResponse{Total: total, Unread: unread})
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
			_ = s.store.MarkRead(r.Context(), project, msgID, agentID)
		case "ack":
			_ = s.store.MarkAck(r.Context(), project, msgID, agentID)
		}
	}

	_, err := s.store.AppendEvent(r.Context(), core.Event{Type: evType, Agent: agentID, Project: project, Message: core.Message{ID: msgID, Project: project}})
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

func (s *Service) handleTopicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Path: /api/topics/{project}/{topic}
	path := strings.TrimPrefix(r.URL.Path, "/api/topics/")
	parts := strings.SplitN(strings.Trim(path, "/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	project := parts[0]
	topic := parts[1]

	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		if project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	cursor := uint64(0)
	if v := r.URL.Query().Get("since_cursor"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			cursor = parsed
		}
	}
	var limit int
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	msgs, err := s.store.TopicMessages(r.Context(), project, topic, cursor, limit)
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
			Topic:       m.Topic,
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
