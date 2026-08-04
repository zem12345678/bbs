package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	accountDomain "reaction-service/internal/domain/account"
	reactionDomain "reaction-service/internal/domain/reaction"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAccountErasurePostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_REACTION_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_REACTION_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	likeRepo := NewPostgresLikeRepository(db)
	favoriteRepo := NewPostgresFavoriteRepository(db)
	collectionRepo := NewPostgresCollectionRepository(db)
	reportRepo := NewPostgresReportRepository(db)
	erasureRepo := NewAccountErasureRepository(db)
	require.NoError(t, likeRepo.EnsureSchema(ctx))
	require.NoError(t, favoriteRepo.EnsureSchema(ctx))
	require.NoError(t, collectionRepo.EnsureSchema(ctx))
	require.NoError(t, reportRepo.EnsureSchema(ctx))
	require.NoError(t, erasureRepo.EnsureSchema(ctx))

	seed := time.Now().UnixNano()
	targetUserID := seed
	otherUserID := seed + 1
	collisionUserID := seed + 2
	raceLikeUserID := seed + 3
	raceCollectionUserID := seed + 4
	userIDs := []int64{targetUserID, otherUserID, collisionUserID, raceLikeUserID, raceCollectionUserID}
	article := reactionDomain.EntityRef{Type: reactionDomain.EntityArticle, ID: seed + 100}
	topic := reactionDomain.EntityRef{Type: reactionDomain.EntityTopic, ID: seed + 101}
	otherArticle := reactionDomain.EntityRef{Type: reactionDomain.EntityArticle, ID: seed + 102}
	raceLikeRef := reactionDomain.EntityRef{Type: reactionDomain.EntityArticle, ID: seed + 103}
	entityIDs := []int64{article.ID, topic.ID, otherArticle.ID, raceLikeRef.ID}

	t.Cleanup(func() {
		cleanup := db.WithContext(context.Background())
		_ = cleanup.Where("entity_id IN ?", entityIDs).Delete(&reportPO{}).Error
		_ = cleanup.Where("user_id IN ?", userIDs).Delete(&collectionPO{}).Error
		_ = cleanup.Where("user_id IN ?", userIDs).Delete(&favoritePO{}).Error
		_ = cleanup.Where("user_id IN ?", userIDs).Delete(&likePO{}).Error
		_ = cleanup.Where("user_id IN ?", userIDs).Delete(&reactionErasedUserPO{}).Error
	})

	_, _, err = likeRepo.Like(ctx, article, targetUserID)
	require.NoError(t, err)
	_, _, err = likeRepo.Like(ctx, topic, targetUserID)
	require.NoError(t, err)
	_, _, err = likeRepo.Unlike(ctx, topic, targetUserID)
	require.NoError(t, err)
	_, _, err = favoriteRepo.Favorite(ctx, article, targetUserID)
	require.NoError(t, err)
	_, _, err = favoriteRepo.Favorite(ctx, topic, targetUserID)
	require.NoError(t, err)
	_, _, err = favoriteRepo.Unfavorite(ctx, topic, targetUserID)
	require.NoError(t, err)

	targetCollection := &reactionDomain.Collection{UserID: targetUserID, Name: "Erasure target"}
	require.NoError(t, collectionRepo.CreateCollection(ctx, targetCollection))
	changed, err := collectionRepo.AddCollectionItem(ctx, targetUserID, targetCollection.ID, article)
	require.NoError(t, err)
	require.True(t, changed)
	otherCollection := &reactionDomain.Collection{UserID: otherUserID, Name: "Other collection"}
	require.NoError(t, collectionRepo.CreateCollection(ctx, otherCollection))

	targetReport := newIntegrationReport(t, article, targetUserID)
	created, err := reportRepo.CreateReport(ctx, targetReport)
	require.NoError(t, err)
	require.True(t, created)
	handledReport := newIntegrationReport(t, topic, otherUserID)
	created, err = reportRepo.CreateReport(ctx, handledReport)
	require.NoError(t, err)
	require.True(t, created)
	_, err = reportRepo.AuditReport(ctx, handledReport.ID, reactionDomain.ReportStatusResolved, targetUserID, "resolved", "")
	require.NoError(t, err)
	collisionReport := newIntegrationReport(t, article, collisionUserID)
	created, err = reportRepo.CreateReport(ctx, collisionReport)
	require.NoError(t, err)
	require.True(t, created)

	result, err := erasureRepo.EraseAccountReactions(ctx, targetUserID, seed+1000, 3)
	require.NoError(t, err)
	require.Equal(t, accountDomain.ErasureResult{
		DeletedLikes: 2, DeletedFavorites: 2, DeletedCollections: 1, AnonymizedReports: 1, AnonymizedHandledReports: 1,
	}, result)
	assertReactionAccountErased(t, ctx, db, targetUserID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT reporter_id FROM user_reports WHERE id = ?`, targetReport.ID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT handled_by FROM user_reports WHERE id = ?`, handledReport.ID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT COUNT(*) FROM favorite_collection_items WHERE collection_id = ?`, targetCollection.ID)

	var firstReceipt reactionErasedUserPO
	require.NoError(t, db.WithContext(ctx).First(&firstReceipt, "user_id = ?", targetUserID).Error)
	repeated, err := erasureRepo.EraseAccountReactions(ctx, targetUserID, seed+1000, 3)
	require.NoError(t, err)
	require.Equal(t, result, repeated)
	var repeatedReceipt reactionErasedUserPO
	require.NoError(t, db.WithContext(ctx).First(&repeatedReceipt, "user_id = ?", targetUserID).Error)
	require.True(t, firstReceipt.ErasedAt.Equal(repeatedReceipt.ErasedAt), "exact replay changed erased_at")

	upgraded, err := erasureRepo.EraseAccountReactions(ctx, targetUserID, seed+1001, 4)
	require.NoError(t, err)
	require.Equal(t, result, upgraded)
	var upgradedReceipt reactionErasedUserPO
	require.NoError(t, db.WithContext(ctx).First(&upgradedReceipt, "user_id = ?", targetUserID).Error)
	require.Equal(t, int32(4), upgradedReceipt.PolicyVersion)
	require.Equal(t, seed+1001, upgradedReceipt.DeletionJobID)
	require.True(t, firstReceipt.ErasedAt.Equal(upgradedReceipt.ErasedAt), "policy upgrade changed erased_at")

	_, _, err = likeRepo.Like(ctx, otherArticle, targetUserID)
	require.ErrorIs(t, err, accountDomain.ErrUserErased)
	_, _, err = favoriteRepo.Favorite(ctx, otherArticle, targetUserID)
	require.ErrorIs(t, err, accountDomain.ErrUserErased)
	require.ErrorIs(t, collectionRepo.CreateCollection(ctx, &reactionDomain.Collection{UserID: targetUserID, Name: "Late"}), accountDomain.ErrUserErased)
	_, err = collectionRepo.AddCollectionItem(ctx, targetUserID, otherCollection.ID, otherArticle)
	require.ErrorIs(t, err, accountDomain.ErrUserErased)
	lateReport := newIntegrationReport(t, otherArticle, targetUserID)
	_, err = reportRepo.CreateReport(ctx, lateReport)
	require.ErrorIs(t, err, accountDomain.ErrUserErased)
	_, err = reportRepo.AuditReport(ctx, collisionReport.ID, reactionDomain.ReportStatusResolved, targetUserID, "late", "")
	require.ErrorIs(t, err, accountDomain.ErrUserErased)

	_, err = erasureRepo.EraseAccountReactions(ctx, collisionUserID, seed+1002, 3)
	require.NoError(t, err)
	assertDatabaseValue(t, ctx, db, 2, `SELECT COUNT(*) FROM user_reports WHERE entity_type = ? AND entity_id = ? AND reporter_id = 0 AND status = ?`, string(article.Type), article.ID, int32(reactionDomain.ReportStatusPending))

	assertConcurrentLikeAndErasure(t, ctx, db, likeRepo, erasureRepo, raceLikeUserID, raceLikeRef, seed+1003)
	assertConcurrentCollectionAndErasure(t, ctx, db, collectionRepo, erasureRepo, raceCollectionUserID, seed+1004)

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	rebuilder := NewReactionCacheRebuilder(db, redisClient)
	stats, err := rebuilder.Rebuild(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.ErasedUsersLoaded, int64(4))
	exists, err := redisClient.Exists(ctx, erasedUserKey(targetUserID)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

func newIntegrationReport(t *testing.T, entity reactionDomain.EntityRef, reporterID int64) *reactionDomain.Report {
	t.Helper()
	report, err := reactionDomain.NewReport(reactionDomain.SubmitReportCmd{Entity: entity, ReporterID: reporterID, Reason: "integration"})
	require.NoError(t, err)
	return report
}

func assertReactionAccountErased(t *testing.T, ctx context.Context, db *gorm.DB, userID int64) {
	t.Helper()
	assertDatabaseValue(t, ctx, db, 1, `SELECT COUNT(*) FROM reaction_erased_users WHERE user_id = ?`, userID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT COUNT(*) FROM user_likes WHERE user_id = ?`, userID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT COUNT(*) FROM favorites WHERE user_id = ?`, userID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT COUNT(*) FROM favorite_collections WHERE user_id = ?`, userID)
	assertDatabaseValue(t, ctx, db, 0, `SELECT COUNT(*) FROM user_reports WHERE reporter_id = ? OR handled_by = ?`, userID, userID)
}

func assertConcurrentLikeAndErasure(t *testing.T, ctx context.Context, db *gorm.DB, likeRepo *PostgresLikeRepository, erasureRepo *AccountErasureRepository, userID int64, ref reactionDomain.EntityRef, jobID int64) {
	t.Helper()
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		_, _, err := likeRepo.Like(ctx, ref, userID)
		errorsChannel <- err
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := erasureRepo.EraseAccountReactions(ctx, userID, jobID, 3)
		errorsChannel <- err
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for operationErr := range errorsChannel {
		if operationErr != nil && !errors.Is(operationErr, accountDomain.ErrUserErased) {
			t.Fatalf("concurrent like/erasure error = %v", operationErr)
		}
	}
	assertReactionAccountErased(t, ctx, db, userID)
}

func assertConcurrentCollectionAndErasure(t *testing.T, ctx context.Context, db *gorm.DB, collectionRepo *PostgresCollectionRepository, erasureRepo *AccountErasureRepository, userID, jobID int64) {
	t.Helper()
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errorsChannel <- collectionRepo.CreateCollection(ctx, &reactionDomain.Collection{UserID: userID, Name: "Race"})
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := erasureRepo.EraseAccountReactions(ctx, userID, jobID, 3)
		errorsChannel <- err
	}()
	close(start)
	workers.Wait()
	close(errorsChannel)
	for operationErr := range errorsChannel {
		if operationErr != nil && !errors.Is(operationErr, accountDomain.ErrUserErased) {
			t.Fatalf("concurrent collection/erasure error = %v", operationErr)
		}
	}
	assertReactionAccountErased(t, ctx, db, userID)
}

func assertDatabaseValue(t *testing.T, ctx context.Context, db *gorm.DB, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	require.NoError(t, db.WithContext(ctx).Raw(query, args...).Scan(&got).Error)
	require.Equal(t, want, got, "query: %s", query)
}
