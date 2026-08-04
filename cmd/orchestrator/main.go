package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/cliagent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/simagent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
	grpchealth "google.golang.org/grpc/health"
)

func main() {
	addr := ":50051"
	if envAddr := os.Getenv("GRPC_ADDR"); envAddr != "" {
		addr = envAddr
	}

	healthAddr := ":8080"
	if envHealth := os.Getenv("HEALTH_ADDR"); envHealth != "" {
		healthAddr = envHealth
	}

	minAgents := 1
	if envMin := os.Getenv("MIN_AGENT_COUNT"); envMin != "" {
		if n, err := strconv.Atoi(envMin); err == nil && n > 0 {
			minAgents = n
		}
	}

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy()

	registry := supervisor.NewCompletionRegistry()

	var svOpts []supervisor.SupervisorOption
	svOpts = append(svOpts, supervisor.WithCompletionRegistry(registry))
	if sub, err := terminal.NewTmuxSubstrate(); err != nil {
		log.Printf("terminal substrate unavailable, running headless: %v", err)
	} else {
		log.Println("terminal substrate: tmux ready")
		svOpts = append(svOpts, supervisor.WithSubstrate(sub))
	}

	sv := supervisor.NewSupervisor(machine, store, policy, svOpts...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the tombstone reaper now that ctx exists. It is tied to the
	// top-level ctx, which is cancelled first in the shutdown sequence below
	// (cancel() at the start of graceful shutdown), so the reaper stops
	// cleanly with no goroutine leak.
	registry.StartReaper(ctx)

	submitter := dag.NewBlockingSubmitter(store, registry)
	executor := dag.NewExecutor(ctx, submitter)

	agg := health.NewAggregator(store, executor, health.WithMinAgents(minAgents))

	hs := grpchealth.NewServer()
	server := ipc.NewOrchestratorServer(sv, store, executor, ipc.WithHealthServer(hs))

	httpHealth := ipc.NewHealthHTTPServer(healthAddr, agg)

	// HTTP/SSE bridge for Godot UI.
	bridgeAddr := "127.0.0.1:8081"
	if envBridge := os.Getenv("BRIDGE_ADDR"); envBridge != "" {
		bridgeAddr = envBridge
	}
	var bridgeOpts []httpbridge.BridgeOption
	if sub, subOK := sv.Substrate(); subOK {
		bridgeOpts = append(bridgeOpts, httpbridge.WithSubstrate(sub))
	}
	// Same registry passed to supervisor.WithCompletionRegistry and
	// dag.NewBlockingSubmitter above, so a cancel via the HTTP bridge can
	// unblock a DAG node waiting on that exact task (T14 / #119, council
	// finding #2 — see httpbridge.WithCompletionRegistry doc comment).
	bridgeOpts = append(bridgeOpts, httpbridge.WithCompletionRegistry(registry))
	// Demo scaffolding: let the Godot UI summon simulated in-process worker
	// agents via POST /api/agents/spawn. The spawner dials our own gRPC
	// endpoint so sim agents exercise the exact same API as external ones.
	loopbackAddr := addr
	if loopbackAddr[0] == ':' {
		loopbackAddr = "127.0.0.1" + loopbackAddr
	}
	// CLI agents resolve their assigned task's prompt straight from the
	// store (they run in-process, so no extra RPC surface is needed).
	promptFn := func(pctx context.Context, agentID string) (string, string, error) {
		fields, err := store.GetAgentFields(pctx, agentID)
		if err != nil {
			return "", "", err
		}
		meta, err := store.GetTaskMeta(pctx, fields.CurrentTaskID)
		if err != nil {
			return fields.CurrentTaskID, "", err
		}
		return fields.CurrentTaskID, meta.Description, nil
	}
	bridgeOpts = append(bridgeOpts, httpbridge.WithAgentSpawner(func(kind, name, tier string) (string, error) {
		if kind == "sim" {
			return simagent.Spawn(ctx, loopbackAddr, name, tier)
		}
		var cliOpts []cliagent.SpawnOption
		if sub, ok := sv.Substrate(); ok {
			cliOpts = append(cliOpts, cliagent.WithSubstrate(sub))
		}
		return cliagent.Spawn(ctx, loopbackAddr, kind, name, tier, promptFn, cliOpts...)
	}))
	bridgeOpts = append(bridgeOpts, httpbridge.WithSupervisor(sv))
	if apiKey := os.Getenv("BRIDGE_API_KEY"); apiKey != "" {
		bridgeOpts = append(bridgeOpts, httpbridge.WithAPIKey(apiKey))
		log.Println("HTTP bridge: bearer-token auth enabled")
	} else {
		log.Println("HTTP bridge: no BRIDGE_API_KEY set — running without auth")
	}

	// bridge is declared before assignment so restartFn's closure can
	// reference it. restartFn only runs after POST /api/admin/restart, by
	// which point NewBridge below has already assigned it.
	var bridge *httpbridge.Bridge
	restartFn := func() {
		gracefulShutdown(cancel, server, httpHealth, bridge, executor, sv)
		time.Sleep(200 * time.Millisecond)
		if err := syscall.Exec(os.Args[0], os.Args, os.Environ()); err != nil { //nolint:gosec // re-exec of our own already-running binary
			log.Printf("re-exec failed: %v", err)
		}
	}
	bridgeOpts = append(bridgeOpts, httpbridge.WithRestartFunc(restartFn))
	bridge = httpbridge.NewBridge(bridgeAddr, store, executor, bridgeOpts...)

	// Run supervisor loops in background.
	go func() {
		if err := sv.Run(ctx); err != nil {
			log.Printf("supervisor exited: %v", err)
		}
	}()

	// Run gRPC health updater in background.
	go ipc.RunHealthUpdater(ctx, hs, agg, 2*time.Second)

	// Run HTTP health server in background.
	go func() {
		if err := httpHealth.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("health HTTP server exited: %v", err)
		}
	}()

	// Run HTTP/SSE bridge in background.
	go func() {
		if err := bridge.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP bridge server exited: %v", err)
		}
	}()

	// Graceful shutdown on signal.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		gracefulShutdown(cancel, server, httpHealth, bridge, executor, sv)
	}()

	fmt.Printf("agenKic-orKistrator gRPC on %s, health HTTP on %s, bridge HTTP on %s\n", addr, healthAddr, bridgeAddr)
	if err := server.StartGRPC(addr); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

// gracefulShutdown drains and stops every long-running component in the
// order the shutdown path has always used: cancel context first (stops all
// context-dependent loops such as the supervisor and health updater), then
// drain external-facing servers, then shut down internal components. Shared
// by the SIGTERM/SIGINT signal handler and POST /api/admin/restart's
// restartFn (F4 / power controls), so both shutdown paths stay identical.
func gracefulShutdown(cancel context.CancelFunc, server *ipc.OrchestratorServer, httpHealth *ipc.HealthHTTPServer, bridge *httpbridge.Bridge, executor *dag.Executor, sv *supervisor.Supervisor) {
	fmt.Println("shutting down...")
	cancel()
	server.GracefulStop()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	_ = httpHealth.Shutdown(shutCtx)
	_ = bridge.Shutdown(shutCtx)
	executor.Shutdown()
	sv.Stop()
}
