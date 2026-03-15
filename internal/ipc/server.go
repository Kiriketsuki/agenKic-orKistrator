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
// DAG methods (SubmitDAG, GetDAGStatus) return Unimplemented via the embedded type
// and will be wired in the F3 integration PR.
type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	supervisor *supervisor.Supervisor
	store      state.StateStore
	dag        DAGEngine // nil until integration — unused in F2
	grpcServer *grpc.Server
}

// NewOrchestratorServer creates a server wired to the given supervisor and store.
func NewOrchestratorServer(sv *supervisor.Supervisor, store state.StateStore) *OrchestratorServer {
	return &OrchestratorServer{
		supervisor: sv,
		store:      store,
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
