//go:build tmux_integration

package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/livetransport"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
)

func TestE2ELiveTransportRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}

	sessionName := "intermute-e2e-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	socketName := sessionName + "-sock"
	// Session-only target: tmux picks the active window/pane.  Avoids
	// base-index dependency.
	targetName := sessionName

	srv := newE2ETestServer(t, socketName)
	if srv == nil {
		return
	}
	defer srv.Close()

	newSession := e2eTmuxCmd(socketName, "new-session", "-d", "-s", sessionName, "-x", "80", "-y", "24", "cat")
	if out, err := newSession.CombinedOutput(); err != nil {
		if isE2ETmuxUnavailable(out) {
			t.Skipf("tmux session creation unavailable in this environment: %s", strings.TrimSpace(string(out)))
		}
		t.Fatalf("create tmux session: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = e2eTmuxCmd(socketName, "kill-session", "-t", sessionName).Run()
	})

	bob := registerAgentHTTP(t, srv.URL, "bob", "p1")
	setLivePolicyHTTP(t, srv.URL, bob.AgentID, core.PolicyOpen)
	upsertWindowHTTP(t, srv.URL, "p1", "win-bob", bob.AgentID, bob.Token, targetName)
	heartbeatWithFocus(t, srv.URL, bob.AgentID, core.FocusStateAtPrompt)

	const probe = "E2E-LIVE-PROBE"
	sendResp := postJSON(t, srv.URL+"/api/messages", map[string]any{
		"project":   "p1",
		"from":      "alice",
		"to":        []string{bob.AgentID},
		"body":      probe,
		"transport": string(core.TransportBoth),
	})
	requireStatus(t, sendResp, http.StatusOK)
	sendResult := decodeJSON[map[string]any](t, sendResp)
	if got, _ := sendResult["delivery"].(string); got != "injected" {
		t.Fatalf("delivery = %v, want injected", sendResult["delivery"])
	}

	time.Sleep(200 * time.Millisecond)

	out, err := e2eTmuxCmd(socketName, "capture-pane", "-t", targetName, "-p").CombinedOutput()
	if err != nil {
		t.Fatalf("capture pane: %v\n%s", err, out)
	}
	gotPane := string(out)
	if !strings.Contains(gotPane, probe) {
		t.Fatalf("probe did not reach pane; captured:\n%s", gotPane)
	}
	if !strings.Contains(gotPane, "INTERMUTE-PEER-MESSAGE START") {
		t.Fatalf("envelope start missing; captured:\n%s", gotPane)
	}

	inboxResp := fetchInboxHTTP(t, srv.URL, "p1", bob.AgentID)
	requireStatus(t, inboxResp, http.StatusOK)
	inbox := decodeJSON[inboxResponse](t, inboxResp)
	if len(inbox.Messages) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(inbox.Messages))
	}
	if inbox.Messages[0].Body != probe {
		t.Fatalf("inbox body = %q, want %q", inbox.Messages[0].Body, probe)
	}
}

func newE2ETestServer(t *testing.T, socketName string) (srv *httptest.Server) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Skipf("httptest server unavailable in this environment: %v", r)
			srv = nil
		}
	}()

	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st).WithLiveDelivery(livetransport.NewInjector(e2eTmuxRunner{socketName: socketName}))
	return httptest.NewServer(NewRouter(svc, nil, nil))
}

type e2eRegisteredAgent struct {
	AgentID string
	Token   string
}

func registerAgentHTTP(t *testing.T, baseURL, name, project string) e2eRegisteredAgent {
	t.Helper()

	resp := postJSON(t, baseURL+"/api/agents", map[string]any{
		"name":    name,
		"project": project,
		"status":  "active",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[registerAgentResponse](t, resp)
	return e2eRegisteredAgent{AgentID: result.AgentID, Token: result.Token}
}

func setLivePolicyHTTP(t *testing.T, baseURL, agentID string, policy core.ContactPolicy) {
	t.Helper()

	resp := putJSON(t, fmt.Sprintf("%s/api/agents/%s/policy", baseURL, agentID), map[string]any{
		"live_contact_policy": string(policy),
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func upsertWindowHTTP(t *testing.T, baseURL, project, windowUUID, agentID, token, tmuxTarget string) {
	t.Helper()

	resp := postJSON(t, baseURL+"/api/windows", map[string]any{
		"project":            project,
		"window_uuid":        windowUUID,
		"agent_id":           agentID,
		"tmux_target":        tmuxTarget,
		"registration_token": token,
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func heartbeatWithFocus(t *testing.T, baseURL, agentID, focusState string) {
	t.Helper()

	resp := postJSON(t, fmt.Sprintf("%s/api/agents/%s/heartbeat", baseURL, agentID), map[string]any{
		"focus_state": focusState,
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func fetchInboxHTTP(t *testing.T, baseURL, project, agentID string) *http.Response {
	t.Helper()

	resp, err := http.Get(fmt.Sprintf("%s/api/inbox/%s?project=%s", baseURL, agentID, project))
	if err != nil {
		t.Fatalf("GET inbox: %v", err)
	}
	return resp
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()

	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func putJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()

	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func e2eTmuxCmd(socketName string, args ...string) *exec.Cmd {
	cmd := exec.Command("tmux", append([]string{"-L", socketName}, args...)...)
	cmd.Env = e2eWithoutTmuxEnv()
	return cmd
}

type e2eTmuxRunner struct {
	socketName string
}

func (r e2eTmuxRunner) Run(args ...string) ([]byte, error) {
	return e2eTmuxCmd(r.socketName, args...).CombinedOutput()
}

func (r e2eTmuxRunner) WriteBuffer(name, data string) error {
	cmd := e2eTmuxCmd(r.socketName, "load-buffer", "-b", name, "-")
	cmd.Stdin = strings.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func e2eWithoutTmuxEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX=") {
			env = append(env, kv)
		}
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func isE2ETmuxUnavailable(out []byte) bool {
	msg := strings.ToLower(string(out))
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied")
}
