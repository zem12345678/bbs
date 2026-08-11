package messaging

import (
	"context"
	"encoding/json"
	"time"

	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"
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
		if err := p.service.UpsertArticle(ctx, payload.ArticleID, payload.AuthorID, payload.Title, env.OccurredAt); err != nil {
			return err
		}
		if env.EventID == "" {
			return nil
		}
		return p.service.EnqueueWebhookEvent(ctx, payload.AuthorID, domain.WebhookEventNote, env.EventID, map[string]any{
			"note": map[string]any{"id": payload.ArticleID, "title": payload.Title, "authorId": payload.AuthorID, "type": "article"},
		}, env.OccurredAt)
	case "topic.published.v1":
		var payload topicPublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		if err := p.service.UpsertTopic(ctx, payload.TopicID, payload.AuthorID, payload.Title, env.OccurredAt); err != nil {
			return err
		}
		if env.EventID == "" {
			return nil
		}
		return p.service.EnqueueWebhookEvent(ctx, payload.AuthorID, domain.WebhookEventNote, env.EventID, map[string]any{
			"note": map[string]any{"id": payload.TopicID, "title": payload.Title, "authorId": payload.AuthorID, "type": "topic"},
		}, env.OccurredAt)
	case "content.qa.accepted.v1":
		var payload qaAcceptedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		eventID := env.EventID
		if payload.EventID != "" {
			eventID = payload.EventID
		}
		return p.service.NotifyQAAccepted(ctx, eventID, payload.TopicID, payload.Title, payload.QuestionAuthorID, payload.AcceptedCommentID, payload.AcceptedCommentAuthorID, payload.RewardCredits, env.OccurredAt)
	default:
		return nil
	}
}

func (p *Projector) HandleUser(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "user.followed", "user.unfollowed", "user.follow_requested", "user.follow_request_accepted":
	default:
		return nil
	}
	var payload followPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	switch env.EventType {
	case "user.followed":
		if err := p.service.NotifyFollow(ctx, env.EventID, payload.FollowerID, payload.FolloweeID, env.OccurredAt); err != nil {
			return err
		}
		if env.EventID == "" {
			return nil
		}
		if err := p.service.EnqueueWebhookEvent(ctx, payload.FollowerID, domain.WebhookEventFollow, env.EventID, map[string]any{
			"user": map[string]any{"id": payload.FolloweeID},
		}, env.OccurredAt); err != nil {
			return err
		}
		return p.service.EnqueueWebhookEvent(ctx, payload.FolloweeID, domain.WebhookEventFollowed, env.EventID, map[string]any{
			"user": map[string]any{"id": payload.FollowerID},
		}, env.OccurredAt)
	case "user.unfollowed":
		if env.EventID == "" {
			return nil
		}
		return p.service.EnqueueWebhookEvent(ctx, payload.FollowerID, domain.WebhookEventUnfollow, env.EventID, map[string]any{
			"user": map[string]any{"id": payload.FolloweeID},
		}, env.OccurredAt)
	case "user.follow_requested":
		return p.service.NotifyFollowRequestReceived(ctx, env.EventID, payload.FollowerID, payload.FolloweeID, env.OccurredAt)
	case "user.follow_request_accepted":
		return p.service.NotifyFollowRequestAccepted(ctx, env.EventID, payload.FollowerID, payload.FolloweeID, env.OccurredAt)
	default:
		return nil
	}
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
	case "mall.order.paid.v1":
		return p.handleMallOrderPaid(ctx, env)
	case "mall.order.shipped.v1", "mall.order.completed.v1":
		return p.handleMallOrderStatus(ctx, env)
	case "mall.product_review.published.v1", "mall.product_review.hidden.v1":
		return p.handleMallProductReviewStatus(ctx, env)
	case "mall.digital_entitlement.revoked.v1":
		return p.handleMallDigitalEntitlementRevoked(ctx, env)
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
	return p.service.NotifyMallRefund(ctx, eventID, approved, payload.RefundID, payload.OrderID, payload.UserID, payload.AmountCredits, payload.OrderNo, payload.Reason, payload.AdminNote, mallDigitalEntitlements(payload.DigitalEntitlements), occurredAt)
}

func (p *Projector) handleMallOrderPaid(ctx context.Context, env eventEnvelope) error {
	var payload mallOrderPaidPayload
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
	return p.service.NotifyMallOrderPaid(ctx, eventID, payload.OrderID, payload.UserID, payload.TotalCredits, payload.OrderNo, payload.PaymentMethod, mallDigitalEntitlements(payload.DigitalEntitlements), occurredAt)
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

func (p *Projector) handleMallProductReviewStatus(ctx context.Context, env eventEnvelope) error {
	var payload mallProductReviewStatusPayload
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
	published := env.EventType == "mall.product_review.published.v1"
	return p.service.NotifyMallProductReviewStatus(ctx, eventID, published, payload.ReviewID, payload.ProductID, payload.OrderID, payload.UserID, payload.ProductTitle, occurredAt)
}

func (p *Projector) handleMallDigitalEntitlementRevoked(ctx context.Context, env eventEnvelope) error {
	var payload mallDigitalEntitlementRevokedPayload
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
	return p.service.NotifyMallDigitalEntitlementRevoked(ctx, eventID, payload.EntitlementID, payload.OrderID, payload.UserID, payload.OrderNo, payload.OperatorID, payload.Reason, app.MallDigitalEntitlement{
		ProductID:       payload.ProductID,
		SKU:             payload.SKU,
		Title:           payload.Title,
		Quantity:        1,
		FulfillmentCode: payload.FulfillmentCode,
		GrantType:       payload.GrantType,
		GrantKey:        payload.GrantKey,
		Status:          payload.Status,
	}, occurredAt)
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

type qaAcceptedPayload struct {
	EventID                 string `json:"event_id"`
	TopicID                 int64  `json:"topic_id"`
	Title                   string `json:"title"`
	QuestionAuthorID        int64  `json:"question_author_id"`
	AcceptedCommentID       int64  `json:"accepted_comment_id"`
	AcceptedCommentAuthorID int64  `json:"accepted_comment_author_id"`
	RewardCredits           int64  `json:"reward_credits"`
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
	EventID             string                          `json:"event_id"`
	OccurredAtUnixMs    int64                           `json:"occurred_at_unix_ms"`
	RefundID            int64                           `json:"refund_id"`
	OrderID             int64                           `json:"order_id"`
	OrderNo             string                          `json:"order_no"`
	UserID              int64                           `json:"user_id"`
	AmountCredits       int64                           `json:"amount_credits"`
	Reason              string                          `json:"reason"`
	AdminNote           string                          `json:"admin_note"`
	DigitalEntitlements []mallDigitalEntitlementPayload `json:"digital_entitlements"`
}

type mallDigitalEntitlementPayload struct {
	ProductID       int64  `json:"product_id"`
	SKU             string `json:"sku"`
	Title           string `json:"title"`
	Quantity        int32  `json:"quantity"`
	FulfillmentCode string `json:"fulfillment_code"`
	GrantType       string `json:"grant_type"`
	GrantKey        string `json:"grant_key"`
	Status          string `json:"status"`
	RefundID        int64  `json:"refund_id"`
}

func mallDigitalEntitlements(items []mallDigitalEntitlementPayload) []app.MallDigitalEntitlement {
	if len(items) == 0 {
		return nil
	}
	entitlements := make([]app.MallDigitalEntitlement, 0, len(items))
	for _, item := range items {
		entitlements = append(entitlements, app.MallDigitalEntitlement{
			ProductID:       item.ProductID,
			SKU:             item.SKU,
			Title:           item.Title,
			Quantity:        item.Quantity,
			FulfillmentCode: item.FulfillmentCode,
			GrantType:       item.GrantType,
			GrantKey:        item.GrantKey,
			Status:          item.Status,
			RefundID:        item.RefundID,
		})
	}
	return entitlements
}

type mallOrderPaidPayload struct {
	EventID             string                          `json:"event_id"`
	OccurredAtUnixMs    int64                           `json:"occurred_at_unix_ms"`
	OrderID             int64                           `json:"order_id"`
	OrderNo             string                          `json:"order_no"`
	UserID              int64                           `json:"user_id"`
	TotalCredits        int64                           `json:"total_credits"`
	PaymentMethod       string                          `json:"payment_method"`
	PaymentID           int64                           `json:"payment_id"`
	DigitalEntitlements []mallDigitalEntitlementPayload `json:"digital_entitlements"`
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

type mallProductReviewStatusPayload struct {
	EventID          string `json:"event_id"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	ReviewID         int64  `json:"review_id"`
	ProductID        int64  `json:"product_id"`
	ProductTitle     string `json:"product_title"`
	OrderID          int64  `json:"order_id"`
	UserID           int64  `json:"user_id"`
	Status           string `json:"status"`
}

type mallDigitalEntitlementRevokedPayload struct {
	EventID          string `json:"event_id"`
	OccurredAtUnixMs int64  `json:"occurred_at_unix_ms"`
	EntitlementID    int64  `json:"entitlement_id"`
	OrderID          int64  `json:"order_id"`
	OrderNo          string `json:"order_no"`
	UserID           int64  `json:"user_id"`
	ProductID        int64  `json:"product_id"`
	SKU              string `json:"sku"`
	Title            string `json:"title"`
	FulfillmentCode  string `json:"fulfillment_code"`
	GrantType        string `json:"grant_type"`
	GrantKey         string `json:"grant_key"`
	Status           string `json:"status"`
	OperatorID       string `json:"operator_id"`
	Reason           string `json:"reason"`
}
