package grpc

import (
	"testing"

	topicDomain "content-service/internal/domain/topic"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestToStatusMapsMembershipEntitlementRequired(t *testing.T) {
	err := toStatus(topicDomain.ErrMembershipEntitlementRequired)
	if grpcstatus.Code(err) != codes.PermissionDenied {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.PermissionDenied)
	}
}

func TestToStatusMapsCannotAcceptOwnComment(t *testing.T) {
	err := toStatus(topicDomain.ErrCannotAcceptOwnComment)
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusMapsBountyCreditInsufficient(t *testing.T) {
	err := toStatus(topicDomain.ErrBountyCreditInsufficient)
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusMapsQAAcceptanceReversalErrors(t *testing.T) {
	if err := toStatus(topicDomain.ErrQAAcceptanceSettlementPending); grpcstatus.Code(err) != codes.Aborted {
		t.Fatalf("pending status code = %v, want %v", grpcstatus.Code(err), codes.Aborted)
	}
	if err := toStatus(topicDomain.ErrQAAcceptanceReversalInsufficientCredit); grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("insufficient status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}
