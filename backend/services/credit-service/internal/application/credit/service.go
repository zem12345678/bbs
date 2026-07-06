package credit

import (
	"context"
	"fmt"
	"time"

	domain "credit-service/internal/domain/credit"
)

const (
	WelcomeDelta          int64 = 20
	ArticlePublishedDelta int64 = 10
	CommentCreatedDelta   int64 = 3
	FollowDelta           int64 = 1
	LikeGivenDelta        int64 = 1
	FavoriteGivenDelta    int64 = 2
	CommentReceivedDelta  int64 = 1
	LikeReceivedDelta     int64 = 1
	FavoriteReceivedDelta int64 = 2
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetBalance(ctx context.Context, userID int64) (domain.Balance, error) {
	return s.repo.GetBalance(ctx, userID)
}

func (s *Service) ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	return s.repo.ListLedger(ctx, userID, limit, offset)
}

func (s *Service) HandleUserCreated(ctx context.Context, eventID string, userID int64, occurredAt time.Time) error {
	return s.add(ctx, eventID, userID, WelcomeDelta, "welcome", "注册奖励", "user", userID, occurredAt)
}

func (s *Service) HandleFollowed(ctx context.Context, eventID string, followerID, followeeID int64, occurredAt time.Time) error {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil
	}
	return s.add(ctx, eventID, followerID, FollowDelta, "follow_user", fmt.Sprintf("关注用户 #%d", followeeID), "user", followeeID, occurredAt)
}

func (s *Service) HandleArticlePublished(ctx context.Context, eventID string, articleID, authorID int64, title string, occurredAt time.Time) error {
	article := domain.ArticleRef{ID: articleID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveArticle(ctx, article, occurredAt); err != nil {
		return err
	}
	if err := s.add(ctx, eventID, authorID, ArticlePublishedDelta, "article_published", fmt.Sprintf("发布文章《%s》", title), "article", articleID, occurredAt); err != nil {
		return err
	}
	return s.repo.FlushPendingArticleCredits(ctx, article)
}

func (s *Service) HandleCommentCreated(ctx context.Context, eventID string, commentID, articleID, authorID int64, occurredAt time.Time) error {
	if err := s.add(ctx, eventID, authorID, CommentCreatedDelta, "comment_created", fmt.Sprintf("评论文章 #%d", articleID), "comment", commentID, occurredAt); err != nil {
		return err
	}
	return s.addArticleOwnerCredit(ctx, eventID, "article_commented", articleID, authorID, CommentReceivedDelta, "comment", commentID, occurredAt)
}

func (s *Service) HandleReaction(ctx context.Context, eventID, eventType string, articleID, actorID int64, changed bool, occurredAt time.Time) error {
	if !changed {
		return nil
	}
	switch eventType {
	case "reaction.liked.v1":
		if err := s.add(ctx, eventID, actorID, LikeGivenDelta, "like_given", fmt.Sprintf("点赞文章 #%d", articleID), "article", articleID, occurredAt); err != nil {
			return err
		}
		return s.addArticleOwnerCredit(ctx, eventID, "article_liked", articleID, actorID, LikeReceivedDelta, "article", articleID, occurredAt)
	case "reaction.favorited.v1":
		if err := s.add(ctx, eventID, actorID, FavoriteGivenDelta, "favorite_given", fmt.Sprintf("收藏文章 #%d", articleID), "article", articleID, occurredAt); err != nil {
			return err
		}
		return s.addArticleOwnerCredit(ctx, eventID, "article_favorited", articleID, actorID, FavoriteReceivedDelta, "article", articleID, occurredAt)
	default:
		return nil
	}
}

func (s *Service) addArticleOwnerCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, occurredAt time.Time) error {
	article, err := s.repo.GetArticle(ctx, articleID)
	if err != nil {
		return err
	}
	if article.AuthorID <= 0 {
		return s.repo.SavePendingArticleCredit(ctx, eventID, reason, articleID, actorID, delta, sourceType, sourceID, occurredAt)
	}
	if article.AuthorID == actorID {
		return nil
	}
	return s.add(ctx, eventID, article.AuthorID, delta, reason, ownerDescription(reason, actorID, article.Title), sourceType, sourceID, occurredAt)
}

func (s *Service) add(ctx context.Context, eventID string, userID, delta int64, reason, description, sourceType string, sourceID int64, occurredAt time.Time) error {
	return s.repo.AddCredit(ctx, domain.LedgerEntry{
		UserID:        userID,
		Delta:         delta,
		Reason:        reason,
		Description:   description,
		SourceEventID: eventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	})
}

func ownerDescription(reason string, actorID int64, title string) string {
	switch reason {
	case "article_commented":
		return fmt.Sprintf("用户 #%d 评论了你的文章《%s》", actorID, title)
	case "article_liked":
		return fmt.Sprintf("用户 #%d 点赞了你的文章《%s》", actorID, title)
	case "article_favorited":
		return fmt.Sprintf("用户 #%d 收藏了你的文章《%s》", actorID, title)
	default:
		return fmt.Sprintf("文章《%s》收到互动", title)
	}
}
