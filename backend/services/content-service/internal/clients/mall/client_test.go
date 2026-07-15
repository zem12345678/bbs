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
			name:        "missing expiry",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month"},
			want:        false,
		},
		{
			name:        "keyed grant",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "qa_bounty", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
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

func TestHasActiveMembershipSkipsDirtyLatestGrant(t *testing.T) {
	mallClient := &fakeMallServiceClient{
		resp: &mallpb.ListDigitalEntitlementsResponse{Items: []*mallpb.DigitalEntitlement{
			{Status: "ACTIVE", GrantType: "membership", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
		}},
	}
	client := &Client{client: mallClient}

	got, err := client.HasActiveMembership(context.Background(), 42)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if !got {
		t.Fatal("HasActiveMembership() = false, want true")
	}
	if mallClient.req.GetLimit() != digitalEntitlementLookupLimit {
		t.Fatalf("ListUserDigitalEntitlements limit = %d, want %d", mallClient.req.GetLimit(), digitalEntitlementLookupLimit)
	}
}

func TestHasActiveMembershipScansPastDirtyFirstPage(t *testing.T) {
	mallClient := &fakeMallServiceClient{
		responsesByOffset: map[int32]*mallpb.ListDigitalEntitlementsResponse{
			0: {Items: dirtyMembershipEntitlements(int(digitalEntitlementLookupLimit))},
			digitalEntitlementLookupLimit: {Items: []*mallpb.DigitalEntitlement{
				{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			}},
		},
	}
	client := &Client{client: mallClient}

	got, err := client.HasActiveMembership(context.Background(), 42)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if !got {
		t.Fatal("HasActiveMembership() = false, want true")
	}
	if len(mallClient.reqs) != 2 {
		t.Fatalf("ListUserDigitalEntitlements calls = %d, want 2", len(mallClient.reqs))
	}
	if mallClient.reqs[0].GetOffset() != 0 || mallClient.reqs[1].GetOffset() != digitalEntitlementLookupLimit {
		t.Fatalf("ListUserDigitalEntitlements offsets = %d, %d; want 0, %d", mallClient.reqs[0].GetOffset(), mallClient.reqs[1].GetOffset(), digitalEntitlementLookupLimit)
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
	req               *mallpb.ListUserDigitalEntitlementsRequest
	reqs              []*mallpb.ListUserDigitalEntitlementsRequest
	resp              *mallpb.ListDigitalEntitlementsResponse
	responsesByOffset map[int32]*mallpb.ListDigitalEntitlementsResponse
	err               error
}

func (f *fakeMallServiceClient) ListUserDigitalEntitlements(_ context.Context, req *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.req = req
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.responsesByOffset != nil {
		if resp, ok := f.responsesByOffset[req.GetOffset()]; ok {
			return resp, nil
		}
		return &mallpb.ListDigitalEntitlementsResponse{}, nil
	}
	return f.resp, nil
}

func dirtyMembershipEntitlements(count int) []*mallpb.DigitalEntitlement {
	items := make([]*mallpb.DigitalEntitlement, 0, count)
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	for i := 0; i < count; i++ {
		items = append(items, &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", ExpiresAt: expiresAt})
	}
	return items
}
