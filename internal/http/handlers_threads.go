package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
)

type threadSummaryJSON struct {
	ThreadID     string `json:"thread_id"`
	LastCursor   uint64 `json:"last_cursor"`
	MessageCount int    `json:"message_count"`
	LastFrom     string `json:"last_from"`
	LastBody     string `json:"last_body"`
	LastAt       string `json:"last_at"`
}

type listThreadsResponse struct {
	Threads []threadSummaryJSON `json:"threads"`
	Cursor  uint64              `json:"cursor"`
}

type threadMessagesResponse struct {
	ThreadID string       `json:"thread_id"`
	Messages []apiMessage `json:"messages"`
	Cursor   uint64       `json:"cursor"`
}

func (s *Service) handleListThreads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	agent := strings.TrimSpace(r.URL.Query().Get("agent"))
	if agent == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}

	var cursor uint64
	if v := r.URL.Query().Get("cursor"); v != "" {
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
	if limit <= 0 {
		limit = 50
	}

	threads, err := s.store.ListThreads(r.Context(), project, agent, cursor, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	nextCursor := cursor
	out := make([]threadSummaryJSON, 0, len(threads))
	for _, t := range threads {
		out = append(out, threadSummaryJSON{
			ThreadID:     t.ThreadID,
			LastCursor:   t.LastCursor,
			MessageCount: t.MessageCount,
			LastFrom:     t.LastFrom,
			LastBody:     t.LastBody,
			LastAt:       t.LastAt.Format(time.RFC3339),
		})
	}
	if len(threads) > 0 {
		// Threads are ordered DESC, so the last item is the next cursor.
		nextCursor = threads[len(threads)-1].LastCursor
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listThreadsResponse{Threads: out, Cursor: nextCursor})
}

func (s *Service) handleThreadMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	threadID := strings.TrimPrefix(r.URL.Path, "/api/threads/")
	threadID = strings.Trim(threadID, "/")
	if threadID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}

	var cursor uint64
	if v := r.URL.Query().Get("cursor"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			cursor = parsed
		}
	}

	msgs, err := s.store.ThreadMessages(r.Context(), project, threadID, cursor)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var lastCursor uint64
	apiMsgs := make([]apiMessage, 0, len(msgs))
	for _, m := range msgs {
		apiMsgs = append(apiMsgs, apiMessage{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			Project:   m.Project,
			From:      m.From,
			To:        m.To,
			Body:      m.Body,
			CreatedAt: m.CreatedAt.Format(time.RFC3339Nano),
			Cursor:    m.Cursor,
		})
		if m.Cursor > lastCursor {
			lastCursor = m.Cursor
		}
	}
	if lastCursor == 0 {
		lastCursor = cursor
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(threadMessagesResponse{
		ThreadID: threadID,
		Messages: apiMsgs,
		Cursor:   lastCursor,
	})
}
