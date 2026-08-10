package clients

import (
	"testing"

	"api-gateway/api/proto/adminpb"

	"github.com/stretchr/testify/require"
)

func TestAdminAdProtoContract(t *testing.T) {
	assertProtoFields(t, (&adminpb.AdInfo{}).ProtoReflect().Descriptor(), []protoField{
		{name: "id", number: 1},
		{name: "url", number: 2},
		{name: "memo", number: 3},
		{name: "place", number: 4},
		{name: "priority", number: 5},
		{name: "ratio", number: 6},
		{name: "starts_at", number: 7},
		{name: "expires_at", number: 8},
		{name: "image_url", number: 9},
		{name: "day_of_week", number: 10},
		{name: "created_at", number: 11},
		{name: "updated_at", number: 12},
	})
	assertProtoFields(t, (&adminpb.ActiveAdInfo{}).ProtoReflect().Descriptor(), []protoField{
		{name: "id", number: 1},
		{name: "url", number: 2},
		{name: "place", number: 3},
		{name: "ratio", number: 4},
		{name: "image_url", number: 5},
		{name: "day_of_week", number: 6},
	})
	assertProtoFields(t, (&adminpb.ListAdsRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "limit", number: 2},
		{name: "since_id", number: 3},
		{name: "until_id", number: 4},
		{name: "publishing", number: 5},
	})
	assertProtoFields(t, (&adminpb.CreateAdRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "url", number: 2},
		{name: "memo", number: 3},
		{name: "place", number: 4},
		{name: "priority", number: 5},
		{name: "ratio", number: 6},
		{name: "starts_at", number: 7},
		{name: "expires_at", number: 8},
		{name: "image_url", number: 9},
		{name: "day_of_week", number: 10},
	})
	assertProtoFields(t, (&adminpb.UpdateAdRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "actor", number: 1},
		{name: "id", number: 2},
		{name: "url", number: 3},
		{name: "memo", number: 4},
		{name: "place", number: 5},
		{name: "priority", number: 6},
		{name: "ratio", number: 7},
		{name: "starts_at", number: 8},
		{name: "expires_at", number: 9},
		{name: "image_url", number: 10},
		{name: "day_of_week", number: 11},
	})

	service := adminpb.File_api_gateway_api_proto_admin_proto.Services().ByName("AdminService")
	require.NotNil(t, service)
	assertProtoMethod(t, service, "ListAds", "bbs.admin.v1.ListAdsRequest", "bbs.admin.v1.AdListResponse")
	assertProtoMethod(t, service, "ListActiveAds", "bbs.admin.v1.ListActiveAdsRequest", "bbs.admin.v1.ActiveAdListResponse")
	assertProtoMethod(t, service, "CreateAd", "bbs.admin.v1.CreateAdRequest", "bbs.admin.v1.AdResponse")
	assertProtoMethod(t, service, "UpdateAd", "bbs.admin.v1.UpdateAdRequest", "bbs.admin.v1.AdResponse")
	assertProtoMethod(t, service, "DeleteAd", "bbs.admin.v1.AdIDRequest", "bbs.admin.v1.SimpleResponse")
}
