package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

func main() {
	addr := ":50051"
	if envAddr := os.Getenv("GRPC_ADDR"); envAddr != "" {
		addr = envAddr
	}

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy()
	sv := supervisor.NewSupervisor(machine, store, policy)

	submitter := dag.NewStoreSubmitter(store)
	executor := dag.NewExecutor(submitter)
	server := ipc.NewOrchestratorServer(sv, store, executor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run supervisor loops in background.
	go func() {
		if err := sv.Run(ctx); err != nil {
			log.Printf("supervisor exited: %v", err)
		}
	}()

	// Graceful shutdown on signal.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		fmt.Println("shutting down...")
		sv.Stop()
		server.GracefulStop()
		cancel()
	}()

	fmt.Printf("agenKic-orKistrator listening on %s\n", addr)
	if err := server.StartGRPC(addr); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
