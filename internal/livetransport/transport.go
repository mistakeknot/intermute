package livetransport

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mistakeknot/intermute/internal/core"
)

// Target is an alias for core.WindowTarget so existing call sites continue
// to work while HTTP-layer callers migrate to the core type directly.
type Target = core.WindowTarget

// LiveDelivery is the abstraction Service depends on. Tests and async-only
// deployments use a no-op; production wires Injector via main().
type LiveDelivery interface {
	Deliver(target *core.WindowTarget, envelope string) error
	ValidateTarget(target *core.WindowTarget) error
}

// Runner abstracts tmux CLI for Injector testability.
type Runner interface {
	Run(args ...string) ([]byte, error)
	WriteBuffer(name, data string) error
}

type defaultRunner struct{}

func (defaultRunner) Run(args ...string) ([]byte, error) {
	cmd := exec.Command("tmux", args...)
	return cmd.CombinedOutput()
}

func (defaultRunner) WriteBuffer(name, data string) error {
	cmd := exec.Command("tmux", "load-buffer", "-b", name, "-")
	cmd.Stdin = strings.NewReader(data)
	_, err := cmd.CombinedOutput()
	return err
}

// Injector is the concrete LiveDelivery that shells out to tmux.
type Injector struct {
	r Runner
}

var _ LiveDelivery = (*Injector)(nil)

func NewInjector(r Runner) *Injector {
	if r == nil {
		r = defaultRunner{}
	}
	return &Injector{r: r}
}

var validTarget = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?::[A-Za-z0-9_.-]+)?(?:\.[0-9]+)?$`)

func (i *Injector) ValidateTarget(t *core.WindowTarget) error {
	if t == nil || t.TmuxTarget == "" {
		return errors.New("empty tmux target")
	}
	if !validTarget.MatchString(t.TmuxTarget) {
		return fmt.Errorf("invalid tmux target: %q", t.TmuxTarget)
	}
	// Validate the SESSION portion only — pane/window validity varies with
	// tmux's base-index config and can surface from paste-buffer itself.
	sessionName := t.TmuxTarget
	if i := strings.IndexByte(sessionName, ':'); i >= 0 {
		sessionName = sessionName[:i]
	}
	if _, err := i.r.Run("has-session", "-t", sessionName); err != nil {
		return fmt.Errorf("stale target: %w", err)
	}
	return nil
}

func (i *Injector) Deliver(t *core.WindowTarget, envelope string) error {
	if err := i.ValidateTarget(t); err != nil {
		return err
	}

	bufferName := fmt.Sprintf("intermute-%s", t.AgentID)
	if err := i.r.WriteBuffer(bufferName, envelope+"\n"); err != nil {
		return fmt.Errorf("load-buffer: %w", err)
	}
	if _, err := i.r.Run("paste-buffer", "-b", bufferName, "-t", t.TmuxTarget, "-d"); err != nil {
		return fmt.Errorf("paste-buffer: %w", err)
	}
	if _, err := i.r.Run("send-keys", "-t", t.TmuxTarget, "Enter"); err != nil {
		return fmt.Errorf("send-keys Enter: %w", err)
	}

	return nil
}

// WrapEnvelope is kept here as a convenience re-export for existing callers
// inside this package; it delegates to core.WrapEnvelope so there is one
// canonical implementation.
//
// New callers should use core.WrapEnvelope directly.
func WrapEnvelope(sender, threadID, body string) string {
	return core.WrapEnvelope(sender, threadID, body)
}
