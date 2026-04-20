package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
)

// InboxPoke is the wire shape of a pending peer poke.
// Exported so the intermute CLI (and any external client) can decode
// responses from GET /api/inbox/pokes without duplicating the struct.
type InboxPoke struct {
	MessageID string `json:"message_id"`
	Sender    string `json:"sender"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// InboxPokesResponse is the response shape of GET /api/inbox/pokes.
type InboxPokesResponse struct {
	Pokes []InboxPoke `json:"pokes"`
}

type inboxPokeAckResponse struct {
	Status string `json:"status"`
}

func (s *Service) handleInboxPokes(w http.ResponseWriter, r *http.Request) {
	dispatchByMethod(w, r, methodHandlers{
		get: s.listInboxPokes,
	})
}

func (s *Service) handleInboxPokeAction(w http.ResponseWriter, r *http.Request) {
	dispatchByMethod(w, r, methodHandlers{
		post: s.ackInboxPoke,
	})
}

func (s *Service) listInboxPokes(w http.ResponseWriter, r *http.Request) {
	project, agent, ok := inboxPokeScope(r)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pending, err := s.store.ListPendingPokes(r.Context(), project, agent)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := InboxPokesResponse{Pokes: make([]InboxPoke, 0, len(pending))}
	for _, poke := range pending {
		resp.Pokes = append(resp.Pokes, InboxPoke{
			MessageID: poke.MessageID,
			Sender:    poke.Sender,
			Body:      core.WrapEnvelope(poke.Sender, "", poke.Body),
			CreatedAt: poke.CreatedAt.UTC().Format(http.TimeFormat),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Service) ackInboxPoke(w http.ResponseWriter, r *http.Request) {
	project, agent, ok := inboxPokeScope(r)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	messageID := strings.TrimPrefix(r.URL.Path, "/api/inbox/pokes/")
	messageID = strings.TrimSuffix(messageID, "/ack")
	messageID = strings.Trim(messageID, "/")
	if messageID == "" || !strings.HasSuffix(r.URL.Path, "/ack") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.store.MarkPokeSurfaced(r.Context(), project, agent, messageID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inboxPokeAckResponse{Status: "ok"})
}

func inboxPokeScope(r *http.Request) (project, agent string, ok bool) {
	info, _ := auth.FromContext(r.Context())
	project = strings.TrimSpace(info.Project)
	if project == "" {
		project = strings.TrimSpace(r.URL.Query().Get("project"))
	}
	agent = strings.TrimSpace(r.URL.Query().Get("agent"))
	if project == "" || agent == "" {
		return "", "", false
	}
	return project, agent, true
}
