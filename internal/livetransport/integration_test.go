//go:build tmux_integration

package livetransport_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/livetransport"
)

func TestTmuxRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}

	sessionName := "intermute-it-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	socketName := sessionName + "-sock"
	// Use session-only target so tmux picks the active window/pane.
	// Avoids base-index dependency (which varies by server config).
	targetName := sessionName

	newSession := tmuxCmd(socketName, "new-session", "-d", "-s", sessionName, "-x", "80", "-y", "24", "cat")
	if out, err := newSession.CombinedOutput(); err != nil {
		if isTmuxUnavailable(out) {
			t.Skipf("tmux session creation unavailable in this environment: %s", strings.TrimSpace(string(out)))
		}
		t.Fatalf("create tmux session: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = tmuxCmd(socketName, "kill-session", "-t", sessionName).Run()
	})

	inj := livetransport.NewInjector(tmuxRunner{socketName: socketName})
	target := &livetransport.Target{
		AgentID:    "test-agent",
		TmuxTarget: targetName,
	}

	const probe = "HELLO-INTEGRATION-PROBE"
	envelope := livetransport.WrapEnvelope("alice", "thr-1", probe)
	if err := inj.Deliver(target, envelope); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	capture := tmuxCmd(socketName, "capture-pane", "-t", targetName, "-p")
	out, err := capture.CombinedOutput()
	if err != nil {
		t.Fatalf("capture pane: %v\n%s", err, out)
	}

	got := string(out)
	if !strings.Contains(got, probe) {
		t.Fatalf("probe did not reach pane; captured:\n%s", got)
	}
	if !strings.Contains(got, "INTERMUTE-PEER-MESSAGE START") {
		t.Fatalf("envelope start missing; captured:\n%s", got)
	}
	if !strings.Contains(got, "INTERMUTE-PEER-MESSAGE END") {
		t.Fatalf("envelope end missing; captured:\n%s", got)
	}
}

func tmuxCmd(socketName string, args ...string) *exec.Cmd {
	cmd := exec.Command("tmux", append([]string{"-L", socketName}, args...)...)
	cmd.Env = withoutTmuxEnv()
	return cmd
}

type tmuxRunner struct {
	socketName string
}

func (r tmuxRunner) Run(args ...string) ([]byte, error) {
	cmd := tmuxCmd(r.socketName, args...)
	return cmd.CombinedOutput()
}

func (r tmuxRunner) WriteBuffer(name, data string) error {
	cmd := tmuxCmd(r.socketName, "load-buffer", "-b", name, "-")
	cmd.Stdin = strings.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func withoutTmuxEnv() []string {
	env := []string{}
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

func isTmuxUnavailable(out []byte) bool {
	msg := strings.ToLower(string(out))
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied")
}
