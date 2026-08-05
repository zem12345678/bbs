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
		{name: "invalid file capacity", err: domain.ErrInvalidFileCapacity, want: codes.InvalidArgument},
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

func TestFileUsageToPBPreservesCapacityMetadata(t *testing.T) {
	overrideBytes := int64(200)
	converted := fileUsageToPB(domain.FileUsage{
		UsedBytes:             75,
		CapacityBytes:         200,
		RemainingBytes:        125,
		FileCount:             3,
		PolicyCapacityBytes:   100,
		MaxFileSizeBytes:      50,
		OverrideCapacityBytes: &overrideBytes,
	})
	if converted.GetUsedBytes() != 75 || converted.GetCapacityBytes() != 200 || converted.GetRemainingBytes() != 125 ||
		converted.GetFileCount() != 3 || converted.GetPolicyCapacityBytes() != 100 || converted.GetMaxFileSizeBytes() != 50 ||
		!converted.GetHasOverride() || converted.GetOverrideCapacityBytes() != 200 {
		t.Fatalf("fileUsageToPB() = %+v", converted)
	}

	withoutOverride := fileUsageToPB(domain.FileUsage{PolicyCapacityBytes: 100, CapacityBytes: 100})
	if withoutOverride.GetHasOverride() || withoutOverride.GetOverrideCapacityBytes() != 0 {
		t.Fatalf("fileUsageToPB() without override = %+v", withoutOverride)
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
