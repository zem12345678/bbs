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

func TestToStatusMapsSystemDeletePreconditions(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "menu children", err: domain.ErrSystemMenuHasChildren},
		{name: "dept children", err: domain.ErrSystemDeptHasChildren},
		{name: "dept users", err: domain.ErrSystemDeptHasUsers},
		{name: "role users", err: domain.ErrSystemRoleHasUsers},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := toStatus(tt.err)
			if status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("status.Code(toStatus(%v)) = %s, want %s", tt.err, status.Code(err), codes.FailedPrecondition)
			}
			if got := status.Convert(err).Message(); got != tt.err.Error() {
				t.Fatalf("status message = %q, want %q", got, tt.err.Error())
			}
		})
	}
}

func TestToStatusMapsSystemParentValidation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "menu parent not found", err: domain.ErrSystemMenuParentNotFound},
		{name: "menu invalid parent", err: domain.ErrSystemMenuInvalidParent},
		{name: "dept parent not found", err: domain.ErrSystemDeptParentNotFound},
		{name: "dept invalid parent", err: domain.ErrSystemDeptInvalidParent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := toStatus(tt.err)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("status.Code(toStatus(%v)) = %s, want %s", tt.err, status.Code(err), codes.InvalidArgument)
			}
			if got := status.Convert(err).Message(); got != tt.err.Error() {
				t.Fatalf("status message = %q, want %q", got, tt.err.Error())
			}
		})
	}
}
