package grpc

import (
	"testing"

	domain "mall-service/internal/domain/mall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusErrorMapsProductGrantLocked(t *testing.T) {
	err := toStatusError(domain.ErrProductGrantLocked)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsProductFulfillmentLocked(t *testing.T) {
	err := toStatusError(domain.ErrProductFulfillmentLocked)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}
