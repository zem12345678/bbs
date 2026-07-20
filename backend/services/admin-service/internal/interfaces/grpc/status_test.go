package grpc

import (
	"testing"

	domain "admin/internal/domain/admin"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusMapsManagedTaskDefinitionsToFailedPrecondition(t *testing.T) {
	result, ok := status.FromError(toStatus(domain.ErrTaskDefinitionsManaged))
	if !ok {
		t.Fatal("toStatus() did not return a gRPC status error")
	}
	if result.Code() != codes.FailedPrecondition {
		t.Fatalf("toStatus() code = %s, want %s", result.Code(), codes.FailedPrecondition)
	}
}
