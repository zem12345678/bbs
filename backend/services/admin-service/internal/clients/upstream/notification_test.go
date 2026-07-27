package upstream

import (
	"context"
	"testing"

	"admin/api/proto/notificationpb"
	domain "admin/internal/domain/admin"

	"google.golang.org/grpc"
)

func TestDispatchSystemNotificationsMapsAdminCommandToInternalRequest(t *testing.T) {
	client := &recordingNotificationClient{response: &notificationpb.DispatchSystemNotificationsResponse{DeliveredCount: 2}}
	clients := &Clients{notification: client}

	delivered, err := clients.DispatchSystemNotifications(t.Context(), 7, domain.SystemNotificationCommand{
		RecipientIDs:   []int64{11, 22},
		Title:          "系统维护",
		Content:        "今晚维护",
		IdempotencyKey: "maintenance-20260725",
	})
	if err != nil {
		t.Fatalf("DispatchSystemNotifications() error = %v", err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want 2", delivered)
	}
	request := client.request
	if request == nil || request.GetActorId() != 7 || request.GetTitle() != "系统维护" || request.GetContent() != "今晚维护" || request.GetIdempotencyKey() != "maintenance-20260725" {
		t.Fatalf("internal request = %#v", request)
	}
	if len(request.GetRecipientIds()) != 2 || request.GetRecipientIds()[0] != 11 || request.GetRecipientIds()[1] != 22 {
		t.Fatalf("recipient ids = %v", request.GetRecipientIds())
	}
}

type recordingNotificationClient struct {
	request  *notificationpb.DispatchSystemNotificationsRequest
	response *notificationpb.DispatchSystemNotificationsResponse
	err      error
}

func (c *recordingNotificationClient) DispatchSystemNotifications(_ context.Context, request *notificationpb.DispatchSystemNotificationsRequest, _ ...grpc.CallOption) (*notificationpb.DispatchSystemNotificationsResponse, error) {
	c.request = request
	return c.response, c.err
}
