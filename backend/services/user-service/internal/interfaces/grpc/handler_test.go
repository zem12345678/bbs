package grpc

import (
	domain "user-service/internal/domain/user"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"testing"
)

func TestToStatusMapsProfileThemeEntitlementRequired(t *testing.T) {
	err := toStatus(domain.ErrProfileThemeEntitlementRequired)
	if grpcstatus.Code(err) != codes.PermissionDenied {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.PermissionDenied)
	}
}

func TestToStatusMapsSecurityEmailDeliveryUnavailable(t *testing.T) {
	err := toStatus(domain.ErrSecurityEmailDeliveryUnavailable)
	if grpcstatus.Code(err) != codes.Unavailable {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.Unavailable)
	}
}
