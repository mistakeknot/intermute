package httpapi

import (
	"net/http"
	"testing"
)

// --- Spec CRUD ---

func TestSpecHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	var specID string
	var version int64

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/specs", map[string]any{
			"project": project,
			"title":   "Test Spec",
			"vision":  "A test vision",
			"status":  "draft",
		})
		requireStatus(t, resp, http.StatusCreated)
		spec := decodeJSON[map[string]any](t, resp)
		specID = spec["id"].(string)
		if specID == "" {
			t.Fatal("expected non-empty id")
		}
		if spec["title"] != "Test Spec" {
			t.Fatalf("expected title 'Test Spec', got %v", spec["title"])
		}
		version = int64(spec["version"].(float64))
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/specs/"+specID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		spec := decodeJSON[map[string]any](t, resp)
		if spec["id"] != specID {
			t.Fatalf("expected id %s, got %v", specID, spec["id"])
		}
	})

	t.Run("get not found", func(t *testing.T) {
		resp := env.get(t, "/api/specs/nonexistent?project="+project)
		requireStatus(t, resp, http.StatusNotFound)
		resp.Body.Close()
	})

	t.Run("list all", func(t *testing.T) {
		resp := env.get(t, "/api/specs?project="+project)
		requireStatus(t, resp, http.StatusOK)
		specs := decodeJSON[[]map[string]any](t, resp)
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
	})

	t.Run("list by status", func(t *testing.T) {
		resp := env.get(t, "/api/specs?project="+project+"&status=draft")
		requireStatus(t, resp, http.StatusOK)
		specs := decodeJSON[[]map[string]any](t, resp)
		if len(specs) != 1 {
			t.Fatalf("expected 1 draft spec, got %d", len(specs))
		}

		resp2 := env.get(t, "/api/specs?project="+project+"&status=validated")
		requireStatus(t, resp2, http.StatusOK)
		specs2 := decodeJSON[[]map[string]any](t, resp2)
		if len(specs2) != 0 {
			t.Fatalf("expected 0 validated specs, got %d", len(specs2))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/specs/"+specID, map[string]any{
			"project": project,
			"title":   "Updated Spec",
			"status":  "research",
			"version": version,
		})
		requireStatus(t, resp, http.StatusOK)
		spec := decodeJSON[map[string]any](t, resp)
		if spec["title"] != "Updated Spec" {
			t.Fatalf("expected updated title, got %v", spec["title"])
		}
		version = int64(spec["version"].(float64))
	})

	t.Run("version conflict", func(t *testing.T) {
		// Use stale version (version-1) to trigger 409
		resp := env.put(t, "/api/specs/"+specID, map[string]any{
			"project": project,
			"title":   "Conflict",
			"version": version - 1,
		})
		requireStatus(t, resp, http.StatusConflict)
		resp.Body.Close()
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/specs/"+specID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()

		// Verify it's gone
		resp2 := env.get(t, "/api/specs/"+specID+"?project="+project)
		requireStatus(t, resp2, http.StatusNotFound)
		resp2.Body.Close()
	})
}

// --- Epic CRUD ---

func TestEpicHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	// Create a spec first (epics reference specs)
	specResp := env.post(t, "/api/specs", map[string]any{
		"project": project, "title": "Parent Spec", "status": "draft",
	})
	requireStatus(t, specResp, http.StatusCreated)
	specData := decodeJSON[map[string]any](t, specResp)
	specID := specData["id"].(string)

	var epicID string
	var version int64

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/epics", map[string]any{
			"project":     project,
			"spec_id":     specID,
			"title":       "Test Epic",
			"description": "An epic description",
			"status":      "open",
		})
		requireStatus(t, resp, http.StatusCreated)
		epic := decodeJSON[map[string]any](t, resp)
		epicID = epic["id"].(string)
		if epicID == "" {
			t.Fatal("expected non-empty id")
		}
		version = int64(epic["version"].(float64))
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/epics/"+epicID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		epic := decodeJSON[map[string]any](t, resp)
		if epic["title"] != "Test Epic" {
			t.Fatalf("expected 'Test Epic', got %v", epic["title"])
		}
	})

	t.Run("list by spec", func(t *testing.T) {
		resp := env.get(t, "/api/epics?project="+project+"&spec="+specID)
		requireStatus(t, resp, http.StatusOK)
		epics := decodeJSON[[]map[string]any](t, resp)
		if len(epics) != 1 {
			t.Fatalf("expected 1 epic, got %d", len(epics))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/epics/"+epicID, map[string]any{
			"project": project,
			"title":   "Updated Epic",
			"status":  "in_progress",
			"version": version,
		})
		requireStatus(t, resp, http.StatusOK)
		epic := decodeJSON[map[string]any](t, resp)
		version = int64(epic["version"].(float64))
	})

	t.Run("version conflict", func(t *testing.T) {
		resp := env.put(t, "/api/epics/"+epicID, map[string]any{
			"project": project,
			"title":   "Conflict",
			"version": version - 1,
		})
		requireStatus(t, resp, http.StatusConflict)
		resp.Body.Close()
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/epics/"+epicID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- Story CRUD ---

func TestStoryHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	// Create parent spec + epic
	specResp := env.post(t, "/api/specs", map[string]any{
		"project": project, "title": "Spec", "status": "draft",
	})
	specData := decodeJSON[map[string]any](t, specResp)
	epicResp := env.post(t, "/api/epics", map[string]any{
		"project": project, "spec_id": specData["id"], "title": "Epic", "status": "open",
	})
	epicData := decodeJSON[map[string]any](t, epicResp)
	epicID := epicData["id"].(string)

	var storyID string
	var version int64

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/stories", map[string]any{
			"project":             project,
			"epic_id":             epicID,
			"title":               "Test Story",
			"acceptance_criteria": []string{"criterion 1", "criterion 2"},
			"status":              "todo",
		})
		requireStatus(t, resp, http.StatusCreated)
		story := decodeJSON[map[string]any](t, resp)
		storyID = story["id"].(string)
		version = int64(story["version"].(float64))
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/stories/"+storyID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		story := decodeJSON[map[string]any](t, resp)
		if story["title"] != "Test Story" {
			t.Fatalf("expected 'Test Story', got %v", story["title"])
		}
	})

	t.Run("list by epic", func(t *testing.T) {
		resp := env.get(t, "/api/stories?project="+project+"&epic="+epicID)
		requireStatus(t, resp, http.StatusOK)
		stories := decodeJSON[[]map[string]any](t, resp)
		if len(stories) != 1 {
			t.Fatalf("expected 1 story, got %d", len(stories))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/stories/"+storyID, map[string]any{
			"project": project,
			"epic_id": epicID,
			"title":   "Updated Story",
			"status":  "in_progress",
			"version": version,
		})
		requireStatus(t, resp, http.StatusOK)
		story := decodeJSON[map[string]any](t, resp)
		version = int64(story["version"].(float64))
	})

	t.Run("version conflict", func(t *testing.T) {
		resp := env.put(t, "/api/stories/"+storyID, map[string]any{
			"project": project,
			"title":   "Conflict",
			"version": version - 1,
		})
		requireStatus(t, resp, http.StatusConflict)
		resp.Body.Close()
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/stories/"+storyID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- Task CRUD ---

func TestTaskHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	var taskID string
	var version int64

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/tasks", map[string]any{
			"project": project,
			"title":   "Test Task",
			"status":  "pending",
		})
		requireStatus(t, resp, http.StatusCreated)
		task := decodeJSON[map[string]any](t, resp)
		taskID = task["id"].(string)
		version = int64(task["version"].(float64))
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/tasks/"+taskID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		task := decodeJSON[map[string]any](t, resp)
		if task["title"] != "Test Task" {
			t.Fatalf("expected 'Test Task', got %v", task["title"])
		}
	})

	t.Run("list by status", func(t *testing.T) {
		resp := env.get(t, "/api/tasks?project="+project+"&status=pending")
		requireStatus(t, resp, http.StatusOK)
		tasks := decodeJSON[[]map[string]any](t, resp)
		if len(tasks) != 1 {
			t.Fatalf("expected 1 pending task, got %d", len(tasks))
		}
	})

	t.Run("assign", func(t *testing.T) {
		resp := env.post(t, "/api/tasks/"+taskID+"/assign?project="+project, map[string]any{
			"agent": "agent-x",
		})
		requireStatus(t, resp, http.StatusOK)
		task := decodeJSON[map[string]any](t, resp)
		if task["agent"] != "agent-x" {
			t.Fatalf("expected agent 'agent-x', got %v", task["agent"])
		}
		if task["status"] != "running" {
			t.Fatalf("expected status 'running', got %v", task["status"])
		}
		version = int64(task["version"].(float64))
	})

	t.Run("list by agent", func(t *testing.T) {
		resp := env.get(t, "/api/tasks?project="+project+"&agent=agent-x")
		requireStatus(t, resp, http.StatusOK)
		tasks := decodeJSON[[]map[string]any](t, resp)
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task for agent-x, got %d", len(tasks))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/tasks/"+taskID, map[string]any{
			"project": project,
			"title":   "Updated Task",
			"status":  "done",
			"version": version,
		})
		requireStatus(t, resp, http.StatusOK)
		task := decodeJSON[map[string]any](t, resp)
		version = int64(task["version"].(float64))
	})

	t.Run("version conflict", func(t *testing.T) {
		resp := env.put(t, "/api/tasks/"+taskID, map[string]any{
			"project": project,
			"title":   "Conflict",
			"version": version - 1,
		})
		requireStatus(t, resp, http.StatusConflict)
		resp.Body.Close()
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/tasks/"+taskID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- Insight CRUD ---

func TestInsightHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	// Create a spec for linking
	specResp := env.post(t, "/api/specs", map[string]any{
		"project": project, "title": "Spec for Insights", "status": "draft",
	})
	specData := decodeJSON[map[string]any](t, specResp)
	specID := specData["id"].(string)

	var insightID string

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/insights", map[string]any{
			"project":  project,
			"source":   "user-interview",
			"category": "usability",
			"title":    "Users find nav confusing",
			"body":     "Multiple users reported difficulty...",
			"score":    0.85,
		})
		requireStatus(t, resp, http.StatusCreated)
		insight := decodeJSON[map[string]any](t, resp)
		insightID = insight["id"].(string)
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/insights/"+insightID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		insight := decodeJSON[map[string]any](t, resp)
		if insight["title"] != "Users find nav confusing" {
			t.Fatalf("wrong title: %v", insight["title"])
		}
	})

	t.Run("list by category", func(t *testing.T) {
		resp := env.get(t, "/api/insights?project="+project+"&category=usability")
		requireStatus(t, resp, http.StatusOK)
		insights := decodeJSON[[]map[string]any](t, resp)
		if len(insights) != 1 {
			t.Fatalf("expected 1 insight, got %d", len(insights))
		}
	})

	t.Run("link to spec", func(t *testing.T) {
		resp := env.post(t, "/api/insights/"+insightID+"/link?project="+project, map[string]any{
			"spec_id": specID,
		})
		requireStatus(t, resp, http.StatusOK)
		resp.Body.Close()

		// Now list by spec
		resp2 := env.get(t, "/api/insights?project="+project+"&spec="+specID)
		requireStatus(t, resp2, http.StatusOK)
		insights := decodeJSON[[]map[string]any](t, resp2)
		if len(insights) != 1 {
			t.Fatalf("expected 1 insight linked to spec, got %d", len(insights))
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/insights/"+insightID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- Session CRUD ---

func TestSessionHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	var sessionID string

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/sessions", map[string]any{
			"project": project,
			"name":    "tmux-main",
			"agent":   "agent-a",
			"status":  "running",
		})
		requireStatus(t, resp, http.StatusCreated)
		session := decodeJSON[map[string]any](t, resp)
		sessionID = session["id"].(string)
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/sessions/"+sessionID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		session := decodeJSON[map[string]any](t, resp)
		if session["name"] != "tmux-main" {
			t.Fatalf("expected 'tmux-main', got %v", session["name"])
		}
	})

	t.Run("list by status", func(t *testing.T) {
		resp := env.get(t, "/api/sessions?project="+project+"&status=running")
		requireStatus(t, resp, http.StatusOK)
		sessions := decodeJSON[[]map[string]any](t, resp)
		if len(sessions) != 1 {
			t.Fatalf("expected 1 running session, got %d", len(sessions))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/sessions/"+sessionID, map[string]any{
			"project": project,
			"name":    "tmux-main",
			"agent":   "agent-a",
			"status":  "idle",
		})
		requireStatus(t, resp, http.StatusOK)
		session := decodeJSON[map[string]any](t, resp)
		if session["status"] != "idle" {
			t.Fatalf("expected 'idle', got %v", session["status"])
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/sessions/"+sessionID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- CUJ (Critical User Journey) CRUD ---

func TestCUJHTTP(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj-test"

	// Create parent spec
	specResp := env.post(t, "/api/specs", map[string]any{
		"project": project, "title": "CUJ Spec", "status": "draft",
	})
	specData := decodeJSON[map[string]any](t, specResp)
	specID := specData["id"].(string)

	var cujID string
	var version int64

	t.Run("create", func(t *testing.T) {
		resp := env.post(t, "/api/cujs", map[string]any{
			"project":    project,
			"spec_id":    specID,
			"title":      "Onboarding Flow",
			"persona":    "new-user",
			"priority":   "high",
			"entry_point": "landing page",
			"exit_point":  "dashboard",
			"steps": []map[string]any{
				{"order": 1, "action": "Click sign up", "expected": "See form"},
				{"order": 2, "action": "Fill form", "expected": "Account created"},
			},
			"success_criteria": []string{"Under 2 minutes"},
			"status":           "draft",
		})
		requireStatus(t, resp, http.StatusCreated)
		cuj := decodeJSON[map[string]any](t, resp)
		cujID = cuj["id"].(string)
		version = int64(cuj["version"].(float64))
	})

	t.Run("get", func(t *testing.T) {
		resp := env.get(t, "/api/cujs/"+cujID+"?project="+project)
		requireStatus(t, resp, http.StatusOK)
		cuj := decodeJSON[map[string]any](t, resp)
		if cuj["title"] != "Onboarding Flow" {
			t.Fatalf("expected 'Onboarding Flow', got %v", cuj["title"])
		}
	})

	t.Run("list", func(t *testing.T) {
		resp := env.get(t, "/api/cujs?project="+project)
		requireStatus(t, resp, http.StatusOK)
		cujs := decodeJSON[[]map[string]any](t, resp)
		if len(cujs) != 1 {
			t.Fatalf("expected 1 CUJ, got %d", len(cujs))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := env.put(t, "/api/cujs/"+cujID, map[string]any{
			"project":  project,
			"spec_id":  specID,
			"title":    "Updated Onboarding",
			"priority": "medium",
			"status":   "validated",
			"version":  version,
		})
		requireStatus(t, resp, http.StatusOK)
		cuj := decodeJSON[map[string]any](t, resp)
		version = int64(cuj["version"].(float64))
	})

	t.Run("version conflict", func(t *testing.T) {
		resp := env.put(t, "/api/cujs/"+cujID, map[string]any{
			"project": project,
			"title":   "Conflict",
			"version": version - 1,
		})
		requireStatus(t, resp, http.StatusConflict)
		resp.Body.Close()
	})

	t.Run("link feature", func(t *testing.T) {
		resp := env.post(t, "/api/cujs/"+cujID+"/link?project="+project, map[string]any{
			"feature_id": "feat-auth",
		})
		requireStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	})

	t.Run("get links", func(t *testing.T) {
		resp := env.get(t, "/api/cujs/"+cujID+"/links?project="+project)
		requireStatus(t, resp, http.StatusOK)
		links := decodeJSON[[]map[string]any](t, resp)
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(links))
		}
		if links[0]["feature_id"] != "feat-auth" {
			t.Fatalf("expected feature_id 'feat-auth', got %v", links[0]["feature_id"])
		}
	})

	t.Run("unlink feature", func(t *testing.T) {
		resp := env.post(t, "/api/cujs/"+cujID+"/unlink?project="+project, map[string]any{
			"feature_id": "feat-auth",
		})
		requireStatus(t, resp, http.StatusOK)
		resp.Body.Close()

		// Verify unlinked
		resp2 := env.get(t, "/api/cujs/"+cujID+"/links?project="+project)
		links := decodeJSON[[]map[string]any](t, resp2)
		if len(links) != 0 {
			t.Fatalf("expected 0 links after unlink, got %d", len(links))
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := env.delete(t, "/api/cujs/"+cujID+"?project="+project)
		requireStatus(t, resp, http.StatusNoContent)
		resp.Body.Close()
	})
}

// --- Method not allowed tests (covers all domain collection endpoints) ---

func TestDomainMethodNotAllowed(t *testing.T) {
	env := newTestEnv(t)

	endpoints := []string{"/api/specs", "/api/epics", "/api/stories", "/api/tasks", "/api/insights", "/api/sessions", "/api/cujs"}
	for _, ep := range endpoints {
		t.Run("DELETE "+ep, func(t *testing.T) {
			resp := env.delete(t, ep)
			requireStatus(t, resp, http.StatusMethodNotAllowed)
			resp.Body.Close()
		})
	}
}
