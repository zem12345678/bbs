package upstream

import (
	"context"

	"admin/api/proto/notificationpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) DispatchSystemNotifications(ctx context.Context, actorID int64, command domain.SystemNotificationCommand) (int32, error) {
	resp, err := c.notification.DispatchSystemNotifications(ctx, &notificationpb.DispatchSystemNotificationsRequest{
		RecipientIds:   command.RecipientIDs,
		Title:          command.Title,
		Content:        command.Content,
		ActorId:        actorID,
		IdempotencyKey: command.IdempotencyKey,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetDeliveredCount(), nil
}
