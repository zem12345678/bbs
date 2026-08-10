package grpc

import (
	"testing"

	pb "admin/api/proto/adminpb"
	"admin/api/proto/contentpb"
	domain "admin/internal/domain/admin"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestAdminChannelProtoContract(t *testing.T) {
	service := pb.File_admin_service_api_proto_admin_proto.Services().ByName("AdminService")
	assertChannelMethod(t, service, "ListChannels", "bbs.admin.v1.ListChannelsRequest", "bbs.admin.v1.ChannelListResponse")
	assertChannelMethod(t, service, "SetChannelFeatured", "bbs.admin.v1.ChannelFeaturedRequest", "bbs.admin.v1.ChannelResponse")
	assertChannelMethod(t, service, "SetChannelArchived", "bbs.admin.v1.ChannelArchivedRequest", "bbs.admin.v1.ChannelResponse")

	assertFieldNumbers(t, (&pb.ChannelInfo{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"id": 1, "owner_id": 2, "category_id": 3, "name": 4, "description": 5, "color": 6,
		"is_archived": 7, "followers_count": 8, "topics_count": 9, "last_posted_at": 10,
		"created_at": 11, "updated_at": 12, "is_featured": 13,
	})
	assertFieldNumbers(t, (&pb.ListChannelsRequest{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"actor": 1, "query": 2, "category_id": 3, "archived_status": 4, "limit": 5, "offset": 6,
	})
	assertFieldNumbers(t, (&pb.ChannelFeaturedRequest{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"actor": 1, "id": 2, "featured": 3,
	})
	assertFieldNumbers(t, (&pb.ChannelArchivedRequest{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"actor": 1, "id": 2, "archived": 3,
	})

	contentService := contentpb.File_admin_service_api_proto_content_proto.Services().ByName("ContentService")
	assertChannelMethod(t, contentService, "ListChannels", "bbs.content.v1.ListChannelsRequest", "bbs.content.v1.ChannelListResponse")
	assertChannelMethod(t, contentService, "SetChannelFeatured", "bbs.content.v1.SetChannelFeaturedRequest", "bbs.content.v1.ChannelResponse")
	assertChannelMethod(t, contentService, "SetChannelArchived", "bbs.content.v1.SetChannelArchivedRequest", "bbs.content.v1.ChannelResponse")
	assertFieldNumbers(t, (&contentpb.ChannelInfo{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"is_featured": 15,
	})
	assertFieldNumbers(t, (&contentpb.ListChannelsRequest{}).ProtoReflect().Descriptor(), map[protoreflect.Name]protoreflect.FieldNumber{
		"archived_status": 12,
	})
}

func TestToPbChannelMapsGovernanceFields(t *testing.T) {
	channel := toPbChannel(domain.Channel{ID: 1, OwnerID: 2, CategoryID: 3, Name: "Go", IsArchived: true, FollowersCount: 4, TopicsCount: 5, IsFeatured: true})
	if channel.GetId() != 1 || channel.GetOwnerId() != 2 || channel.GetCategoryId() != 3 || channel.GetName() != "Go" || !channel.GetIsArchived() || channel.GetFollowersCount() != 4 || channel.GetTopicsCount() != 5 || !channel.GetIsFeatured() {
		t.Fatalf("toPbChannel() = %#v", channel)
	}
}

func assertChannelMethod(t *testing.T, service protoreflect.ServiceDescriptor, name protoreflect.Name, input protoreflect.FullName, output protoreflect.FullName) {
	t.Helper()
	method := service.Methods().ByName(name)
	if method == nil {
		t.Fatalf("AdminService.%s is missing", name)
	}
	if method.Input().FullName() != input || method.Output().FullName() != output {
		t.Fatalf("AdminService.%s = %s -> %s, want %s -> %s", name, method.Input().FullName(), method.Output().FullName(), input, output)
	}
}

func assertFieldNumbers(t *testing.T, message protoreflect.MessageDescriptor, want map[protoreflect.Name]protoreflect.FieldNumber) {
	t.Helper()
	for name, number := range want {
		field := message.Fields().ByName(name)
		if field == nil || field.Number() != number {
			t.Fatalf("%s.%s field number = %v, want %d", message.FullName(), name, field, number)
		}
	}
}
