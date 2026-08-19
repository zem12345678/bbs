package grpc

import (
	"context"
	"time"

	pb "notification-service/api/proto/notificationpb"
	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"
)

type Handler struct {
	pb.UnimplementedNotificationServiceServer
	pb.UnimplementedInternalNotificationServiceServer
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.ListNotificationsResponse, error) {
	items, total, unread, err := h.service.List(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset(), req.GetUnreadOnly())
	if err != nil {
		return nil, err
	}
	resp := &pb.ListNotificationsResponse{Items: make([]*pb.Notification, 0, len(items)), Total: total, UnreadCount: unread}
	for _, item := range items {
		resp.Items = append(resp.Items, toPB(item))
	}
	return resp, nil
}

func (h *Handler) ListNotificationsCompat(ctx context.Context, req *pb.ListNotificationsCompatRequest) (*pb.ListNotificationsResponse, error) {
	items, err := h.service.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{
		UserID:          req.GetUserId(),
		Limit:           req.GetLimit(),
		SinceID:         req.GetSinceId(),
		UntilID:         req.GetUntilId(),
		IncludeTypes:    req.GetIncludeTypes(),
		ExcludeTypes:    req.GetExcludeTypes(),
		IncludeTypesSet: req.GetIncludeTypesSet(),
		ExcludeTypesSet: req.GetExcludeTypesSet(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &pb.ListNotificationsResponse{Items: make([]*pb.Notification, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toPB(item))
	}
	return resp, nil
}

func (h *Handler) CountUnread(ctx context.Context, req *pb.CountUnreadRequest) (*pb.CountUnreadResponse, error) {
	count, err := h.service.CountUnread(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &pb.CountUnreadResponse{Count: count}, nil
}

func (h *Handler) MarkRead(ctx context.Context, req *pb.MarkReadRequest) (*pb.MutationResponse, error) {
	if err := h.service.MarkRead(ctx, req.GetUserId(), req.GetId()); err != nil {
		return nil, err
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) MarkAllRead(ctx context.Context, req *pb.MarkAllReadRequest) (*pb.MutationResponse, error) {
	if err := h.service.MarkAllRead(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) GetPreferences(ctx context.Context, req *pb.GetPreferencesRequest) (*pb.PreferencesResponse, error) {
	items, err := h.service.GetNotificationPreferences(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.PreferencesResponse{Items: toPBPreferences(items)}, nil
}

func (h *Handler) UpdatePreferences(ctx context.Context, req *pb.UpdatePreferencesRequest) (*pb.PreferencesResponse, error) {
	items := make([]domain.NotificationPreference, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		if item == nil {
			return nil, toStatus(domain.ErrInvalidNotificationPreferences)
		}
		items = append(items, domain.NotificationPreference{Type: item.GetType(), Enabled: item.GetEnabled()})
	}
	preferences, err := h.service.UpdateNotificationPreferences(ctx, req.GetUserId(), items)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.PreferencesResponse{Items: toPBPreferences(preferences)}, nil
}

func (h *Handler) GetWebPushConfig(context.Context, *pb.GetWebPushConfigRequest) (*pb.WebPushConfigResponse, error) {
	config := h.service.GetWebPushConfig()
	return &pb.WebPushConfigResponse{Enabled: config.Enabled, PublicKey: config.PublicKey}, nil
}

func (h *Handler) RegisterWebPushSubscription(ctx context.Context, req *pb.RegisterWebPushSubscriptionRequest) (*pb.WebPushSubscriptionResponse, error) {
	subscription, err := h.service.RegisterWebPushSubscription(ctx, domain.WebPushSubscription{
		UserID:          req.GetUserId(),
		Endpoint:        req.GetEndpoint(),
		Auth:            req.GetAuth(),
		PublicKey:       req.GetPublicKey(),
		SendReadMessage: req.GetSendReadMessage(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return toPBWebPushSubscription(subscription, true), nil
}

func (h *Handler) GetWebPushSubscription(ctx context.Context, req *pb.GetWebPushSubscriptionRequest) (*pb.WebPushSubscriptionResponse, error) {
	subscription, err := h.service.GetWebPushSubscription(ctx, req.GetUserId(), req.GetEndpoint())
	if err != nil {
		return nil, toStatus(err)
	}
	if subscription.ID == 0 {
		return &pb.WebPushSubscriptionResponse{
			Registered: false,
			State:      "unregistered",
			UserId:     req.GetUserId(),
			Endpoint:   req.GetEndpoint(),
		}, nil
	}
	return toPBWebPushSubscription(subscription, subscription.State == domain.WebPushSubscriptionStateActive), nil
}

func (h *Handler) UnregisterWebPushSubscription(ctx context.Context, req *pb.UnregisterWebPushSubscriptionRequest) (*pb.MutationResponse, error) {
	if err := h.service.UnregisterWebPushSubscription(ctx, req.GetUserId(), req.GetEndpoint()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListWebhooks(ctx context.Context, req *pb.ListWebhooksRequest) (*pb.WebhookListResponse, error) {
	items, err := h.service.ListWebhooks(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	result := &pb.WebhookListResponse{Items: make([]*pb.Webhook, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, toPBWebhook(item))
	}
	return result, nil
}

func (h *Handler) CreateWebhook(ctx context.Context, req *pb.CreateWebhookRequest) (*pb.WebhookResponse, error) {
	item, err := h.service.CreateWebhook(ctx, domain.Webhook{
		UserID: req.GetUserId(), Name: req.GetName(), URL: req.GetUrl(), Secret: req.GetSecret(), Events: req.GetOn(), Active: true,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.WebhookResponse{Webhook: toPBWebhook(item)}, nil
}

func (h *Handler) ShowWebhook(ctx context.Context, req *pb.ShowWebhookRequest) (*pb.WebhookResponse, error) {
	item, err := h.service.ShowWebhook(ctx, req.GetUserId(), req.GetWebhookId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.WebhookResponse{Webhook: toPBWebhook(item)}, nil
}

func (h *Handler) UpdateWebhook(ctx context.Context, req *pb.UpdateWebhookRequest) (*pb.WebhookResponse, error) {
	item, err := h.service.UpdateWebhook(ctx, domain.Webhook{
		ID: req.GetWebhookId(), UserID: req.GetUserId(), Name: req.GetName(), URL: req.GetUrl(), Secret: req.GetSecret(), Events: req.GetOn(), Active: req.GetActive(),
	}, req.GetActiveSet(), req.GetSecretSet())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.WebhookResponse{Webhook: toPBWebhook(item)}, nil
}

func (h *Handler) DeleteWebhook(ctx context.Context, req *pb.DeleteWebhookRequest) (*pb.MutationResponse, error) {
	if err := h.service.DeleteWebhook(ctx, req.GetUserId(), req.GetWebhookId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) TestWebhook(ctx context.Context, req *pb.TestWebhookRequest) (*pb.MutationResponse, error) {
	if err := h.service.TestWebhook(ctx, req.GetUserId(), req.GetWebhookId(), req.GetType(), req.GetOverrideUrl(), req.GetOverrideSecret(), req.GetOverrideSecretSet()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.MutationResponse{Success: true, Message: "queued"}, nil
}

func (h *Handler) DispatchSystemNotifications(ctx context.Context, req *pb.DispatchSystemNotificationsRequest) (*pb.DispatchSystemNotificationsResponse, error) {
	delivered, err := h.service.DispatchSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs:   req.GetRecipientIds(),
		Title:          req.GetTitle(),
		Content:        req.GetContent(),
		ActorID:        req.GetActorId(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.DispatchSystemNotificationsResponse{DeliveredCount: delivered}, nil
}

func (h *Handler) CreateExportCompletedNotification(ctx context.Context, req *pb.CreateExportCompletedNotificationRequest) (*pb.MutationResponse, error) {
	err := h.service.CreateExportCompletedNotification(ctx, domain.ExportCompletedNotificationCommand{
		RecipientID:    req.GetRecipientId(),
		FileID:         req.GetFileId(),
		ExportedEntity: req.GetExportedEntity(),
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) EraseUserData(ctx context.Context, req *pb.EraseUserDataRequest) (*pb.MutationResponse, error) {
	if err := h.service.EraseUserData(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.MutationResponse{Success: true, Message: "ok"}, nil
}

func toPB(item domain.Notification) *pb.Notification {
	readAt := int64(0)
	if item.ReadAt != nil {
		readAt = millis(*item.ReadAt)
	}
	return &pb.Notification{
		Id:         item.ID,
		UserId:     item.UserID,
		Type:       item.Type,
		Title:      item.Title,
		Content:    item.Content,
		ActorId:    item.ActorID,
		EntityType: item.EntityType,
		EntityId:   item.EntityID,
		SourceId:   item.SourceID,
		Read:       item.ReadAt != nil,
		CreatedAt:  millis(item.CreatedAt),
		ReadAt:     readAt,
	}
}

func toPBPreferences(items []domain.NotificationPreference) []*pb.NotificationPreference {
	preferences := make([]*pb.NotificationPreference, 0, len(items))
	for _, item := range items {
		preferences = append(preferences, &pb.NotificationPreference{Type: item.Type, Enabled: item.Enabled})
	}
	return preferences
}

func toPBWebPushSubscription(subscription domain.WebPushSubscription, registered bool) *pb.WebPushSubscriptionResponse {
	state := subscription.RegistrationState
	if state == "" {
		if registered {
			state = "subscribed"
		} else {
			state = "unregistered"
		}
	}
	return &pb.WebPushSubscriptionResponse{
		Registered:      registered,
		State:           state,
		UserId:          subscription.UserID,
		Endpoint:        subscription.Endpoint,
		SendReadMessage: subscription.SendReadMessage,
		CreatedAt:       millis(subscription.CreatedAt),
		UpdatedAt:       millis(subscription.UpdatedAt),
	}
}

func toPBWebhook(item domain.Webhook) *pb.Webhook {
	latestSentAt := int64(0)
	if item.LatestSentAt != nil {
		latestSentAt = millis(*item.LatestSentAt)
	}
	latestStatus := int32(0)
	if item.LatestStatus != nil {
		latestStatus = *item.LatestStatus
	}
	return &pb.Webhook{
		Id: item.ID, UserId: item.UserID, Name: item.Name, Url: item.URL, Secret: item.Secret, On: append([]string(nil), item.Events...), Active: item.Active,
		LatestSentAt: latestSentAt, LatestStatus: latestStatus, CreatedAt: millis(item.CreatedAt), UpdatedAt: millis(item.UpdatedAt),
	}
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
