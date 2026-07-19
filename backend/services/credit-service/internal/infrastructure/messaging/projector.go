package messaging

import (
	"context"
	"encoding/json"

	app "credit-service/internal/application/credit"
)

type Projector struct {
	service *app.Service
}

func NewProjector(service *app.Service) *Projector {
	return &Projector{service: service}
}

func (p *Projector) HandleUser(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "user.created":
		var payload userPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.HandleUserCreated(ctx, env.EventID, payload.UserID, env.OccurredAt)
	case "user.followed":
		var payload followPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.HandleFollowed(ctx, env.EventID, payload.FollowerID, payload.FolloweeID, env.OccurredAt)
	default:
		return nil
	}
}

func (p *Projector) HandleArticle(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "article.published.v1":
		var payload articlePublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.HandleArticlePublished(ctx, env.EventID, payload.ArticleID, payload.AuthorID, payload.Title, env.OccurredAt)
	case "topic.published.v1":
		var payload topicPublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return p.service.HandleTopicPublished(ctx, payload.TopicID, payload.AuthorID, payload.Title, env.OccurredAt)
	case "content.qa.accepted.v1":
		var payload qaAcceptedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		eventID := env.EventID
		if payload.EventID != "" {
			eventID = payload.EventID
		}
		return p.service.HandleQAAcceptedWithCycle(ctx, eventID, payload.TopicID, payload.Title, payload.QuestionAuthorID, payload.AcceptedCommentID, payload.AcceptedCommentAuthorID, payload.RewardCredits, payload.AcceptanceCycle, env.OccurredAt)
	default:
		return nil
	}
}

func (p *Projector) HandleComment(ctx context.Context, env eventEnvelope) error {
	if env.EventType != "comment.created" {
		return nil
	}
	var payload commentPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	return p.service.HandleCommentCreated(ctx, env.EventID, payload.CommentID, payload.EntityType, payload.EntityID, payload.AuthorID, env.OccurredAt)
}

func (p *Projector) HandleReaction(ctx context.Context, env eventEnvelope) error {
	var payload reactionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if payload.EntityType != "article" {
		return nil
	}
	return p.service.HandleReaction(ctx, env.EventID, env.EventType, payload.EntityID, payload.UserID, payload.Changed, env.OccurredAt)
}

type userPayload struct {
	UserID int64 `json:"user_id"`
}

type followPayload struct {
	FollowerID int64 `json:"follower_id"`
	FolloweeID int64 `json:"followee_id"`
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
	AcceptanceCycle         int64  `json:"acceptance_cycle"`
}

type commentPayload struct {
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
