package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
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
		tx, cleanupErr := pool.Begin(cleanupCtx)
		if cleanupErr != nil {
			t.Errorf("begin account-erasure cleanup: %v", cleanupErr)
			return
		}
		defer func() { _ = tx.Rollback(cleanupCtx) }()
		var anonymizedUserID int64
		cleanupErr = tx.QueryRow(cleanupCtx, `SELECT anonymized_user_id FROM credit_erased_users WHERE user_id = $1`, userID).Scan(&anonymizedUserID)
		if cleanupErr != nil && !errors.Is(cleanupErr, pgx.ErrNoRows) {
			t.Errorf("read anonymized identity during cleanup: %v", cleanupErr)
			return
		}
		identityIDs := []int64{userID, otherUserID}
		if anonymizedUserID != 0 {
			identityIDs = append(identityIDs, anonymizedUserID)
		}
		cleanupStatements := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM pending_article_credits WHERE actor_id = ANY($1) OR event_id LIKE $2`, []any{identityIDs, prefix + "%"}},
			{`DELETE FROM article_authors WHERE author_id = ANY($1) OR article_id IN ($2, $3)`, []any{identityIDs, seed + 10, seed + 13}},
			{`DELETE FROM published_topics WHERE author_id = ANY($1) OR topic_id = $2`, []any{identityIDs, seed + 11}},
			{`DELETE FROM created_comments WHERE author_id = ANY($1) OR comment_id = $2`, []any{identityIDs, seed + 12}},
			{`DELETE FROM credit_ledger WHERE user_id = ANY($1) OR source_event_id LIKE $2`, []any{identityIDs, prefix + "%"}},
			{`DELETE FROM credit_reservations WHERE user_id = ANY($1) OR source_event_id LIKE $2`, []any{identityIDs, prefix + "%"}},
			{`DELETE FROM check_ins WHERE user_id = ANY($1)`, []any{identityIDs}},
			{`DELETE FROM credit_balances WHERE user_id = ANY($1)`, []any{identityIDs}},
			{`DELETE FROM credit_erased_users WHERE user_id = $1`, []any{userID}},
		}
		for _, statement := range cleanupStatements {
			if _, cleanupErr = tx.Exec(cleanupCtx, statement.query, statement.args...); cleanupErr != nil {
				t.Errorf("account-erasure cleanup failed: %v", cleanupErr)
				return
			}
		}
		if cleanupErr = tx.Commit(cleanupCtx); cleanupErr != nil {
			t.Errorf("commit account-erasure cleanup: %v", cleanupErr)
		}
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
	start := make(chan struct{})
	transferErrors := make(chan error, 2)
	for _, transfer := range []struct {
		from    int64
		to      int64
		eventID string
	}{
		{from: userID, to: otherUserID, eventID: prefix + "transfer-forward"},
		{from: otherUserID, to: userID, eventID: prefix + "transfer-reverse"},
	} {
		transfer := transfer
		go func() {
			<-start
			transferErrors <- repo.TransferCredit(ctx,
				domain.LedgerEntry{UserID: transfer.from, Delta: -1, Reason: "integration_transfer_debit", SourceEventID: transfer.eventID, CreatedAt: now},
				domain.LedgerEntry{UserID: transfer.to, Delta: 1, Reason: "integration_transfer_credit", SourceEventID: transfer.eventID, CreatedAt: now},
			)
		}()
	}
	close(start)
	for range 2 {
		if err := <-transferErrors; err != nil {
			t.Fatalf("opposite-direction transfer failed: %v", err)
		}
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
	if anonymizedUserID >= 0 {
		t.Fatalf("anonymized user ID = %d, want reserved negative identity", anonymizedUserID)
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
	if err := repo.AddCredit(ctx, domain.LedgerEntry{UserID: userID, Delta: 1, Reason: "late", SourceEventID: prefix + "late", CreatedAt: now}); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late credit write error = %v, want ErrAccountErased", err)
	}
	if err := repo.SaveArticle(ctx, domain.ArticleRef{ID: seed + 13, AuthorID: userID, Title: "late private title"}, now); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late article projection error = %v, want ErrAccountErased", err)
	}
	if _, err := repo.exec(ctx, `INSERT INTO credit_balances(user_id, total) VALUES($1, 0) ON CONFLICT(user_id) DO UPDATE SET total = EXCLUDED.total`, anonymizedUserID); !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("late anonymized credit write error = %v, want ErrAccountErased", err)
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

func TestLegacyPositivePseudonymSequenceMigrationPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_CREDIT_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_CREDIT_POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.MaxConns = 1
	config.ConnConfig.RuntimeParams["search_path"] = "pg_temp"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open isolated postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
CREATE TEMP TABLE migration_scope_init(id INTEGER);
CREATE SEQUENCE credit_erased_user_id_seq AS BIGINT START WITH 900000000000000000;
CREATE TABLE credit_erased_users (
  user_id BIGINT PRIMARY KEY,
  anonymized_user_id BIGINT NOT NULL UNIQUE,
  deletion_job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  anonymized_ledger_entries BIGINT NOT NULL DEFAULT 0,
  anonymized_reservations BIGINT NOT NULL DEFAULT 0,
  deleted_check_ins BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  CONSTRAINT credit_erased_users_identity_check CHECK (user_id > 0 AND anonymized_user_id > 0 AND deletion_job_id > 0 AND policy_version > 0),
  CONSTRAINT credit_erased_users_counts_check CHECK (anonymized_ledger_entries >= 0 AND anonymized_reservations >= 0 AND deleted_check_ins >= 0)
);
INSERT INTO credit_erased_users(user_id, anonymized_user_id, deletion_job_id, policy_version, completed_at)
VALUES(101, nextval('credit_erased_user_id_seq'), 201, 1, NOW());
`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	migrationStart := strings.Index(schemaSQL, "CREATE SEQUENCE IF NOT EXISTS credit_erased_user_id_seq")
	migrationEnd := strings.Index(schemaSQL, "CREATE OR REPLACE FUNCTION credit_reject_erased_identity()")
	if migrationStart < 0 || migrationEnd <= migrationStart {
		t.Fatal("account-erasure identity migration is missing from schemaSQL")
	}
	if _, err := pool.Exec(ctx, schemaSQL[migrationStart:migrationEnd]); err != nil {
		t.Fatalf("migrate legacy credit schema: %v", err)
	}

	var preservedID int64
	if err := pool.QueryRow(ctx, `SELECT anonymized_user_id FROM credit_erased_users WHERE user_id = 101`).Scan(&preservedID); err != nil {
		t.Fatalf("read preserved legacy pseudonym: %v", err)
	}
	if preservedID != 900000000000000000 {
		t.Fatalf("legacy pseudonym = %d, want preserved positive value", preservedID)
	}
	var nextID int64
	if err := pool.QueryRow(ctx, `SELECT nextval('credit_erased_user_id_seq')`).Scan(&nextID); err != nil {
		t.Fatalf("read migrated sequence: %v", err)
	}
	if nextID != -1 {
		t.Fatalf("first migrated pseudonym = %d, want -1", nextID)
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
