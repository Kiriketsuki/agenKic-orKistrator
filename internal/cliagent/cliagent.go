// Package cliagent runs real CLI coding agents (claude, codex, opencode, pi)
// as orchestrator workers. Each spawned agent registers over gRPC like any
// external worker. The package supports two modes.
//
// Interactive mode is the primary path. The caller passes WithSubstrate, so
// the agent launches the interactive CLI inside its own tmux session
// "agent-<id>". A task assignment types the prompt into that session, and the
// user chats with the same session through the HTTP bridge input endpoint.
//
// Headless mode is the fallback when no substrate exists, for example on
// Windows or a headless orchestrator. It execs the CLI one-shot with the task
// description as the prompt and streams the output through StreamOutput.
package cliagent

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	heartbeatInterval = 3 * time.Second
	pollInterval      = 500 * time.Millisecond
	taskTimeout       = 5 * time.Minute

	// sessionWaitTries and sessionWaitDelay bound the wait for the tmux
	// session the supervisor creates during RegisterAgent.
	sessionWaitTries = 10
	sessionWaitDelay = 200 * time.Millisecond

	// settleInterval and settleMinWait drive the interactive completion
	// heuristic. Two identical pane snapshots after the minimum wait mean
	// the CLI stopped producing output.
	settleInterval = 5 * time.Second
	settleMinWait  = 15 * time.Second
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
	"pi": func(prompt string) *exec.Cmd {
		return exec.Command("pi", "-p", prompt)
	},
}

// interactiveCommands maps kind to the interactive CLI launched inside the
// agent tmux session. codex runs plain, with no --yolo flag, by request.
var interactiveCommands = map[string]string{
	"claude":   "claude",
	"codex":    "codex",
	"opencode": "opencode",
	"pi":       "pi",
}

// Binary names for availability checks, per kind.
var cliBinaries = map[string]string{
	"claude":   "claude",
	"codex":    "codex",
	"opencode": "opencode",
	"pi":       "pi",
}

// Supported reports whether kind names a known CLI agent.
func Supported(kind string) bool {
	_, ok := cliBinaries[kind]
	return ok
}

// spawnCfg holds the optional dependencies of Spawn.
type spawnCfg struct {
	substrate terminal.Substrate
}

// SpawnOption configures a Spawn call.
type SpawnOption func(*spawnCfg)

// WithSubstrate enables the interactive path. After registration the CLI
// launches inside the agent tmux session, and prompts are typed into it.
func WithSubstrate(s terminal.Substrate) SpawnOption {
	return func(c *spawnCfg) { c.substrate = s }
}

// Spawn checks the CLI binary is installed, registers one agent over gRPC at
// addr, and runs its worker loop in a goroutine until ctx is cancelled.
func Spawn(ctx context.Context, addr, kind, name, tier string, promptFn PromptFunc, opts ...SpawnOption) (string, error) {
	var cfg spawnCfg
	for _, opt := range opts {
		opt(&cfg)
	}
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

	session := "agent-" + resp.AgentId
	interactive := false
	if cfg.substrate != nil {
		if launchInteractive(ctx, cfg.substrate, session, kind, workdir) {
			interactive = true
		} else {
			log.Printf("cliagent [%s] tmux launch failed; falling back to headless exec", name)
		}
	}

	go func() {
		defer conn.Close()
		run(ctx, client, resp.AgentId, kind, name, workdir, promptFn, cfg.substrate, interactive)
	}()
	return resp.AgentId, nil
}

// launchInteractive starts the interactive CLI inside the agent session. The
// supervisor creates the bare session during RegisterAgent, and cliagent is
// the only writer of the launch command, so the CLI starts exactly once.
// Returns false when the session never appears.
func launchInteractive(ctx context.Context, sub terminal.Substrate, session, kind, workdir string) bool {
	bin, ok := interactiveCommands[kind]
	if !ok {
		return false
	}

	ready := false
	for i := 0; i < sessionWaitTries; i++ {
		if _, err := sub.CaptureOutput(ctx, session, 1); err == nil {
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(sessionWaitDelay):
		}
	}
	if !ready {
		return false
	}

	// exec replaces the shell, so a CLI exit ends the pane visibly and no
	// shell prompt interleaves with the TUI output.
	launch := fmt.Sprintf("cd %q && clear && exec %s", workdir, bin)
	if err := sub.SendCommand(ctx, session, launch); err != nil {
		log.Printf("cliagent: launch %s in %s: %v", bin, session, err)
		return false
	}
	return true
}

func run(ctx context.Context, client pb.OrchestratorServiceClient, agentID, kind, name, workdir string, promptFn PromptFunc, sub terminal.Substrate, interactive bool) {
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
				if interactive {
					typePromptAndSettle(ctx, client, sub, agentID, kind, name, promptFn)
				} else {
					workTask(ctx, client, agentID, kind, name, workdir, promptFn)
				}
			}
		}
	}
}

// typePromptAndSettle types the task prompt into the agent tmux session and
// waits for the CLI to stop redrawing. Completion is a heuristic: two equal
// pane snapshots after settleMinWait, or the taskTimeout cap.
func typePromptAndSettle(ctx context.Context, client pb.OrchestratorServiceClient, sub terminal.Substrate, agentID, kind, name string, promptFn PromptFunc) {
	taskID, prompt, err := promptFn(ctx, agentID)
	if err != nil || prompt == "" {
		log.Printf("cliagent [%s] prompt lookup failed (taskID=%q): %v", name, taskID, err)
		prompt = "Say hello and describe what kind of agent you are in one sentence."
	}

	if _, err := client.StartWork(ctx, &pb.StartWorkRequest{AgentId: agentID}); err != nil {
		log.Printf("cliagent [%s] StartWork: %v", name, err)
		return
	}

	session := "agent-" + agentID
	typed := sanitizePrompt(prompt)
	log.Printf("cliagent [%s/%s] typing task %s into %s: %.60q", name, kind, taskID, session, typed)
	if err := sub.SendCommand(ctx, session, typed); err != nil {
		// The CLI never received the prompt, so the settle watch would only
		// time out and then report a false completion. Surface the failure on
		// the output stream and release the agent immediately.
		log.Printf("cliagent [%s] send prompt: %v", name, err)
		failText := fmt.Sprintf("[%s] failed to type task %s into %s: %v\n", name, taskID, session, err)
		streamOne(ctx, client, agentID, taskID, failText)
		finishTask(ctx, client, agentID, taskID, name)
		return
	}

	final := settleWatch(ctx, sub, session)

	// One chunk keeps the spell-scroll view populated. The pane itself stays
	// the live output surface, read through the output endpoint.
	if final != "" {
		streamOne(ctx, client, agentID, taskID, final)
	}

	finishTask(ctx, client, agentID, taskID, name)
	log.Printf("cliagent [%s/%s] task %s settled", name, kind, taskID)
}

// streamOne sends a single output chunk and closes the stream. It is a
// best-effort surface for the pane snapshot or for a send failure notice.
func streamOne(ctx context.Context, client pb.OrchestratorServiceClient, agentID, taskID, text string) {
	stream, err := client.StreamOutput(ctx)
	if err != nil || stream == nil {
		return
	}
	_ = stream.Send(&pb.OutputChunk{
		AgentId:   agentID,
		TaskId:    taskID,
		Type:      pb.OutputType_OUTPUT_TYPE_STDOUT,
		Payload:   []byte(text),
		Timestamp: time.Now().UnixMilli(),
		Sequence:  1,
	})
	_ = stream.CloseSend()
}

// finishTask walks the agent from WORKING through REPORTING back to IDLE.
func finishTask(ctx context.Context, client pb.OrchestratorServiceClient, agentID, taskID, name string) {
	if _, err := client.ReportOutput(ctx, &pb.ReportOutputRequest{AgentId: agentID}); err != nil {
		log.Printf("cliagent [%s] ReportOutput: %v", name, err)
	}
	if _, err := client.CompleteAgent(ctx, &pb.CompleteAgentRequest{AgentId: agentID, TaskId: taskID}); err != nil {
		log.Printf("cliagent [%s] CompleteAgent: %v", name, err)
	}
}

// sanitizePrompt makes a prompt safe to type into a TUI input box. An
// embedded newline submits the prompt halfway, so it becomes a space. The
// length cap keeps ValidateCommand from rejecting the send.
func sanitizePrompt(prompt string) string {
	// ValidateCommand rejects every control character below 0x20 except tab,
	// newline and carriage return, so each one becomes a space here. DEL goes
	// too, because a TUI input box treats it as an edit key.
	out := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, prompt)
	if len(out) > terminal.MaxCommandLen-1 {
		out = out[:terminal.MaxCommandLen-1]
	}
	return out
}

// settleWatch polls the pane until two consecutive snapshots match and the
// minimum wait elapsed. It returns the last snapshot.
func settleWatch(ctx context.Context, sub terminal.Substrate, session string) string {
	start := time.Now()
	deadline := time.After(taskTimeout)
	ticker := time.NewTicker(settleInterval)
	defer ticker.Stop()

	var prev string
	var have bool
	for {
		select {
		case <-ctx.Done():
			return prev
		case <-deadline:
			return prev
		case <-ticker.C:
			snap, err := sub.CaptureOutput(ctx, session, 50)
			if err != nil {
				continue
			}
			if have && snap == prev && time.Since(start) >= settleMinWait {
				return snap
			}
			prev = snap
			have = true
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
