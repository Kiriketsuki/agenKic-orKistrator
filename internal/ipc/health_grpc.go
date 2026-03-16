package ipc

import (
	"context"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"google.golang.org/grpc"
)

// RegisterHealthService registers the standard gRPC health service on gs and
// returns the server so callers can set service statuses.
func RegisterHealthService(gs *grpc.Server) *grpchealth.Server {
	hs := grpchealth.NewServer()
	grpc_health_v1.RegisterHealthServer(gs, hs)
	return hs
}

// RunHealthUpdater polls the aggregator on interval and updates the gRPC
// health server status for "" (overall) and the orchestrator service name.
// It returns when ctx is cancelled.
func RunHealthUpdater(ctx context.Context, hs *grpchealth.Server, agg *health.Aggregator, interval time.Duration) {
	const svcName = "orchestrator.OrchestratorService"

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	update := func() {
		snap := agg.Check(ctx)
		s := grpc_health_v1.HealthCheckResponse_NOT_SERVING
		if snap.Ready {
			s = grpc_health_v1.HealthCheckResponse_SERVING
		}
		hs.SetServingStatus("", s)
		hs.SetServingStatus(svcName, s)
	}

	// Run once immediately before the first tick.
	update()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
