package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// screenSubstrate implements the optional ScreenCapturer capability so the
// output endpoint can serve visible-pane snapshots.
type screenSubstrate struct {
	stubSubstrate
	lastSession string
}

func (s *screenSubstrate) CaptureScreen(_ context.Context, session string) (string, error) {
	s.lastSession = session
	return "VISIBLE-PANE", nil
}

func decodeOutput(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body["output"]
}

func TestAgentOutput_ScreenMode(t *testing.T) {
	sub := &screenSubstrate{}
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil, httpbridge.WithSubstrate(sub))

	for _, param := range []string{"1", "true"} {
		req := httptest.NewRequest("GET", "/api/agents/agent-1/output?screen="+param, nil)
		w := httptest.NewRecorder()
		bridge.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("screen=%s: expected 200, got %d", param, w.Code)
		}
		if got := decodeOutput(t, w); got != "VISIBLE-PANE" {
			t.Errorf("screen=%s: output = %q, want VISIBLE-PANE", param, got)
		}
		if sub.lastSession != "agent-agent-1" {
			t.Errorf("screen=%s: session = %q", param, sub.lastSession)
		}
	}
}

func TestAgentOutput_NoScreenParamUsesHistory(t *testing.T) {
	sub := &screenSubstrate{}
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil, httpbridge.WithSubstrate(sub))

	req := httptest.NewRequest("GET", "/api/agents/agent-1/output", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := decodeOutput(t, w); got == "VISIBLE-PANE" {
		t.Error("output endpoint used CaptureScreen without the screen param")
	}
}

func TestAgentOutput_ScreenModeFallsBackWithoutCapability(t *testing.T) {
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil, httpbridge.WithSubstrate(&stubSubstrate{}))

	req := httptest.NewRequest("GET", "/api/agents/agent-1/output?screen=1", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 fallback, got %d", w.Code)
	}
}

func TestSpawnAgent_PiKindAccepted(t *testing.T) {
	var gotKind string
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(kind, _, _, _ string) (string, error) {
			gotKind = kind
			return "agent-pi", nil
		}))

	req := httptest.NewRequest("POST", "/api/agents/spawn", bytes.NewReader([]byte(`{"kind":"pi"}`)))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if gotKind != "pi" {
		t.Errorf("spawner got kind %q, want pi", gotKind)
	}
}

func TestSpawnAgent_UnknownKindListsPi(t *testing.T) {
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _, _ string) (string, error) { return "x", nil }))

	req := httptest.NewRequest("POST", "/api/agents/spawn", bytes.NewReader([]byte(`{"kind":"bogus"}`)))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "pi") {
		t.Errorf("error message does not list pi: %s", body)
	}
}

// stubSubstrate must not satisfy ScreenCapturer, which keeps the fallback
// branch of the output handler reachable in tests.
var _ terminal.Substrate = (*stubSubstrate)(nil)
