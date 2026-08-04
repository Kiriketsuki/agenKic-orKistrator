package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// TestListProviders_ReturnsFullRoster verifies GET /api/providers serves the
// spawn-kind sigils the Grimoire flyout builds its grid from.
func TestListProviders_ReturnsFullRoster(t *testing.T) {
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil)

	req := httptest.NewRequest("GET", "/api/providers", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Providers []struct {
			Kind    string `json:"kind"`
			Display string `json:"display"`
			Accent  string `json:"accent"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	wantKinds := []string{"sim", "claude", "codex", "opencode", "pi"}
	if len(body.Providers) != len(wantKinds) {
		t.Fatalf("providers = %d, want %d", len(body.Providers), len(wantKinds))
	}
	for i, kind := range wantKinds {
		p := body.Providers[i]
		if p.Kind != kind {
			t.Errorf("providers[%d].kind = %q, want %q", i, p.Kind, kind)
		}
		if p.Display == "" {
			t.Errorf("providers[%d] (%s) has empty display", i, kind)
		}
		if p.Accent == "" {
			t.Errorf("providers[%d] (%s) has empty accent", i, kind)
		}
	}
}

func spawnOnFloor(t *testing.T, bridge *httpbridge.Bridge, floor int) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"kind":"sim","floor":%d}`, floor)
	req := httptest.NewRequest("POST", "/api/agents/spawn", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)
	return w
}

// TestSpawnAgent_WithFloor verifies a spawn naming a real floor succeeds,
// records the assignment, and echoes the floor in both the spawn response
// and the agent list.
func TestSpawnAgent_WithFloor(t *testing.T) {
	n := 0
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			n++
			return fmt.Sprintf("agent-%d", n), nil
		}))

	w := spawnOnFloor(t, bridge, 1)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp httpbridge.SpawnAgentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode spawn response: %v", err)
	}
	if resp.Floor != 1 {
		t.Errorf("spawn response floor = %d, want 1", resp.Floor)
	}

	// The spawner is a stub, so register the agent in the store by hand, per
	// the pattern in TestSpawnAgent_NamePersistsInAgentList.
	if err := store.SetAgentFields(context.Background(), "agent-1", state.AgentFields{
		State: state.AgentStateIdle,
	}); err != nil {
		t.Fatal(err)
	}

	listReq := httptest.NewRequest("GET", "/api/agents", nil)
	listW := httptest.NewRecorder()
	bridge.ServeHTTP(listW, listReq)

	var listBody struct {
		Agents []httpbridge.AgentJSON `json:"agents"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode agent list: %v", err)
	}
	if len(listBody.Agents) != 1 || listBody.Agents[0].Floor != 1 {
		t.Fatalf("agent list = %+v, want one agent on floor 1", listBody.Agents)
	}
}

// TestSpawnAgent_FloorFullReturns409 verifies the fifth spawn onto a floor
// already holding floorCapacity (4) agents is rejected with 409, and that
// no fifth assignment lands on the floor.
func TestSpawnAgent_FloorFullReturns409(t *testing.T) {
	n := 0
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			n++
			return fmt.Sprintf("agent-%d", n), nil
		}))

	for i := 0; i < 4; i++ {
		w := spawnOnFloor(t, bridge, 2)
		if w.Code != http.StatusOK {
			t.Fatalf("spawn %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	w := spawnOnFloor(t, bridge, 2)
	if w.Code != http.StatusConflict {
		t.Fatalf("fifth spawn: expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if n != 4 {
		t.Errorf("spawner called %d times, want 4 (rejected spawn must not call spawner)", n)
	}
}

// TestSpawnAgent_GroundFloorReturns400 verifies floor 0 is always rejected,
// because it is reserved for the future archmage and never a drop target.
func TestSpawnAgent_GroundFloorReturns400(t *testing.T) {
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) { return "agent-x", nil }))

	w := spawnOnFloor(t, bridge, 0)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSpawnAgent_OmittedFloorKeepsAutoBehavior verifies a spawn request
// without a floor field still succeeds and records no floor assignment,
// preserving the pre-F3 auto-placement behavior.
func TestSpawnAgent_OmittedFloorKeepsAutoBehavior(t *testing.T) {
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) { return "agent-auto", nil }))

	req := httptest.NewRequest("POST", "/api/agents/spawn", bytes.NewReader([]byte(`{"kind":"sim"}`)))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp httpbridge.SpawnAgentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode spawn response: %v", err)
	}
	if resp.Floor != 0 {
		t.Errorf("spawn response floor = %d, want 0 (unassigned)", resp.Floor)
	}
}
