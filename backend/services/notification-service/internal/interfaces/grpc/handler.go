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

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
