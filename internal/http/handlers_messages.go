package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
)

type sendMessageRequest struct {
	ID               string             `json:"id"`
	ThreadID         string             `json:"thread_id"`
	Project          string             `json:"project"`
	From             string             `json:"from"`
	To               []string           `json:"to"`
	CC               []string           `json:"cc,omitempty"`
	BCC              []string           `json:"bcc,omitempty"`
	Subject          string             `json:"subject,omitempty"`
	Topic            string             `json:"topic,omitempty"`
	Body             string             `json:"body"`
	Importance       string             `json:"importance,omitempty"`
	Transport        core.TransportMode `json:"transport,omitempty"`
	TargetWindowUUID string             `json:"target_window_uuid,omitempty"`
	AckRequired      bool               `json:"ack_required,omitempty"`
}

type sendMessageResponse struct {
	MessageID string   `json:"message_id"`
	Cursor    uint64   `json:"cursor"`
	Denied    []string `json:"denied,omitempty"`
	Delivery  any      `json:"delivery,omitempty"`
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

type recipientPlan struct {
	Agent      string
	FocusState string
	Deliver    string
	Target     *core.WindowTarget
}

// allowedRecipients is the result of per-field policy gating.
type allowedRecipients struct {
	To     []string
	CC     []string
	BCC    []string
	Denied []string
}

func (s *Service) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limitBody(w, r)
	req, ok := parseSendRequest(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	project := strings.TrimSpace(req.Project)

	transport := s.resolveTransport(ctx, req.Transport)
	if !core.ValidTransport(transport) {
		http.Error(w, "invalid transport", http.StatusBadRequest)
		return
	}

	allowed, ok := s.resolveAllowedRecipients(ctx, w, project, req, transport)
	if !ok {
		return
	}
	if !s.enforceLiveRateLimit(w, req.From, allowed.To, transport) {
		return
	}

	plans, busy := s.resolveRecipientPlans(ctx, project, req.TargetWindowUUID, transport, allowed.To)
	if busy != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":       "recipient_busy",
			"agent":       busy.Agent,
			"focus_state": busy.FocusState,
		})
		return
	}

	msg := buildSendMessage(req, project, transport, allowed)
	deliveries, pokeEvents := s.deliverLive(ctx, project, msg, transport, plans)

	if transport == core.TransportLive {
		s.respondLive(w, ctx, pokeEvents, deliveries, allowed.Denied)
		return
	}
	s.respondDurable(w, ctx, project, msg, pokeEvents, deliveries, allowed.Denied)
}

// parseSendRequest decodes the request body, runs API-key authz, and writes
// the error response itself on failure. Returns (req, false) on failure.
func parseSendRequest(w http.ResponseWriter, r *http.Request) (sendMessageRequest, bool) {
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return req, false
	}
	if strings.TrimSpace(req.From) == "" || len(req.To) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return req, false
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		if strings.TrimSpace(req.Project) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return req, false
		}
		if req.Project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return req, false
		}
	}
	return req, true
}

// resolveTransport normalizes the request transport and applies the feature
// flag gate. If the flag is disabled, any non-async transport falls back to
// async. Invalid transport strings are returned as-is for the caller to
// 400 on via ValidTransport.
func (s *Service) resolveTransport(ctx context.Context, requested core.TransportMode) core.TransportMode {
	transport := core.TransportOrDefault(requested)
	if transport == core.TransportAsync {
		return transport
	}
	if enabled, err := s.store.LiveTransportEnabled(ctx); err == nil && !enabled {
		return core.TransportAsync
	}
	return transport
}

// resolveAllowedRecipients applies per-field contact policies. For
// transport != async, the To list is re-gated against live_contact_policy.
// Writes the 403 policy-denied response and returns (_, false) when every
// recipient is denied.
func (s *Service) resolveAllowedRecipients(ctx context.Context, w http.ResponseWriter, project string, req sendMessageRequest, transport core.TransportMode) (allowedRecipients, bool) {
	allowedTo, deniedTo := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.To)
	if transport != core.TransportAsync {
		allowedTo = allowedTo[:0]
		deniedTo = deniedTo[:0]
		for _, recipient := range req.To {
			policy, err := s.store.GetLiveContactPolicy(ctx, recipient)
			if err != nil {
				policy = core.PolicyContactsOnly
			}
			if s.checkPolicy(ctx, project, req.From, recipient, req.ThreadID, policy) {
				allowedTo = append(allowedTo, recipient)
				continue
			}
			deniedTo = append(deniedTo, recipient)
		}
	}
	allowedCC, deniedCC := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.CC)
	allowedBCC, deniedBCC := s.filterByPolicy(ctx, project, req.From, req.ThreadID, req.BCC)
	allDenied := append(append(deniedTo, deniedCC...), deniedBCC...)

	if len(allowedTo) == 0 && len(allowedCC) == 0 && len(allowedBCC) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(policyDeniedResponse{
			Error:  "policy_denied",
			Denied: allDenied,
		})
		return allowedRecipients{}, false
	}
	return allowedRecipients{
		To:     allowedTo,
		CC:     allowedCC,
		BCC:    allowedBCC,
		Denied: allDenied,
	}, true
}

// enforceLiveRateLimit returns false after writing a 429 response when the
// sender has exceeded the per-(sender, recipient) live-send budget. No-op
// for transport=async.
func (s *Service) enforceLiveRateLimit(w http.ResponseWriter, from string, to []string, transport core.TransportMode) bool {
	if transport == core.TransportAsync {
		return true
	}
	for _, recipient := range to {
		if s.liveAllow(from, recipient) {
			continue
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":               "rate_limit",
			"retry_after_seconds": int(liveRateWindow / time.Second),
		})
		return false
	}
	return true
}

func buildSendMessage(req sendMessageRequest, project string, transport core.TransportMode, allowed allowedRecipients) core.Message {
	msgID := req.ID
	if msgID == "" {
		msgID = uuid.NewString()
	}
	return core.Message{
		ID:          msgID,
		ThreadID:    req.ThreadID,
		Project:     project,
		From:        req.From,
		To:          allowed.To,
		CC:          allowed.CC,
		BCC:         allowed.BCC,
		Subject:     req.Subject,
		Topic:       req.Topic,
		Body:        req.Body,
		Importance:  req.Importance,
		Transport:   transport,
		AckRequired: req.AckRequired,
		CreatedAt:   time.Now().UTC(),
	}
}

// respondLive writes the HTTP response for a transport=live send. Any
// failure in the delivery set means no durable message exists to audit
// against, so AppendEvents is skipped entirely — no orphan audit rows.
// Partial-inject (one recipient delivered, another failed) returns 503;
// the successful inject is not audited, matching plan intent.
func (s *Service) respondLive(w http.ResponseWriter, ctx context.Context, pokeEvents []core.Event, deliveries map[string]string, denied []string) {
	if len(pokeEvents) == 0 || hasFailedDelivery(deliveries) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":    "delivery_failed",
			"delivery": collapseDeliveries(deliveries),
		})
		return
	}
	if _, err := s.store.AppendEvents(ctx, pokeEvents...); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"delivery": collapseDeliveries(deliveries),
		"denied":   denied,
	})
}

// respondDurable writes the HTTP response for transport in {async, both}.
// Durable message + poke events commit atomically via AppendEvents.
func (s *Service) respondDurable(w http.ResponseWriter, ctx context.Context, project string, msg core.Message, pokeEvents []core.Event, deliveries map[string]string, denied []string) {
	events := []core.Event{{Type: core.EventMessageCreated, Project: project, Message: msg}}
	events = append(events, pokeEvents...)
	cursors, err := s.store.AppendEvents(ctx, events...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cursor := uint64(0)
	if len(cursors) > 0 {
		cursor = cursors[0]
	}
	if s.bus != nil {
		for _, agent := range msg.To {
			s.bus.Broadcast(project, agent, map[string]any{
				"type":       string(core.EventMessageCreated),
				"project":    project,
				"message_id": msg.ID,
				"cursor":     cursor,
				"agent":      agent,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sendMessageResponse{
		MessageID: msg.ID,
		Cursor:    cursor,
		Denied:    denied,
		Delivery:  collapseDeliveries(deliveries),
	})
}

func (s *Service) resolveRecipientPlans(ctx context.Context, project, requestedWindowUUID string, transport core.TransportMode, recipients []string) ([]recipientPlan, *recipientPlan) {
	plans := make([]recipientPlan, 0, len(recipients))
	for _, recipient := range recipients {
		plan := recipientPlan{Agent: recipient}
		if transport == core.TransportAsync {
			plan.Deliver = "async"
			plans = append(plans, plan)
			continue
		}

		focusState, _, err := s.store.GetAgentFocusState(ctx, recipient)
		if err != nil || focusState == "" {
			focusState = core.FocusStateUnknown
		}
		plan.FocusState = focusState

		if focusState != core.FocusStateAtPrompt {
			if transport == core.TransportLive {
				plan.Deliver = "busy"
				return nil, &plan
			}
			plan.Deliver = "defer"
			plans = append(plans, plan)
			continue
		}

		target, err := s.resolveTarget(ctx, project, recipient, requestedWindowUUID)
		if err != nil {
			if transport == core.TransportLive {
				plan.Deliver = "busy"
				return nil, &plan
			}
			plan.Deliver = "defer"
			plans = append(plans, plan)
			continue
		}

		plan.Deliver = "inject"
		plan.Target = target
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *Service) resolveTarget(ctx context.Context, project, agentID, requestedWindowUUID string) (*core.WindowTarget, error) {
	if strings.TrimSpace(requestedWindowUUID) != "" {
		wi, err := s.store.LookupWindowIdentity(ctx, project, requestedWindowUUID)
		if err != nil {
			return nil, err
		}
		if wi == nil || wi.AgentID != agentID || strings.TrimSpace(wi.TmuxTarget) == "" {
			return nil, errors.New("window target not found")
		}
		return &core.WindowTarget{AgentID: agentID, TmuxTarget: wi.TmuxTarget}, nil
	}

	windows, err := s.store.ListWindowIdentities(ctx, project)
	if err != nil {
		return nil, err
	}
	for _, wi := range windows {
		if wi.AgentID != agentID || strings.TrimSpace(wi.TmuxTarget) == "" {
			continue
		}
		return &core.WindowTarget{AgentID: agentID, TmuxTarget: wi.TmuxTarget}, nil
	}
	return nil, errors.New("window target not found")
}

func (s *Service) deliverLive(ctx context.Context, project string, msg core.Message, transport core.TransportMode, plans []recipientPlan) (map[string]string, []core.Event) {
	deliveries := make(map[string]string, len(plans))
	pokeEvents := make([]core.Event, 0, len(plans))
	envelope := core.WrapEnvelope(msg.From, msg.ThreadID, msg.Body)

	for _, plan := range plans {
		switch plan.Deliver {
		case "async":
			deliveries[plan.Agent] = "async"
		case "defer":
			deliveries[plan.Agent] = "deferred"
			pokeEvents = append(pokeEvents, core.Event{
				Type:    core.EventPeerWindowPoke,
				Project: project,
				Agent:   plan.Agent,
				Message: core.Message{
					ID:        msg.ID,
					Project:   project,
					From:      msg.From,
					To:        []string{plan.Agent},
					Body:      msg.Body,
					Transport: msg.Transport,
					CreatedAt: time.Now().UTC(),
					Metadata: map[string]string{
						"poke_result": core.PokeResultDeferred,
						"poke_reason": "recipient_" + plan.FocusState,
					},
				},
			})
		case "inject":
			err := s.liveDelivery.Deliver(plan.Target, envelope)
			if err != nil {
				if transport == core.TransportLive {
					deliveries[plan.Agent] = "failed"
					pokeEvents = append(pokeEvents, core.Event{
						Type:    core.EventPeerWindowPoke,
						Project: project,
						Agent:   plan.Agent,
						Message: core.Message{
							ID:        msg.ID,
							Project:   project,
							From:      msg.From,
							To:        []string{plan.Agent},
							Body:      msg.Body,
							Transport: msg.Transport,
							CreatedAt: time.Now().UTC(),
							Metadata: map[string]string{
								"poke_result": core.PokeResultFailed,
								"poke_reason": err.Error(),
							},
						},
					})
					continue
				}
				deliveries[plan.Agent] = "deferred"
				pokeEvents = append(pokeEvents, core.Event{
					Type:    core.EventPeerWindowPoke,
					Project: project,
					Agent:   plan.Agent,
					Message: core.Message{
						ID:        msg.ID,
						Project:   project,
						From:      msg.From,
						To:        []string{plan.Agent},
						Body:      msg.Body,
						Transport: msg.Transport,
						CreatedAt: time.Now().UTC(),
						Metadata: map[string]string{
							"poke_result": core.PokeResultDeferred,
							"poke_reason": "inject_failed: " + err.Error(),
						},
					},
				})
				continue
			}

			deliveries[plan.Agent] = "injected"
			pokeEvents = append(pokeEvents, core.Event{
				Type:    core.EventPeerWindowPoke,
				Project: project,
				Agent:   plan.Agent,
				Message: core.Message{
					ID:        msg.ID,
					Project:   project,
					From:      msg.From,
					To:        []string{plan.Agent},
					Body:      msg.Body,
					Transport: msg.Transport,
					CreatedAt: time.Now().UTC(),
					Metadata: map[string]string{
						"poke_result": core.PokeResultInjected,
					},
				},
			})
		}
	}

	return deliveries, pokeEvents
}

func collapseDeliveries(deliveries map[string]string) any {
	if len(deliveries) == 1 {
		for _, delivery := range deliveries {
			return delivery
		}
	}
	if len(deliveries) == 0 {
		return nil
	}
	return deliveries
}

func hasFailedDelivery(deliveries map[string]string) bool {
	for _, delivery := range deliveries {
		if delivery == "failed" {
			return true
		}
	}
	return false
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
		if s.checkPolicy(ctx, project, sender, recipient, threadID, policy) {
			allowed = append(allowed, recipient)
			continue
		}
		denied = append(denied, recipient)
	}
	return
}

// checkPolicy returns true if sender can send to recipient under the given policy.
// Shared between async filtering and live-delivery gating.
func (s *Service) checkPolicy(ctx context.Context, project, sender, recipient, threadID string, policy core.ContactPolicy) bool {
	switch policy {
	case core.PolicyOpen, "":
		return true
	case core.PolicyBlockAll:
		return false
	case core.PolicyContactsOnly:
		return s.senderAllowed(ctx, project, sender, recipient, threadID)
	case core.PolicyAuto:
		return s.senderAllowedAuto(ctx, project, sender, recipient, threadID)
	default:
		return true
	}
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
	// Check if this is a stale-acks request: /api/inbox/{agent}/stale-acks
	if strings.HasSuffix(path, "/stale-acks") {
		s.handleStaleAcks(w, r)
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

type staleAckItem struct {
	ID         string   `json:"id"`
	ThreadID   string   `json:"thread_id"`
	Project    string   `json:"project"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	Subject    string   `json:"subject,omitempty"`
	Body       string   `json:"body"`
	CreatedAt  string   `json:"created_at"`
	Kind       string   `json:"kind"`
	ReadAt     *string  `json:"read_at"`
	AgeSeconds int      `json:"age_seconds"`
}

type staleAcksResponse struct {
	Project    string         `json:"project"`
	Agent      string         `json:"agent"`
	TTLSeconds int            `json:"ttl_seconds"`
	Count      int            `json:"count"`
	Messages   []staleAckItem `json:"messages"`
}

func (s *Service) handleStaleAcks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Path: /api/inbox/{agent}/stale-acks
	path := strings.TrimPrefix(r.URL.Path, "/api/inbox/")
	path = strings.TrimSuffix(path, "/stale-acks")
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

	ttlSeconds := 1800 // Default: 30 minutes
	if v := r.URL.Query().Get("ttl_seconds"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			ttlSeconds = parsed
		}
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	staleAcks, err := s.store.InboxStaleAcks(r.Context(), project, agent, ttlSeconds, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	items := make([]staleAckItem, 0, len(staleAcks))
	for _, sa := range staleAcks {
		item := staleAckItem{
			ID:         sa.Message.ID,
			ThreadID:   sa.Message.ThreadID,
			Project:    sa.Message.Project,
			From:       sa.Message.From,
			To:         sa.Message.To,
			Subject:    sa.Message.Subject,
			Body:       sa.Message.Body,
			CreatedAt:  sa.Message.CreatedAt.Format(time.RFC3339Nano),
			Kind:       sa.Kind,
			AgeSeconds: sa.AgeSeconds,
		}
		if sa.ReadAt != nil {
			s := sa.ReadAt.Format(time.RFC3339Nano)
			item.ReadAt = &s
		}
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(staleAcksResponse{
		Project:    project,
		Agent:      agent,
		TTLSeconds: ttlSeconds,
		Count:      len(items),
		Messages:   items,
	})
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

// --- Broadcast messaging ---

type broadcastRequest struct {
	From    string `json:"from"`
	Project string `json:"project"`
	Topic   string `json:"topic"`
	Body    string `json:"body"`
	Subject string `json:"subject,omitempty"`
}

type broadcastResponse struct {
	MessageID string   `json:"message_id"`
	Cursor    uint64   `json:"cursor"`
	Delivered int      `json:"delivered"`
	Denied    []string `json:"denied,omitempty"`
}

func (s *Service) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req broadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.Topic) == "" || strings.TrimSpace(req.Body) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, _ := auth.FromContext(r.Context())
	project := strings.TrimSpace(req.Project)
	if info.Mode == auth.ModeAPIKey {
		if project == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	// Rate limit: 10 broadcasts per minute per sender
	if s.broadcastExceeded(project, req.From) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	ctx := r.Context()

	// Resolve all agents in the project
	agents, err := s.store.ListAgents(ctx, project, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Build To list: all agents except sender
	var toList []string
	for _, a := range agents {
		if a.ID != req.From {
			toList = append(toList, a.ID)
		}
	}
	if len(toList) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(broadcastResponse{Delivered: 0})
		return
	}

	// Filter by contact policies (no threadID exception for broadcasts)
	allowed, denied := s.filterByPolicy(ctx, project, req.From, "", toList)
	if len(allowed) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(policyDeniedResponse{
			Error:  "policy_denied",
			Denied: denied,
		})
		return
	}

	msgID := uuid.NewString()
	msg := core.Message{
		ID:        msgID,
		Project:   project,
		From:      req.From,
		To:        allowed,
		Subject:   req.Subject,
		Topic:     req.Topic,
		Body:      req.Body,
		CreatedAt: time.Now().UTC(),
	}
	cursor, err := s.store.AppendEvent(ctx, core.Event{
		Type:    core.EventMessageCreated,
		Project: project,
		Message: msg,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// SSE notification per recipient
	if s.bus != nil {
		for _, agent := range allowed {
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
	_ = json.NewEncoder(w).Encode(broadcastResponse{
		MessageID: msgID,
		Cursor:    cursor,
		Delivered: len(allowed),
		Denied:    denied,
	})
}
