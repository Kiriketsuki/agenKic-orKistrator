package ipc_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func setupHealthGRPC(t *testing.T, store *state.MockStore, executor *dag.Executor) (grpc_health_v1.HealthClient, context.CancelFunc) {
	t.Helper()

	agg := health.NewAggregator(store, executor)

	gs := grpc.NewServer()
	hs := ipc.RegisterHealthService(gs)

	ctx, cancel := context.WithCancel(context.Background())
	go ipc.RunHealthUpdater(ctx, hs, agg, 10*time.Millisecond)

	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = gs.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
		gs.GracefulStop()
	})

	return grpc_health_v1.NewHealthClient(conn), cancel
}

func waitForStatus(t *testing.T, client grpc_health_v1.HealthClient, want grpc_health_v1.HealthCheckResponse_ServingStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: ""})
		if err == nil && resp.Status == want {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: ""})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	t.Fatalf("status = %v, want %v", resp.Status, want)
}

func TestGRPCHealth_Serving(t *testing.T) {
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")

	executor := dag.NewExecutor(ctx, dag.NewStoreSubmitter(store))
	client, cancel := setupHealthGRPC(t, store, executor)
	defer cancel()

	waitForStatus(t, client, grpc_health_v1.HealthCheckResponse_SERVING, 500*time.Millisecond)
}

func TestGRPCHealth_NotServing_NoAgents(t *testing.T) {
	store := state.NewMockStore()
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	client, cancel := setupHealthGRPC(t, store, executor)
	defer cancel()

	waitForStatus(t, client, grpc_health_v1.HealthCheckResponse_NOT_SERVING, 500*time.Millisecond)
}

func TestGRPCHealth_NotServing_RedisDown(t *testing.T) {
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	store.SetPingError(errors.New("connection refused"))

	executor := dag.NewExecutor(ctx, dag.NewStoreSubmitter(store))
	client, cancel := setupHealthGRPC(t, store, executor)
	defer cancel()

	waitForStatus(t, client, grpc_health_v1.HealthCheckResponse_NOT_SERVING, 500*time.Millisecond)
}
