// Package cliagent runs real CLI coding agents (claude, codex, opencode) as
// orchestrator workers. Each spawned agent registers over gRPC like any
// external worker; when a task is assigned, the CLI binary is exec'd headless
// with the task description as its prompt and its output is streamed back
// through StreamOutput. Demo/dev scaffolding for Windows, where the tmux
// substrate (the real terminal-per-agent design) is unavailable.
package cliagent

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	heartbeatInterval = 3 * time.Second
	pollInterval      = 500 * time.Millisecond
	taskTimeout       = 5 * time.Minute
)

// PromptFunc resolves the currently assigned task's ID and prompt text for
// an agent. Wired by the orchestrator main from its state store.
type PromptFunc func(ctx context.Context, agentID string) (taskID, prompt string, err error)

// cliCommands maps agent kind to the headless one-shot invocation.
var cliCommands = map[string]func(prompt string) *exec.Cmd{
	"claude": func(prompt string) *exec.Cmd {
		return exec.Command("claude", "-p", prompt, "--output-format", "text")
	},
	"codex": func(prompt string) *exec.Cmd {
		return exec.Command("codex", "exec", "--skip-git-repo-check", prompt)
	},
	"opencode": func(prompt string) *exec.Cmd {
		return exec.Command("opencode", "run", prompt)
	},
}

// Binary names for availability checks, per kind.
var cliBinaries = map[string]string{
	"claude":   "claude",
	"codex":    "codex",
	"opencode": "opencode",
}

// Supported reports whether kind names a known CLI agent.
func Supported(kind string) bool {
	_, ok := cliCommands[kind]
	return ok
}

// Spawn checks the CLI binary is installed, registers one agent over gRPC at
// addr, and runs its worker loop in a goroutine until ctx is cancelled.
func Spawn(ctx context.Context, addr, kind, name, tier string, promptFn PromptFunc) (string, error) {
	bin, ok := cliBinaries[kind]
	if !ok {
		return "", fmt.Errorf("cliagent: unknown kind %q", kind)
	}
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("cliagent: %q CLI not installed: %w", bin, err)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("cliagent dial %s: %w", addr, err)
	}
	client := pb.NewOrchestratorServiceClient(conn)

	resp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: name, ModelTier: tier},
	})
	if err != nil {
		_ = conn.Close()
		return "", fmt.Errorf("cliagent register %q: %w", name, err)
	}

	workdir := filepath.Join(os.TempDir(), "agenkic-agents", resp.AgentId)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		_ = conn.Close()
		return "", fmt.Errorf("cliagent workdir: %w", err)
	}

	go func() {
		defer conn.Close()
		run(ctx, client, resp.AgentId, kind, name, workdir, promptFn)
	}()
	return resp.AgentId, nil
}

func run(ctx context.Context, client pb.OrchestratorServiceClient, agentID, kind, name, workdir string, promptFn PromptFunc) {
	heartbeat := time.NewTicker(heartbeatInterval)
	poll := time.NewTicker(pollInterval)
	defer heartbeat.Stop()
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{AgentId: agentID}); err != nil {
				log.Printf("cliagent [%s] heartbeat: %v", name, err)
			}
		case <-poll.C:
			st, err := client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
			if err != nil {
				log.Printf("cliagent [%s] state: %v", name, err)
				continue
			}
			if st.State == pb.AgentState_AGENT_STATE_ASSIGNED {
				workTask(ctx, client, agentID, kind, name, workdir, promptFn)
			}
		}
	}
}

func workTask(ctx context.Context, client pb.OrchestratorServiceClient, agentID, kind, name, workdir string, promptFn PromptFunc) {
	taskID, prompt, err := promptFn(ctx, agentID)
	if err != nil || prompt == "" {
		log.Printf("cliagent [%s] prompt lookup failed (taskID=%q): %v", name, taskID, err)
		prompt = "Say hello and describe what kind of agent you are in one sentence."
	}

	if _, err := client.StartWork(ctx, &pb.StartWorkRequest{AgentId: agentID}); err != nil {
		log.Printf("cliagent [%s] StartWork: %v", name, err)
		return
	}
	log.Printf("cliagent [%s/%s] running task %s: %.60q", name, kind, taskID, prompt)

	stream, streamErr := client.StreamOutput(ctx)
	send := func(seq int64, text string) {
		if streamErr != nil || stream == nil {
			return
		}
		if err := stream.Send(&pb.OutputChunk{
			AgentId:   agentID,
			TaskId:    taskID,
			Type:      pb.OutputType_OUTPUT_TYPE_STDOUT,
			Payload:   []byte(text),
			Timestamp: time.Now().UnixMilli(),
			Sequence:  seq,
		}); err != nil {
			streamErr = err
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	cmd := cliCommands[kind](prompt)
	cmd.Dir = workdir
	cmd.Env = os.Environ()

	stdout, pipeErr := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout // interleave; single reader below owns both
	var seq int64
	if pipeErr == nil {
		pipeErr = cmd.Start()
	}
	if pipeErr != nil {
		send(seq, fmt.Sprintf("[%s] failed to launch %s: %v\n", name, kind, pipeErr))
	} else {
		// Kill the CLI if the timeout or shutdown hits mid-run.
		done := make(chan struct{})
		go func() {
			select {
			case <-cmdCtx.Done():
				_ = cmd.Process.Kill()
			case <-done:
			}
		}()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			seq++
			send(seq, scanner.Text()+"\n")
		}
		waitErr := cmd.Wait()
		close(done)
		if waitErr != nil {
			seq++
			send(seq, fmt.Sprintf("[%s] %s exited: %v\n", name, kind, waitErr))
		}
	}
	cancel()
	if stream != nil {
		_ = stream.CloseSend()
	}

	if _, err := client.ReportOutput(ctx, &pb.ReportOutputRequest{AgentId: agentID}); err != nil {
		log.Printf("cliagent [%s] ReportOutput: %v", name, err)
	}
	if _, err := client.CompleteAgent(ctx, &pb.CompleteAgentRequest{AgentId: agentID, TaskId: taskID}); err != nil {
		log.Printf("cliagent [%s] CompleteAgent: %v", name, err)
	}
	log.Printf("cliagent [%s/%s] task %s complete", name, kind, taskID)
}
