package ipc

import (
	"context"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
)

// DAGEngine is the interface F2's service delegates DAG operations to.
// F3's Executor will satisfy this interface at integration time.
type DAGEngine interface {
	Execute(ctx context.Context, spec *pb.DAGSpec) (executionID string, err error)
	Status(ctx context.Context, executionID string) (*pb.GetDAGStatusResponse, error)
}
