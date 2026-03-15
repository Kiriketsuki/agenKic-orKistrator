package ipc

import (
	"fmt"
	"net"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"google.golang.org/grpc"
)

// OrchestratorServer implements the gRPC OrchestratorService.
type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	supervisor *supervisor.Supervisor
	store      state.StateStore
	dag        DAGEngine
	grpcServer *grpc.Server
}

// NewOrchestratorServer creates a server wired to the given supervisor, store, and DAG engine.
func NewOrchestratorServer(sv *supervisor.Supervisor, store state.StateStore, dag DAGEngine) *OrchestratorServer {
	return &OrchestratorServer{
		supervisor: sv,
		store:      store,
		dag:        dag,
	}
}

// StartGRPC binds to addr (e.g. ":50051") and serves in a blocking call.
func (s *OrchestratorServer) StartGRPC(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterOrchestratorServiceServer(s.grpcServer, s)

	return s.grpcServer.Serve(lis)
}

// GracefulStop stops accepting new RPCs and blocks until existing ones finish.
func (s *OrchestratorServer) GracefulStop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}
