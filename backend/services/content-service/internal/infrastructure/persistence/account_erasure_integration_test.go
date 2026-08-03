package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	accountDomain "content-service/internal/domain/account"
	articleDomain "content-service/internal/domain/article"
	outboxDomain "content-service/internal/domain/outbox"
	topicDomain "content-service/internal/domain/topic"
	"content-service/migrations"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAccountErasurePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CONTENT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("BBS_CONTENT_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	migration, err := migrations.Files.ReadFile("0015_add_account_erasure.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)

	ctx := context.Background()
	seed := time.Now().UnixNano()
	targetUserID := seed
	otherUserID := seed + 1
	raceAuthorID := seed + 2
	raceVoterID := seed + 3
	jobID := seed + 100
	articleID := seed + 1_000
	topicID := seed + 1_001
	otherTopicID := seed + 1_002
	raceArticleID := seed + 1_003
	raceVoteTopicID := seed + 1_004
	contentIDs := []int64{articleID, topicID, otherTopicID, raceArticleID, raceVoteTopicID}
	userIDs := []int64{targetUserID, otherUserID, raceAuthorID, raceVoterID}

	t.Cleanup(func() {
		cleanup := db.WithContext(context.Background())
		_ = cleanup.Where("message_key IN ?", stringIDs(contentIDs)).Delete(&contentLifecycleOutboxPO{}).Error
		_ = cleanup.Where("id IN ?", []int64{articleID, raceArticleID}).Delete(&articlePO{}).Error
		_ = cleanup.Where("id IN ?", []int64{topicID, otherTopicID, raceVoteTopicID}).Delete(&topicPO{}).Error
		_ = cleanup.Where("user_id IN ?", userIDs).Delete(&contentErasedUserPO{}).Error
	})

	articleRepo := NewRepo(db)
	topicRepo := NewTopicRepo(db)
	erasureRepo := NewAccountErasureRepository(db)

	article, err := articleDomain.New(articleID, articleDomain.CreateCmd{
		Slug: fmt.Sprintf("erasure-article-%d", articleID), Title: "article", Body: "body", AuthorID: targetUserID,
	})
	require.NoError(t, err)
	article.Status = articleDomain.StatusPublished
	publishedAt := time.Now().UTC()
	article.PublishedAt = &publishedAt
	require.NoError(t, articleRepo.Create(ctx, article))

	targetTopic := accountErasurePollTopic(t, topicID, targetUserID, "target")
	otherTopic := accountErasurePollTopic(t, otherTopicID, otherUserID, "other")
	require.NoError(t, topicRepo.CreateTopic(ctx, targetTopic))
	require.NoError(t, topicRepo.CreateTopic(ctx, otherTopic))
	_, err = topicRepo.VoteTopicPoll(ctx, targetTopic.ID, targetUserID, []int32{0}, time.Now())
	require.NoError(t, err)
	_, err = topicRepo.VoteTopicPoll(ctx, otherTopic.ID, targetUserID, []int32{1}, time.Now())
	require.NoError(t, err)

	result, err := erasureRepo.ArchiveAccountContent(ctx, targetUserID, jobID, 3)
	require.NoError(t, err)
	require.EqualValues(t, 1, result.ArchivedArticles)
	require.EqualValues(t, 1, result.ArchivedTopics)
	require.EqualValues(t, 2, result.DeletedPollBallots)
	require.Equal(t, []string{article.Slug}, result.ArticleSlugs)
	assertAccountContentErased(t, ctx, db, targetUserID, articleID, topicID)
	assertPollVoteCount(t, ctx, db, targetTopic.ID, 0, 0)
	assertPollVoteCount(t, ctx, db, otherTopic.ID, 1, 0)
	assertContentLifecycleEvent(t, ctx, db, articleID, "article.archived.v1")
	assertContentLifecycleEvent(t, ctx, db, topicID, "topic.archived.v1")

	var firstErasedAt time.Time
	require.NoError(t, db.WithContext(ctx).Model(&contentErasedUserPO{}).Select("erased_at").Where("user_id = ?", targetUserID).Scan(&firstErasedAt).Error)
	repeated, err := erasureRepo.ArchiveAccountContent(ctx, targetUserID, jobID, 3)
	require.NoError(t, err)
	require.Equal(t, result.ArchivedArticles, repeated.ArchivedArticles)
	require.Equal(t, result.ArchivedTopics, repeated.ArchivedTopics)
	require.Equal(t, result.DeletedPollBallots, repeated.DeletedPollBallots)
	var repeatedErasedAt time.Time
	require.NoError(t, db.WithContext(ctx).Model(&contentErasedUserPO{}).Select("erased_at").Where("user_id = ?", targetUserID).Scan(&repeatedErasedAt).Error)
	require.True(t, firstErasedAt.Equal(repeatedErasedAt), "exact replay changed erased_at")
	assertOutboxCount(t, ctx, db, []int64{articleID, topicID}, 2)

	lateArticle, err := articleDomain.New(seed+2_000, articleDomain.CreateCmd{Slug: fmt.Sprintf("late-article-%d", seed), Title: "late", Body: "body", AuthorID: targetUserID})
	require.NoError(t, err)
	require.ErrorIs(t, articleRepo.Create(ctx, lateArticle), accountDomain.ErrUserErased)
	lateTopic := accountErasurePollTopic(t, seed+2_001, targetUserID, "late")
	require.ErrorIs(t, topicRepo.CreateTopic(ctx, lateTopic), accountDomain.ErrUserErased)
	_, err = topicRepo.VoteTopicPoll(ctx, otherTopic.ID, targetUserID, []int32{0}, time.Now())
	require.ErrorIs(t, err, accountDomain.ErrUserErased)
	require.ErrorIs(t, articleRepo.UpdateStatus(ctx, articleID, articleDomain.StatusPublished, &publishedAt), articleDomain.ErrNotFound)
	require.ErrorIs(t, topicRepo.UpdateTopicStatus(ctx, topicID, topicDomain.StatusPublished, &publishedAt), topicDomain.ErrNotFound)
	lateEventID := fmt.Sprintf("late-erasure-%d", articleID)
	require.ErrorIs(t, articleRepo.UpdateStatusWithOutbox(ctx, articleID, articleDomain.StatusPublished, &publishedAt, time.Now(), outboxDomain.LifecycleEvent{
		EventID: lateEventID, MessageKey: fmt.Sprint(articleID), EventType: "article.published.v1", Payload: []byte(`{"event_id":"late"}`),
	}), articleDomain.ErrNotFound)
	assertOutboxEventIDCount(t, ctx, db, lateEventID, 0)
	assertAccountContentErased(t, ctx, db, targetUserID, articleID, topicID)

	assertConcurrentAccountCreateErasure(t, ctx, db, articleRepo, erasureRepo, raceAuthorID, raceArticleID, jobID+1)
	assertConcurrentAccountVoteErasure(t, ctx, db, topicRepo, erasureRepo, raceVoterID, otherUserID, raceVoteTopicID, jobID+2)
}

func accountErasurePollTopic(t *testing.T, id, authorID int64, label string) *topicDomain.Topic {
	t.Helper()
	expiresAt := time.Now().Add(time.Hour)
	topic, err := topicDomain.New(id, topicDomain.CreateCmd{
		Slug: fmt.Sprintf("erasure-topic-%s-%d", label, id), Title: label, Body: "body", AuthorID: authorID,
		Poll: &topicDomain.PollInput{Enabled: true, Choices: []string{"first", "second"}, ExpiresAt: &expiresAt},
	})
	require.NoError(t, err)
	topic.Status = topicDomain.StatusPublished
	publishedAt := time.Now().UTC()
	topic.PublishedAt = &publishedAt
	return topic
}

func assertConcurrentAccountCreateErasure(t *testing.T, ctx context.Context, db *gorm.DB, articleRepo *Repo, erasureRepo *AccountErasureRepository, userID, articleID, jobID int64) {
	t.Helper()
	article, err := articleDomain.New(articleID, articleDomain.CreateCmd{
		Slug: fmt.Sprintf("race-erasure-article-%d", articleID), Title: "race", Body: "body", AuthorID: userID,
	})
	require.NoError(t, err)
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- articleRepo.Create(ctx, article)
	}()
	go func() {
		defer workers.Done()
		<-start
		_, eraseErr := erasureRepo.ArchiveAccountContent(ctx, userID, jobID, 3)
		errorsChannel <- eraseErr
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for operationErr := range errorsChannel {
		if operationErr != nil && !errors.Is(operationErr, accountDomain.ErrUserErased) {
			t.Fatalf("concurrent create/erasure error = %v", operationErr)
		}
	}
	assertDatabaseCount(t, ctx, db, 0, `SELECT COUNT(*) FROM articles WHERE id = ? AND status <> ?`, articleID, int32(articleDomain.StatusArchived))
	assertDatabaseCount(t, ctx, db, 1, `SELECT COUNT(*) FROM content_erased_users WHERE user_id = ?`, userID)
}

func assertConcurrentAccountVoteErasure(t *testing.T, ctx context.Context, db *gorm.DB, topicRepo *TopicRepo, erasureRepo *AccountErasureRepository, voterID, ownerID, topicID, jobID int64) {
	t.Helper()
	topic := accountErasurePollTopic(t, topicID, ownerID, "race-vote")
	require.NoError(t, topicRepo.CreateTopic(ctx, topic))
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, voteErr := topicRepo.VoteTopicPoll(ctx, topicID, voterID, []int32{0}, time.Now())
		errorsChannel <- voteErr
	}()
	go func() {
		defer workers.Done()
		<-start
		_, eraseErr := erasureRepo.ArchiveAccountContent(ctx, voterID, jobID, 3)
		errorsChannel <- eraseErr
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for operationErr := range errorsChannel {
		if operationErr != nil && !errors.Is(operationErr, accountDomain.ErrUserErased) {
			t.Fatalf("concurrent vote/erasure error = %v", operationErr)
		}
	}
	assertDatabaseCount(t, ctx, db, 0, `SELECT COUNT(*) FROM topic_poll_ballots WHERE topic_id = ? AND user_id = ?`, topicID, voterID)
	assertPollVoteCount(t, ctx, db, topicID, 0, 0)
}

func assertAccountContentErased(t *testing.T, ctx context.Context, db *gorm.DB, userID, articleID, topicID int64) {
	t.Helper()
	assertDatabaseCount(t, ctx, db, 1, `SELECT COUNT(*) FROM content_erased_users WHERE user_id = ?`, userID)
	assertDatabaseCount(t, ctx, db, 1, `SELECT COUNT(*) FROM articles WHERE id = ? AND status = ?`, articleID, int32(articleDomain.StatusArchived))
	assertDatabaseCount(t, ctx, db, 1, `SELECT COUNT(*) FROM topics WHERE id = ? AND status = ?`, topicID, int32(topicDomain.StatusArchived))
	assertDatabaseCount(t, ctx, db, 0, `SELECT COUNT(*) FROM topic_poll_ballots WHERE user_id = ?`, userID)
}

func assertPollVoteCount(t *testing.T, ctx context.Context, db *gorm.DB, topicID int64, choiceIndex int32, want int64) {
	t.Helper()
	assertDatabaseCount(t, ctx, db, want, `SELECT votes_count FROM topic_poll_choices WHERE topic_id = ? AND choice_index = ?`, topicID, choiceIndex)
}

func assertContentLifecycleEvent(t *testing.T, ctx context.Context, db *gorm.DB, contentID int64, eventType string) {
	t.Helper()
	assertDatabaseCount(t, ctx, db, 1, `SELECT COUNT(*) FROM content_lifecycle_outbox WHERE message_key = ? AND event_type = ?`, fmt.Sprint(contentID), eventType)
}

func assertOutboxCount(t *testing.T, ctx context.Context, db *gorm.DB, contentIDs []int64, want int64) {
	t.Helper()
	assertDatabaseCount(t, ctx, db, want, `SELECT COUNT(*) FROM content_lifecycle_outbox WHERE message_key IN ?`, stringIDs(contentIDs))
}

func assertOutboxEventIDCount(t *testing.T, ctx context.Context, db *gorm.DB, eventID string, want int64) {
	t.Helper()
	assertDatabaseCount(t, ctx, db, want, `SELECT COUNT(*) FROM content_lifecycle_outbox WHERE event_id = ?`, eventID)
}

func assertDatabaseCount(t *testing.T, ctx context.Context, db *gorm.DB, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	require.NoError(t, db.WithContext(ctx).Raw(query, args...).Scan(&got).Error)
	require.Equal(t, want, got, "query: %s", query)
}

func stringIDs(ids []int64) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, fmt.Sprint(id))
	}
	return values
}
