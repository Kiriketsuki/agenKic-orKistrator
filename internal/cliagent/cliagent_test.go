package cliagent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

func TestSupported(t *testing.T) {
	for _, kind := range []string{"claude", "codex", "opencode", "pi"} {
		if !Supported(kind) {
			t.Errorf("Supported(%q) = false, want true", kind)
		}
	}
	if Supported("sim") {
		t.Error("Supported(\"sim\") = true, want false")
	}
}

func TestKindTablesAgree(t *testing.T) {
	for kind := range cliBinaries {
		if _, ok := cliCommands[kind]; !ok {
			t.Errorf("cliCommands is missing kind %q", kind)
		}
		if _, ok := interactiveCommands[kind]; !ok {
			t.Errorf("interactiveCommands is missing kind %q", kind)
		}
	}
	if len(interactiveCommands) != len(cliBinaries) {
		t.Errorf("table sizes differ: %d interactive, %d binaries", len(interactiveCommands), len(cliBinaries))
	}
}

func TestSanitizePrompt(t *testing.T) {
	got := sanitizePrompt("first line\nsecond\rthird")
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("sanitizePrompt kept a line break: %q", got)
	}
	if got != "first line second third" {
		t.Errorf("sanitizePrompt = %q", got)
	}

	long := sanitizePrompt(strings.Repeat("x", terminal.MaxCommandLen+100))
	if len(long) != terminal.MaxCommandLen-1 {
		t.Errorf("sanitizePrompt length = %d, want %d", len(long), terminal.MaxCommandLen-1)
	}
	if err := terminal.ValidateCommand(long); err != nil {
		t.Errorf("sanitized prompt fails ValidateCommand: %v", err)
	}
}

// fakeSubstrate records SendCommand calls and reports session readiness.
type fakeSubstrate struct {
	mu       sync.Mutex
	ready    bool
	sent     []string
	captured string
}

func (f *fakeSubstrate) SpawnSession(context.Context, string, string) (terminal.Session, error) {
	return terminal.Session{}, nil
}
func (f *fakeSubstrate) DestroySession(context.Context, string) error { return nil }
func (f *fakeSubstrate) SendCommand(_ context.Context, _ string, cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, cmd)
	return nil
}
func (f *fakeSubstrate) SendKey(_ context.Context, _ string, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, key)
	return nil
}
func (f *fakeSubstrate) CaptureOutput(context.Context, string, int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ready {
		return "", errors.New("no session")
	}
	return f.captured, nil
}
func (f *fakeSubstrate) ListSessions(context.Context) ([]terminal.Session, error) { return nil, nil }
func (f *fakeSubstrate) SplitPane(context.Context, string, terminal.Direction) (terminal.Pane, error) {
	return terminal.Pane{}, nil
}

func TestLaunchInteractive_SendsSingleLaunchCommand(t *testing.T) {
	f := &fakeSubstrate{ready: true}
	if !launchInteractive(context.Background(), f, "agent-abc", "claude", "/tmp/agenkic-agents/abc") {
		t.Fatal("launchInteractive = false, want true")
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent %d commands, want 1: %v", len(f.sent), f.sent)
	}
	cmd := f.sent[0]
	if !strings.Contains(cmd, "exec claude") || !strings.Contains(cmd, "/tmp/agenkic-agents/abc") {
		t.Errorf("launch command = %q", cmd)
	}
	if err := terminal.ValidateCommand(cmd); err != nil {
		t.Errorf("launch command fails ValidateCommand: %v", err)
	}
}

func TestLaunchInteractive_UnknownKind(t *testing.T) {
	f := &fakeSubstrate{ready: true}
	if launchInteractive(context.Background(), f, "agent-abc", "sim", "/tmp/x") {
		t.Error("launchInteractive(sim) = true, want false")
	}
	if len(f.sent) != 0 {
		t.Errorf("unexpected commands sent: %v", f.sent)
	}
}

func TestLaunchInteractive_SessionNeverAppears(t *testing.T) {
	f := &fakeSubstrate{ready: false}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if launchInteractive(ctx, f, "agent-abc", "claude", "/tmp/x") {
		t.Error("launchInteractive with no session = true, want false")
	}
}
