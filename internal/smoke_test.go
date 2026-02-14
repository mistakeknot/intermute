package internal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"github.com/mistakeknot/intermute/internal/ws"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func getJSON(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func putJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

// TestSmokeMessageFlow exercises the full lifecycle:
// register agent → connect WS → send message → verify WS event → fetch inbox → mark read → verify counts
func TestSmokeMessageFlow(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := ws.NewHub()
	svc := httpapi.NewDomainService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewDomainRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	const project = "smoke-proj"

	// 1. Register agent
	regResp := postJSON(t, srv.URL+"/api/agents", map[string]any{
		"id": "bob", "name": "Bob Agent", "project": project,
	})
	if regResp.StatusCode != http.StatusOK {
		t.Fatalf("register: %d", regResp.StatusCode)
	}
	regResp.Body.Close()

	// 2. Connect WebSocket for bob
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/bob?project=" + project
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// 3. Send message via HTTP
	sendResp := postJSON(t, srv.URL+"/api/messages", map[string]any{
		"project": project, "from": "alice", "to": []string{"bob"}, "body": "smoke test",
	})
	if sendResp.StatusCode != http.StatusOK {
		t.Fatalf("send: %d", sendResp.StatusCode)
	}
	sendData := decode[map[string]any](t, sendResp)
	msgID := sendData["message_id"].(string)

	// 4. Verify WS event
	var event map[string]any
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if event["type"] != "message.created" {
		t.Fatalf("expected message.created, got %v", event["type"])
	}

	// 5. Fetch inbox
	inboxResp := getJSON(t, srv.URL+"/api/inbox/bob?project="+project)
	if inboxResp.StatusCode != http.StatusOK {
		t.Fatalf("inbox: %d", inboxResp.StatusCode)
	}
	inbox := decode[map[string]any](t, inboxResp)
	messages := inbox["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(messages))
	}
	if messages[0].(map[string]any)["body"] != "smoke test" {
		t.Fatalf("wrong body: %v", messages[0].(map[string]any)["body"])
	}

	// 6. Mark read
	readResp := postJSON(t, srv.URL+"/api/messages/"+msgID+"/read?project="+project, map[string]any{
		"agent": "bob",
	})
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("mark read: %d", readResp.StatusCode)
	}
	readResp.Body.Close()

	// 7. Verify counts updated
	countsResp := getJSON(t, srv.URL+"/api/inbox/bob/counts?project="+project)
	if countsResp.StatusCode != http.StatusOK {
		t.Fatalf("counts: %d", countsResp.StatusCode)
	}
	counts := decode[map[string]any](t, countsResp)
	if int(counts["total"].(float64)) != 1 {
		t.Fatalf("expected total=1, got %v", counts["total"])
	}
	if int(counts["unread"].(float64)) != 0 {
		t.Fatalf("expected unread=0, got %v", counts["unread"])
	}
}

// TestSmokeDomainFlow exercises: create spec → epic → story → task → assign → list filters
func TestSmokeDomainFlow(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := httpapi.NewDomainService(st)
	srv := httptest.NewServer(httpapi.NewDomainRouter(svc, nil, nil))
	defer srv.Close()

	const project = "smoke-proj"

	// Create spec
	specResp := postJSON(t, srv.URL+"/api/specs", map[string]any{
		"project": project, "title": "Smoke Spec", "status": "draft",
	})
	if specResp.StatusCode != http.StatusCreated {
		t.Fatalf("create spec: %d", specResp.StatusCode)
	}
	spec := decode[map[string]any](t, specResp)
	specID := spec["id"].(string)

	// Create epic under spec
	epicResp := postJSON(t, srv.URL+"/api/epics", map[string]any{
		"project": project, "spec_id": specID, "title": "Smoke Epic", "status": "open",
	})
	if epicResp.StatusCode != http.StatusCreated {
		t.Fatalf("create epic: %d", epicResp.StatusCode)
	}
	epic := decode[map[string]any](t, epicResp)
	epicID := epic["id"].(string)

	// Create story under epic
	storyResp := postJSON(t, srv.URL+"/api/stories", map[string]any{
		"project": project, "epic_id": epicID, "title": "Smoke Story", "status": "todo",
	})
	if storyResp.StatusCode != http.StatusCreated {
		t.Fatalf("create story: %d", storyResp.StatusCode)
	}
	story := decode[map[string]any](t, storyResp)
	storyID := story["id"].(string)

	// Create task
	taskResp := postJSON(t, srv.URL+"/api/tasks", map[string]any{
		"project": project, "story_id": storyID, "title": "Smoke Task", "status": "pending",
	})
	if taskResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task: %d", taskResp.StatusCode)
	}
	task := decode[map[string]any](t, taskResp)
	taskID := task["id"].(string)

	// Assign task
	assignResp := postJSON(t, srv.URL+"/api/tasks/"+taskID+"/assign?project="+project, map[string]any{
		"agent": "builder-1",
	})
	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("assign task: %d", assignResp.StatusCode)
	}
	assigned := decode[map[string]any](t, assignResp)
	taskVersion := assigned["version"]

	// Verify list filters work
	t.Run("list specs", func(t *testing.T) {
		resp := getJSON(t, srv.URL+"/api/specs?project="+project)
		specs := decode[[]map[string]any](t, resp)
		if len(specs) != 1 || specs[0]["id"] != specID {
			t.Fatalf("unexpected specs: %v", specs)
		}
	})

	t.Run("list epics by spec", func(t *testing.T) {
		resp := getJSON(t, srv.URL+"/api/epics?project="+project+"&spec="+specID)
		epics := decode[[]map[string]any](t, resp)
		if len(epics) != 1 || epics[0]["id"] != epicID {
			t.Fatalf("unexpected epics: %v", epics)
		}
	})

	t.Run("list tasks by agent", func(t *testing.T) {
		resp := getJSON(t, srv.URL+"/api/tasks?project="+project+"&agent=builder-1")
		tasks := decode[[]map[string]any](t, resp)
		if len(tasks) != 1 || tasks[0]["agent"] != "builder-1" {
			t.Fatalf("unexpected tasks: %v", tasks)
		}
	})

	t.Run("list tasks by status", func(t *testing.T) {
		resp := getJSON(t, srv.URL+"/api/tasks?project="+project+"&status=running")
		tasks := decode[[]map[string]any](t, resp)
		if len(tasks) != 1 {
			t.Fatalf("expected 1 running task, got %d", len(tasks))
		}
	})

	// Complete the task (use version from assign, not from create)
	completeResp := putJSON(t, srv.URL+"/api/tasks/"+taskID, map[string]any{
		"project": project, "title": "Smoke Task", "status": "done",
		"version": taskVersion,
	})
	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("complete task: %d", completeResp.StatusCode)
	}
	completeResp.Body.Close()
}

// TestSmokeReservationFlow exercises: reserve → verify active → overlapping fails → release → verify released
func TestSmokeReservationFlow(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := httpapi.NewService(st)
	// Use NewRouter which includes reservation endpoints
	srv := httptest.NewServer(httpapi.NewRouter(svc, nil, nil))
	defer srv.Close()

	const project = "smoke-proj"

	// Register agent first
	regResp := postJSON(t, srv.URL+"/api/agents", map[string]any{
		"id": "agent-a", "name": "Agent A", "project": project,
	})
	if regResp.StatusCode != http.StatusOK {
		t.Fatalf("register: %d", regResp.StatusCode)
	}
	regResp.Body.Close()

	// Reserve a file pattern
	resResp := postJSON(t, srv.URL+"/api/reservations", map[string]any{
		"agent_id":     "agent-a",
		"project":      project,
		"path_pattern": "cmd/intermute/*.go",
		"exclusive":    true,
		"reason":       "refactoring main",
		"ttl_minutes":  5,
	})
	if resResp.StatusCode != http.StatusOK {
		t.Fatalf("reserve: %d", resResp.StatusCode)
	}
	reservation := decode[map[string]any](t, resResp)
	resID := reservation["id"].(string)
	if reservation["is_active"] != true {
		t.Fatal("expected reservation to be active")
	}

	// Verify it appears in active list
	activeResp := getJSON(t, srv.URL+"/api/reservations?project="+project)
	if activeResp.StatusCode != http.StatusOK {
		t.Fatalf("list active: %d", activeResp.StatusCode)
	}
	active := decode[map[string]any](t, activeResp)
	reservations := active["reservations"].([]any)
	if len(reservations) != 1 {
		t.Fatalf("expected 1 active reservation, got %d", len(reservations))
	}

	// Attempt overlapping exclusive reservation (should fail)
	conflictResp := postJSON(t, srv.URL+"/api/reservations", map[string]any{
		"agent_id":     "agent-b",
		"project":      project,
		"path_pattern": "cmd/intermute/main.go",
		"exclusive":    true,
	})
	if conflictResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for conflict, got %d", conflictResp.StatusCode)
	}
	conflictResp.Body.Close()

	// Release via store (HTTP DELETE requires auth agent matching)
	if err := st.ReleaseReservation(nil, resID, "agent-a"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Verify released
	activeResp2 := getJSON(t, srv.URL+"/api/reservations?project="+project)
	active2 := decode[map[string]any](t, activeResp2)
	reservations2 := active2["reservations"].([]any)
	if len(reservations2) != 0 {
		t.Fatalf("expected 0 active after release, got %d", len(reservations2))
	}
}
