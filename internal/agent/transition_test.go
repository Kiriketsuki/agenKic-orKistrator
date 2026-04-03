package agent_test

import (
	"errors"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

func TestValidTransition_AllValid(t *testing.T) {
	cases := []struct {
		from  agent.AgentState
		event agent.AgentEvent
		want  agent.AgentState
	}{
		{agent.StateIdle, agent.EventTaskAssigned, agent.StateAssigned},
		{agent.StateAssigned, agent.EventWorkStarted, agent.StateWorking},
		{agent.StateWorking, agent.EventOutputReady, agent.StateReporting},
		{agent.StateReporting, agent.EventOutputDelivered, agent.StateIdle},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_"+string(tc.event), func(t *testing.T) {
			got, err := agent.ValidTransition(tc.from, tc.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestValidTransition_AgentFailed_FromAnyState(t *testing.T) {
	states := []agent.AgentState{
		agent.StateIdle,
		agent.StateAssigned,
		agent.StateWorking,
		agent.StateReporting,
	}
	for _, from := range states {
		from := from
		t.Run(string(from), func(t *testing.T) {
			got, err := agent.ValidTransition(from, agent.EventAgentFailed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != agent.StateIdle {
				t.Fatalf("want idle, got %s", got)
			}
		})
	}
}

func TestValidTransition_InvalidTransitions(t *testing.T) {
	cases := []struct {
		from  agent.AgentState
		event agent.AgentEvent
	}{
		// idle cannot receive non-TaskAssigned events (except AgentFailed)
		{agent.StateIdle, agent.EventWorkStarted},
		{agent.StateIdle, agent.EventOutputReady},
		{agent.StateIdle, agent.EventOutputDelivered},
		// assigned cannot receive OutputReady or OutputDelivered
		{agent.StateAssigned, agent.EventOutputReady},
		{agent.StateAssigned, agent.EventOutputDelivered},
		{agent.StateAssigned, agent.EventTaskAssigned},
		// working cannot skip reporting
		{agent.StateWorking, agent.EventOutputDelivered},
		{agent.StateWorking, agent.EventTaskAssigned},
		{agent.StateWorking, agent.EventWorkStarted},
		// reporting cannot restart
		{agent.StateReporting, agent.EventTaskAssigned},
		{agent.StateReporting, agent.EventWorkStarted},
		{agent.StateReporting, agent.EventOutputReady},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_"+string(tc.event), func(t *testing.T) {
			_, err := agent.ValidTransition(tc.from, tc.event)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var te *agent.InvalidTransitionError
			if !errors.As(err, &te) {
				t.Fatalf("want *InvalidTransitionError, got %T: %v", err, err)
			}
			if te.From != tc.from || te.Event != tc.event {
				t.Fatalf("error fields mismatch: got From=%s Event=%s", te.From, te.Event)
			}
		})
	}
}
