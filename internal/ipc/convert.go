package ipc

import (
	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

// AgentStateToProto converts a domain AgentState to the proto enum.
// Unknown states map to AGENT_STATE_UNSPECIFIED.
func AgentStateToProto(s agent.AgentState) pb.AgentState {
	switch s {
	case agent.StateIdle:
		return pb.AgentState_AGENT_STATE_IDLE
	case agent.StateAssigned:
		return pb.AgentState_AGENT_STATE_ASSIGNED
	case agent.StateWorking:
		return pb.AgentState_AGENT_STATE_WORKING
	case agent.StateReporting:
		return pb.AgentState_AGENT_STATE_REPORTING
	default:
		return pb.AgentState_AGENT_STATE_UNSPECIFIED
	}
}

// AgentStateFromProto converts a proto AgentState enum to the domain string.
// UNSPECIFIED and unknown values default to idle.
func AgentStateFromProto(s pb.AgentState) agent.AgentState {
	switch s {
	case pb.AgentState_AGENT_STATE_ASSIGNED:
		return agent.StateAssigned
	case pb.AgentState_AGENT_STATE_WORKING:
		return agent.StateWorking
	case pb.AgentState_AGENT_STATE_REPORTING:
		return agent.StateReporting
	default:
		return agent.StateIdle
	}
}
