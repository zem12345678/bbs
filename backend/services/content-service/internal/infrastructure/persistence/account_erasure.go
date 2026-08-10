package persistence

import (
	"context"
	"errors"
	"strconv"
	"time"

	accountDomain "content-service/internal/domain/account"
	articleDomain "content-service/internal/domain/article"
	outboxDomain "content-service/internal/domain/outbox"
	topicDomain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type contentErasedUserPO struct {
	UserID             int64 `gorm:"primaryKey"`
	JobID              int64
	PolicyVersion      int32
	ArchivedArticles   int64
	ArchivedTopics     int64
	DeletedPollBallots int64
	ErasedAt           time.Time
}

func (contentErasedUserPO) TableName() string { return "content_erased_users" }

type AccountErasureRepository struct {
	db *gorm.DB
}

func NewAccountErasureRepository(db *gorm.DB) *AccountErasureRepository {
	return &AccountErasureRepository{db: db}
}

func (r *AccountErasureRepository) ArchiveAccountContent(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (accountDomain.ErasureResult, error) {
	if r == nil || r.db == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return accountDomain.ErasureResult{}, accountDomain.ErrInvalidErasure
	}

	var result accountDomain.ErasureResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockContentUser(tx, userID); err != nil {
			return err
		}

		var receipt contentErasedUserPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&receipt, "user_id = ?", userID).Error
		switch {
		case err == nil && policyVersion <= receipt.PolicyVersion:
			result = erasureResult(receipt)
			if err := eraseAccountChannelState(tx, userID, time.Now().UTC()); err != nil {
				return err
			}
			return loadErasedArticleSlugs(tx, userID, &result)
		case err == nil:
			result = erasureResult(receipt)
			if err := tx.Model(&contentErasedUserPO{}).Where("user_id = ?", userID).Updates(map[string]any{
				"job_id":         deletionJobID,
				"policy_version": policyVersion,
			}).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			receipt = contentErasedUserPO{UserID: userID, JobID: deletionJobID, PolicyVersion: policyVersion, ErasedAt: time.Now().UTC()}
			if err := tx.Create(&receipt).Error; err != nil {
				return err
			}
		default:
			return err
		}

		now := time.Now().UTC()
		if err := eraseAccountChannelState(tx, userID, now); err != nil {
			return err
		}
		var articles []articlePO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("author_id = ? AND status <> ?", userID, int32(articleDomain.StatusArchived)).
			Find(&articles).Error; err != nil {
			return err
		}
		var topics []topicPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("author_id = ? AND status <> ?", userID, int32(topicDomain.StatusArchived)).
			Find(&topics).Error; err != nil {
			return err
		}

		for _, article := range articles {
			event := articleDomain.NewArticleArchivedEvent(&articleDomain.Article{ID: article.ID, Slug: article.Slug, UpdatedAt: now})
			payload, err := messaging.EncodeDomainEvent(ctx, event)
			if err != nil {
				return err
			}
			if err := insertContentLifecycleOutboxEvent(tx, outboxDomain.LifecycleEvent{
				EventID: event.EventID(), MessageKey: strconv.FormatInt(article.ID, 10), EventType: event.EventName(), Payload: payload,
			}); err != nil {
				return err
			}
		}
		for _, topic := range topics {
			event := topicDomain.NewTopicArchivedEvent(&topicDomain.Topic{ID: topic.ID, Slug: topic.Slug, UpdatedAt: now})
			payload, err := messaging.EncodeDomainEvent(ctx, event)
			if err != nil {
				return err
			}
			if err := insertContentLifecycleOutboxEvent(tx, outboxDomain.LifecycleEvent{
				EventID: event.EventID(), MessageKey: strconv.FormatInt(topic.ID, 10), EventType: event.EventName(), Payload: payload,
			}); err != nil {
				return err
			}
		}

		if len(articles) > 0 {
			ids := make([]int64, 0, len(articles))
			for _, article := range articles {
				ids = append(ids, article.ID)
			}
			if err := tx.Model(&articlePO{}).Where("id IN ?", ids).Updates(map[string]any{
				"status": int32(articleDomain.StatusArchived), "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		if len(topics) > 0 {
			ids := make([]int64, 0, len(topics))
			for _, topic := range topics {
				ids = append(ids, topic.ID)
			}
			if err := tx.Model(&topicPO{}).Where("id IN ?", ids).Updates(map[string]any{
				"status": int32(topicDomain.StatusArchived), "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}

		var deletedBallots int64
		if err := tx.Model(&topicPollBallotPO{}).Where("user_id = ?", userID).Count(&deletedBallots).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
WITH removed AS (
  DELETE FROM topic_poll_ballot_choices
  WHERE user_id = $1
  RETURNING topic_id, choice_index
), decrements AS (
  SELECT topic_id, choice_index, COUNT(*)::BIGINT AS amount
  FROM removed
  GROUP BY topic_id, choice_index
)
UPDATE topic_poll_choices AS choices
SET votes_count = GREATEST(0, choices.votes_count - decrements.amount)
FROM decrements
WHERE choices.topic_id = decrements.topic_id
  AND choices.choice_index = decrements.choice_index
`, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&topicPollBallotPO{}).Error; err != nil {
			return err
		}

		result.ArchivedArticles += int64(len(articles))
		result.ArchivedTopics += int64(len(topics))
		result.DeletedPollBallots += deletedBallots
		if err := tx.Model(&contentErasedUserPO{}).Where("user_id = ?", userID).Updates(map[string]any{
			"archived_articles":    result.ArchivedArticles,
			"archived_topics":      result.ArchivedTopics,
			"deleted_poll_ballots": result.DeletedPollBallots,
		}).Error; err != nil {
			return err
		}
		return loadErasedArticleSlugs(tx, userID, &result)
	})
	return result, err
}

func eraseAccountChannelState(tx *gorm.DB, userID int64, now time.Time) error {
	if err := tx.Model(&channelPO{}).
		Where("owner_id = ? AND is_archived = FALSE", userID).
		Updates(map[string]any{"is_archived": true, "is_featured": false, "updated_at": now}).Error; err != nil {
		return err
	}
	if err := tx.Table("channel_followers").Where("user_id = ?", userID).Delete(&channelRelationPO{}).Error; err != nil {
		return err
	}
	return tx.Table("channel_favorites").Where("user_id = ?", userID).Delete(&channelRelationPO{}).Error
}

func erasureResult(receipt contentErasedUserPO) accountDomain.ErasureResult {
	return accountDomain.ErasureResult{
		ArchivedArticles:   receipt.ArchivedArticles,
		ArchivedTopics:     receipt.ArchivedTopics,
		DeletedPollBallots: receipt.DeletedPollBallots,
	}
}

func loadErasedArticleSlugs(tx *gorm.DB, userID int64, result *accountDomain.ErasureResult) error {
	return tx.Model(&articlePO{}).Where("author_id = ?", userID).Order("id ASC").Pluck("slug", &result.ArticleSlugs).Error
}

func lockContentUser(tx *gorm.DB, userID int64) error {
	return tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended('bbs-content-user:' || CAST(CAST(? AS BIGINT) AS TEXT), 0))`, userID).Error
}

func contentUserErased(tx *gorm.DB, userID int64) (bool, error) {
	var count int64
	if err := tx.Model(&contentErasedUserPO{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

var _ accountDomain.ErasureRepository = (*AccountErasureRepository)(nil)
