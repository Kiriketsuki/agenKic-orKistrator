package supervisor

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// defaultReapInterval is how often the reaper sweeps for agents whose
// terminal session has vanished. Slower than the heartbeat loop on purpose:
// a vanished session is a terminal condition, not a transient blip, so there
// is nothing to gain from checking it every few seconds.
const defaultReapInterval = 15 * time.Second

// sessionPrefix is the tmux session name prefix the supervisor gives every
// agent (see RemoveAgent / cliagent.launchInteractive). Kept in one place so
// the reaper's name matching cannot drift from the name construction.
const sessionPrefix = "agent-"

// WithReapInterval sets how often the reaper checks for agents whose
// terminal session is gone. A non-positive duration disables the reaper.
func WithReapInterval(d time.Duration) SupervisorOption {
	return func(sv *Supervisor) { sv.reapInterval = d }
}

// reapLoop periodically drops agents whose terminal session no longer
// exists.
//
// Why this exists: heartbeatLoop deliberately skips agents in the idle state
// (see checkHeartbeats), because an idle agent legitimately sends no
// heartbeats while waiting for work. That left a real gap — when an
// interactive CLI exits, its tmux pane closes and the session disappears,
// but the agent stays registered forever in the idle state. The UI kept
// showing a healthy-looking agent, and every keystroke routed to it failed
// with a 404 from the input endpoint because the session it names is gone.
//
// A vanished session is unambiguous and unrecoverable: the supervisor never
// recreates a session for an already-registered agent, so there is nothing
// to restart and no reason to keep the registration.
func (sv *Supervisor) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(sv.reapInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sv.reapOrphanedAgents(ctx)
		}
	}
}

// reapOrphanedAgents removes every registered agent whose tmux session is
// missing. It takes ONE session listing per sweep and matches every agent
// against that snapshot, rather than probing per agent, so a roster of N
// agents costs one substrate call instead of N.
//
// Conservative by design: any error listing sessions aborts the whole sweep
// without removing anything. Treating an unreadable session list as "every
// session is gone" would deregister the entire roster the first time tmux
// hiccups, which is far worse than leaving a dead agent up for one more
// interval.
func (sv *Supervisor) reapOrphanedAgents(ctx context.Context) {
	if sv.substrate == nil {
		return
	}

	sessions, err := sv.substrate.ListSessions(ctx)
	if err != nil {
		log.Printf("supervisor: reaper — ListSessions failed, skipping sweep: %v", err)
		return
	}
	live := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		if id := strings.TrimPrefix(session.Name, sessionPrefix); id != session.Name {
			live[id] = struct{}{}
		}
	}

	agents, err := sv.store.ListAgents(ctx)
	if err != nil {
		log.Printf("supervisor: reaper — ListAgents failed: %v", err)
		return
	}

	for _, agentID := range agents {
		if _, ok := live[agentID]; ok {
			continue
		}
		// A just-registered agent races the supervisor's own session
		// creation: it is in the store before its session exists. Skipping
		// anything younger than one full reap interval keeps the reaper from
		// deregistering an agent that is still starting up.
		if sv.agentIsTooYoungToReap(ctx, agentID) {
			continue
		}
		log.Printf("supervisor: reaper — agent %s has no terminal session, deregistering", agentID)
		sv.reapAgent(ctx, agentID)
	}
}

// agentIsTooYoungToReap reports whether the agent registered so recently
// that its session may not exist yet. A store error answers "too young", so
// an unreadable agent is left alone rather than reaped on missing data.
func (sv *Supervisor) agentIsTooYoungToReap(ctx context.Context, agentID string) bool {
	fields, err := sv.store.GetAgentFields(ctx, agentID)
	if err != nil {
		log.Printf("supervisor: reaper — GetAgentFields %s failed, leaving agent alone: %v", agentID, err)
		return true
	}
	if fields.RegisteredAt == 0 {
		return false
	}
	age := time.Now().UnixMilli() - fields.RegisteredAt
	return age < sv.reapInterval.Milliseconds()
}

// reapAgent forgets one orphaned agent. Mirrors the despawn path in
// httpbridge.handleDespawnAgent (supervisor bookkeeping, then the store,
// then the deregistered event) so a reaped agent and a banished one leave
// the system in the same shape and the UI drops both the same way.
func (sv *Supervisor) reapAgent(ctx context.Context, agentID string) {
	sv.RemoveAgent(ctx, agentID)

	if err := sv.store.DeleteAgent(ctx, agentID); err != nil {
		log.Printf("supervisor: reaper — DeleteAgent %s failed: %v", agentID, err)
	}

	if err := sv.store.PublishEvent(ctx, state.Event{
		Type:      "agent_deregistered",
		AgentID:   agentID,
		Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		log.Printf("supervisor: reaper — PublishEvent %s failed: %v", agentID, err)
	}
}
