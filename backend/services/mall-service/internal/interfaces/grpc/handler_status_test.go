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

func TestToStatusErrorMapsCouponTermsLocked(t *testing.T) {
	err := toStatusError(domain.ErrCouponTermsLocked)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsActiveThemeEntitlementExists(t *testing.T) {
	err := toStatusError(domain.ErrActiveThemeEntitlementExists)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsPendingThemeOrderExists(t *testing.T) {
	err := toStatusError(domain.ErrPendingThemeOrderExists)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsPendingMembershipOrderExists(t *testing.T) {
	err := toStatusError(domain.ErrPendingMembershipOrderExists)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsDuplicateThemeGrantInOrder(t *testing.T) {
	err := toStatusError(domain.ErrDuplicateThemeGrantInOrder)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusErrorMapsBadgeGrantErrors(t *testing.T) {
	for _, err := range []error{
		domain.ErrActiveBadgeEntitlementExists,
		domain.ErrPendingBadgeOrderExists,
		domain.ErrDuplicateBadgeGrantInOrder,
	} {
		if status.Code(toStatusError(err)) != codes.FailedPrecondition {
			t.Fatalf("status code for %v = %s, want %s", err, status.Code(toStatusError(err)), codes.FailedPrecondition)
		}
	}
}
