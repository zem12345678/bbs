package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/api/proto/notificationpb"
	"api-gateway/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListNotificationsSerializesEntityIDsAsStrings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const maxID = int64(9223372036854775807)
	client := &notificationPayloadClient{response: &notificationpb.ListNotificationsResponse{
		Items: []*notificationpb.Notification{{
			Id: maxID, UserId: maxID - 1, Type: "export_completed", Title: "done", Content: "ready",
			ActorId: maxID - 2, EntityType: "file", EntityId: maxID - 3, SourceId: maxID - 4,
			CreatedAt: 1800000000000,
		}},
		Total: 1, UnreadCount: 1,
	}}
	h := NewHandler(&clients.Clients{Notification: client}, "Authorization", "Bearer", testJWTSecret)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(stdhttp.MethodGet, "/api/v1/notifications", nil)
	c.Set("user_id", maxID-1)

	h.listNotifications(c)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	item := envelope.Data.Items[0]
	require.Equal(t, "9223372036854775807", item["id"])
	require.Equal(t, "9223372036854775806", item["user_id"])
	require.Equal(t, "9223372036854775805", item["actor_id"])
	require.Equal(t, "9223372036854775804", item["entity_id"])
	require.Equal(t, "9223372036854775803", item["source_id"])
}

type notificationPayloadClient struct {
	notificationpb.NotificationServiceClient
	response *notificationpb.ListNotificationsResponse
}

func (c *notificationPayloadClient) ListNotifications(context.Context, *notificationpb.ListNotificationsRequest, ...grpc.CallOption) (*notificationpb.ListNotificationsResponse, error) {
	return c.response, nil
}
