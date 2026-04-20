package livetransport_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mistakeknot/intermute/internal/livetransport"
)

type fakeTmux struct {
	mu         sync.Mutex
	validateOK bool
	calls      [][]string
}

func (f *fakeTmux) Run(args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, append([]string{}, args...))
	if len(args) > 0 && args[0] == "has-session" && !f.validateOK {
		return nil, fmt.Errorf("no server")
	}
	return nil, nil
}

func (f *fakeTmux) WriteBuffer(name, data string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, []string{"load-buffer", "-b", name, "-"})
	return nil
}

func (f *fakeTmux) snapshotCalls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([][]string, len(f.calls))
	for i := range f.calls {
		out[i] = append([]string{}, f.calls[i]...)
	}
	return out
}

func TestInjectorDeliverSuccess(t *testing.T) {
	t.Parallel()

	fake := &fakeTmux{validateOK: true}
	inj := livetransport.NewInjector(fake)
	err := inj.Deliver(&livetransport.Target{TmuxTarget: "s:0.0"}, "hello")
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}

	calls := fake.snapshotCalls()
	if len(calls) != 4 {
		t.Fatalf("want 4 tmux calls (has-session, load-buffer, paste-buffer, send-keys), got %d: %v", len(calls), calls)
	}
	if calls[1][0] != "load-buffer" || calls[2][0] != "paste-buffer" || calls[3][0] != "send-keys" {
		t.Errorf("wrong call order: %v", calls)
	}
}

func TestInjectorValidateFailsFast(t *testing.T) {
	t.Parallel()

	fake := &fakeTmux{validateOK: false}
	inj := livetransport.NewInjector(fake)
	err := inj.ValidateTarget(&livetransport.Target{TmuxTarget: "s:0.0"})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Errorf("want stale-target error, got %v", err)
	}
}

func TestInjectorRejectsShellMetacharsInTarget(t *testing.T) {
	t.Parallel()

	fake := &fakeTmux{validateOK: true}
	inj := livetransport.NewInjector(fake)
	err := inj.Deliver(&livetransport.Target{TmuxTarget: "s:0.0; rm -rf /"}, "x")
	if err == nil {
		t.Error("expected rejection of shell metachar")
	}
}

func TestLiveDeliveryInterfaceSatisfied(t *testing.T) {
	var _ livetransport.LiveDelivery = (*livetransport.Injector)(nil)
}
