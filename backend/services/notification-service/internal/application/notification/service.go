package notification

import (
	"context"
	"fmt"
	"time"

	domain "notification-service/internal/domain/notification"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]domain.Notification, int64, int64, error) {
	return s.repo.List(ctx, userID, limit, offset, unreadOnly)
}

func (s *Service) CountUnread(ctx context.Context, userID int64) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, id int64) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, userID int64) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *Service) UpsertArticle(ctx context.Context, articleID, authorID int64, title string, publishedAt time.Time) error {
	article := domain.ArticleRef{ID: articleID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveArticle(ctx, article, publishedAt); err != nil {
		return err
	}
	if err := s.upsertContent(ctx, "article", articleID, authorID, title, publishedAt); err != nil {
		return err
	}
	return s.repo.FlushPendingArticleNotifications(ctx, article)
}

func (s *Service) UpsertTopic(ctx context.Context, topicID, authorID int64, title string, publishedAt time.Time) error {
	return s.upsertContent(ctx, "topic", topicID, authorID, title, publishedAt)
}

func (s *Service) upsertContent(ctx context.Context, entityType string, entityID, authorID int64, title string, publishedAt time.Time) error {
	content := domain.ContentRef{EntityType: entityType, ID: entityID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveContent(ctx, content, publishedAt); err != nil {
		return err
	}
	return s.repo.FlushPendingContentNotifications(ctx, content)
}

func (s *Service) NotifyFollow(ctx context.Context, eventID string, followerID, followeeID int64, occurredAt time.Time) error {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     followeeID,
		Type:       "follow",
		Title:      "有新粉丝",
		Content:    fmt.Sprintf("用户 #%d 关注了你", followerID),
		ActorID:    followerID,
		EntityType: "user",
		EntityID:   followerID,
		SourceID:   followerID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyComment(ctx context.Context, eventID string, commentID int64, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	if commentID <= 0 || entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	content, err := s.repo.GetContent(ctx, entityType, entityID)
	if err != nil {
		return err
	}
	if content.AuthorID <= 0 {
		return s.repo.SavePendingContentNotification(ctx, eventID, "comment", entityType, entityID, actorID, commentID, occurredAt)
	}
	if content.AuthorID == actorID {
		return nil
	}
	label := contentLabel(entityType)
	return s.repo.Create(ctx, domain.Notification{
		UserID:     content.AuthorID,
		Type:       "comment",
		Title:      label + "收到新评论",
		Content:    fmt.Sprintf("用户 #%d 评论了《%s》", actorID, content.Title),
		ActorID:    actorID,
		EntityType: entityType,
		EntityID:   entityID,
		SourceID:   commentID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyReaction(ctx context.Context, eventID string, kind string, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	if entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	content, err := s.repo.GetContent(ctx, entityType, entityID)
	if err != nil {
		return err
	}
	if content.AuthorID <= 0 {
		return s.repo.SavePendingContentNotification(ctx, eventID, kind, entityType, entityID, actorID, entityID, occurredAt)
	}
	if content.AuthorID == actorID {
		return nil
	}
	label := contentLabel(entityType)
	title := label + "收到互动"
	message := fmt.Sprintf("用户 #%d 与《%s》发生了互动", actorID, content.Title)
	if kind == "like" {
		title = label + "被点赞"
		message = fmt.Sprintf("用户 #%d 点赞了《%s》", actorID, content.Title)
	}
	if kind == "favorite" {
		title = label + "被收藏"
		message = fmt.Sprintf("用户 #%d 收藏了《%s》", actorID, content.Title)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     content.AuthorID,
		Type:       kind,
		Title:      title,
		Content:    message,
		ActorID:    actorID,
		EntityType: entityType,
		EntityID:   entityID,
		SourceID:   entityID,
	}, eventID, occurredAt)
}

func supportedContentType(entityType string) bool {
	return entityType == "article" || entityType == "topic"
}

func contentLabel(entityType string) string {
	if entityType == "topic" {
		return "话题"
	}
	return "文章"
}
