package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/storage"
)

// DomainService extends Service with domain operations
type DomainService struct {
	*Service
	domainStore storage.DomainStore
}

func NewDomainService(store storage.DomainStore) *DomainService {
	return &DomainService{
		Service:     NewService(store),
		domainStore: store,
	}
}

func (s *DomainService) WithBroadcaster(b Broadcaster) *DomainService {
	s.Service.WithBroadcaster(b)
	return s
}

// Spec handlers

func (s *DomainService) handleSpecs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSpecs(w, r)
	case http.MethodPost:
		s.createSpec(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleSpecByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/specs/")
	id = strings.Trim(id, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getSpec(w, r, id)
	case http.MethodPut:
		s.updateSpec(w, r, id)
	case http.MethodDelete:
		s.deleteSpec(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createSpec(w http.ResponseWriter, r *http.Request) {
	var spec core.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && spec.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateSpec(spec)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(spec.Project, core.EventSpecCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getSpec(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	spec, err := s.domainStore.GetSpec(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

func (s *DomainService) listSpecs(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	status := r.URL.Query().Get("status")
	specs, err := s.domainStore.ListSpecs(project, status)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if specs == nil {
		specs = []core.Spec{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(specs)
}

func (s *DomainService) updateSpec(w http.ResponseWriter, r *http.Request, id string) {
	var spec core.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	spec.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && spec.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	updated, err := s.domainStore.UpdateSpec(spec)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(spec.Project, core.EventSpecUpdated, updated.ID, updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteSpec(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteSpec(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(project, core.EventSpecArchived, id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// Epic handlers

func (s *DomainService) handleEpics(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listEpics(w, r)
	case http.MethodPost:
		s.createEpic(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleEpicByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/epics/")
	id = strings.Trim(id, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getEpic(w, r, id)
	case http.MethodPut:
		s.updateEpic(w, r, id)
	case http.MethodDelete:
		s.deleteEpic(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createEpic(w http.ResponseWriter, r *http.Request) {
	var epic core.Epic
	if err := json.NewDecoder(r.Body).Decode(&epic); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && epic.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateEpic(epic)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(epic.Project, core.EventEpicCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getEpic(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	epic, err := s.domainStore.GetEpic(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(epic)
}

func (s *DomainService) listEpics(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	specID := r.URL.Query().Get("spec")
	epics, err := s.domainStore.ListEpics(project, specID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if epics == nil {
		epics = []core.Epic{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(epics)
}

func (s *DomainService) updateEpic(w http.ResponseWriter, r *http.Request, id string) {
	var epic core.Epic
	if err := json.NewDecoder(r.Body).Decode(&epic); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	epic.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && epic.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	updated, err := s.domainStore.UpdateEpic(epic)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(epic.Project, core.EventEpicUpdated, updated.ID, updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteEpic(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteEpic(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Story handlers

func (s *DomainService) handleStories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listStories(w, r)
	case http.MethodPost:
		s.createStory(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleStoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	id = strings.Trim(id, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getStory(w, r, id)
	case http.MethodPut:
		s.updateStory(w, r, id)
	case http.MethodDelete:
		s.deleteStory(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createStory(w http.ResponseWriter, r *http.Request) {
	var story core.Story
	if err := json.NewDecoder(r.Body).Decode(&story); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && story.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateStory(story)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(story.Project, core.EventStoryCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getStory(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	story, err := s.domainStore.GetStory(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(story)
}

func (s *DomainService) listStories(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	epicID := r.URL.Query().Get("epic")
	stories, err := s.domainStore.ListStories(project, epicID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if stories == nil {
		stories = []core.Story{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stories)
}

func (s *DomainService) updateStory(w http.ResponseWriter, r *http.Request, id string) {
	var story core.Story
	if err := json.NewDecoder(r.Body).Decode(&story); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	story.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && story.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	updated, err := s.domainStore.UpdateStory(story)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(story.Project, core.EventStoryUpdated, updated.ID, updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteStory(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteStory(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Task handlers

func (s *DomainService) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w, r)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id := parts[0]

	if len(parts) == 2 && parts[1] == "assign" {
		s.assignTask(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getTask(w, r, id)
	case http.MethodPut:
		s.updateTask(w, r, id)
	case http.MethodDelete:
		s.deleteTask(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createTask(w http.ResponseWriter, r *http.Request) {
	var task core.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && task.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateTask(task)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(task.Project, core.EventTaskCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getTask(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	task, err := s.domainStore.GetTask(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (s *DomainService) listTasks(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	status := r.URL.Query().Get("status")
	agent := r.URL.Query().Get("agent")
	tasks, err := s.domainStore.ListTasks(project, status, agent)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []core.Task{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *DomainService) updateTask(w http.ResponseWriter, r *http.Request, id string) {
	var task core.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	task.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && task.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	updated, err := s.domainStore.UpdateTask(task)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if updated.Status == core.TaskStatusDone {
		s.broadcastDomainEvent(task.Project, core.EventTaskCompleted, updated.ID, updated)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) assignTask(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	task, err := s.domainStore.GetTask(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	task.Agent = req.Agent
	task.Status = core.TaskStatusRunning
	updated, err := s.domainStore.UpdateTask(task)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(project, core.EventTaskAssigned, updated.ID, updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteTask(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteTask(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Insight handlers

func (s *DomainService) handleInsights(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listInsights(w, r)
	case http.MethodPost:
		s.createInsight(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleInsightByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/insights/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id := parts[0]

	if len(parts) == 2 && parts[1] == "link" {
		s.linkInsight(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getInsight(w, r, id)
	case http.MethodDelete:
		s.deleteInsight(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createInsight(w http.ResponseWriter, r *http.Request) {
	var insight core.Insight
	if err := json.NewDecoder(r.Body).Decode(&insight); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && insight.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateInsight(insight)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(insight.Project, core.EventInsightCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getInsight(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	insight, err := s.domainStore.GetInsight(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(insight)
}

func (s *DomainService) listInsights(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	specID := r.URL.Query().Get("spec")
	category := r.URL.Query().Get("category")
	insights, err := s.domainStore.ListInsights(project, specID, category)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if insights == nil {
		insights = []core.Insight{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(insights)
}

func (s *DomainService) linkInsight(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SpecID string `json:"spec_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.LinkInsightToSpec(project, id, req.SpecID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(project, core.EventInsightLinked, id, map[string]string{"spec_id": req.SpecID})
	w.WriteHeader(http.StatusOK)
}

func (s *DomainService) deleteInsight(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteInsight(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Session handlers

func (s *DomainService) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	id = strings.Trim(id, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, id)
	case http.MethodPut:
		s.updateSession(w, r, id)
	case http.MethodDelete:
		s.deleteSession(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createSession(w http.ResponseWriter, r *http.Request) {
	var session core.Session
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && session.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateSession(session)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(session.Project, core.EventSessionStarted, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getSession(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	session, err := s.domainStore.GetSession(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *DomainService) listSessions(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	status := r.URL.Query().Get("status")
	sessions, err := s.domainStore.ListSessions(project, status)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []core.Session{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (s *DomainService) updateSession(w http.ResponseWriter, r *http.Request, id string) {
	var session core.Session
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	session.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && session.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	updated, err := s.domainStore.UpdateSession(session)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteSession(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteSession(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(project, core.EventSessionStopped, id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// Helper to broadcast domain events
func (s *DomainService) broadcastDomainEvent(project string, eventType core.EventType, entityID string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Broadcast(project, "", map[string]any{
		"type":      string(eventType),
		"project":   project,
		"entity_id": entityID,
		"data":      data,
	})
}

// CUJ (Critical User Journey) handlers

func (s *DomainService) handleCUJs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listCUJs(w, r)
	case http.MethodPost:
		s.createCUJ(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) handleCUJByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/cujs/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Handle /api/cujs/{id}/link and /api/cujs/{id}/unlink
	if len(parts) >= 2 {
		switch parts[1] {
		case "link":
			s.linkCUJToFeature(w, r, id)
			return
		case "unlink":
			s.unlinkCUJFromFeature(w, r, id)
			return
		case "links":
			s.getCUJFeatureLinks(w, r, id)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		s.getCUJ(w, r, id)
	case http.MethodPut:
		s.updateCUJ(w, r, id)
	case http.MethodDelete:
		s.deleteCUJ(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *DomainService) createCUJ(w http.ResponseWriter, r *http.Request) {
	var cuj core.CriticalUserJourney
	if err := json.NewDecoder(r.Body).Decode(&cuj); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && cuj.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	created, err := s.domainStore.CreateCUJ(cuj)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(cuj.Project, core.EventCUJCreated, created.ID, created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (s *DomainService) getCUJ(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	cuj, err := s.domainStore.GetCUJ(project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cuj)
}

func (s *DomainService) listCUJs(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	specID := r.URL.Query().Get("spec")
	cujs, err := s.domainStore.ListCUJs(project, specID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cujs == nil {
		cujs = []core.CriticalUserJourney{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cujs)
}

func (s *DomainService) updateCUJ(w http.ResponseWriter, r *http.Request, id string) {
	var cuj core.CriticalUserJourney
	if err := json.NewDecoder(r.Body).Decode(&cuj); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cuj.ID = id
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey && cuj.Project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Determine which event to broadcast based on status change
	eventType := core.EventCUJUpdated
	if cuj.Status == core.CUJStatusValidated {
		eventType = core.EventCUJValidated
	}

	updated, err := s.domainStore.UpdateCUJ(cuj)
	if err != nil {
		if errors.Is(err, core.ErrConcurrentModification) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(cuj.Project, eventType, updated.ID, updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

func (s *DomainService) deleteCUJ(w http.ResponseWriter, r *http.Request, id string) {
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.DeleteCUJ(project, id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.broadcastDomainEvent(project, core.EventCUJArchived, id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *DomainService) linkCUJToFeature(w http.ResponseWriter, r *http.Request, cujID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FeatureID string `json:"feature_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.LinkCUJToFeature(project, cujID, req.FeatureID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *DomainService) unlinkCUJFromFeature(w http.ResponseWriter, r *http.Request, cujID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FeatureID string `json:"feature_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if err := s.domainStore.UnlinkCUJFromFeature(project, cujID, req.FeatureID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *DomainService) getCUJFeatureLinks(w http.ResponseWriter, r *http.Request, cujID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	info, _ := auth.FromContext(r.Context())
	project := info.Project
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	links, err := s.domainStore.GetCUJFeatureLinks(project, cujID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if links == nil {
		links = []core.CUJFeatureLink{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}
