package mall

import (
	"context"
	"testing"
	"time"

	"user-service/api/proto/mallpb"

	"google.golang.org/grpc"
)

func TestHasActiveProfileThemeRequiresExactThemeGrant(t *testing.T) {
	tests := []struct {
		name        string
		entitlement *mallpb.DigitalEntitlement
		want        bool
	}{
		{
			name:        "active exact theme grant",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro"},
			want:        true,
		},
		{
			name:        "blank status",
			entitlement: &mallpb.DigitalEntitlement{GrantType: "theme", GrantKey: "theme-pro"},
			want:        false,
		},
		{
			name:        "blank grant key",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme"},
			want:        false,
		},
		{
			name:        "wrong grant key",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "badge-pro"},
			want:        false,
		},
		{
			name:        "wrong grant type",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "digital", GrantKey: "theme-pro"},
			want:        false,
		},
		{
			name:        "revoked",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro", RevokedAt: 1000},
			want:        false,
		},
		{
			name:        "expired",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro", ExpiresAt: time.Now().Add(-time.Hour).UnixMilli()},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mallClient := &fakeMallServiceClient{
				resp: &mallpb.ListDigitalEntitlementsResponse{Items: []*mallpb.DigitalEntitlement{tt.entitlement}},
			}
			client := &Client{client: mallClient}

			got, err := client.HasActiveProfileTheme(context.Background(), 42, " theme-pro ")
			if err != nil {
				t.Fatalf("HasActiveProfileTheme() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("HasActiveProfileTheme() = %v, want %v", got, tt.want)
			}
			if mallClient.req.GetUserId() != 42 {
				t.Fatalf("ListUserDigitalEntitlements user id = %d, want 42", mallClient.req.GetUserId())
			}
			if mallClient.req.GetStatus() != digitalEntitlementStatusActive {
				t.Fatalf("ListUserDigitalEntitlements status = %q, want %q", mallClient.req.GetStatus(), digitalEntitlementStatusActive)
			}
			if mallClient.req.GetGrantType() != digitalEntitlementGrantTypeTheme {
				t.Fatalf("ListUserDigitalEntitlements grant type = %q, want %q", mallClient.req.GetGrantType(), digitalEntitlementGrantTypeTheme)
			}
			if mallClient.req.GetGrantKey() != "theme-pro" {
				t.Fatalf("ListUserDigitalEntitlements grant key = %q, want theme-pro", mallClient.req.GetGrantKey())
			}
			if mallClient.req.GetLimit() != digitalEntitlementLookupLimit {
				t.Fatalf("ListUserDigitalEntitlements limit = %d, want %d", mallClient.req.GetLimit(), digitalEntitlementLookupLimit)
			}
		})
	}
}

func TestHasActiveMembershipRequiresGrantKeyAndFutureExpiry(t *testing.T) {
	tests := []struct {
		name        string
		entitlement *mallpb.DigitalEntitlement
		want        bool
	}{
		{
			name:        "future membership grant",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			want:        true,
		},
		{
			name:        "blank grant key",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()},
			want:        false,
		},
		{
			name:        "perpetual membership rejected",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month"},
			want:        false,
		},
		{
			name:        "expired membership",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: time.Now().Add(-time.Hour).UnixMilli()},
			want:        false,
		},
		{
			name:        "theme grant is not membership",
			entitlement: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro"},
			want:        false,
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
			if mallClient.req.GetStatus() != digitalEntitlementStatusActive {
				t.Fatalf("ListUserDigitalEntitlements status = %q, want %q", mallClient.req.GetStatus(), digitalEntitlementStatusActive)
			}
			if mallClient.req.GetGrantType() != digitalEntitlementGrantTypeMembership {
				t.Fatalf("ListUserDigitalEntitlements grant type = %q, want %q", mallClient.req.GetGrantType(), digitalEntitlementGrantTypeMembership)
			}
			if mallClient.req.GetGrantKey() != "" {
				t.Fatalf("ListUserDigitalEntitlements grant key = %q, want blank", mallClient.req.GetGrantKey())
			}
		})
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

func TestHasActiveProfileThemeScansPastRevokedFirstPage(t *testing.T) {
	mallClient := &fakeMallServiceClient{
		responsesByOffset: map[int32]*mallpb.ListDigitalEntitlementsResponse{
			0: {Items: dirtyThemeEntitlements(int(digitalEntitlementLookupLimit))},
			digitalEntitlementLookupLimit: {Items: []*mallpb.DigitalEntitlement{
				{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro"},
			}},
		},
	}
	client := &Client{client: mallClient}

	got, err := client.HasActiveProfileTheme(context.Background(), 42, "theme-pro")
	if err != nil {
		t.Fatalf("HasActiveProfileTheme() error = %v", err)
	}
	if !got {
		t.Fatal("HasActiveProfileTheme() = false, want true")
	}
	if len(mallClient.reqs) != 2 {
		t.Fatalf("ListUserDigitalEntitlements calls = %d, want 2", len(mallClient.reqs))
	}
	if mallClient.reqs[0].GetOffset() != 0 || mallClient.reqs[1].GetOffset() != digitalEntitlementLookupLimit {
		t.Fatalf("ListUserDigitalEntitlements offsets = %d, %d; want 0, %d", mallClient.reqs[0].GetOffset(), mallClient.reqs[1].GetOffset(), digitalEntitlementLookupLimit)
	}
}

func TestHasActiveProfileThemeSkipsBlankTheme(t *testing.T) {
	mallClient := &fakeMallServiceClient{}
	client := &Client{client: mallClient}

	got, err := client.HasActiveProfileTheme(context.Background(), 42, " ")
	if err != nil {
		t.Fatalf("HasActiveProfileTheme() error = %v", err)
	}
	if got {
		t.Fatal("HasActiveProfileTheme() = true, want false")
	}
	if mallClient.req != nil {
		t.Fatal("ListUserDigitalEntitlements called for blank theme")
	}
}

func TestListActiveMembershipUserIDsDeduplicatesAndBatches(t *testing.T) {
	userIDs := make([]int64, 0, digitalEntitlementBatchUserLookupLimit+3)
	for userID := int64(1); userID <= digitalEntitlementBatchUserLookupLimit+1; userID++ {
		userIDs = append(userIDs, userID)
	}
	userIDs = append(userIDs, 1, 0, -1)
	mallClient := &fakeMallServiceClient{
		activeUserIDsByGrant: map[string][]int64{
			"membership:": {1, digitalEntitlementBatchUserLookupLimit + 1, 999},
		},
	}
	client := &Client{client: mallClient}

	active, err := client.ListActiveMembershipUserIDs(context.Background(), userIDs)
	if err != nil {
		t.Fatalf("ListActiveMembershipUserIDs() error = %v", err)
	}
	if !active[1] || !active[digitalEntitlementBatchUserLookupLimit+1] || active[999] {
		t.Fatalf("active user ids = %v, want only requested active IDs", active)
	}
	if len(mallClient.batchRequests) != 2 {
		t.Fatalf("ListActiveEntitlementUserIDs calls = %d, want 2", len(mallClient.batchRequests))
	}
	if got := mallClient.batchRequests[0]; got.GetGrantType() != digitalEntitlementGrantTypeMembership || got.GetGrantKey() != "" || len(got.GetUserIds()) != digitalEntitlementBatchUserLookupLimit {
		t.Fatalf("first batch request = %+v, want membership batch of %d users", got, digitalEntitlementBatchUserLookupLimit)
	}
	if got := mallClient.batchRequests[1]; len(got.GetUserIds()) != 1 || got.GetUserIds()[0] != digitalEntitlementBatchUserLookupLimit+1 {
		t.Fatalf("second batch request = %+v, want final user %d", got, digitalEntitlementBatchUserLookupLimit+1)
	}
	if mallClient.req != nil {
		t.Fatal("ListUserDigitalEntitlements must not be used for batch profile checks")
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
	req                  *mallpb.ListUserDigitalEntitlementsRequest
	reqs                 []*mallpb.ListUserDigitalEntitlementsRequest
	resp                 *mallpb.ListDigitalEntitlementsResponse
	responsesByOffset    map[int32]*mallpb.ListDigitalEntitlementsResponse
	err                  error
	batchRequests        []*mallpb.ListActiveEntitlementUserIDsRequest
	activeUserIDsByGrant map[string][]int64
	batchErr             error
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

func (f *fakeMallServiceClient) ListActiveEntitlementUserIDs(_ context.Context, req *mallpb.ListActiveEntitlementUserIDsRequest, _ ...grpc.CallOption) (*mallpb.ListActiveEntitlementUserIDsResponse, error) {
	f.batchRequests = append(f.batchRequests, req)
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	return &mallpb.ListActiveEntitlementUserIDsResponse{
		UserIds: f.activeUserIDsByGrant[req.GetGrantType()+":"+req.GetGrantKey()],
	}, nil
}

func dirtyThemeEntitlements(count int) []*mallpb.DigitalEntitlement {
	items := make([]*mallpb.DigitalEntitlement, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro", RevokedAt: time.Now().UnixMilli()})
	}
	return items
}

func dirtyMembershipEntitlements(count int) []*mallpb.DigitalEntitlement {
	items := make([]*mallpb.DigitalEntitlement, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()})
	}
	return items
}
