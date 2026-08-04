package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool             *pgxpool.Pool
	leaderboardCache *RedisLeaderboardCache
}

type leaderboardSyncPlan struct {
	expectedRevision int64
	revision         int64
	entries          []domain.LeaderboardEntry
}

func NewPostgresRepository(pool *pgxpool.Pool, leaderboardCaches ...*RedisLeaderboardCache) *PostgresRepository {
	repo := &PostgresRepository{pool: pool}
	if len(leaderboardCaches) > 0 {
		repo.leaderboardCache = leaderboardCaches[0]
	}
	return repo
}

type normalizedTx struct {
	pgx.Tx
}

type normalizedRow struct {
	pgx.Row
}

func (r *PostgresRepository) exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tag, err := r.pool.Exec(ctx, sql, arguments...)
	return tag, normalizePostgresError(err)
}

func (r *PostgresRepository) begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, normalizePostgresError(err)
	}
	return normalizedTx{Tx: tx}, nil
}

func (r *PostgresRepository) beginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := r.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, normalizePostgresError(err)
	}
	return normalizedTx{Tx: tx}, nil
}

func (tx normalizedTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, sql, arguments...)
	return tag, normalizePostgresError(err)
}

func (tx normalizedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Tx.Query(ctx, sql, args...)
	return rows, normalizePostgresError(err)
}

func (tx normalizedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return normalizedRow{Row: tx.Tx.QueryRow(ctx, sql, args...)}
}

func (tx normalizedTx) Commit(ctx context.Context) error {
	return normalizePostgresError(tx.Tx.Commit(ctx))
}

func (row normalizedRow) Scan(dest ...any) error {
	return normalizePostgresError(row.Row.Scan(dest...))
}

func normalizePostgresError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "P0001" && pgErr.Message == "credit account erased" {
		return domain.ErrAccountErased
	}
	return err
}

func (r *PostgresRepository) prepareLeaderboardSync(ctx context.Context, tx creditQueryer, mutationCount int64, entries ...domain.LeaderboardEntry) leaderboardSyncPlan {
	if r == nil || r.leaderboardCache == nil || mutationCount <= 0 || len(entries) == 0 {
		return leaderboardSyncPlan{}
	}
	revision, err := leaderboardRevisionFrom(ctx, tx)
	if err != nil || revision < mutationCount {
		return leaderboardSyncPlan{}
	}
	return leaderboardSyncPlan{
		expectedRevision: revision - mutationCount,
		revision:         revision,
		entries:          entries,
	}
}

func (r *PostgresRepository) syncLeaderboard(ctx context.Context, plan leaderboardSyncPlan) {
	if r == nil || r.leaderboardCache == nil || plan.revision <= plan.expectedRevision || len(plan.entries) == 0 {
		return
	}
	_, _ = r.leaderboardCache.Apply(ctx, plan.expectedRevision, plan.revision, plan.entries)
}

func leaderboardRevisionFrom(ctx context.Context, db creditQueryer) (int64, error) {
	var revision int64
	err := db.QueryRow(ctx, `SELECT revision FROM credit_leaderboard_state WHERE id = TRUE`).Scan(&revision)
	return revision, err
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.exec(ctx, schemaSQL)
	return err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS credit_balances (
  user_id BIGINT PRIMARY KEY,
  total BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_balances_leaderboard
  ON credit_balances(total DESC, user_id DESC)
  WHERE total > 0;

CREATE TABLE IF NOT EXISTS credit_leaderboard_state (
  id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
  revision BIGINT NOT NULL DEFAULT 0
);

INSERT INTO credit_leaderboard_state(id, revision)
VALUES(TRUE, 0)
ON CONFLICT(id) DO NOTHING;

CREATE OR REPLACE FUNCTION credit_bump_leaderboard_revision()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF NEW.total = 0 THEN
      RETURN NEW;
    END IF;
  ELSIF NEW.total = OLD.total THEN
    RETURN NEW;
  END IF;
  UPDATE credit_leaderboard_state
  SET revision = revision + 1
  WHERE id = TRUE;
  RETURN NEW;
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgname = 'credit_balances_leaderboard_revision'
      AND tgrelid = 'credit_balances'::regclass
      AND NOT tgisinternal
  ) THEN
    EXECUTE 'CREATE TRIGGER credit_balances_leaderboard_revision
      AFTER INSERT OR UPDATE OF total ON credit_balances
      FOR EACH ROW
      EXECUTE FUNCTION credit_bump_leaderboard_revision()';
  END IF;
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'credit_balances_nonnegative_check' AND conrelid = 'credit_balances'::regclass) THEN
    ALTER TABLE credit_balances ADD CONSTRAINT credit_balances_nonnegative_check CHECK (total >= 0) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS credit_ledger (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  delta BIGINT NOT NULL,
  balance_after BIGINT NOT NULL,
  reason VARCHAR(64) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  source_event_id VARCHAR(128) NOT NULL,
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, source_event_id, reason)
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_user_created
  ON credit_ledger(user_id, created_at DESC, id DESC);

DO $$
DECLARE
  constraint_record RECORD;
BEGIN
  FOR constraint_record IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'credit_ledger'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%user_id > 0%'
  LOOP
    EXECUTE format('ALTER TABLE credit_ledger DROP CONSTRAINT %I', constraint_record.conname);
  END LOOP;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'credit_ledger_snapshot_check' AND conrelid = 'credit_ledger'::regclass) THEN
    ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_snapshot_check CHECK (
      user_id <> 0 AND delta <> 0 AND balance_after >= 0 AND BTRIM(reason) <> '' AND BTRIM(source_event_id) <> ''
    ) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS credit_reservations (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  amount BIGINT NOT NULL,
  status VARCHAR(16) NOT NULL,
  reason VARCHAR(64) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  source_event_id VARCHAR(128) NOT NULL,
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  settled_at TIMESTAMPTZ,
  UNIQUE(user_id, source_event_id, reason)
);

CREATE INDEX IF NOT EXISTS idx_credit_reservations_user_status
  ON credit_reservations(user_id, status, created_at DESC);

DO $$
DECLARE
  constraint_record RECORD;
BEGIN
  FOR constraint_record IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'credit_reservations'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%user_id > 0%'
  LOOP
    EXECUTE format('ALTER TABLE credit_reservations DROP CONSTRAINT %I', constraint_record.conname);
  END LOOP;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'credit_reservations_lifecycle_check' AND conrelid = 'credit_reservations'::regclass) THEN
    ALTER TABLE credit_reservations ADD CONSTRAINT credit_reservations_lifecycle_check CHECK (
      user_id <> 0 AND amount > 0 AND BTRIM(reason) <> '' AND BTRIM(source_event_id) <> ''
      AND (
        (status = 'ACTIVE' AND settled_at IS NULL)
        OR (status IN ('RELEASED', 'SETTLED') AND settled_at IS NOT NULL)
      )
    ) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS check_ins (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL UNIQUE,
  latest_day DATE NOT NULL,
  consecutive_days INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_check_ins_latest_day
  ON check_ins(latest_day DESC, user_id ASC);

DO $$
DECLARE
  constraint_record RECORD;
BEGIN
  FOR constraint_record IN
    SELECT conname FROM pg_constraint
    WHERE conrelid = 'check_ins'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%user_id > 0%'
  LOOP
    EXECUTE format('ALTER TABLE check_ins DROP CONSTRAINT %I', constraint_record.conname);
  END LOOP;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_ins_valid_check' AND conrelid = 'check_ins'::regclass) THEN
    ALTER TABLE check_ins ADD CONSTRAINT check_ins_valid_check CHECK (
      user_id <> 0 AND consecutive_days > 0
    ) NOT VALID;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS article_authors (
  article_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS published_topics (
  topic_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_published_topics_author
  ON published_topics(author_id, published_at ASC, topic_id ASC);

CREATE TABLE IF NOT EXISTS created_comments (
  comment_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  entity_type VARCHAR(16) NOT NULL,
  entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_created_comments_author
  ON created_comments(author_id, created_at ASC, comment_id ASC);

CREATE TABLE IF NOT EXISTS pending_article_credits (
  event_id VARCHAR(128) NOT NULL,
  reason VARCHAR(64) NOT NULL,
  article_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  delta BIGINT NOT NULL,
  source_type VARCHAR(64) NOT NULL DEFAULT '',
  source_id BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(event_id, reason)
);

CREATE INDEX IF NOT EXISTS idx_pending_article_credits_article
  ON pending_article_credits(article_id, created_at);

CREATE SEQUENCE IF NOT EXISTS credit_erased_user_id_seq AS BIGINT
  INCREMENT BY -1 MINVALUE -9223372036854775808 MAXVALUE -1 START WITH -1;

DO $$
DECLARE
  sequence_increment BIGINT;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended('bbs-credit-erased-user-id-sequence-migration', 0));
  SELECT seqincrement INTO sequence_increment
  FROM pg_sequence
  WHERE seqrelid = 'credit_erased_user_id_seq'::regclass;
  IF sequence_increment > 0 THEN
    ALTER SEQUENCE credit_erased_user_id_seq
      INCREMENT BY -1 MINVALUE -9223372036854775808 MAXVALUE -1 START WITH -1 RESTART WITH -1;
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS credit_erased_users (
  user_id BIGINT PRIMARY KEY,
  anonymized_user_id BIGINT NOT NULL UNIQUE,
  deletion_job_id BIGINT NOT NULL,
  policy_version INTEGER NOT NULL,
  anonymized_ledger_entries BIGINT NOT NULL DEFAULT 0,
  anonymized_reservations BIGINT NOT NULL DEFAULT 0,
  deleted_check_ins BIGINT NOT NULL DEFAULT 0,
  erased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  CONSTRAINT credit_erased_users_identity_check CHECK (user_id > 0 AND anonymized_user_id <> 0 AND anonymized_user_id <> user_id AND deletion_job_id > 0 AND policy_version > 0),
  CONSTRAINT credit_erased_users_counts_check CHECK (anonymized_ledger_entries >= 0 AND anonymized_reservations >= 0 AND deleted_check_ins >= 0)
);

CREATE INDEX IF NOT EXISTS idx_credit_erased_users_job ON credit_erased_users(deletion_job_id);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'credit_erased_users_identity_check'
      AND conrelid = 'credit_erased_users'::regclass
      AND pg_get_constraintdef(oid) NOT LIKE '%anonymized_user_id <> 0%'
  ) THEN
    ALTER TABLE credit_erased_users DROP CONSTRAINT credit_erased_users_identity_check;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'credit_erased_users_identity_check' AND conrelid = 'credit_erased_users'::regclass) THEN
    ALTER TABLE credit_erased_users ADD CONSTRAINT credit_erased_users_identity_check CHECK (
      user_id > 0 AND anonymized_user_id <> 0 AND anonymized_user_id <> user_id AND deletion_job_id > 0 AND policy_version > 0
    ) NOT VALID;
  END IF;
END $$;

CREATE OR REPLACE FUNCTION credit_reject_erased_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  identity_id BIGINT;
BEGIN
  identity_id := (to_jsonb(NEW)->>TG_ARGV[0])::BIGINT;
  IF NOT pg_try_advisory_xact_lock(hashtextextended('bbs-credit-user:' || identity_id::TEXT, 0)) THEN
    RAISE EXCEPTION 'credit account erased' USING ERRCODE = 'P0001';
  END IF;
  IF current_setting('bbs.credit_erasure', true) = 'on' THEN
    RETURN NEW;
  END IF;
  IF EXISTS (SELECT 1 FROM credit_erased_users WHERE user_id = identity_id OR anonymized_user_id = identity_id) THEN
    RAISE EXCEPTION 'credit account erased' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END;
$$;

DO $$
DECLARE
  item RECORD;
BEGIN
  FOR item IN SELECT * FROM (VALUES
    ('credit_balances', 'user_id', 'credit_balances_erased_user_guard'),
    ('credit_ledger', 'user_id', 'credit_ledger_erased_user_guard'),
    ('credit_reservations', 'user_id', 'credit_reservations_erased_user_guard'),
    ('check_ins', 'user_id', 'check_ins_erased_user_guard'),
    ('article_authors', 'author_id', 'article_authors_erased_user_guard'),
    ('published_topics', 'author_id', 'published_topics_erased_user_guard'),
    ('created_comments', 'author_id', 'created_comments_erased_user_guard'),
    ('pending_article_credits', 'actor_id', 'pending_article_credits_erased_user_guard')
  ) AS guards(table_name, column_name, trigger_name)
  LOOP
    IF NOT EXISTS (
      SELECT 1 FROM pg_trigger
      WHERE tgname = item.trigger_name
        AND tgrelid = item.table_name::regclass
        AND NOT tgisinternal
    ) THEN
      EXECUTE format('CREATE TRIGGER %I BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION credit_reject_erased_identity(%L)', item.trigger_name, item.table_name, item.column_name);
    END IF;
  END LOOP;
END;
$$;
`

func (r *PostgresRepository) SaveArticle(ctx context.Context, article domain.ArticleRef, publishedAt time.Time) error {
	if article.ID <= 0 || article.AuthorID <= 0 {
		return nil
	}
	_, err := r.exec(ctx, `
INSERT INTO article_authors(article_id, author_id, title, published_at, updated_at)
VALUES($1, $2, $3, $4, NOW())
ON CONFLICT(article_id) DO UPDATE SET
  author_id = EXCLUDED.author_id,
  title = EXCLUDED.title,
  published_at = EXCLUDED.published_at,
  updated_at = NOW()
`, article.ID, article.AuthorID, article.Title, nullableTime(publishedAt))
	return err
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int64) (domain.ArticleRef, error) {
	var article domain.ArticleRef
	err := r.pool.QueryRow(ctx, `SELECT article_id, author_id, title FROM article_authors WHERE article_id = $1`, id).Scan(&article.ID, &article.AuthorID, &article.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ArticleRef{}, nil
	}
	return article, err
}

func (r *PostgresRepository) SavePublishedTopic(ctx context.Context, topic domain.TopicPublicationRef, publishedAt time.Time) error {
	if topic.ID <= 0 || topic.AuthorID <= 0 {
		return nil
	}
	if publishedAt.IsZero() {
		publishedAt = time.Now()
	}
	_, err := r.exec(ctx, `
INSERT INTO published_topics(topic_id, author_id, title, published_at, updated_at)
VALUES($1, $2, $3, $4, NOW())
ON CONFLICT(topic_id) DO NOTHING
`, topic.ID, topic.AuthorID, topic.Title, publishedAt)
	return err
}

func (r *PostgresRepository) HasPublishedTopic(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM published_topics WHERE author_id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) SaveCreatedComment(ctx context.Context, comment domain.CommentCreationRef, createdAt time.Time) error {
	if comment.ID <= 0 || comment.AuthorID <= 0 || comment.EntityID <= 0 || (comment.EntityType != "article" && comment.EntityType != "topic") {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := r.exec(ctx, `
INSERT INTO created_comments(comment_id, author_id, entity_type, entity_id, created_at, updated_at)
VALUES($1, $2, $3, $4, $5, NOW())
ON CONFLICT(comment_id) DO NOTHING
`, comment.ID, comment.AuthorID, comment.EntityType, comment.EntityID, createdAt)
	return err
}

func (r *PostgresRepository) HasCreatedComment(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM created_comments WHERE author_id = $1)`, userID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) GetTaskClaimSnapshot(ctx context.Context, userID int64, lookups []domain.TaskClaimLedgerLookup) (domain.TaskClaimSnapshot, error) {
	snapshot := domain.TaskClaimSnapshot{
		ClaimedLedgerLookups: make(map[domain.TaskClaimLedgerLookup]bool, len(lookups)),
	}
	if userID <= 0 {
		return snapshot, nil
	}
	if err := r.pool.QueryRow(ctx, `
SELECT
  COALESCE((SELECT latest_day::TEXT FROM check_ins WHERE user_id = $1), ''),
  EXISTS(SELECT 1 FROM published_topics WHERE author_id = $1),
  EXISTS(SELECT 1 FROM created_comments WHERE author_id = $1)
`, userID).Scan(&snapshot.LatestCheckInDay, &snapshot.HasPublishedTopic, &snapshot.HasCreatedComment); err != nil {
		return domain.TaskClaimSnapshot{}, err
	}
	if len(lookups) == 0 {
		return snapshot, nil
	}

	eventIDs := make([]string, 0, len(lookups))
	reasons := make([]string, 0, len(lookups))
	for _, lookup := range lookups {
		eventIDs = append(eventIDs, lookup.SourceEventID)
		reasons = append(reasons, lookup.Reason)
	}
	rows, err := r.pool.Query(ctx, `
SELECT ledger.source_event_id, ledger.reason
FROM credit_ledger AS ledger
JOIN UNNEST($2::TEXT[], $3::TEXT[]) AS requested(source_event_id, reason)
  ON requested.source_event_id = ledger.source_event_id
  AND requested.reason = ledger.reason
WHERE ledger.user_id = $1
`, userID, eventIDs, reasons)
	if err != nil {
		return domain.TaskClaimSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var lookup domain.TaskClaimLedgerLookup
		if err := rows.Scan(&lookup.SourceEventID, &lookup.Reason); err != nil {
			return domain.TaskClaimSnapshot{}, err
		}
		snapshot.ClaimedLedgerLookups[lookup] = true
	}
	if err := rows.Err(); err != nil {
		return domain.TaskClaimSnapshot{}, err
	}
	return snapshot, nil
}

func (r *PostgresRepository) AddCredit(ctx context.Context, entry domain.LedgerEntry) error {
	if entry.UserID <= 0 || entry.Delta == 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, entry.UserID); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
INSERT INTO credit_ledger(user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at)
VALUES($1, $2, 0, $3, $4, $5, $6, $7, $8)
ON CONFLICT(user_id, source_event_id, reason) DO NOTHING
`, entry.UserID, entry.Delta, entry.Reason, entry.Description, entry.SourceEventID, entry.SourceType, entry.SourceID, entry.CreatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}

	var total int64
	if err := tx.QueryRow(ctx, `
INSERT INTO credit_balances(user_id, total, updated_at)
VALUES($1, $2, NOW())
ON CONFLICT(user_id) DO UPDATE SET
  total = credit_balances.total + EXCLUDED.total,
  updated_at = NOW()
RETURNING total
`, entry.UserID, entry.Delta).Scan(&total); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE credit_ledger
SET balance_after = $1
WHERE user_id = $2 AND source_event_id = $3 AND reason = $4
`, total, entry.UserID, entry.SourceEventID, entry.Reason); err != nil {
		return err
	}
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: entry.UserID, Total: total})
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	r.syncLeaderboard(ctx, plan)
	return nil
}

func (r *PostgresRepository) AdjustCredit(ctx context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	if entry.UserID <= 0 || entry.Delta == 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, entry.UserID); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	balance, existing, duplicate, err := lockBalanceBeforeLedgerLookup(ctx, tx, entry.UserID, entry.SourceEventID, entry.Reason, true)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	if duplicate {
		if err := validateDuplicateLedger(existing, entry); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	newTotal := balance.Total + entry.Delta
	if newTotal < 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	now := time.Now()
	if _, err := tx.Exec(ctx, `
UPDATE credit_balances
SET total = $1, updated_at = $2
WHERE user_id = $3
`, newTotal, now, entry.UserID); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	entry.BalanceAfter = newTotal
	if err := tx.QueryRow(ctx, `
INSERT INTO credit_ledger(user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at
`, entry.UserID, entry.Delta, entry.BalanceAfter, entry.Reason, entry.Description, entry.SourceEventID, entry.SourceType, entry.SourceID, entry.CreatedAt).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Delta,
		&entry.BalanceAfter,
		&entry.Reason,
		&entry.Description,
		&entry.SourceEventID,
		&entry.SourceType,
		&entry.SourceID,
		&entry.CreatedAt,
	); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	balance.Total = newTotal
	balance.UpdatedAt = now
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: balance.Total})
	if err := tx.Commit(ctx); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	r.syncLeaderboard(ctx, plan)
	return entry, balance, false, nil
}

func (r *PostgresRepository) DebitCredit(ctx context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	if entry.UserID <= 0 || entry.Delta >= 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, entry.UserID); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	balance, existing, duplicate, err := lockBalanceBeforeLedgerLookup(ctx, tx, entry.UserID, entry.SourceEventID, entry.Reason, false)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	if duplicate {
		if err := validateDuplicateLedger(existing, entry); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	if balance.Total+entry.Delta < 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	newTotal := balance.Total + entry.Delta
	now := time.Now()
	if balance.UpdatedAt.IsZero() {
		if _, err := tx.Exec(ctx, `
INSERT INTO credit_balances(user_id, total, updated_at)
VALUES($1, $2, $3)
`, entry.UserID, newTotal, now); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
	} else if _, err := tx.Exec(ctx, `
UPDATE credit_balances
SET total = $1, updated_at = $2
WHERE user_id = $3
`, newTotal, now, entry.UserID); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	entry.BalanceAfter = newTotal
	if err := tx.QueryRow(ctx, `
INSERT INTO credit_ledger(user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at
`, entry.UserID, entry.Delta, entry.BalanceAfter, entry.Reason, entry.Description, entry.SourceEventID, entry.SourceType, entry.SourceID, entry.CreatedAt).Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Delta,
		&entry.BalanceAfter,
		&entry.Reason,
		&entry.Description,
		&entry.SourceEventID,
		&entry.SourceType,
		&entry.SourceID,
		&entry.CreatedAt,
	); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	balance.Total = newTotal
	balance.UpdatedAt = now
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: balance.Total})
	if err := tx.Commit(ctx); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	r.syncLeaderboard(ctx, plan)
	return entry, balance, false, nil
}

func (r *PostgresRepository) ReserveCredit(ctx context.Context, reservation domain.CreditReservation, ledger domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	if reservation.UserID <= 0 || reservation.Amount <= 0 || reservation.SourceEventID == "" || reservation.Reason == "" {
		return domain.CreditReservation{}, domain.Balance{}, false, nil
	}
	if reservation.CreatedAt.IsZero() {
		reservation.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, reservation.UserID); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}

	if err := ensureBalanceRow(ctx, tx, reservation.UserID); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	balance, err := balanceForUpdate(ctx, tx, reservation.UserID)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	existing, err := reservationByEventForUpdate(ctx, tx, reservation.UserID, reservation.SourceEventID, reservation.Reason)
	if err == nil {
		if err := validateDuplicateReservation(existing, reservation); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}

	newTotal := balance.Total - reservation.Amount
	if newTotal < 0 {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	now := time.Now()
	if err := updateCreditBalance(ctx, tx, reservation.UserID, newTotal, now); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	reservation.Status = "ACTIVE"
	reservation.UpdatedAt = now
	created, err := insertReservation(ctx, tx, reservation)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	ledger.BalanceAfter = newTotal
	if err := insertLedgerEntry(ctx, tx, ledger); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	balance.Total = newTotal
	balance.UpdatedAt = now
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: balance.Total})
	if err := tx.Commit(ctx); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	r.syncLeaderboard(ctx, plan)
	return created, balance, false, nil
}

func (r *PostgresRepository) ReleaseCredit(ctx context.Context, reservation domain.CreditReservation, ledger domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	if reservation.UserID <= 0 || reservation.Amount <= 0 || reservation.SourceEventID == "" || reservation.Reason == "" {
		return domain.CreditReservation{}, domain.Balance{}, false, nil
	}
	if ledger.UserID <= 0 || ledger.Delta <= 0 || ledger.SourceEventID == "" || ledger.Reason == "" {
		return domain.CreditReservation{}, domain.Balance{}, false, nil
	}
	if ledger.CreatedAt.IsZero() {
		ledger.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, reservation.UserID); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}

	if err := ensureBalanceRow(ctx, tx, reservation.UserID); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	balance, err := balanceForUpdate(ctx, tx, reservation.UserID)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	existing, err := reservationByEventForUpdate(ctx, tx, reservation.UserID, reservation.SourceEventID, reservation.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrCreditReservationNotFound
	}
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	if err := validateDuplicateReservation(existing, reservation); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	if existing.Status == "RELEASED" {
		if err := tx.Commit(ctx); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	if existing.Status != "ACTIVE" {
		return domain.CreditReservation{}, domain.Balance{}, false, domain.ErrCreditReservationNotFound
	}

	releaseLedgerExists, err := ledgerExists(ctx, tx, ledger.UserID, ledger.SourceEventID, ledger.Reason)
	if err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	balanceChanged := false
	updatedAt := time.Now()
	if !releaseLedgerExists {
		newTotal := balance.Total + existing.Amount
		if err := updateCreditBalance(ctx, tx, reservation.UserID, newTotal, updatedAt); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		ledger.BalanceAfter = newTotal
		if err := insertLedgerEntry(ctx, tx, ledger); err != nil {
			return domain.CreditReservation{}, domain.Balance{}, false, err
		}
		balance.Total = newTotal
		balance.UpdatedAt = updatedAt
		balanceChanged = true
	}
	existing.Status = "RELEASED"
	existing.UpdatedAt = updatedAt
	existing.SettledAt = updatedAt
	if _, err := tx.Exec(ctx, `
UPDATE credit_reservations
SET status = 'RELEASED', settled_at = $1, updated_at = $1
WHERE id = $2
`, updatedAt, existing.ID); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	plan := leaderboardSyncPlan{}
	if balanceChanged {
		plan = r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: balance.Total})
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.CreditReservation{}, domain.Balance{}, false, err
	}
	r.syncLeaderboard(ctx, plan)
	return existing, balance, false, nil
}

func (r *PostgresRepository) SettleCreditReservation(ctx context.Context, reservation domain.CreditReservation, credit domain.LedgerEntry) error {
	if reservation.UserID <= 0 || reservation.Amount <= 0 || reservation.SourceEventID == "" || reservation.Reason == "" {
		return domain.ErrCreditReservationNotFound
	}
	if credit.UserID <= 0 || credit.Delta <= 0 || credit.SourceEventID == "" || credit.Reason == "" {
		return nil
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, reservation.UserID, credit.UserID); err != nil {
		return err
	}

	existing, err := reservationByEventForUpdate(ctx, tx, reservation.UserID, reservation.SourceEventID, reservation.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrCreditReservationNotFound
	}
	if err != nil {
		return err
	}
	if err := validateReservationSettlement(existing, reservation, credit); err != nil {
		return err
	}
	creditExists, err := ledgerExists(ctx, tx, credit.UserID, credit.SourceEventID, credit.Reason)
	if err != nil {
		return err
	}
	if existing.Status == "SETTLED" {
		if !creditExists {
			return domain.ErrInconsistentCreditTransfer
		}
		return tx.Commit(ctx)
	}
	if existing.Status != "ACTIVE" {
		return domain.ErrCreditReservationNotFound
	}
	updatedAt := time.Now()
	var leaderboardUpdate domain.LeaderboardEntry
	balanceChanged := false
	if !creditExists {
		if err := ensureBalanceRow(ctx, tx, credit.UserID); err != nil {
			return err
		}
		balance, err := balanceForUpdate(ctx, tx, credit.UserID)
		if err != nil {
			return err
		}
		newTotal := balance.Total + credit.Delta
		if err := updateCreditBalance(ctx, tx, credit.UserID, newTotal, updatedAt); err != nil {
			return err
		}
		credit.BalanceAfter = newTotal
		if err := insertLedgerEntry(ctx, tx, credit); err != nil {
			return err
		}
		leaderboardUpdate = domain.LeaderboardEntry{UserID: credit.UserID, Total: newTotal}
		balanceChanged = true
	}
	if _, err := tx.Exec(ctx, `
UPDATE credit_reservations
SET status = 'SETTLED', settled_at = $1, updated_at = $1
WHERE id = $2
`, updatedAt, existing.ID); err != nil {
		return err
	}
	plan := leaderboardSyncPlan{}
	if balanceChanged {
		plan = r.prepareLeaderboardSync(ctx, tx, 1, leaderboardUpdate)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	r.syncLeaderboard(ctx, plan)
	return nil
}

func validateReservationSettlement(existing domain.CreditReservation, reservation domain.CreditReservation, credit domain.LedgerEntry) error {
	if existing.Amount != credit.Delta ||
		(reservation.SourceID > 0 && existing.SourceID != reservation.SourceID) ||
		(reservation.SourceType != "" && existing.SourceType != reservation.SourceType) {
		return domain.ErrCreditReservationMismatch
	}
	return nil
}

func (r *PostgresRepository) ReverseQAAcceptance(ctx context.Context, reversal domain.QAAcceptanceReversal) (bool, error) {
	if reversal.QuestionAuthorID <= 0 || reversal.TopicID <= 0 || reversal.AcceptedCommentID <= 0 || reversal.AcceptedCommentAuthorID <= 0 || reversal.QuestionAuthorID == reversal.AcceptedCommentAuthorID || reversal.Amount <= 0 || reversal.AcceptedEventID == "" || reversal.ReversalEventID == "" {
		return false, domain.ErrCreditReservationMismatch
	}
	if reversal.OccurredAt.IsZero() {
		reversal.OccurredAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, reversal.QuestionAuthorID, reversal.AcceptedCommentAuthorID); err != nil {
		return false, err
	}

	reservation := domain.CreditReservation{
		UserID:        reversal.QuestionAuthorID,
		Amount:        reversal.Amount,
		Reason:        "qa_bounty_reserved",
		SourceEventID: fmt.Sprintf("content.qa.bounty:%d", reversal.TopicID),
		SourceType:    "topic",
		SourceID:      reversal.TopicID,
	}
	storedReservation, reservationErr := reservationByEventForUpdate(ctx, tx, reservation.UserID, reservation.SourceEventID, reservation.Reason)
	if reservationErr == nil {
		if err := validateDuplicateReservation(storedReservation, reservation); err != nil {
			return false, err
		}
		return reverseReservedQAAcceptance(ctx, r, tx, storedReservation, reversal)
	}
	if !errors.Is(reservationErr, pgx.ErrNoRows) {
		return false, reservationErr
	}
	return reverseTransferredQAAcceptance(ctx, r, tx, reversal)
}

func reverseReservedQAAcceptance(ctx context.Context, r *PostgresRepository, tx pgx.Tx, reservation domain.CreditReservation, reversal domain.QAAcceptanceReversal) (bool, error) {
	reversalLedger := qaAcceptanceReversalDebit(reversal)
	existingReversal, err := ledgerByEvent(ctx, tx, reversalLedger.UserID, reversalLedger.SourceEventID, reversalLedger.Reason)
	if err == nil {
		if err := validateDuplicateLedger(existingReversal, reversalLedger); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if reservation.Status != "SETTLED" {
		return false, domain.ErrQAAcceptanceSettlementPending
	}
	originalReward, err := ledgerByEvent(ctx, tx, reversal.AcceptedCommentAuthorID, reversal.AcceptedEventID, "qa_answer_accepted")
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrInconsistentCreditTransfer
	}
	if err != nil {
		return false, err
	}
	if err := validateDuplicateLedger(originalReward, domain.LedgerEntry{
		UserID:        reversal.AcceptedCommentAuthorID,
		Delta:         reversal.Amount,
		Reason:        "qa_answer_accepted",
		SourceEventID: reversal.AcceptedEventID,
		SourceType:    "comment",
		SourceID:      reversal.AcceptedCommentID,
	}); err != nil {
		return false, err
	}
	if err := ensureBalanceRow(ctx, tx, reversal.AcceptedCommentAuthorID); err != nil {
		return false, err
	}
	balance, err := balanceForUpdate(ctx, tx, reversal.AcceptedCommentAuthorID)
	if err != nil {
		return false, err
	}
	newTotal := balance.Total + reversalLedger.Delta
	if newTotal < 0 {
		return false, domain.ErrInsufficientCredit
	}
	if err := updateCreditBalance(ctx, tx, reversal.AcceptedCommentAuthorID, newTotal, reversal.OccurredAt); err != nil {
		return false, err
	}
	reversalLedger.BalanceAfter = newTotal
	if err := insertLedgerEntry(ctx, tx, reversalLedger); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE credit_reservations
SET status = 'ACTIVE', settled_at = NULL, updated_at = $1
WHERE id = $2 AND status = 'SETTLED'
`, reversal.OccurredAt, reservation.ID); err != nil {
		return false, err
	}
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: newTotal})
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	r.syncLeaderboard(ctx, plan)
	return false, nil
}

func reverseTransferredQAAcceptance(ctx context.Context, r *PostgresRepository, tx pgx.Tx, reversal domain.QAAcceptanceReversal) (bool, error) {
	if err := ensureBalanceRow(ctx, tx, reversal.AcceptedCommentAuthorID); err != nil {
		return false, err
	}
	if err := ensureBalanceRow(ctx, tx, reversal.QuestionAuthorID); err != nil {
		return false, err
	}
	balances, err := lockTransferBalances(ctx, tx, reversal.AcceptedCommentAuthorID, reversal.QuestionAuthorID)
	if err != nil {
		return false, err
	}
	debit := qaAcceptanceReversalDebit(reversal)
	credit := domain.LedgerEntry{
		UserID:        reversal.QuestionAuthorID,
		Delta:         reversal.Amount,
		Reason:        "qa_bounty_refunded",
		Description:   reversal.QuestionerDescription,
		SourceEventID: reversal.ReversalEventID,
		SourceType:    "topic",
		SourceID:      reversal.TopicID,
		CreatedAt:     reversal.OccurredAt,
	}
	existingDebit, debitErr := ledgerByEvent(ctx, tx, debit.UserID, debit.SourceEventID, debit.Reason)
	existingCredit, creditErr := ledgerByEvent(ctx, tx, credit.UserID, credit.SourceEventID, credit.Reason)
	if debitErr == nil || creditErr == nil {
		if (debitErr == nil) != (creditErr == nil) {
			return false, domain.ErrInconsistentCreditTransfer
		}
		if err := validateDuplicateLedger(existingDebit, debit); err != nil {
			return false, err
		}
		if err := validateDuplicateLedger(existingCredit, credit); err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	if !errors.Is(debitErr, pgx.ErrNoRows) {
		return false, debitErr
	}
	if !errors.Is(creditErr, pgx.ErrNoRows) {
		return false, creditErr
	}
	originalReward, err := ledgerByEvent(ctx, tx, reversal.AcceptedCommentAuthorID, reversal.AcceptedEventID, "qa_answer_accepted")
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrQAAcceptanceSettlementPending
	}
	if err != nil {
		return false, err
	}
	if err := validateDuplicateLedger(originalReward, domain.LedgerEntry{
		UserID:        reversal.AcceptedCommentAuthorID,
		Delta:         reversal.Amount,
		Reason:        "qa_answer_accepted",
		SourceEventID: reversal.AcceptedEventID,
		SourceType:    "comment",
		SourceID:      reversal.AcceptedCommentID,
	}); err != nil {
		return false, err
	}
	originalDebit, err := ledgerByEvent(ctx, tx, reversal.QuestionAuthorID, reversal.AcceptedEventID, "qa_bounty_paid")
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrQAAcceptanceSettlementPending
	}
	if err != nil {
		return false, err
	}
	if err := validateDuplicateLedger(originalDebit, domain.LedgerEntry{
		UserID:        reversal.QuestionAuthorID,
		Delta:         -reversal.Amount,
		Reason:        "qa_bounty_paid",
		SourceEventID: reversal.AcceptedEventID,
		SourceType:    "topic",
		SourceID:      reversal.TopicID,
	}); err != nil {
		return false, err
	}
	answererBalance := balances[debit.UserID]
	if answererBalance.Total+debit.Delta < 0 {
		return false, domain.ErrInsufficientCredit
	}
	answererBalance.Total += debit.Delta
	if err := updateCreditBalance(ctx, tx, debit.UserID, answererBalance.Total, reversal.OccurredAt); err != nil {
		return false, err
	}
	debit.BalanceAfter = answererBalance.Total
	if err := insertLedgerEntry(ctx, tx, debit); err != nil {
		return false, err
	}
	questionerBalance := balances[credit.UserID]
	questionerBalance.Total += credit.Delta
	if err := updateCreditBalance(ctx, tx, credit.UserID, questionerBalance.Total, reversal.OccurredAt); err != nil {
		return false, err
	}
	credit.BalanceAfter = questionerBalance.Total
	if err := insertLedgerEntry(ctx, tx, credit); err != nil {
		return false, err
	}
	plan := r.prepareLeaderboardSync(ctx, tx, 2,
		domain.LeaderboardEntry{UserID: debit.UserID, Total: debit.BalanceAfter},
		domain.LeaderboardEntry{UserID: credit.UserID, Total: credit.BalanceAfter},
	)
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	r.syncLeaderboard(ctx, plan)
	return false, nil
}

func qaAcceptanceReversalDebit(reversal domain.QAAcceptanceReversal) domain.LedgerEntry {
	return domain.LedgerEntry{
		UserID:        reversal.AcceptedCommentAuthorID,
		Delta:         -reversal.Amount,
		Reason:        "qa_answer_unaccepted",
		Description:   reversal.AnswererDescription,
		SourceEventID: reversal.ReversalEventID,
		SourceType:    "comment",
		SourceID:      reversal.AcceptedCommentID,
		CreatedAt:     reversal.OccurredAt,
	}
}

func (r *PostgresRepository) TransferCredit(ctx context.Context, debit domain.LedgerEntry, credit domain.LedgerEntry) error {
	if debit.UserID <= 0 || debit.Delta >= 0 || debit.SourceEventID == "" || debit.Reason == "" {
		return nil
	}
	if credit.UserID <= 0 || credit.Delta <= 0 || credit.SourceEventID == "" || credit.Reason == "" {
		return nil
	}
	if err := validateTransferBalance(debit, credit); err != nil {
		return err
	}
	now := time.Now()
	if debit.CreatedAt.IsZero() {
		debit.CreatedAt = now
	}
	if credit.CreatedAt.IsZero() {
		credit.CreatedAt = debit.CreatedAt
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, debit.UserID, credit.UserID); err != nil {
		return err
	}

	if err := ensureBalanceRow(ctx, tx, debit.UserID); err != nil {
		return err
	}
	if credit.UserID != debit.UserID {
		if err := ensureBalanceRow(ctx, tx, credit.UserID); err != nil {
			return err
		}
	}
	balances, err := lockTransferBalances(ctx, tx, debit.UserID, credit.UserID)
	if err != nil {
		return err
	}

	existingDebit, debitErr := ledgerByEvent(ctx, tx, debit.UserID, debit.SourceEventID, debit.Reason)
	if debitErr != nil && !errors.Is(debitErr, pgx.ErrNoRows) {
		return debitErr
	}
	existingCredit, creditErr := ledgerByEvent(ctx, tx, credit.UserID, credit.SourceEventID, credit.Reason)
	if creditErr != nil && !errors.Is(creditErr, pgx.ErrNoRows) {
		return creditErr
	}
	debitExists := debitErr == nil
	creditExists := creditErr == nil
	if err := validateTransferLedgerState(debitExists, creditExists); err != nil {
		return err
	}
	if debitExists && creditExists {
		if err := validateDuplicateLedger(existingDebit, debit); err != nil {
			return err
		}
		if err := validateDuplicateLedger(existingCredit, credit); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	updatedAt := time.Now()
	if !debitExists {
		balance := balances[debit.UserID]
		newTotal := balance.Total + debit.Delta
		if newTotal < 0 {
			return domain.ErrInsufficientCredit
		}
		if err := updateCreditBalance(ctx, tx, debit.UserID, newTotal, updatedAt); err != nil {
			return err
		}
		debit.BalanceAfter = newTotal
		if err := insertLedgerEntry(ctx, tx, debit); err != nil {
			return err
		}
		balance.Total = newTotal
		balance.UpdatedAt = updatedAt
		balances[debit.UserID] = balance
	}
	if !creditExists {
		balance := balances[credit.UserID]
		newTotal := balance.Total + credit.Delta
		if err := updateCreditBalance(ctx, tx, credit.UserID, newTotal, updatedAt); err != nil {
			return err
		}
		credit.BalanceAfter = newTotal
		if err := insertLedgerEntry(ctx, tx, credit); err != nil {
			return err
		}
	}
	entries := []domain.LeaderboardEntry{
		{UserID: debit.UserID, Total: debit.BalanceAfter},
		{UserID: credit.UserID, Total: credit.BalanceAfter},
	}
	if debit.UserID == credit.UserID {
		entries = entries[1:]
	}
	plan := r.prepareLeaderboardSync(ctx, tx, 2, entries...)
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	r.syncLeaderboard(ctx, plan)
	return nil
}

func validateTransferLedgerState(debitExists, creditExists bool) error {
	if debitExists != creditExists {
		return domain.ErrInconsistentCreditTransfer
	}
	return nil
}

func validateDuplicateLedger(existing domain.LedgerEntry, requested domain.LedgerEntry) error {
	if existing.Delta != requested.Delta ||
		(requested.SourceID > 0 && existing.SourceID != requested.SourceID) ||
		(requested.SourceType != "" && existing.SourceType != requested.SourceType) {
		return domain.ErrCreditLedgerMismatch
	}
	return nil
}

func validateDuplicateReservation(existing domain.CreditReservation, requested domain.CreditReservation) error {
	if existing.Amount != requested.Amount ||
		(requested.SourceID > 0 && existing.SourceID != requested.SourceID) ||
		(requested.SourceType != "" && existing.SourceType != requested.SourceType) {
		return domain.ErrCreditReservationMismatch
	}
	return nil
}

func validateTransferBalance(debit domain.LedgerEntry, credit domain.LedgerEntry) error {
	if debit.Delta+credit.Delta != 0 {
		return domain.ErrUnbalancedCreditTransfer
	}
	return nil
}

func (r *PostgresRepository) SavePendingArticleCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, createdAt time.Time) error {
	if eventID == "" || reason == "" || articleID <= 0 || actorID <= 0 || delta == 0 {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := r.exec(ctx, `
INSERT INTO pending_article_credits(event_id, reason, article_id, actor_id, delta, source_type, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT(event_id, reason) DO NOTHING
`, eventID, reason, articleID, actorID, delta, sourceType, sourceID, createdAt)
	return err
}

func (r *PostgresRepository) FlushPendingArticleCredits(ctx context.Context, article domain.ArticleRef) error {
	if article.ID <= 0 || article.AuthorID <= 0 {
		return nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT event_id, reason, actor_id, delta, source_type, source_id, created_at
FROM pending_article_credits
WHERE article_id = $1
ORDER BY created_at ASC
`, article.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pending struct {
		eventID    string
		reason     string
		actorID    int64
		delta      int64
		sourceType string
		sourceID   int64
		createdAt  time.Time
	}
	items := make([]pending, 0)
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.eventID, &item.reason, &item.actorID, &item.delta, &item.sourceType, &item.sourceID, &item.createdAt); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		if item.actorID == article.AuthorID {
			continue
		}
		if err := r.AddCredit(ctx, domain.LedgerEntry{
			UserID:        article.AuthorID,
			Delta:         item.delta,
			Reason:        item.reason,
			Description:   articleOwnerDescription(item.reason, item.actorID, article.Title),
			SourceEventID: item.eventID,
			SourceType:    item.sourceType,
			SourceID:      item.sourceID,
			CreatedAt:     item.createdAt,
		}); err != nil {
			return err
		}
	}
	_, err = r.exec(ctx, `DELETE FROM pending_article_credits WHERE article_id = $1`, article.ID)
	return err
}

func (r *PostgresRepository) GetBalance(ctx context.Context, userID int64) (domain.Balance, error) {
	var balance domain.Balance
	err := r.pool.QueryRow(ctx, `SELECT user_id, total, updated_at FROM credit_balances WHERE user_id = $1`, userID).Scan(&balance.UserID, &balance.Total, &balance.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Balance{UserID: userID, Total: 0}, nil
	}
	return balance, err
}

func (r *PostgresRepository) ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	balance, err := r.GetBalance(ctx, userID)
	if err != nil {
		return nil, 0, domain.Balance{}, err
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, domain.Balance{}, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at
FROM credit_ledger
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3
`, userID, limit, offset)
	if err != nil {
		return nil, 0, domain.Balance{}, err
	}
	defer rows.Close()
	items := make([]domain.LedgerEntry, 0, limit)
	for rows.Next() {
		var item domain.LedgerEntry
		if err := rows.Scan(&item.ID, &item.UserID, &item.Delta, &item.BalanceAfter, &item.Reason, &item.Description, &item.SourceEventID, &item.SourceType, &item.SourceID, &item.CreatedAt); err != nil {
			return nil, 0, domain.Balance{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, domain.Balance{}, err
	}
	return items, total, balance, nil
}

func (r *PostgresRepository) ListLeaderboard(ctx context.Context, limit int32) ([]domain.LeaderboardEntry, error) {
	if r.leaderboardCache == nil {
		return r.listLeaderboardFromDatabase(ctx, limit)
	}
	revision, revisionErr := r.leaderboardRevision(ctx)
	if revisionErr == nil {
		cached, cachedRevision, hit, cacheErr := r.leaderboardCache.List(ctx, limit)
		if cacheErr == nil && hit && cachedRevision == revision {
			return cached, nil
		}
	}
	snapshot, snapshotRevision, err := r.leaderboardSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.leaderboardCache.Replace(ctx, snapshotRevision, snapshot); err != nil {
		return leaderboardPage(snapshot, limit), nil
	}
	return leaderboardPage(snapshot, limit), nil
}

func (r *PostgresRepository) listLeaderboardFromDatabase(ctx context.Context, limit int32) ([]domain.LeaderboardEntry, error) {
	if limit <= 0 {
		return []domain.LeaderboardEntry{}, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT user_id, total
FROM credit_balances
WHERE total > 0
ORDER BY total DESC, user_id DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.LeaderboardEntry, 0, limit)
	for rows.Next() {
		var item domain.LeaderboardEntry
		if err := rows.Scan(&item.UserID, &item.Total); err != nil {
			return nil, err
		}
		item.Rank = int32(len(items) + 1)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) leaderboardRevision(ctx context.Context) (int64, error) {
	var revision int64
	err := r.pool.QueryRow(ctx, `SELECT revision FROM credit_leaderboard_state WHERE id = TRUE`).Scan(&revision)
	return revision, err
}

func (r *PostgresRepository) leaderboardSnapshot(ctx context.Context) ([]domain.LeaderboardEntry, int64, error) {
	tx, err := r.beginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	revision, err := leaderboardRevisionFrom(ctx, tx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, `
SELECT user_id, total
FROM credit_balances
WHERE total > 0
ORDER BY total DESC, user_id DESC
`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.LeaderboardEntry, 0)
	for rows.Next() {
		var item domain.LeaderboardEntry
		if err := rows.Scan(&item.UserID, &item.Total); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return items, revision, nil
}

func leaderboardPage(entries []domain.LeaderboardEntry, limit int32) []domain.LeaderboardEntry {
	if limit <= 0 || len(entries) == 0 {
		return []domain.LeaderboardEntry{}
	}
	count := int(limit)
	if count > len(entries) {
		count = len(entries)
	}
	items := make([]domain.LeaderboardEntry, count)
	copy(items, entries[:count])
	for index := range items {
		items[index].Rank = int32(index + 1)
	}
	return items
}

func (r *PostgresRepository) GetLedgerEntry(ctx context.Context, userID int64, sourceEventID, reason string) (domain.LedgerEntry, bool, error) {
	item, err := ledgerByEvent(ctx, r.pool, userID, sourceEventID, reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LedgerEntry{}, false, nil
	}
	if err != nil {
		return domain.LedgerEntry{}, false, err
	}
	return item, true, nil
}

func (r *PostgresRepository) GetCheckIn(ctx context.Context, userID int64) (domain.CheckIn, error) {
	if userID <= 0 {
		return domain.CheckIn{}, nil
	}
	var checkIn domain.CheckIn
	err := r.pool.QueryRow(ctx, `
SELECT id, user_id, latest_day::TEXT, consecutive_days, created_at, updated_at
FROM check_ins
WHERE user_id = $1
`, userID).Scan(
		&checkIn.ID,
		&checkIn.UserID,
		&checkIn.LatestDay,
		&checkIn.ConsecutiveDays,
		&checkIn.CreatedAt,
		&checkIn.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CheckIn{UserID: userID}, nil
	}
	return checkIn, err
}

func (r *PostgresRepository) RecordCheckIn(ctx context.Context, requested domain.CheckIn, ledger domain.LedgerEntry) (domain.CheckIn, domain.LedgerEntry, domain.Balance, bool, error) {
	if requested.UserID <= 0 || requested.LatestDay == "" || ledger.UserID != requested.UserID || ledger.Delta <= 0 || ledger.SourceEventID == "" || ledger.Reason == "" {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, nil
	}
	if ledger.CreatedAt.IsZero() {
		ledger.CreatedAt = time.Now()
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCreditUsers(ctx, tx, requested.UserID); err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	if err := ensureBalanceRow(ctx, tx, requested.UserID); err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	balance, err := balanceForUpdate(ctx, tx, requested.UserID)
	if err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	checkIn, err := checkInForUpdate(ctx, tx, requested.UserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	existingLedger, err := ledgerByEvent(ctx, tx, requested.UserID, ledger.SourceEventID, ledger.Reason)
	if err == nil {
		if checkIn.UserID == 0 || checkIn.LatestDay != requested.LatestDay {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrCheckInStateMismatch
		}
		if err := validateDuplicateLedger(existingLedger, ledger); err != nil {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return checkIn, existingLedger, balance, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	now := ledger.CreatedAt
	if checkIn.UserID == 0 {
		checkIn, err = insertCheckIn(ctx, tx, domain.CheckIn{
			UserID:          requested.UserID,
			LatestDay:       requested.LatestDay,
			ConsecutiveDays: 1,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	} else {
		streak, streakErr := nextCheckInStreak(checkIn.LatestDay, requested.LatestDay, checkIn.ConsecutiveDays)
		if streakErr != nil {
			return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, streakErr
		}
		checkIn.LatestDay = requested.LatestDay
		checkIn.ConsecutiveDays = streak
		checkIn.UpdatedAt = now
		err = updateCheckIn(ctx, tx, checkIn)
	}
	if err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	newTotal := balance.Total + ledger.Delta
	if newTotal < 0 {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, domain.ErrInsufficientCredit
	}
	if err := updateCreditBalance(ctx, tx, requested.UserID, newTotal, now); err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	ledger.BalanceAfter = newTotal
	if err := tx.QueryRow(ctx, `
INSERT INTO credit_ledger(user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at
`, ledger.UserID, ledger.Delta, ledger.BalanceAfter, ledger.Reason, ledger.Description, ledger.SourceEventID, ledger.SourceType, ledger.SourceID, ledger.CreatedAt).Scan(
		&ledger.ID,
		&ledger.UserID,
		&ledger.Delta,
		&ledger.BalanceAfter,
		&ledger.Reason,
		&ledger.Description,
		&ledger.SourceEventID,
		&ledger.SourceType,
		&ledger.SourceID,
		&ledger.CreatedAt,
	); err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	balance.Total = newTotal
	balance.UpdatedAt = now
	plan := r.prepareLeaderboardSync(ctx, tx, 1, domain.LeaderboardEntry{UserID: balance.UserID, Total: balance.Total})
	if err := tx.Commit(ctx); err != nil {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	r.syncLeaderboard(ctx, plan)
	return checkIn, ledger, balance, false, nil
}

func checkInForUpdate(ctx context.Context, tx creditQueryer, userID int64) (domain.CheckIn, error) {
	var checkIn domain.CheckIn
	err := tx.QueryRow(ctx, `
SELECT id, user_id, latest_day::TEXT, consecutive_days, created_at, updated_at
FROM check_ins
WHERE user_id = $1
FOR UPDATE
`, userID).Scan(
		&checkIn.ID,
		&checkIn.UserID,
		&checkIn.LatestDay,
		&checkIn.ConsecutiveDays,
		&checkIn.CreatedAt,
		&checkIn.UpdatedAt,
	)
	return checkIn, err
}

func insertCheckIn(ctx context.Context, tx pgx.Tx, checkIn domain.CheckIn) (domain.CheckIn, error) {
	err := tx.QueryRow(ctx, `
INSERT INTO check_ins(user_id, latest_day, consecutive_days, created_at, updated_at)
VALUES($1, $2::date, $3, $4, $5)
RETURNING id, user_id, latest_day::TEXT, consecutive_days, created_at, updated_at
`, checkIn.UserID, checkIn.LatestDay, checkIn.ConsecutiveDays, checkIn.CreatedAt, checkIn.UpdatedAt).Scan(
		&checkIn.ID,
		&checkIn.UserID,
		&checkIn.LatestDay,
		&checkIn.ConsecutiveDays,
		&checkIn.CreatedAt,
		&checkIn.UpdatedAt,
	)
	return checkIn, err
}

func updateCheckIn(ctx context.Context, tx pgx.Tx, checkIn domain.CheckIn) error {
	_, err := tx.Exec(ctx, `
UPDATE check_ins
SET latest_day = $1::date, consecutive_days = $2, updated_at = $3
WHERE id = $4
`, checkIn.LatestDay, checkIn.ConsecutiveDays, checkIn.UpdatedAt, checkIn.ID)
	return err
}

func nextCheckInStreak(latestDay, requestedDay string, current int32) (int32, error) {
	latest, err := time.Parse(time.DateOnly, latestDay)
	if err != nil {
		return 0, domain.ErrCheckInStateMismatch
	}
	requested, err := time.Parse(time.DateOnly, requestedDay)
	if err != nil {
		return 0, domain.ErrCheckInStateMismatch
	}
	if !requested.After(latest) {
		if requested.Equal(latest) {
			return 0, domain.ErrCheckInStateMismatch
		}
		return 0, domain.ErrCheckInDayRegression
	}
	if current <= 0 {
		return 0, domain.ErrCheckInStateMismatch
	}
	if latest.AddDate(0, 0, 1).Equal(requested) {
		return current + 1, nil
	}
	return 1, nil
}

type creditQueryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

const creditUserAdvisoryLockSQL = `SELECT pg_advisory_xact_lock(hashtextextended('bbs-credit-user:' || $1::BIGINT::TEXT, 0))`

func lockCreditUsers(ctx context.Context, tx creditQueryer, userIDs ...int64) error {
	ordered := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID != 0 {
			ordered = append(ordered, userID)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for i, userID := range ordered {
		if i > 0 && userID == ordered[i-1] {
			continue
		}
		if _, err := tx.Exec(ctx, creditUserAdvisoryLockSQL, userID); err != nil {
			return normalizePostgresError(err)
		}
	}
	return nil
}

func lockBalanceBeforeLedgerLookup(ctx context.Context, tx creditQueryer, userID int64, eventID, reason string, createBalance bool) (domain.Balance, domain.LedgerEntry, bool, error) {
	if createBalance {
		if err := ensureBalanceRow(ctx, tx, userID); err != nil {
			return domain.Balance{}, domain.LedgerEntry{}, false, err
		}
	}
	balance, err := balanceForUpdate(ctx, tx, userID)
	if err != nil {
		return domain.Balance{}, domain.LedgerEntry{}, false, err
	}
	existing, err := ledgerByEvent(ctx, tx, userID, eventID, reason)
	if err == nil {
		return balance, existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Balance{}, domain.LedgerEntry{}, false, err
	}
	return balance, domain.LedgerEntry{}, false, nil
}

func ensureBalanceRow(ctx context.Context, tx creditQueryer, userID int64) error {
	_, err := tx.Exec(ctx, `
INSERT INTO credit_balances(user_id, total, updated_at)
VALUES($1, 0, NOW())
ON CONFLICT(user_id) DO NOTHING
`, userID)
	return normalizePostgresError(err)
}

func lockTransferBalances(ctx context.Context, tx pgx.Tx, debitUserID, creditUserID int64) (map[int64]domain.Balance, error) {
	userIDs := []int64{debitUserID}
	if creditUserID != debitUserID {
		if creditUserID < debitUserID {
			userIDs = []int64{creditUserID, debitUserID}
		} else {
			userIDs = append(userIDs, creditUserID)
		}
	}
	balances := make(map[int64]domain.Balance, len(userIDs))
	for _, userID := range userIDs {
		balance, err := balanceForUpdate(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		balances[userID] = balance
	}
	return balances, nil
}

func updateCreditBalance(ctx context.Context, tx pgx.Tx, userID, total int64, updatedAt time.Time) error {
	_, err := tx.Exec(ctx, `
UPDATE credit_balances
SET total = $1, updated_at = $2
WHERE user_id = $3
`, total, updatedAt, userID)
	return err
}

func insertLedgerEntry(ctx context.Context, tx pgx.Tx, entry domain.LedgerEntry) error {
	_, err := tx.Exec(ctx, `
INSERT INTO credit_ledger(user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, entry.UserID, entry.Delta, entry.BalanceAfter, entry.Reason, entry.Description, entry.SourceEventID, entry.SourceType, entry.SourceID, entry.CreatedAt)
	return err
}

func insertReservation(ctx context.Context, tx pgx.Tx, reservation domain.CreditReservation) (domain.CreditReservation, error) {
	err := tx.QueryRow(ctx, `
INSERT INTO credit_reservations(user_id, amount, status, reason, description, source_event_id, source_type, source_id, created_at, updated_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, user_id, amount, status, reason, description, source_event_id, source_type, source_id, created_at, updated_at, COALESCE(settled_at, '0001-01-01T00:00:00Z'::timestamptz)
`, reservation.UserID, reservation.Amount, reservation.Status, reservation.Reason, reservation.Description, reservation.SourceEventID, reservation.SourceType, reservation.SourceID, reservation.CreatedAt, reservation.UpdatedAt).Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.Amount,
		&reservation.Status,
		&reservation.Reason,
		&reservation.Description,
		&reservation.SourceEventID,
		&reservation.SourceType,
		&reservation.SourceID,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
		&reservation.SettledAt,
	)
	return reservation, err
}

func reservationByEventForUpdate(ctx context.Context, tx pgx.Tx, userID int64, eventID, reason string) (domain.CreditReservation, error) {
	var reservation domain.CreditReservation
	err := tx.QueryRow(ctx, `
SELECT id, user_id, amount, status, reason, description, source_event_id, source_type, source_id, created_at, updated_at, COALESCE(settled_at, '0001-01-01T00:00:00Z'::timestamptz)
FROM credit_reservations
WHERE user_id = $1 AND source_event_id = $2 AND reason = $3
FOR UPDATE
`, userID, eventID, reason).Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.Amount,
		&reservation.Status,
		&reservation.Reason,
		&reservation.Description,
		&reservation.SourceEventID,
		&reservation.SourceType,
		&reservation.SourceID,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
		&reservation.SettledAt,
	)
	return reservation, err
}

func balanceForUpdate(ctx context.Context, tx creditQueryer, userID int64) (domain.Balance, error) {
	var balance domain.Balance
	err := tx.QueryRow(ctx, `
SELECT user_id, total, updated_at
FROM credit_balances
WHERE user_id = $1
FOR UPDATE
`, userID).Scan(&balance.UserID, &balance.Total, &balance.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Balance{UserID: userID, Total: 0}, nil
	}
	return balance, err
}

func ledgerExists(ctx context.Context, tx creditQueryer, userID int64, eventID, reason string) (bool, error) {
	_, err := ledgerByEvent(ctx, tx, userID, eventID, reason)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func ledgerByEvent(ctx context.Context, tx creditQueryer, userID int64, eventID, reason string) (domain.LedgerEntry, error) {
	var item domain.LedgerEntry
	err := tx.QueryRow(ctx, `
SELECT id, user_id, delta, balance_after, reason, description, source_event_id, source_type, source_id, created_at
FROM credit_ledger
WHERE user_id = $1 AND source_event_id = $2 AND reason = $3
`, userID, eventID, reason).Scan(&item.ID, &item.UserID, &item.Delta, &item.BalanceAfter, &item.Reason, &item.Description, &item.SourceEventID, &item.SourceType, &item.SourceID, &item.CreatedAt)
	return item, err
}

func articleOwnerDescription(reason string, actorID int64, title string) string {
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

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
