package messaging

import (
	"context"
	"encoding/json"

	app "notification-service/internal/application/notification"
)

type Projector struct {
	service *app.Service
}

func NewProjector(service *app.Service) *Projector {
	return &Projector{service: service}
}

func (p *Projector) HandleArticle(ctx context.Context, env eventEnvelope) error {
	if env.EventType != "article.published.v1" {
		return nil
	}
	var payload articlePublishedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	return p.service.UpsertArticle(ctx, payload.ArticleID, payload.AuthorID, payload.Title, env.OccurredAt)
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
	if payload.EntityType != "article" {
		return nil
	}
	return p.service.NotifyComment(ctx, env.EventID, payload.CommentID, payload.EntityID, payload.AuthorID, env.OccurredAt)
}

func (p *Projector) HandleReaction(ctx context.Context, env eventEnvelope) error {
	var payload reactionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if payload.EntityType != "article" || !payload.Changed {
		return nil
	}
	switch env.EventType {
	case "reaction.liked.v1":
		return p.service.NotifyReaction(ctx, env.EventID, "like", payload.EntityID, payload.UserID, env.OccurredAt)
	case "reaction.favorited.v1":
		return p.service.NotifyReaction(ctx, env.EventID, "favorite", payload.EntityID, payload.UserID, env.OccurredAt)
	default:
		return nil
	}
}

type articlePublishedPayload struct {
	ArticleID int64  `json:"article_id"`
	Title     string `json:"title"`
	AuthorID  int64  `json:"author_id"`
}

type followPayload struct {
	FollowerID int64 `json:"follower_id"`
	FolloweeID int64 `json:"followee_id"`
}

type commentCreatedPayload struct {
	CommentID  int64  `json:"comment_id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	AuthorID   int64  `json:"author_id"`
}

type reactionPayload struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	UserID     int64  `json:"user_id"`
	Changed    bool   `json:"changed"`
}
