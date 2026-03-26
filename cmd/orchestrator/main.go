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
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
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

	submitter := dag.NewBlockingSubmitter(store, registry)
	executor := dag.NewExecutor(ctx, submitter)

	agg := health.NewAggregator(store, executor, health.WithMinAgents(minAgents))

	hs := grpchealth.NewServer()
	server := ipc.NewOrchestratorServer(sv, store, executor, ipc.WithHealthServer(hs))

	httpHealth := ipc.NewHealthHTTPServer(healthAddr, agg)

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

	// Graceful shutdown on signal.
	// Order: cancel context first (stops all context-dependent loops such as
	// the supervisor and health updater), then drain external-facing servers,
	// then shut down internal components.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		fmt.Println("shutting down...")
		cancel()
		server.GracefulStop()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = httpHealth.Shutdown(shutCtx)
		executor.Shutdown()
		sv.Stop()
	}()

	fmt.Printf("agenKic-orKistrator gRPC on %s, health HTTP on %s\n", addr, healthAddr)
	if err := server.StartGRPC(addr); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
