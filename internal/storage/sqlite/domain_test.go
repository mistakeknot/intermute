package sqlite

import (
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestSpecCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create
	spec := core.Spec{
		Project: "test-project",
		Title:   "Test Spec",
		Vision:  "A great vision",
		Status:  core.SpecStatusDraft,
	}
	created, err := store.CreateSpec(spec)
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if created.ID == "" {
		t.Error("expected ID to be set")
	}
	if created.Title != "Test Spec" {
		t.Errorf("title = %q, want %q", created.Title, "Test Spec")
	}

	// Get
	fetched, err := store.GetSpec("test-project", created.ID)
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if fetched.Title != "Test Spec" {
		t.Errorf("title = %q, want %q", fetched.Title, "Test Spec")
	}

	// List
	specs, err := store.ListSpecs("test-project", "")
	if err != nil {
		t.Fatalf("ListSpecs: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("len(specs) = %d, want 1", len(specs))
	}

	// List with status filter
	specs, err = store.ListSpecs("test-project", "draft")
	if err != nil {
		t.Fatalf("ListSpecs with status: %v", err)
	}
	if len(specs) != 1 {
		t.Errorf("len(specs) = %d, want 1", len(specs))
	}
	specs, err = store.ListSpecs("test-project", "validated")
	if err != nil {
		t.Fatalf("ListSpecs with status: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("len(specs) = %d, want 0", len(specs))
	}

	// Update
	fetched.Vision = "Updated vision"
	fetched.Status = core.SpecStatusValidated
	updated, err := store.UpdateSpec(fetched)
	if err != nil {
		t.Fatalf("UpdateSpec: %v", err)
	}
	if updated.Vision != "Updated vision" {
		t.Errorf("vision = %q, want %q", updated.Vision, "Updated vision")
	}

	// Delete
	if err := store.DeleteSpec("test-project", created.ID); err != nil {
		t.Fatalf("DeleteSpec: %v", err)
	}
	specs, err = store.ListSpecs("test-project", "")
	if err != nil {
		t.Fatalf("ListSpecs after delete: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("len(specs) = %d, want 0 after delete", len(specs))
	}
}

func TestEpicCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create spec first
	spec, _ := store.CreateSpec(core.Spec{
		Project: "test-project",
		Title:   "Parent Spec",
	})

	// Create epic
	epic := core.Epic{
		Project:     "test-project",
		SpecID:      spec.ID,
		Title:       "Test Epic",
		Description: "Epic description",
		Status:      core.EpicStatusOpen,
	}
	created, err := store.CreateEpic(epic)
	if err != nil {
		t.Fatalf("CreateEpic: %v", err)
	}
	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	// List by spec
	epics, err := store.ListEpics("test-project", spec.ID)
	if err != nil {
		t.Fatalf("ListEpics: %v", err)
	}
	if len(epics) != 1 {
		t.Errorf("len(epics) = %d, want 1", len(epics))
	}

	// Update
	created.Status = core.EpicStatusInProgress
	updated, err := store.UpdateEpic(created)
	if err != nil {
		t.Fatalf("UpdateEpic: %v", err)
	}
	if updated.Status != core.EpicStatusInProgress {
		t.Errorf("status = %v, want %v", updated.Status, core.EpicStatusInProgress)
	}
}

func TestStoryCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create epic first
	epic, _ := store.CreateEpic(core.Epic{
		Project: "test-project",
		Title:   "Parent Epic",
	})

	// Create story
	story := core.Story{
		Project:            "test-project",
		EpicID:             epic.ID,
		Title:              "Test Story",
		AcceptanceCriteria: []string{"AC1", "AC2"},
		Status:             core.StoryStatusTodo,
	}
	created, err := store.CreateStory(story)
	if err != nil {
		t.Fatalf("CreateStory: %v", err)
	}
	if len(created.AcceptanceCriteria) != 2 {
		t.Errorf("len(ac) = %d, want 2", len(created.AcceptanceCriteria))
	}

	// Get and verify acceptance criteria persisted
	fetched, err := store.GetStory("test-project", created.ID)
	if err != nil {
		t.Fatalf("GetStory: %v", err)
	}
	if len(fetched.AcceptanceCriteria) != 2 {
		t.Errorf("len(ac) = %d, want 2", len(fetched.AcceptanceCriteria))
	}
}

func TestTaskCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create task
	task := core.Task{
		Project: "test-project",
		Title:   "Test Task",
		Status:  core.TaskStatusPending,
	}
	created, err := store.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Assign
	created.Agent = "claude"
	created.Status = core.TaskStatusRunning
	updated, err := store.UpdateTask(created)
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.Agent != "claude" {
		t.Errorf("agent = %q, want %q", updated.Agent, "claude")
	}

	// List by status
	tasks, err := store.ListTasks("test-project", "running", "")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("len(tasks) = %d, want 1", len(tasks))
	}

	// List by agent
	tasks, err = store.ListTasks("test-project", "", "claude")
	if err != nil {
		t.Fatalf("ListTasks by agent: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("len(tasks) = %d, want 1", len(tasks))
	}
}

func TestInsightCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create insight
	insight := core.Insight{
		Project:  "test-project",
		Source:   "github-scout",
		Category: "competitor",
		Title:    "Test Insight",
		Score:    0.85,
	}
	created, err := store.CreateInsight(insight)
	if err != nil {
		t.Fatalf("CreateInsight: %v", err)
	}

	// Link to spec
	spec, _ := store.CreateSpec(core.Spec{
		Project: "test-project",
		Title:   "Test Spec",
	})
	if err := store.LinkInsightToSpec("test-project", created.ID, spec.ID); err != nil {
		t.Fatalf("LinkInsightToSpec: %v", err)
	}

	// List by spec
	insights, err := store.ListInsights("test-project", spec.ID, "")
	if err != nil {
		t.Fatalf("ListInsights: %v", err)
	}
	if len(insights) != 1 {
		t.Errorf("len(insights) = %d, want 1", len(insights))
	}

	// List by category
	insights, err = store.ListInsights("test-project", "", "competitor")
	if err != nil {
		t.Fatalf("ListInsights by category: %v", err)
	}
	if len(insights) != 1 {
		t.Errorf("len(insights) = %d, want 1", len(insights))
	}
}

func TestSessionCRUD(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create session
	session := core.Session{
		Project: "test-project",
		Name:    "claude-main",
		Agent:   "claude",
		Status:  core.SessionStatusRunning,
	}
	created, err := store.CreateSession(session)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	// List by status
	sessions, err := store.ListSessions("test-project", "running")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("len(sessions) = %d, want 1", len(sessions))
	}

	// Update status
	created.Status = core.SessionStatusIdle
	updated, err := store.UpdateSession(created)
	if err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	if updated.Status != core.SessionStatusIdle {
		t.Errorf("status = %v, want %v", updated.Status, core.SessionStatusIdle)
	}

	// Delete
	if err := store.DeleteSession("test-project", created.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
}
