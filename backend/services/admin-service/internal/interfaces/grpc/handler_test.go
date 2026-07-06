package grpc

import (
	"testing"

	domain "admin/internal/domain/admin"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusMapsProtectedSystemRoleToPermissionDenied(t *testing.T) {
	err := toStatus(domain.ErrProtectedSystemRole)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status.Code(toStatus(ErrProtectedSystemRole)) = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	if got := status.Convert(err).Message(); got != domain.ErrProtectedSystemRole.Error() {
		t.Fatalf("status message = %q, want %q", got, domain.ErrProtectedSystemRole.Error())
	}
}

func TestToStatusMapsProtectedSystemUserToPermissionDenied(t *testing.T) {
	err := toStatus(domain.ErrProtectedSystemUser)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status.Code(toStatus(ErrProtectedSystemUser)) = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	if got := status.Convert(err).Message(); got != domain.ErrProtectedSystemUser.Error() {
		t.Fatalf("status message = %q, want %q", got, domain.ErrProtectedSystemUser.Error())
	}
}
