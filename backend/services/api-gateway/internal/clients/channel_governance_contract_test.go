package clients

import (
	"testing"

	"api-gateway/api/proto/adminpb"
	"api-gateway/api/proto/contentpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestAdminChannelGovernanceProtoContract(t *testing.T) {
	assertProtoFields(t, (&adminpb.ChannelInfo{}).ProtoReflect().Descriptor(), []protoField{
		{name: "id", number: 1},
		{name: "owner_id", number: 2},
		{name: "category_id", number: 3},
		{name: "name", number: 4},
		{name: "description", number: 5},
		{name: "color", number: 6},
		{name: "is_archived", number: 7},
		{name: "followers_count", number: 8},
		{name: "topics_count", number: 9},
		{name: "last_posted_at", number: 10},
		{name: "created_at", number: 11},
		{name: "updated_at", number: 12},
		{name: "is_featured", number: 13},
	})
	assertProtoFields(t, (&adminpb.ListChannelsRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "query", number: 2},
		{name: "category_id", number: 3},
		{name: "archived_status", number: 4},
		{name: "limit", number: 5},
		{name: "offset", number: 6},
	})
	assertProtoFields(t, (&adminpb.ChannelFeaturedRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "id", number: 2},
		{name: "featured", number: 3},
	})
	assertProtoFields(t, (&adminpb.ChannelArchivedRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "id", number: 2},
		{name: "archived", number: 3},
	})
	assertProtoFields(t, (&adminpb.ChannelListResponse{}).ProtoReflect().Descriptor(), []protoField{
		{name: "items", number: 1},
		{name: "total", number: 2},
	})
	assertProtoFields(t, (&adminpb.ChannelResponse{}).ProtoReflect().Descriptor(), []protoField{
		{name: "success", number: 1},
		{name: "message", number: 2},
		{name: "channel", number: 3},
	})

	service := adminpb.File_api_gateway_api_proto_admin_proto.Services().ByName("AdminService")
	require.NotNil(t, service)
	assertProtoMethod(t, service, "ListChannels", "bbs.admin.v1.ListChannelsRequest", "bbs.admin.v1.ChannelListResponse")
	assertProtoMethod(t, service, "SetChannelFeatured", "bbs.admin.v1.ChannelFeaturedRequest", "bbs.admin.v1.ChannelResponse")
	assertProtoMethod(t, service, "SetChannelArchived", "bbs.admin.v1.ChannelArchivedRequest", "bbs.admin.v1.ChannelResponse")
}

func TestContentChannelGovernanceProtoContract(t *testing.T) {
	assertProtoField(t, (&contentpb.ChannelInfo{}).ProtoReflect().Descriptor(), "is_featured", 15)
	assertProtoField(t, (&contentpb.ListChannelsRequest{}).ProtoReflect().Descriptor(), "archived_status", 12)
	assertProtoFields(t, (&contentpb.SetChannelFeaturedRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "id", number: 1},
		{name: "featured", number: 2},
	})
	assertProtoFields(t, (&contentpb.SetChannelArchivedRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "id", number: 1},
		{name: "archived", number: 2},
	})

	service := contentpb.File_api_gateway_api_proto_content_proto.Services().ByName("ContentService")
	require.NotNil(t, service)
	assertProtoMethod(t, service, "SetChannelFeatured", "bbs.content.v1.SetChannelFeaturedRequest", "bbs.content.v1.ChannelResponse")
	assertProtoMethod(t, service, "SetChannelArchived", "bbs.content.v1.SetChannelArchivedRequest", "bbs.content.v1.ChannelResponse")
}

type protoField struct {
	name   protoreflect.Name
	number protoreflect.FieldNumber
}

func assertProtoFields(t *testing.T, message protoreflect.MessageDescriptor, expected []protoField) {
	t.Helper()
	require.Equal(t, len(expected), message.Fields().Len())
	for _, field := range expected {
		assertProtoField(t, message, field.name, field.number)
	}
}

func assertProtoField(t *testing.T, message protoreflect.MessageDescriptor, name protoreflect.Name, number protoreflect.FieldNumber) {
	t.Helper()
	field := message.Fields().ByName(name)
	require.NotNil(t, field, string(name))
	require.Equal(t, number, field.Number(), string(name))
}

func assertProtoMethod(t *testing.T, service protoreflect.ServiceDescriptor, name protoreflect.Name, input, output protoreflect.FullName) {
	t.Helper()
	method := service.Methods().ByName(name)
	require.NotNil(t, method, string(name))
	require.Equal(t, input, method.Input().FullName(), string(name))
	require.Equal(t, output, method.Output().FullName(), string(name))
}
