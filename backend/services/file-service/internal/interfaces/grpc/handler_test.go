package grpc

import (
	"testing"

	domain "file-service/internal/domain/file"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusMapsMembershipErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{name: "membership required", err: domain.ErrMembershipEntitlementRequired, want: codes.PermissionDenied},
		{name: "membership unavailable", err: domain.ErrMembershipServiceUnavailable, want: codes.Unavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := status.Code(toStatus(test.err)); got != test.want {
				t.Fatalf("status code = %s, want %s", got, test.want)
			}
		})
	}
}
