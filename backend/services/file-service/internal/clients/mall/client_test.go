package mall

import (
	"context"
	"errors"
	"testing"

	"file-service/api/proto/mallpb"

	"google.golang.org/grpc"
)

func TestHasActiveMembershipUsesSingleBatchLookup(t *testing.T) {
	fake := &fakeMallServiceClient{
		activeResponse: &mallpb.ListActiveEntitlementUserIDsResponse{UserIds: []int64{7, 42}},
	}

	active, err := (&Client{client: fake}).HasActiveMembership(context.Background(), 42)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if !active {
		t.Fatal("HasActiveMembership() = false, want true")
	}
	if len(fake.activeRequests) != 1 {
		t.Fatalf("active entitlement requests = %d, want 1", len(fake.activeRequests))
	}
	request := fake.activeRequests[0]
	if len(request.GetUserIds()) != 1 || request.GetUserIds()[0] != 42 || request.GetGrantType() != digitalEntitlementGrantType || request.GetGrantKey() != "" {
		t.Fatalf("active entitlement request = %+v", request)
	}
	if len(fake.pagedRequests) != 0 {
		t.Fatalf("paged entitlement requests = %d, want 0", len(fake.pagedRequests))
	}
}

func TestHasActiveMembershipReturnsFalseWhenUserIsNotActive(t *testing.T) {
	fake := &fakeMallServiceClient{
		activeResponse: &mallpb.ListActiveEntitlementUserIDsResponse{UserIds: []int64{7}},
	}

	active, err := (&Client{client: fake}).HasActiveMembership(context.Background(), 42)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if active {
		t.Fatal("HasActiveMembership() = true, want false")
	}
}

func TestHasActiveMembershipRejectsInvalidUserIDWithoutLookup(t *testing.T) {
	fake := &fakeMallServiceClient{}

	active, err := (&Client{client: fake}).HasActiveMembership(context.Background(), 0)
	if err != nil {
		t.Fatalf("HasActiveMembership() error = %v", err)
	}
	if active {
		t.Fatal("HasActiveMembership() = true, want false")
	}
	if len(fake.activeRequests) != 0 {
		t.Fatalf("active entitlement requests = %d, want 0", len(fake.activeRequests))
	}
}

func TestHasActiveMembershipReturnsLookupError(t *testing.T) {
	want := errors.New("mall unavailable")
	_, err := (&Client{client: &fakeMallServiceClient{activeErr: want}}).HasActiveMembership(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("HasActiveMembership() error = %v, want %v", err, want)
	}
}

func TestInternalAuthCredentials(t *testing.T) {
	credentials := internalAuthCredentials{token: "mall-internal-token"}

	metadata, err := credentials.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata[internalAuthMetadataKey]; got != "mall-internal-token" {
		t.Fatalf("metadata token = %q", got)
	}
	if credentials.RequireTransportSecurity() {
		t.Fatal("mall internal credential must support the configured local insecure transport")
	}
}

type fakeMallServiceClient struct {
	activeResponse *mallpb.ListActiveEntitlementUserIDsResponse
	activeErr      error
	activeRequests []*mallpb.ListActiveEntitlementUserIDsRequest
	pagedRequests  []*mallpb.ListUserDigitalEntitlementsRequest
}

func (f *fakeMallServiceClient) ListActiveEntitlementUserIDs(_ context.Context, request *mallpb.ListActiveEntitlementUserIDsRequest, _ ...grpc.CallOption) (*mallpb.ListActiveEntitlementUserIDsResponse, error) {
	f.activeRequests = append(f.activeRequests, request)
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	return f.activeResponse, nil
}

func (f *fakeMallServiceClient) ListUserDigitalEntitlements(_ context.Context, request *mallpb.ListUserDigitalEntitlementsRequest, _ ...grpc.CallOption) (*mallpb.ListDigitalEntitlementsResponse, error) {
	f.pagedRequests = append(f.pagedRequests, request)
	return &mallpb.ListDigitalEntitlementsResponse{}, nil
}
