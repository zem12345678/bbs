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

func TestToStatusMapsSearchRebuildErrors(t *testing.T) {
	if got := status.Code(toStatus(domain.ErrSearchRebuildUnavailable)); got != codes.Unavailable {
		t.Fatalf("unavailable code = %s, want %s", got, codes.Unavailable)
	}
	if got := status.Code(toStatus(domain.ErrSearchRebuildInProgress)); got != codes.AlreadyExists {
		t.Fatalf("in-progress code = %s, want %s", got, codes.AlreadyExists)
	}
}

func TestToStatusMapsSystemNotificationRecipientErrors(t *testing.T) {
	if got := status.Code(toStatus(domain.ErrSystemNotificationRecipientValidationUnavailable)); got != codes.Unavailable {
		t.Fatalf("validation unavailable code = %s, want %s", got, codes.Unavailable)
	}
	if got := status.Code(toStatus(domain.ErrSystemNotificationRecipientsNotFound)); got != codes.InvalidArgument {
		t.Fatalf("recipients not found code = %s, want %s", got, codes.InvalidArgument)
	}
}
