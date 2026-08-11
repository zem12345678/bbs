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

func TestToStatusMapsEmojiErrors(t *testing.T) {
	tests := []struct {
		err  error
		code codes.Code
	}{
		{err: domain.ErrInvalidEmoji, code: codes.InvalidArgument},
		{err: domain.ErrInvalidEmojiID, code: codes.InvalidArgument},
		{err: domain.ErrEmojiNotFound, code: codes.NotFound},
		{err: domain.ErrEmojiNameExists, code: codes.AlreadyExists},
		{err: domain.ErrEmojiStoreUnavailable, code: codes.Unavailable},
	}
	for _, test := range tests {
		if got := status.Code(toStatus(test.err)); got != test.code {
			t.Errorf("toStatus(%v) code = %s, want %s", test.err, got, test.code)
		}
	}
}
