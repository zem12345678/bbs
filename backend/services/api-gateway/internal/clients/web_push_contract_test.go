package clients

import (
	"testing"

	"api-gateway/api/proto/notificationpb"

	"github.com/stretchr/testify/require"
)

func TestWebPushProtoContract(t *testing.T) {
	assertProtoFields(t, (&notificationpb.WebPushConfigResponse{}).ProtoReflect().Descriptor(), []protoField{
		{name: "enabled", number: 1},
		{name: "public_key", number: 2},
	})
	assertProtoFields(t, (&notificationpb.RegisterWebPushSubscriptionRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "user_id", number: 1},
		{name: "endpoint", number: 2},
		{name: "auth", number: 3},
		{name: "public_key", number: 4},
		{name: "send_read_message", number: 5},
	})
	assertProtoFields(t, (&notificationpb.GetWebPushSubscriptionRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "user_id", number: 1},
		{name: "endpoint", number: 2},
	})
	assertProtoFields(t, (&notificationpb.UnregisterWebPushSubscriptionRequest{}).ProtoReflect().Descriptor(), []protoField{
		{name: "user_id", number: 1},
		{name: "endpoint", number: 2},
	})
	assertProtoFields(t, (&notificationpb.WebPushSubscriptionResponse{}).ProtoReflect().Descriptor(), []protoField{
		{name: "registered", number: 1},
		{name: "state", number: 2},
		{name: "user_id", number: 3},
		{name: "endpoint", number: 4},
		{name: "send_read_message", number: 5},
		{name: "created_at", number: 6},
		{name: "updated_at", number: 7},
	})

	service := notificationpb.File_notification_proto.Services().ByName("NotificationService")
	require.NotNil(t, service)
	assertProtoMethod(t, service, "GetWebPushConfig", "bbs.notification.v1.GetWebPushConfigRequest", "bbs.notification.v1.WebPushConfigResponse")
	assertProtoMethod(t, service, "RegisterWebPushSubscription", "bbs.notification.v1.RegisterWebPushSubscriptionRequest", "bbs.notification.v1.WebPushSubscriptionResponse")
	assertProtoMethod(t, service, "GetWebPushSubscription", "bbs.notification.v1.GetWebPushSubscriptionRequest", "bbs.notification.v1.WebPushSubscriptionResponse")
	assertProtoMethod(t, service, "UnregisterWebPushSubscription", "bbs.notification.v1.UnregisterWebPushSubscriptionRequest", "bbs.notification.v1.MutationResponse")
}
