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

// Optimistic locking tests

func TestSpecOptimisticLocking(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create spec
	spec := core.Spec{
		Project: "test-project",
		Title:   "Test Spec",
	}
	created, err := store.CreateSpec(spec)
	if err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// First update should succeed
	created.Title = "Updated Title"
	updated, err := store.UpdateSpec(created)
	if err != nil {
		t.Fatalf("UpdateSpec: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Second update with stale version should fail
	created.Title = "Stale Update"
	_, err = store.UpdateSpec(created) // created still has version=1
	if err != core.ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}

	// Update with correct version should succeed
	updated.Title = "Final Title"
	final, err := store.UpdateSpec(updated)
	if err != nil {
		t.Fatalf("UpdateSpec with correct version: %v", err)
	}
	if final.Version != 3 {
		t.Errorf("version = %d, want 3", final.Version)
	}
}

func TestEpicOptimisticLocking(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	epic := core.Epic{
		Project: "test-project",
		Title:   "Test Epic",
	}
	created, err := store.CreateEpic(epic)
	if err != nil {
		t.Fatalf("CreateEpic: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// Successful update
	created.Title = "Updated Epic"
	updated, err := store.UpdateEpic(created)
	if err != nil {
		t.Fatalf("UpdateEpic: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Stale update should fail
	created.Title = "Stale Epic"
	_, err = store.UpdateEpic(created)
	if err != core.ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestStoryOptimisticLocking(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	epic, _ := store.CreateEpic(core.Epic{
		Project: "test-project",
		Title:   "Parent Epic",
	})

	story := core.Story{
		Project: "test-project",
		EpicID:  epic.ID,
		Title:   "Test Story",
	}
	created, err := store.CreateStory(story)
	if err != nil {
		t.Fatalf("CreateStory: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// Successful update
	created.Title = "Updated Story"
	updated, err := store.UpdateStory(created)
	if err != nil {
		t.Fatalf("UpdateStory: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Stale update should fail
	created.Title = "Stale Story"
	_, err = store.UpdateStory(created)
	if err != core.ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestTaskOptimisticLocking(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	task := core.Task{
		Project: "test-project",
		Title:   "Test Task",
	}
	created, err := store.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// Successful update
	created.Agent = "claude"
	updated, err := store.UpdateTask(created)
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Stale update should fail
	created.Agent = "codex"
	_, err = store.UpdateTask(created)
	if err != core.ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestCUJOptimisticLocking(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	spec, _ := store.CreateSpec(core.Spec{
		Project: "test-project",
		Title:   "Test Spec",
	})

	cuj := core.CriticalUserJourney{
		Project: "test-project",
		SpecID:  spec.ID,
		Title:   "Test CUJ",
	}
	created, err := store.CreateCUJ(cuj)
	if err != nil {
		t.Fatalf("CreateCUJ: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("version = %d, want 1", created.Version)
	}

	// Successful update
	created.Title = "Updated CUJ"
	updated, err := store.UpdateCUJ(created)
	if err != nil {
		t.Fatalf("UpdateCUJ: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Stale update should fail
	created.Title = "Stale CUJ"
	_, err = store.UpdateCUJ(created)
	if err != core.ErrConcurrentModification {
		t.Errorf("expected ErrConcurrentModification, got %v", err)
	}
}

func TestVersionPersistedInGet(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create and update spec
	spec, _ := store.CreateSpec(core.Spec{
		Project: "test-project",
		Title:   "Test Spec",
	})
	spec.Title = "Updated"
	spec, _ = store.UpdateSpec(spec)
	spec.Title = "Updated Again"
	spec, _ = store.UpdateSpec(spec)

	// Fetch and verify version is persisted
	fetched, err := store.GetSpec("test-project", spec.ID)
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if fetched.Version != 3 {
		t.Errorf("fetched version = %d, want 3", fetched.Version)
	}
}

func TestVersionPersistedInList(t *testing.T) {
	store, err := NewInMemory()
	if err != nil {
		t.Fatal(err)
	}

	// Create and update epic
	epic, _ := store.CreateEpic(core.Epic{
		Project: "test-project",
		Title:   "Test Epic",
	})
	epic.Title = "Updated"
	epic, _ = store.UpdateEpic(epic)

	// List and verify version is persisted
	epics, err := store.ListEpics("test-project", "")
	if err != nil {
		t.Fatalf("ListEpics: %v", err)
	}
	if len(epics) != 1 {
		t.Fatalf("len(epics) = %d, want 1", len(epics))
	}
	if epics[0].Version != 2 {
		t.Errorf("listed version = %d, want 2", epics[0].Version)
	}
}
