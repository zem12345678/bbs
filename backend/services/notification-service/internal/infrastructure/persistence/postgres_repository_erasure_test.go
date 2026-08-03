package persistence

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEraseUserDataCleansAndBlocksDelayedProjection(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed notification repository tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	defer pool.Close()
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	seed := time.Now().UnixNano() % 100_000_000
	targetUserID := int64(8_200_000_000) + seed
	otherUserID := targetUserID + 1
	activeActorID := targetUserID + 2
	activeRecipientID := targetUserID + 3
	concurrentUserID := targetUserID + 4
	missingUserID := targetUserID + 5
	concurrentContentUserID := targetUserID + 6
	jobID := targetUserID + 100
	const policyVersion int32 = 3
	prefix := fmt.Sprintf("erase-%d-", targetUserID)
	targetArticleID := targetUserID + 1_000
	otherArticleID := targetUserID + 1_001
	targetTopicID := targetUserID + 1_002
	otherTopicID := targetUserID + 1_003
	targetCommentID := targetUserID + 1_004
	otherCommentID := targetUserID + 1_005
	lateContentID := targetUserID + 1_006
	lateArticleID := targetUserID + 1_007
	lateCommentID := targetUserID + 1_008
	activeCommentOnErasedContentID := targetUserID + 1_009
	concurrentContentID := targetUserID + 1_010

	userIDs := []int64{targetUserID, otherUserID, activeActorID, activeRecipientID, concurrentUserID, missingUserID, concurrentContentUserID}
	entityIDs := []int64{
		targetArticleID, otherArticleID, targetTopicID, otherTopicID,
		targetCommentID, otherCommentID, lateContentID, lateArticleID, lateCommentID,
		activeCommentOnErasedContentID, concurrentContentID,
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE source_event_id LIKE $1 OR user_id = ANY($2::BIGINT[]) OR actor_id = ANY($2::BIGINT[])`, prefix+"%", userIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM pending_article_notifications WHERE event_id LIKE $1`, prefix+"%")
		_, _ = pool.Exec(context.Background(), `DELETE FROM pending_content_notifications WHERE event_id LIKE $1`, prefix+"%")
		_, _ = pool.Exec(context.Background(), `DELETE FROM pending_reply_notifications WHERE event_id LIKE $1`, prefix+"%")
		_, _ = pool.Exec(context.Background(), `DELETE FROM article_authors WHERE article_id = ANY($1::BIGINT[])`, entityIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM content_refs WHERE entity_id = ANY($1::BIGINT[])`, entityIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM comment_refs WHERE comment_id = ANY($1::BIGINT[])`, entityIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_content WHERE entity_id = ANY($1::BIGINT[])`, entityIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_comments WHERE comment_id = ANY($1::BIGINT[])`, entityIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM notification_erased_users WHERE user_id = ANY($1::BIGINT[])`, userIDs)
	})

	seedUserErasureFixtures(t, ctx, pool, prefix, targetUserID, otherUserID, targetArticleID, otherArticleID, targetTopicID, otherTopicID, targetCommentID, otherCommentID)

	if err := repo.EraseUserData(ctx, targetUserID, jobID, policyVersion); err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	var firstErasedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT erased_at FROM notification_erased_users WHERE user_id = $1`, targetUserID).Scan(&firstErasedAt); err != nil {
		t.Fatalf("load first erasure receipt: %v", err)
	}
	if err := repo.EraseUserData(ctx, targetUserID, jobID, policyVersion); err != nil {
		t.Fatalf("repeated EraseUserData() error = %v", err)
	}
	var repeatedErasedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT erased_at FROM notification_erased_users WHERE user_id = $1`, targetUserID).Scan(&repeatedErasedAt); err != nil {
		t.Fatalf("load repeated erasure receipt: %v", err)
	}
	if !repeatedErasedAt.Equal(firstErasedAt) {
		t.Fatalf("exact erasure replay changed erased_at: first=%s repeated=%s", firstErasedAt, repeatedErasedAt)
	}
	if err := repo.EraseUserData(ctx, missingUserID, jobID+1, policyVersion); err != nil {
		t.Fatalf("EraseUserData() for missing projection error = %v", err)
	}

	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notification_erased_users WHERE user_id = $1 AND job_id = $2 AND policy_version = $3`, targetUserID, jobID, policyVersion)
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notification_erased_users WHERE user_id = $1`, missingUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 OR actor_id = $1`, targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM article_authors WHERE author_id = $1`, targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM content_refs WHERE author_id = $1`, targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM comment_refs WHERE author_id = $1`, targetUserID)
	assertPostgresCount(t, ctx, pool, 2, `SELECT COUNT(*) FROM notification_erased_content WHERE owner_user_id = $1 AND job_id = $2 AND policy_version = $3`, targetUserID, jobID, policyVersion)
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notification_erased_comments WHERE author_user_id = $1 AND job_id = $2 AND policy_version = $3`, targetUserID, jobID, policyVersion)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_article_notifications WHERE event_id LIKE $1 AND event_id NOT LIKE $2`, prefix+"%", prefix+"article-other%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_content_notifications WHERE event_id LIKE $1 AND event_id NOT LIKE $2`, prefix+"%", prefix+"content-other%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_reply_notifications WHERE event_id LIKE $1 AND event_id NOT LIKE $2`, prefix+"%", prefix+"reply-other%")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notifications WHERE source_event_id = $1`, prefix+"notification-other")

	assertDelayedWritesBlocked(t, ctx, pool, repo, prefix, targetUserID, otherUserID, activeActorID, activeRecipientID, targetArticleID, targetTopicID, targetCommentID, lateContentID, lateArticleID, lateCommentID, activeCommentOnErasedContentID, otherTopicID, otherArticleID, otherCommentID)
	assertConcurrentEraseWins(t, ctx, pool, repo, prefix, concurrentUserID, activeActorID, jobID+2, policyVersion)
	assertConcurrentContentEraseWins(t, ctx, pool, repo, prefix, concurrentContentUserID, activeActorID, concurrentContentID, jobID+3, policyVersion)
}

func seedUserErasureFixtures(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	prefix string,
	targetUserID, otherUserID, targetArticleID, otherArticleID, targetTopicID, otherTopicID, targetCommentID, otherCommentID int64,
) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO article_authors(article_id, author_id, title) VALUES($1, $2, 'target'), ($3, $4, 'other')`, []any{targetArticleID, targetUserID, otherArticleID, otherUserID}},
		{`INSERT INTO content_refs(entity_type, entity_id, author_id, title) VALUES
('article', $1, $2, 'target article'), ('article', $3, $4, 'other article'),
('topic', $5, $2, 'target topic'), ('topic', $6, $4, 'other topic')`, []any{targetArticleID, targetUserID, otherArticleID, otherUserID, targetTopicID, otherTopicID}},
		{`INSERT INTO comment_refs(comment_id, entity_type, entity_id, author_id, parent_id) VALUES
($1, 'topic', $2, $3, 0), ($4, 'topic', $5, $6, 0)`, []any{targetCommentID, targetTopicID, targetUserID, otherCommentID, otherTopicID, otherUserID}},
		{`INSERT INTO notifications(user_id, type, title, content, actor_id, source_event_id) VALUES
($1, 'test', 'target inbox', 'target inbox', $2, $3),
($2, 'test', 'target actor', 'target actor', $1, $4),
($2, 'test', 'unrelated', 'unrelated', 0, $5)`, []any{targetUserID, otherUserID, prefix + "notification-recipient", prefix + "notification-actor", prefix + "notification-other"}},
		{`INSERT INTO pending_article_notifications(event_id, type, article_id, actor_id) VALUES
($1, 'like', $2, $3), ($4, 'like', $5, $6), ($7, 'like', $5, $3)`, []any{prefix + "article-target-actor", otherArticleID, targetUserID, prefix + "article-target-owner", targetArticleID, otherUserID, prefix + "article-other"}},
		{`INSERT INTO pending_content_notifications(event_id, type, entity_type, entity_id, actor_id) VALUES
($1, 'like', 'topic', $2, $3), ($4, 'like', 'topic', $5, $6), ($7, 'like', 'topic', $2, $6)`, []any{prefix + "content-target-actor", otherTopicID, targetUserID, prefix + "content-target-owner", targetTopicID, otherUserID, prefix + "content-other"}},
		{`INSERT INTO pending_reply_notifications(event_id, parent_comment_id, comment_id, entity_type, entity_id, actor_id) VALUES
($1, $2, $3, 'topic', $4, $5), ($6, $7, $8, 'topic', $9, $10), ($11, $2, $12, 'topic', $4, $10)`, []any{
			prefix + "reply-target-actor", otherCommentID, targetCommentID + 100, otherTopicID, targetUserID,
			prefix + "reply-target-owner", targetCommentID, targetCommentID + 101, targetTopicID, otherUserID,
			prefix + "reply-other", targetCommentID + 102,
		}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed erasure fixture: %v", err)
		}
	}
}

func assertDelayedWritesBlocked(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *PostgresRepository,
	prefix string,
	targetUserID, otherUserID, activeActorID, activeRecipientID,
	targetArticleID, targetTopicID, targetCommentID,
	lateContentID, lateArticleID, lateCommentID, activeCommentOnErasedContentID,
	otherTopicID, otherArticleID, otherCommentID int64,
) {
	t.Helper()
	now := time.Now()
	for label, err := range map[string]error{
		"content": repo.SaveContent(ctx, domain.ContentRef{EntityType: "topic", ID: lateContentID, AuthorID: targetUserID, Title: "late"}, now),
		"article": repo.SaveArticle(ctx, domain.ArticleRef{ID: lateArticleID, AuthorID: targetUserID, Title: "late"}, now),
		"comment": repo.SaveComment(ctx, domain.CommentRef{ID: lateCommentID, EntityType: "topic", EntityID: otherTopicID, AuthorID: targetUserID}, now),
		"active content on erased id": repo.SaveContent(ctx, domain.ContentRef{
			EntityType: "topic", ID: targetTopicID, AuthorID: activeActorID, Title: "must not revive",
		}, now),
		"active article on erased id": repo.SaveArticle(ctx, domain.ArticleRef{
			ID: targetArticleID, AuthorID: activeActorID, Title: "must not revive",
		}, now),
		"active comment on erased id": repo.SaveComment(ctx, domain.CommentRef{
			ID: targetCommentID, EntityType: "topic", EntityID: otherTopicID, AuthorID: activeActorID,
		}, now),
		"active comment on erased content": repo.SaveComment(ctx, domain.CommentRef{
			ID: activeCommentOnErasedContentID, EntityType: "topic", EntityID: targetTopicID, AuthorID: activeActorID,
		}, now),
		"pending article": repo.SavePendingArticleNotification(ctx, prefix+"late-pending-article", "like", otherArticleID, targetUserID, otherArticleID, now),
		"pending content": repo.SavePendingContentNotification(ctx, prefix+"late-pending-content", "like", "topic", otherTopicID, targetUserID, otherTopicID, now),
		"pending reply":   repo.SavePendingReplyNotification(ctx, prefix+"late-pending-reply", otherCommentID, lateCommentID, "topic", otherTopicID, targetUserID, now),
		"active pending article on erased id": repo.SavePendingArticleNotification(
			ctx, prefix+"active-erased-article", "like", targetArticleID, activeActorID, targetArticleID, now,
		),
		"active pending content on erased id": repo.SavePendingContentNotification(
			ctx, prefix+"active-erased-content", "like", "topic", targetTopicID, activeActorID, targetTopicID, now,
		),
		"active pending reply on erased id": repo.SavePendingReplyNotification(
			ctx, prefix+"active-erased-reply", targetCommentID, activeCommentOnErasedContentID, "topic", targetTopicID, activeActorID, now,
		),
		"notification for erased content": repo.Create(ctx, domain.Notification{
			UserID: activeRecipientID, Type: "like", Title: "must not deliver", ActorID: activeActorID,
			EntityType: "topic", EntityID: targetTopicID, SourceID: targetTopicID,
		}, prefix+"active-erased-content-create", now),
		"notification for erased comment": repo.Create(ctx, domain.Notification{
			UserID: activeRecipientID, Type: "reply", Title: "must not deliver", ActorID: activeActorID,
			EntityType: "topic", EntityID: otherTopicID, SourceID: targetCommentID,
		}, prefix+"active-erased-comment-create", now),
		"recipient notification": repo.Create(ctx, domain.Notification{UserID: targetUserID, Type: "test", Title: "late", ActorID: activeActorID}, prefix+"late-recipient", now),
		"actor notification":     repo.Create(ctx, domain.Notification{UserID: otherUserID, Type: "test", Title: "late", ActorID: targetUserID}, prefix+"late-actor", now),
	} {
		if err != nil {
			t.Fatalf("%s delayed write: %v", label, err)
		}
	}

	delivered, err := repo.CreateSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs: []int64{activeRecipientID}, Title: "late", Content: "late",
		ActorID: targetUserID, IdempotencyKey: prefix + "erased-actor",
	}, now)
	if err != nil || delivered != 0 {
		t.Fatalf("system notification from erased actor delivered=%d err=%v", delivered, err)
	}
	delivered, err = repo.CreateSystemNotifications(ctx, domain.SystemNotificationCommand{
		RecipientIDs: []int64{targetUserID, activeRecipientID}, Title: "active", Content: "active",
		ActorID: activeActorID, IdempotencyKey: prefix + "active-actor",
	}, now)
	if err != nil || delivered != 1 {
		t.Fatalf("system notification to mixed recipients delivered=%d err=%v", delivered, err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO pending_content_notifications(event_id, type, entity_type, entity_id, actor_id) VALUES($1, 'like', 'topic', $2, $3)`, prefix+"flush-erased-recipient", lateContentID, activeActorID); err != nil {
		t.Fatalf("seed stale recipient flush: %v", err)
	}
	if err := repo.FlushPendingContentNotifications(ctx, domain.ContentRef{EntityType: "topic", ID: lateContentID, AuthorID: targetUserID, Title: "late"}); err != nil {
		t.Fatalf("flush erased recipient: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pending_article_notifications(event_id, type, article_id, actor_id) VALUES($1, 'like', $2, $3)`, prefix+"flush-erased-actor", otherArticleID, targetUserID); err != nil {
		t.Fatalf("seed stale actor flush: %v", err)
	}
	if err := repo.FlushPendingArticleNotifications(ctx, domain.ArticleRef{ID: otherArticleID, AuthorID: otherUserID, Title: "other"}); err != nil {
		t.Fatalf("flush erased actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pending_content_notifications(event_id, type, entity_type, entity_id, actor_id) VALUES($1, 'like', 'topic', $2, $3)`, prefix+"flush-erased-content", targetTopicID, activeActorID); err != nil {
		t.Fatalf("seed erased content flush: %v", err)
	}
	if err := repo.FlushPendingContentNotifications(ctx, domain.ContentRef{EntityType: "topic", ID: targetTopicID, AuthorID: activeRecipientID, Title: "erased"}); err != nil {
		t.Fatalf("flush erased content: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pending_article_notifications(event_id, type, article_id, actor_id) VALUES($1, 'like', $2, $3)`, prefix+"flush-erased-article", targetArticleID, activeActorID); err != nil {
		t.Fatalf("seed erased article flush: %v", err)
	}
	if err := repo.FlushPendingArticleNotifications(ctx, domain.ArticleRef{ID: targetArticleID, AuthorID: activeRecipientID, Title: "erased"}); err != nil {
		t.Fatalf("flush erased article: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pending_reply_notifications(event_id, parent_comment_id, comment_id, entity_type, entity_id, actor_id) VALUES($1, $2, $3, 'topic', $4, $5)`, prefix+"flush-erased-comment", targetCommentID, activeCommentOnErasedContentID, otherTopicID, activeActorID); err != nil {
		t.Fatalf("seed erased comment flush: %v", err)
	}
	if err := repo.FlushPendingReplyNotifications(ctx, domain.CommentRef{ID: targetCommentID, EntityType: "topic", EntityID: otherTopicID, AuthorID: activeRecipientID}); err != nil {
		t.Fatalf("flush erased comment: %v", err)
	}

	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM content_refs WHERE entity_id = $1`, lateContentID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM article_authors WHERE article_id = $1`, lateArticleID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM comment_refs WHERE comment_id = $1`, lateCommentID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM content_refs WHERE entity_type = 'topic' AND entity_id = $1`, targetTopicID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM article_authors WHERE article_id = $1`, targetArticleID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM comment_refs WHERE comment_id IN ($1, $2)`, targetCommentID, activeCommentOnErasedContentID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notifications WHERE source_event_id LIKE $1`, prefix+"active-erased-%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_article_notifications WHERE event_id LIKE $1`, prefix+"active-erased-%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_content_notifications WHERE event_id LIKE $1`, prefix+"active-erased-%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_reply_notifications WHERE event_id LIKE $1`, prefix+"active-erased-%")
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notifications WHERE source_event_id LIKE $1 AND (user_id = $2 OR actor_id = $2)`, prefix+"%", targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_article_notifications WHERE event_id LIKE $1 AND actor_id = $2`, prefix+"%", targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_content_notifications WHERE event_id LIKE $1 AND actor_id = $2`, prefix+"%", targetUserID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_reply_notifications WHERE event_id LIKE $1 AND actor_id = $2`, prefix+"%", targetUserID)
}

func assertConcurrentEraseWins(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *PostgresRepository, prefix string, userID, actorID, jobID int64, policyVersion int32) {
	t.Helper()
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- repo.Create(ctx, domain.Notification{UserID: userID, Type: "test", Title: "racing", ActorID: actorID}, prefix+"concurrent", time.Now())
	}()
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- repo.EraseUserData(ctx, userID, jobID, policyVersion)
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent erasure operation: %v", err)
		}
	}
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID)
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notification_erased_users WHERE user_id = $1`, userID)
}

func assertConcurrentContentEraseWins(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *PostgresRepository,
	prefix string,
	ownerUserID, actorID, contentID, jobID int64,
	policyVersion int32,
) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO content_refs(entity_type, entity_id, author_id, title) VALUES('topic', $1, $2, 'racing')`, contentID, ownerUserID); err != nil {
		t.Fatalf("seed concurrent content erasure: %v", err)
	}

	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- repo.SavePendingContentNotification(ctx, prefix+"concurrent-content", "like", "topic", contentID, actorID, contentID, time.Now())
	}()
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- repo.EraseUserData(ctx, ownerUserID, jobID, policyVersion)
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent content erasure operation: %v", err)
		}
	}

	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM content_refs WHERE entity_type = 'topic' AND entity_id = $1`, contentID)
	assertPostgresCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM pending_content_notifications WHERE event_id = $1`, prefix+"concurrent-content")
	assertPostgresCount(t, ctx, pool, 1, `SELECT COUNT(*) FROM notification_erased_content WHERE entity_type = 'topic' AND entity_id = $1 AND owner_user_id = $2`, contentID, ownerUserID)
}

func assertPostgresCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&got); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d; query=%s", got, want, query)
	}
}
