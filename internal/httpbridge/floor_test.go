package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// TestSpawnAgent_OmittedFloorAutoAssigns verifies a spawn request without a
// floor field is assigned the lowest non-full floor >= 1 by the bridge
// itself, so the agent counts toward that floor's capacity exactly like an
// explicit-floor spawn (finding 8).
func TestSpawnAgent_OmittedFloorAutoAssigns(t *testing.T) {
	n := 0
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			n++
			return fmt.Sprintf("agent-auto-%d", n), nil
		}))

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
	if resp.Floor != 1 {
		t.Errorf("spawn response floor = %d, want 1 (lowest available floor)", resp.Floor)
	}
}

// TestSpawnAgent_OmittedFloorCountsTowardCapacity verifies auto-assigned
// agents fill a floor to floorCapacity, and the fifth auto-assigned agent
// spills onto the next floor rather than overfilling floor 1 (finding 8:
// auto-assigned agents must be visible to the same capacity check as
// explicit-floor ones).
func TestSpawnAgent_OmittedFloorCountsTowardCapacity(t *testing.T) {
	n := 0
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			n++
			return fmt.Sprintf("agent-auto-%d", n), nil
		}))

	var floors []int
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/agents/spawn", bytes.NewReader([]byte(`{"kind":"sim"}`)))
		w := httptest.NewRecorder()
		bridge.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("spawn %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
		var resp httpbridge.SpawnAgentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("spawn %d: decode: %v", i, err)
		}
		floors = append(floors, resp.Floor)
	}

	for i := 0; i < 4; i++ {
		if floors[i] != 1 {
			t.Errorf("spawn %d landed on floor %d, want 1", i, floors[i])
		}
	}
	if floors[4] != 2 {
		t.Errorf("fifth auto-assigned spawn landed on floor %d, want 2 (floor 1 full)", floors[4])
	}
}

// TestSpawnAgent_EmptyAgentIDReleasesReservation verifies a spawner call that
// returns a nil error but an empty agent ID does not leak its floor
// reservation. Before the fix, commitFloorReservation returned early on
// agentID == "" without deleting the reservation token, so the slot stayed
// claimed forever and one real agent's worth of capacity was lost from the
// floor for the rest of the bridge's lifetime.
func TestSpawnAgent_EmptyAgentIDReleasesReservation(t *testing.T) {
	n := 0
	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			n++
			if n == 1 {
				// Simulate a spawner that reports success with no usable ID.
				return "", nil
			}
			return fmt.Sprintf("agent-%d", n), nil
		}))

	w := spawnOnFloor(t, bridge, 4)
	if w.Code != http.StatusOK {
		t.Fatalf("empty-id spawn: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// If the empty-id reservation leaked, only 3 of these 4 real spawns
	// would fit before the floor reports full.
	for i := 0; i < 4; i++ {
		w := spawnOnFloor(t, bridge, 4)
		if w.Code != http.StatusOK {
			t.Fatalf("real spawn %d after empty-id spawn: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	// The floor is now at its real capacity of 4. A further spawn must
	// still be rejected, confirming the fix frees the leaked slot without
	// disabling the capacity check itself.
	w = spawnOnFloor(t, bridge, 4)
	if w.Code != http.StatusConflict {
		t.Fatalf("spawn past real capacity: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSpawnAgent_ConcurrentSpawnsRespectFloorCapacity verifies two
// concurrent spawns racing for the last slot on a floor at 3/4 never both
// succeed — the TOCTOU window between the capacity read and the floor write
// must be closed by reserving the slot atomically (finding 5). Before the
// fix, both requests could read floorAgentCount == 3 under RLock before
// either wrote its assignment under Lock, and both would land on the floor,
// putting it at 5/4.
func TestSpawnAgent_ConcurrentSpawnsRespectFloorCapacity(t *testing.T) {
	var n atomic.Int64
	// release gates the spawner so both racing requests are in flight past
	// the capacity check at the same time before either returns from the
	// spawner call. Seed spawns run before release is armed, so they never
	// block.
	var release atomic.Pointer[chan struct{}]
	unblocked := make(chan struct{})
	close(unblocked)
	release.Store(&unblocked)

	bridge := httpbridge.NewBridge(":0", state.NewMockStore(), nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) {
			id := n.Add(1)
			<-*release.Load()
			return fmt.Sprintf("agent-race-%d", id), nil
		}))

	// Fill the floor to 3/4 first, sequentially, so the race below is only
	// over the fourth and final slot.
	for i := 0; i < 3; i++ {
		w := spawnOnFloor(t, bridge, 3)
		if w.Code != http.StatusOK {
			t.Fatalf("seed spawn %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	// Arm a blocking gate for the two racing spawns.
	gate := make(chan struct{})
	release.Store(&gate)

	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, 2)
	var wgStart sync.WaitGroup
	wgStart.Add(2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wgStart.Done()
			wgStart.Wait()
			results[i] = spawnOnFloor(t, bridge, 3)
		}(i)
	}
	// Give both goroutines time to enter the handler and reach the spawner
	// call before the gate opens.
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	successes := 0
	conflicts := 0
	for _, w := range results {
		switch w.Code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent spawns for the last floor slot: got %d successes and %d conflicts, want exactly 1 of each", successes, conflicts)
	}
}
