package grpc

import (
	"testing"

	domain "file-service/internal/domain/file"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusMapsEnforcementErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{name: "membership required", err: domain.ErrMembershipEntitlementRequired, want: codes.PermissionDenied},
		{name: "attachment owner mismatch", err: domain.ErrAttachmentOwnerMismatch, want: codes.PermissionDenied},
		{name: "file owner mismatch", err: domain.ErrFileOwnerMismatch, want: codes.NotFound},
		{name: "managed media deletion", err: domain.ErrManagedMediaDeletionForbidden, want: codes.FailedPrecondition},
		{name: "file capacity exhausted", err: domain.ErrFileCapacityExceeded, want: codes.ResourceExhausted},
		{name: "membership unavailable", err: domain.ErrMembershipServiceUnavailable, want: codes.Unavailable},
		{name: "inactive author membership sale", err: domain.ErrPaidAttachmentSalesMembershipInactive, want: codes.FailedPrecondition},
		{name: "topic owner mismatch", err: domain.ErrAttachmentTopicOwnerMismatch, want: codes.PermissionDenied},
		{name: "topic unavailable", err: domain.ErrAttachmentTopicUnavailable, want: codes.FailedPrecondition},
		{name: "content unavailable", err: domain.ErrContentServiceUnavailable, want: codes.Unavailable},
		{name: "account erased", err: domain.ErrAccountErased, want: codes.FailedPrecondition},
		{name: "invalid account erasure", err: domain.ErrInvalidAccountErasure, want: codes.InvalidArgument},
		{name: "account erasure unavailable", err: domain.ErrAccountErasureUnavailable, want: codes.Unavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := status.Code(toStatus(test.err)); got != test.want {
				t.Fatalf("status code = %s, want %s", got, test.want)
			}
		})
	}
}

func TestToStatusHidesFileOwnerMismatch(t *testing.T) {
	converted := status.Convert(toStatus(domain.ErrFileOwnerMismatch))
	if converted.Code() != codes.NotFound {
		t.Fatalf("status code = %s, want %s", converted.Code(), codes.NotFound)
	}
	if converted.Message() != domain.ErrFileNotFound.Error() {
		t.Fatalf("status message = %q, want %q", converted.Message(), domain.ErrFileNotFound.Error())
	}
}
