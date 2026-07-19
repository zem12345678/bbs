package mall

import (
	"context"
	"errors"
	"testing"
	"time"

	"file-service/api/proto/mallpb"

	"google.golang.org/grpc"
)

func TestHasActiveMembership(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	tests := []struct {
		name string
		item *mallpb.DigitalEntitlement
		want bool
	}{
		{name: "active membership", item: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: expiresAt}, want: true},
		{name: "missing grant key", item: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", ExpiresAt: expiresAt}},
		{name: "revoked", item: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: expiresAt, RevokedAt: 1}},
		{name: "expired", item: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-month", ExpiresAt: time.Now().Add(-time.Hour).UnixMilli()}},
		{name: "wrong grant type", item: &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "theme", GrantKey: "theme-pro", ExpiresAt: expiresAt}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeMallServiceClient{responses: map[int32]*mallpb.ListDigitalEntitlementsResponse{
				0: {Items: []*mallpb.DigitalEntitlement{test.item}},
			}}
			got, err := (&Client{client: fake}).HasActiveMembership(context.Background(), 42)
			if err != nil {
				t.Fatalf("HasActiveMembership() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("HasActiveMembership() = %t, want %t", got, test.want)
			}
			if len(fake.requests) != 1 {
				t.Fatalf("membership requests = %d, want 1", len(fake.requests))
			}
			request := fake.requests[0]
			if request.GetUserId() != 42 || request.GetStatus() != digitalEntitlementStatusActive || request.GetGrantType() != digitalEntitlementGrantType || request.GetLimit() != digitalEntitlementLookupLimit || request.GetOffset() != 0 {
				t.Fatalf("membership request = %+v", request)
			}
		})
	}
}

func TestHasActiveMembershipScansPages(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).UnixMilli()
	invalidItems := make([]*mallpb.DigitalEntitlement, 0, digitalEntitlementLookupLimit)
	for range digitalEntitlementLookupLimit {
		invalidItems = append(invalidItems, &mallpb.DigitalEntitlement{Status: "ACTIVE", GrantType: "membership", ExpiresAt: expiresAt})
	}
	fake := &fakeMallServiceClient{responses: map[int32]*mallpb.ListDigitalEntitlementsResponse{
		0:                             {Items: invalidItems},
		digitalEntitlementLookupLimit: {Items: []*mallpb.DigitalEntitlement{{Status: "ACTIVE", GrantType: "membership", GrantKey: "vip-year", ExpiresAt: expiresAt}}},
	}}

	got, err := (&Client{client: fake}).HasActiveMembership(context.Background(), 42)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if !got {
		t.Fatal("HasActiveMembership() = false, want true")
	}
	if len(fake.requests) != 2 || fake.requests[0].GetOffset() != 0 || fake.requests[1].GetOffset() != digitalEntitlementLookupLimit {
		t.Fatalf("membership requests = %+v", fake.requests)
	}
}

func TestHasActiveMembershipReturnsLookupError(t *testing.T) {
	want := errors.New("mall unavailable")
	_, err := (&Client{client: &fakeMallServiceClient{err: want}}).HasActiveMembership(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("HasActiveMembership() error = %v, want %v", err, want)
	}
}

type fakeMallServiceClient struct {
	responses map[int32]*mallpb.ListDigitalEntitlementsResponse
	err       error
	requests  []*mallpb.ListUserDigitalEntitlementsRequest
}

func (f *fakeMallServiceClient) ListUserDigitalEntitlements(_ context.Context, request *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	if response, ok := f.responses[request.GetOffset()]; ok {
		return response, nil
	}
	return &mallpb.ListDigitalEntitlementsResponse{}, nil
}
