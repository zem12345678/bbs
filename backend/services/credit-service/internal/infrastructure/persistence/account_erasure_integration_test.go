package persistence

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountErasurePostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_CREDIT_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_CREDIT_POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure credit schema: %v", err)
	}

	seed := time.Now().UnixNano()
	userID := seed
	otherUserID := seed + 1
	jobID := seed + 100
	prefix := fmt.Sprintf("integration-credit:%d:", seed)
	now := time.Now().UTC().Truncate(time.Microsecond)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		var anonymizedUserID int64
		_ = pool.QueryRow(cleanupCtx, `SELECT anonymized_user_id FROM credit_erased_users WHERE user_id = $1`, userID).Scan(&anonymizedUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM pending_article_credits WHERE actor_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM article_authors WHERE author_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM published_topics WHERE author_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM created_comments WHERE author_id = $1`, userID)
		if anonymizedUserID > 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_ledger WHERE user_id = $1`, anonymizedUserID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_reservations WHERE user_id = $1`, anonymizedUserID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_balances WHERE user_id = $1`, anonymizedUserID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM check_ins WHERE user_id = $1`, anonymizedUserID)
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_ledger WHERE user_id = $1`, otherUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_balances WHERE user_id = $1`, otherUserID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM credit_erased_users WHERE user_id = $1`, userID)
	})

	if err := repo.AddCredit(ctx, domain.LedgerEntry{UserID: userID, Delta: 20, Reason: "integration", Description: "用户身份应被擦除", SourceEventID: prefix + "welcome", SourceType: "user", SourceID: userID, CreatedAt: now}); err != nil {
		t.Fatalf("seed credit: %v", err)
	}
	if err := repo.SaveArticle(ctx, domain.ArticleRef{ID: seed + 10, AuthorID: userID, Title: "private title"}, now); err != nil {
		t.Fatalf("seed article: %v", err)
	}
	if err := repo.SavePublishedTopic(ctx, domain.TopicPublicationRef{ID: seed + 11, AuthorID: userID, Title: "private topic"}, now); err != nil {
		t.Fatalf("seed topic: %v", err)
	}
	if err := repo.SaveCreatedComment(ctx, domain.CommentCreationRef{ID: seed + 12, AuthorID: userID, EntityType: "article", EntityID: seed + 10}, now); err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	if _, _, _, _, err := repo.RecordCheckIn(ctx, domain.CheckIn{UserID: userID, LatestDay: "2026-08-04"}, domain.LedgerEntry{UserID: userID, Delta: 5, Reason: "integration_checkin", Description: "签到", SourceEventID: prefix + "checkin", SourceType: "check_in", CreatedAt: now}); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
	if _, _, _, err := repo.ReserveCredit(ctx, domain.CreditReservation{UserID: userID, Amount: 3, Status: "ACTIVE", Reason: "integration_reservation", Description: "reserve", SourceEventID: prefix + "reservation", SourceType: "mall", CreatedAt: now}, domain.LedgerEntry{UserID: userID, Delta: -3, Reason: "integration_reservation", Description: "reserve", SourceEventID: prefix + "reservation", SourceType: "mall", CreatedAt: now}); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}
	if err := repo.AddCredit(ctx, domain.LedgerEntry{UserID: otherUserID, Delta: 1, Reason: "integration", SourceEventID: prefix + "other", CreatedAt: now}); err != nil {
		t.Fatalf("seed other credit: %v", err)
	}

	result, err := repo.EraseUserData(ctx, userID, jobID, 3)
	if err != nil {
		t.Fatalf("erase account: %v", err)
	}
	if result.AnonymizedLedgerEntries < 3 || result.AnonymizedReservations != 1 || result.DeletedCheckIns != 1 {
		t.Fatalf("erase result = %+v", result)
	}
	var anonymizedUserID int64
	if err := pool.QueryRow(ctx, `SELECT anonymized_user_id FROM credit_erased_users WHERE user_id = $1`, userID).Scan(&anonymizedUserID); err != nil {
		t.Fatalf("read anonymized identity: %v", err)
	}
	assertCreditCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1`, userID)
	assertCreditCount(t, ctx, pool, result.AnonymizedLedgerEntries, `SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1 AND description = 'account-erased'`, anonymizedUserID)
	assertCreditCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM credit_reservations WHERE user_id = $1 AND status = 'ACTIVE'`, anonymizedUserID)
	assertCreditCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM article_authors WHERE author_id = $1`, userID)
	assertCreditCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM published_topics WHERE author_id = $1`, userID)
	assertCreditCount(t, ctx, pool, 0, `SELECT COUNT(*) FROM created_comments WHERE author_id = $1`, userID)
	var balanceTotal int64
	if err := pool.QueryRow(ctx, `SELECT total FROM credit_balances WHERE user_id = $1`, anonymizedUserID).Scan(&balanceTotal); err != nil {
		t.Fatalf("read anonymized balance: %v", err)
	}
	if balanceTotal != 0 {
		t.Fatalf("anonymized balance = %d, want 0", balanceTotal)
	}
	if err := repo.AddCredit(ctx, domain.LedgerEntry{UserID: userID, Delta: 1, Reason: "late", SourceEventID: prefix + "late", CreatedAt: now}); err == nil {
		t.Fatal("late credit write succeeded")
	}
	if err := repo.AddCredit(ctx, domain.LedgerEntry{UserID: anonymizedUserID, Delta: 1, Reason: "late-anonymized", SourceEventID: prefix + "late-anonymized", CreatedAt: now}); err == nil {
		t.Fatal("late anonymized credit write succeeded")
	}

	replayed, err := repo.EraseUserData(ctx, userID, jobID, 3)
	if err != nil || replayed != result {
		t.Fatalf("same-policy replay = %+v, error = %v", replayed, err)
	}
	higher, err := repo.EraseUserData(ctx, userID, jobID+1, 4)
	if err != nil || higher != result {
		t.Fatalf("higher-policy replay = %+v, error = %v", higher, err)
	}
}

func assertCreditCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := pool.QueryRow(ctx, query, args...).Scan(&got); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d (query: %s)", got, want, query)
	}
}
