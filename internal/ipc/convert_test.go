package ipc_test

import (
	"testing"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
)

func TestAgentStateToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input agent.AgentState
		want  pb.AgentState
	}{
		{"idle", agent.StateIdle, pb.AgentState_AGENT_STATE_IDLE},
		{"assigned", agent.StateAssigned, pb.AgentState_AGENT_STATE_ASSIGNED},
		{"working", agent.StateWorking, pb.AgentState_AGENT_STATE_WORKING},
		{"reporting", agent.StateReporting, pb.AgentState_AGENT_STATE_REPORTING},
		{"unknown", agent.AgentState("bogus"), pb.AgentState_AGENT_STATE_UNSPECIFIED},
		{"empty", agent.AgentState(""), pb.AgentState_AGENT_STATE_UNSPECIFIED},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ipc.AgentStateToProto(tc.input)
			if got != tc.want {
				t.Errorf("AgentStateToProto(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestAgentStateFromProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input pb.AgentState
		want  agent.AgentState
	}{
		{"idle", pb.AgentState_AGENT_STATE_IDLE, agent.StateIdle},
		{"assigned", pb.AgentState_AGENT_STATE_ASSIGNED, agent.StateAssigned},
		{"working", pb.AgentState_AGENT_STATE_WORKING, agent.StateWorking},
		{"reporting", pb.AgentState_AGENT_STATE_REPORTING, agent.StateReporting},
		{"unspecified", pb.AgentState_AGENT_STATE_UNSPECIFIED, agent.StateIdle},
		{"unknown_value", pb.AgentState(99), agent.StateIdle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ipc.AgentStateFromProto(tc.input)
			if got != tc.want {
				t.Errorf("AgentStateFromProto(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
