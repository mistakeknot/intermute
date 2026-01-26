package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientCreateSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/specs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var spec Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if spec.Title != "Test Spec" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		spec.ID = "spec-1"
		spec.CreatedAt = time.Now()
		spec.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(spec)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	spec, err := c.CreateSpec(ctx, Spec{
		Title:   "Test Spec",
		Vision:  "Test vision",
		Problem: "Test problem",
		Status:  SpecStatusDraft,
	})
	if err != nil {
		t.Fatalf("create spec failed: %v", err)
	}
	if spec.ID != "spec-1" {
		t.Fatalf("expected spec-1, got %s", spec.ID)
	}
}

func TestClientGetSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/specs/spec-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Spec{
			ID:      "spec-1",
			Project: "proj-a",
			Title:   "Test Spec",
			Status:  SpecStatusDraft,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	spec, err := c.GetSpec(ctx, "spec-1")
	if err != nil {
		t.Fatalf("get spec failed: %v", err)
	}
	if spec.Title != "Test Spec" {
		t.Fatalf("expected 'Test Spec', got '%s'", spec.Title)
	}
}

func TestClientListSpecs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/specs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("project") != "proj-a" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Spec{
			{ID: "spec-1", Project: "proj-a", Title: "Spec 1"},
			{ID: "spec-2", Project: "proj-a", Title: "Spec 2"},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	specs, err := c.ListSpecs(ctx, "")
	if err != nil {
		t.Fatalf("list specs failed: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
}

func TestClientUpdateSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/specs/spec-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var spec Spec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		spec.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spec)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	updated, err := c.UpdateSpec(ctx, Spec{
		ID:     "spec-1",
		Title:  "Updated Title",
		Status: SpecStatusValidated,
	})
	if err != nil {
		t.Fatalf("update spec failed: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Fatalf("expected 'Updated Title', got '%s'", updated.Title)
	}
}

func TestClientDeleteSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/specs/spec-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := c.DeleteSpec(ctx, "spec-1")
	if err != nil {
		t.Fatalf("delete spec failed: %v", err)
	}
}

func TestClientCreateEpic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/epics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var epic Epic
		if err := json.NewDecoder(r.Body).Decode(&epic); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		epic.ID = "epic-1"
		epic.CreatedAt = time.Now()
		epic.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(epic)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	epic, err := c.CreateEpic(ctx, Epic{
		SpecID:      "spec-1",
		Title:       "Test Epic",
		Description: "Epic description",
		Status:      EpicStatusOpen,
	})
	if err != nil {
		t.Fatalf("create epic failed: %v", err)
	}
	if epic.ID != "epic-1" {
		t.Fatalf("expected epic-1, got %s", epic.ID)
	}
}

func TestClientCreateStory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stories" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var story Story
		if err := json.NewDecoder(r.Body).Decode(&story); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		story.ID = "story-1"
		story.CreatedAt = time.Now()
		story.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(story)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	story, err := c.CreateStory(ctx, Story{
		EpicID:             "epic-1",
		Title:              "Test Story",
		AcceptanceCriteria: []string{"AC1", "AC2"},
		Status:             StoryStatusTodo,
	})
	if err != nil {
		t.Fatalf("create story failed: %v", err)
	}
	if story.ID != "story-1" {
		t.Fatalf("expected story-1, got %s", story.ID)
	}
}

func TestClientCreateTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var task Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		task.ID = "task-1"
		task.CreatedAt = time.Now()
		task.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	task, err := c.CreateTask(ctx, Task{
		StoryID: "story-1",
		Title:   "Test Task",
		Status:  TaskStatusPending,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if task.ID != "task-1" {
		t.Fatalf("expected task-1, got %s", task.ID)
	}
}

func TestClientAssignTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tasks/task-1/assign" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Task{
			ID:     "task-1",
			Agent:  req.Agent,
			Status: TaskStatusRunning,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	task, err := c.AssignTask(ctx, "task-1", "claude-agent")
	if err != nil {
		t.Fatalf("assign task failed: %v", err)
	}
	if task.Agent != "claude-agent" {
		t.Fatalf("expected claude-agent, got %s", task.Agent)
	}
	if task.Status != TaskStatusRunning {
		t.Fatalf("expected running status, got %s", task.Status)
	}
}

func TestClientCreateInsight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/insights" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var insight Insight
		if err := json.NewDecoder(r.Body).Decode(&insight); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		insight.ID = "insight-1"
		insight.CreatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(insight)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	insight, err := c.CreateInsight(ctx, Insight{
		Source:   "github-scout",
		Category: "competitor",
		Title:    "Test Insight",
		Body:     "Insight body",
		Score:    0.85,
	})
	if err != nil {
		t.Fatalf("create insight failed: %v", err)
	}
	if insight.ID != "insight-1" {
		t.Fatalf("expected insight-1, got %s", insight.ID)
	}
}

func TestClientLinkInsightToSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/insights/insight-1/link" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			SpecID string `json:"spec_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SpecID != "spec-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := c.LinkInsightToSpec(ctx, "insight-1", "spec-1")
	if err != nil {
		t.Fatalf("link insight failed: %v", err)
	}
}

func TestClientConflictError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.UpdateSpec(ctx, Spec{ID: "spec-1", Title: "Update"})
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}
