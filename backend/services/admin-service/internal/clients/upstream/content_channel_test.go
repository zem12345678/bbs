package upstream

import (
	"context"
	"testing"

	"admin/api/proto/contentpb"

	"google.golang.org/grpc"
)

func TestListChannelsMapsAdminFilterAndResponse(t *testing.T) {
	client := &recordingChannelContentClient{listResponse: &contentpb.ChannelListResponse{
		Items: []*contentpb.ChannelInfo{{
			Id: 9, OwnerId: 2, CategoryId: 3, Name: "Go", Description: "Go users", Color: "#00add8",
			FollowersCount: 10, TopicsCount: 11, LastPostedAt: 12, CreatedAt: 13, UpdatedAt: 14, IsFeatured: true,
		}},
		Total: 1,
	}}
	clients := &Clients{content: client}

	result, err := clients.ListChannels(t.Context(), " go ", 3, 2, 25, 5)
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if request := client.listRequest; request.GetQuery() != " go " || request.GetCategoryId() != 3 || request.GetArchivedStatus() != 2 || request.GetIncludeArchived() || request.GetLimit() != 25 || request.GetOffset() != 5 {
		t.Fatalf("ListChannels() request = %#v", request)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("ListChannels() result = %#v", result)
	}
	channel := result.Items[0]
	if channel.ID != 9 || channel.OwnerID != 2 || channel.CategoryID != 3 || channel.Name != "Go" || channel.FollowersCount != 10 || channel.TopicsCount != 11 || !channel.IsFeatured {
		t.Fatalf("mapped channel = %#v", channel)
	}
}

func TestListChannelsStatusZeroRequestsAllChannels(t *testing.T) {
	client := &recordingChannelContentClient{listResponse: &contentpb.ChannelListResponse{}}
	clients := &Clients{content: client}

	if _, err := clients.ListChannels(t.Context(), "", 0, 0, 20, 0); err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if !client.listRequest.GetIncludeArchived() || client.listRequest.GetArchivedStatus() != 0 {
		t.Fatalf("ListChannels() request = %#v, want include archived compatibility", client.listRequest)
	}
}

func TestSetChannelGovernanceMapsRequestsAndResponses(t *testing.T) {
	client := &recordingChannelContentClient{}
	clients := &Clients{content: client}

	featured, err := clients.SetChannelFeatured(t.Context(), 8, true)
	if err != nil {
		t.Fatalf("SetChannelFeatured() error = %v", err)
	}
	if client.featuredRequest.GetId() != 8 || !client.featuredRequest.GetFeatured() || !featured.IsFeatured {
		t.Fatalf("featured request/result = %#v / %#v", client.featuredRequest, featured)
	}

	archived, err := clients.SetChannelArchived(t.Context(), 8, true)
	if err != nil {
		t.Fatalf("SetChannelArchived() error = %v", err)
	}
	if client.archivedRequest.GetId() != 8 || !client.archivedRequest.GetArchived() || !archived.IsArchived {
		t.Fatalf("archived request/result = %#v / %#v", client.archivedRequest, archived)
	}
}

type recordingChannelContentClient struct {
	contentpb.ContentServiceClient
	listRequest     *contentpb.ListChannelsRequest
	listResponse    *contentpb.ChannelListResponse
	featuredRequest *contentpb.SetChannelFeaturedRequest
	archivedRequest *contentpb.SetChannelArchivedRequest
}

func (c *recordingChannelContentClient) ListChannels(_ context.Context, request *contentpb.ListChannelsRequest, _ ...grpc.CallOption) (*contentpb.ChannelListResponse, error) {
	c.listRequest = request
	return c.listResponse, nil
}

func (c *recordingChannelContentClient) SetChannelFeatured(_ context.Context, request *contentpb.SetChannelFeaturedRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.featuredRequest = request
	return &contentpb.ChannelResponse{Channel: &contentpb.ChannelInfo{Id: request.GetId(), IsFeatured: request.GetFeatured()}}, nil
}

func (c *recordingChannelContentClient) SetChannelArchived(_ context.Context, request *contentpb.SetChannelArchivedRequest, _ ...grpc.CallOption) (*contentpb.ChannelResponse, error) {
	c.archivedRequest = request
	return &contentpb.ChannelResponse{Channel: &contentpb.ChannelInfo{Id: request.GetId(), IsArchived: request.GetArchived()}}, nil
}
