package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// tmuxServerReady reports whether the package-level keepalive session (and
// therefore a tmux server) came up successfully. Set once by TestMain.
var tmuxServerReady bool

// TestMain holds a single throwaway tmux session open for the whole package
// run, so that every test which needs a live tmux server has one.
//
// A per-test session does not work here: killing the last session shuts the
// server down, and the next test's new-session then races the dying server and
// fails with "server exited unexpectedly". One session for the whole run keeps
// the server up from first test to last.
func TestMain(m *testing.M) {
	teardown := startTmuxKeepalive()
	code := m.Run()
	teardown()
	os.Exit(code)
}

// startTmuxKeepalive spawns a detached session that outlives every test in the
// package, and returns a teardown func that kills it. It sets tmuxServerReady.
func startTmuxKeepalive() func() {
	if _, err := exec.LookPath("tmux"); err != nil {
		return func() {}
	}

	session := fmt.Sprintf("gok-test-keepalive-%d", os.Getpid())
	if err := exec.Command("tmux", "new-session", "-d", "-s", session, "sleep", "3600").Run(); err != nil {
		return func() {}
	}

	tmuxServerReady = true
	return func() {
		_ = exec.Command("tmux", "kill-session", "-t", session).Run()
	}
}

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found on PATH; skipping integration test")
	}
}

// ensureTmuxServer skips the calling test unless a tmux server is running.
//
// Every test that asserts on ErrSessionNotFound needs one. tmux only reports
// "can't find session" when a server is up to answer the query; with no server
// at all it reports "error connecting to <socket>", which parseTmuxError maps
// to ErrNoServer instead. CI runners ship the tmux binary — so skipIfNoTmux
// does not skip — but have no server running, which is why these assertions
// passed locally and failed on GitHub Actions.
//
// The server itself is started once per package run by TestMain. If it could
// not be started, the test is skipped rather than failed: an environment
// without a usable tmux is not a defect in the code under test.
func ensureTmuxServer(t *testing.T) {
	t.Helper()
	if !tmuxServerReady {
		t.Skip("no tmux server could be started; skipping test that requires one")
	}
}
