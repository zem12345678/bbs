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
			if mallClient.req.GetGrantType() != digitalEntitlementGrantType {
				t.Fatalf("ListUserDigitalEntitlements grant type = %q, want %q", mallClient.req.GetGrantType(), digitalEntitlementGrantType)
			}
			if mallClient.req.GetGrantKey() != "theme-pro" {
				t.Fatalf("ListUserDigitalEntitlements grant key = %q, want theme-pro", mallClient.req.GetGrantKey())
			}
			if mallClient.req.GetLimit() != 1 {
				t.Fatalf("ListUserDigitalEntitlements limit = %d, want 1", mallClient.req.GetLimit())
			}
		})
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
