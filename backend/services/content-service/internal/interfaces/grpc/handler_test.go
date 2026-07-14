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
