package mall

import (
	"context"
	"testing"
	"time"

	"content-service/api/proto/mallpb"

	"google.golang.org/grpc"
)

func TestHasActiveMembershipRequiresGrantKey(t *testing.T) {
	tests := []struct {
		name        string
		entitlement *mallpb.DigitalEntitlement
		want        bool
	}{
		{
			name:        "blank grant key",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership"},
			want:        false,
		},
		{
			name:        "keyed grant",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "qa_bounty"},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mallClient := &fakeMallServiceClient{
				resp: &mallpb.ListDigitalEntitlementsResponse{Items: []*mallpb.DigitalEntitlement{tt.entitlement}},
			}
			client := &Client{client: mallClient}

			got, err := client.HasActiveMembership(context.Background(), 42)
			if err != nil {
				t.Fatalf("HasActiveMembership() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("HasActiveMembership() = %v, want %v", got, tt.want)
			}
			if mallClient.req.GetUserId() != 42 {
				t.Fatalf("ListUserDigitalEntitlements user id = %d, want 42", mallClient.req.GetUserId())
			}
		})
	}
}

func TestDigitalEntitlementIsActive(t *testing.T) {
	now := time.UnixMilli(2000)
	tests := []struct {
		name        string
		entitlement *mallpb.DigitalEntitlement
		want        bool
	}{
		{name: "nil", entitlement: nil, want: false},
		{name: "active", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE"}, want: true},
		{name: "blank status", entitlement: &mallpb.DigitalEntitlement{}, want: false},
		{name: "revoked", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", RevokedAt: 1000}, want: false},
		{name: "expired", entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", ExpiresAt: 1999}, want: false},
		{name: "inactive status", entitlement: &mallpb.DigitalEntitlement{Status: "REVOKED"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := digitalEntitlementIsActive(tt.entitlement, now); got != tt.want {
				t.Fatalf("digitalEntitlementIsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeMallServiceClient struct {
	req  *mallpb.ListUserDigitalEntitlementsRequest
	resp *mallpb.ListDigitalEntitlementsResponse
	err  error
}

func (f *fakeMallServiceClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}
