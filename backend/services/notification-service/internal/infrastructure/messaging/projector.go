package messaging

import (
	"context"
	"encoding/json"
	"time"

	app "notification-service/internal/application/notification"
)

type Projector struct {
	service *app.Service
}

func NewProjector(service *app.Service) *Projector {
	return &Projector{service: service}
}

func (p *Projector) HandleArticle(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "article.published.v1":
		var payload articlePublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.UpsertArticle(ctx, payload.ArticleID, payload.AuthorID, payload.Title, env.OccurredAt)
	case "topic.published.v1":
		var payload topicPublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.UpsertTopic(ctx, payload.TopicID, payload.AuthorID, payload.Title, env.OccurredAt)
	default:
		return nil
	}
}

func (p *Projector) HandleUser(ctx context.Context, env eventEnvelope) error {
	if env.EventType != "user.followed" {
		return nil
	}
	var payload followPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	return p.service.NotifyFollow(ctx, env.EventID, payload.FollowerID, payload.FolloweeID, env.OccurredAt)
}

func (p *Projector) HandleComment(ctx context.Context, env eventEnvelope) error {
	if env.EventType != "comment.created" {
		return nil
	}
	var payload commentCreatedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	return p.service.NotifyComment(ctx, env.EventID, payload.CommentID, payload.ParentID, payload.EntityType, payload.EntityID, payload.AuthorID, env.OccurredAt)
}

func (p *Projector) HandleReaction(ctx context.Context, env eventEnvelope) error {
	var payload reactionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if !payload.Changed {
		return nil
	}
	switch env.EventType {
	case "reaction.liked.v1":
		return p.service.NotifyReaction(ctx, env.EventID, "like", payload.EntityType, payload.EntityID, payload.UserID, env.OccurredAt)
	case "reaction.favorited.v1":
		return p.service.NotifyReaction(ctx, env.EventID, "favorite", payload.EntityType, payload.EntityID, payload.UserID, env.OccurredAt)
	default:
		return nil
	}
}

func (p *Projector) HandleMall(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "mall.refund.approved.v1", "mall.refund.rejected.v1":
		return p.handleMallRefund(ctx, env)
	case "mall.order.shipped.v1", "mall.order.completed.v1":
		return p.handleMallOrderStatus(ctx, env)
	default:
		return nil
	}
}

func (p *Projector) handleMallRefund(ctx context.Context, env eventEnvelope) error {
	var payload mallRefundReviewedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	occurredAt := env.OccurredAt
	if occurredAt.IsZero() && payload.OccurredAtUnixMs > 0 {
		occurredAt = time.UnixMilli(payload.OccurredAtUnixMs)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	eventID := env.EventID
	if eventID == "" {
		eventID = payload.EventID
	}
	approved := env.EventType == "mall.refund.approved.v1"
	return p.service.NotifyMallRefund(ctx, eventID, approved, payload.RefundID, payload.OrderID, payload.UserID, payload.AmountCredits, payload.OrderNo, payload.Reason, payload.AdminNote, occurredAt)
}

func (p *Projector) handleMallOrderStatus(ctx context.Context, env eventEnvelope) error {
	var payload mallOrderStatusUpdatedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	occurredAt := env.OccurredAt
	if occurredAt.IsZero() && payload.OccurredAtUnixMs > 0 {
		occurredAt = time.UnixMilli(payload.OccurredAtUnixMs)
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	eventID := env.EventID
	if eventID == "" {
		eventID = payload.EventID
	}
	shipped := env.EventType == "mall.order.shipped.v1"
	return p.service.NotifyMallOrderStatus(ctx, eventID, shipped, payload.OrderID, payload.UserID, payload.OrderNo, payload.ShippingCarrier, payload.TrackingNo, payload.Note, occurredAt)
}

type articlePublishedPayload struct {
	ArticleID int64  `json:"article_id"`
	Title     string `json:"title"`
	AuthorID  int64  `json:"author_id"`
}

type topicPublishedPayload struct {
	TopicID  int64  `json:"topic_id"`
	Title    string `json:"title"`
	AuthorID int64  `json:"author_id"`
}

type followPayload struct {
	FollowerID int64 `json:"follower_id"`
	FolloweeID int64 `json:"followee_id"`
}

type commentCreatedPayload struct {
	CommentID  int64  `json:"comment_id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	RootID     int64  `json:"root_id"`
	ParentID   int64  `json:"parent_id"`
	AuthorID   int64  `json:"author_id"`
}

type reactionPayload struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	UserID     int64  `json:"user_id"`
	Changed    bool   `json:"changed"`
}

type mallRefundReviewedPayload struct {
	EventID          string `json:"event_id"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	RefundID         int64  `json:"refund_id"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	AmountCredits    int64  `json:"amount_credits"`
	Reason           string `json:"reason"`
	AdminNote        string `json:"admin_note"`
}

type mallOrderStatusUpdatedPayload struct {
	EventID          string `json:"event_id"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	Status           string `json:"status"`
	ShippingCarrier  string `json:"shipping_carrier"`
	TrackingNo       string `json:"tracking_no"`
	Note             string `json:"note"`
}
