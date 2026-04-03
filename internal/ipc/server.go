package ipc

import (
	"fmt"
	"net"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"google.golang.org/grpc"
)

// ServerOption configures an OrchestratorServer.
type ServerOption func(*OrchestratorServer)

// WithHealthServer attaches a pre-created gRPC health server that will be
// registered when StartGRPC is called.
func WithHealthServer(hs *grpchealth.Server) ServerOption {
	return func(s *OrchestratorServer) {
		s.healthServer = hs
	}
}

// OrchestratorServer implements the gRPC OrchestratorService.
type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	supervisor   *supervisor.Supervisor
	store        state.StateStore
	dag          DAGEngine
	grpcServer   *grpc.Server
	healthServer *grpchealth.Server
}

// NewOrchestratorServer creates a server wired to the given supervisor, store, and DAG engine.
func NewOrchestratorServer(sv *supervisor.Supervisor, store state.StateStore, dag DAGEngine, opts ...ServerOption) *OrchestratorServer {
	s := &OrchestratorServer{
		supervisor: sv,
		store:      store,
		dag:        dag,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// StartGRPC binds to addr (e.g. ":50051") and serves in a blocking call.
func (s *OrchestratorServer) StartGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterOrchestratorServiceServer(s.grpcServer, s)

	if s.healthServer != nil {
		grpc_health_v1.RegisterHealthServer(s.grpcServer, s.healthServer)
	}

	return s.grpcServer.Serve(lis)
}

// GracefulStop stops accepting new RPCs and blocks until existing ones finish.
func (s *OrchestratorServer) GracefulStop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}
