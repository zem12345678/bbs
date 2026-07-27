package upstream

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"admin/api/proto/userpb"

	"google.golang.org/grpc"
)

func TestToDomainUserMapsBackgroundURL(t *testing.T) {
	user := toDomainUser(&userpb.UserInfo{
		Id:            42,
		Username:      "alice",
		BackgroundUrl: "https://example.test/background.webp",
	})

	if user.ID != 42 {
		t.Fatalf("id = %d", user.ID)
	}
	if user.BackgroundURL != "https://example.test/background.webp" {
		t.Fatalf("background url = %q", user.BackgroundURL)
	}
}

func TestExistingUserIDsBatchesRequestsAtUpstreamLimit(t *testing.T) {
	ids := make([]int64, 101)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	client := &recordingUserClient{}
	clients := &Clients{user: client}

	existing, err := clients.ExistingUserIDs(t.Context(), ids)
	if err != nil {
		t.Fatalf("ExistingUserIDs() error = %v", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("ListUsers() calls = %d, want 2", len(client.requests))
	}
	if got := client.requests[0]; !reflect.DeepEqual(got.GetIds(), ids[:100]) || got.GetPage() != 1 || got.GetPageSize() != 100 {
		t.Fatalf("first ListUsers() request = %+v", got)
	}
	if got := client.requests[1]; !reflect.DeepEqual(got.GetIds(), ids[100:]) || got.GetPage() != 1 || got.GetPageSize() != 1 {
		t.Fatalf("second ListUsers() request = %+v", got)
	}
	if len(existing) != len(ids) {
		t.Fatalf("existing IDs = %d, want %d", len(existing), len(ids))
	}
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			t.Fatalf("existing IDs missing %d", id)
		}
	}
}

func TestExistingUserIDsReturnsUpstreamError(t *testing.T) {
	wantErr := fmt.Errorf("user service unavailable")
	client := &recordingUserClient{err: wantErr}
	clients := &Clients{user: client}

	_, err := clients.ExistingUserIDs(t.Context(), []int64{1})
	if err != wantErr {
		t.Fatalf("ExistingUserIDs() error = %v, want %v", err, wantErr)
	}
}

type recordingUserClient struct {
	userpb.UserServiceClient
	requests []*userpb.ListUsersRequest
	err      error
}

func (c *recordingUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.requests = append(c.requests, request)
	if c.err != nil {
		return nil, c.err
	}
	items := make([]*userpb.UserInfo, 0, len(request.GetIds()))
	for _, id := range request.GetIds() {
		items = append(items, &userpb.UserInfo{Id: id})
	}
	return &userpb.UserListResponse{Items: items}, nil
}
