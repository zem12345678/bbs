package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS credit_balances (
  user_id BIGINT PRIMARY KEY,
  total BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS article_authors (
  article_id BIGINT PRIMARY KEY,
  author_id BIGINT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
`)
	return err
}

func (r *PostgresRepository) SaveArticle(ctx context.Context, article domain.ArticleRef, publishedAt time.Time) error {
	if article.ID <= 0 || article.AuthorID <= 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
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

func (r *PostgresRepository) AddCredit(ctx context.Context, entry domain.LedgerEntry) error {
	if entry.UserID <= 0 || entry.Delta == 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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
	return tx.Commit(ctx)
}

func (r *PostgresRepository) AdjustCredit(ctx context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	if entry.UserID <= 0 || entry.Delta == 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := ledgerByEvent(ctx, tx, entry.UserID, entry.SourceEventID, entry.Reason)
	if err == nil {
		balance, balanceErr := balanceForUpdate(ctx, tx, entry.UserID)
		if balanceErr != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, balanceErr
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO credit_balances(user_id, total, updated_at)
VALUES($1, 0, NOW())
ON CONFLICT(user_id) DO NOTHING
`, entry.UserID); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	balance, err := balanceForUpdate(ctx, tx, entry.UserID)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
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
	if err := tx.Commit(ctx); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	return entry, balance, false, nil
}

func (r *PostgresRepository) DebitCredit(ctx context.Context, entry domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	if entry.UserID <= 0 || entry.Delta >= 0 || entry.SourceEventID == "" || entry.Reason == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := ledgerByEvent(ctx, tx, entry.UserID, entry.SourceEventID, entry.Reason)
	if err == nil {
		balance, balanceErr := balanceForUpdate(ctx, tx, entry.UserID)
		if balanceErr != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, balanceErr
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.LedgerEntry{}, domain.Balance{}, false, err
		}
		return existing, balance, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}

	balance, err := balanceForUpdate(ctx, tx, entry.UserID)
	if err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
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
	if err := tx.Commit(ctx); err != nil {
		return domain.LedgerEntry{}, domain.Balance{}, false, err
	}
	return entry, balance, false, nil
}

func (r *PostgresRepository) TransferCredit(ctx context.Context, debit domain.LedgerEntry, credit domain.LedgerEntry) error {
	if debit.UserID <= 0 || debit.Delta >= 0 || debit.SourceEventID == "" || debit.Reason == "" {
		return nil
	}
	if credit.UserID <= 0 || credit.Delta <= 0 || credit.SourceEventID == "" || credit.Reason == "" {
		return nil
	}
	now := time.Now()
	if debit.CreatedAt.IsZero() {
		debit.CreatedAt = now
	}
	if credit.CreatedAt.IsZero() {
		credit.CreatedAt = debit.CreatedAt
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

	debitExists, err := ledgerExists(ctx, tx, debit.UserID, debit.SourceEventID, debit.Reason)
	if err != nil {
		return err
	}
	creditExists, err := ledgerExists(ctx, tx, credit.UserID, credit.SourceEventID, credit.Reason)
	if err != nil {
		return err
	}
	if err := validateTransferLedgerState(debitExists, creditExists); err != nil {
		return err
	}
	if debitExists && creditExists {
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
	return tx.Commit(ctx)
}

func validateTransferLedgerState(debitExists, creditExists bool) error {
	if debitExists != creditExists {
		return domain.ErrInconsistentCreditTransfer
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
	_, err := r.pool.Exec(ctx, `
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
	_, err = r.pool.Exec(ctx, `DELETE FROM pending_article_credits WHERE article_id = $1`, article.ID)
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

func ensureBalanceRow(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
INSERT INTO credit_balances(user_id, total, updated_at)
VALUES($1, 0, NOW())
ON CONFLICT(user_id) DO NOTHING
`, userID)
	return err
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

func balanceForUpdate(ctx context.Context, tx pgx.Tx, userID int64) (domain.Balance, error) {
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

func ledgerExists(ctx context.Context, tx pgx.Tx, userID int64, eventID, reason string) (bool, error) {
	_, err := ledgerByEvent(ctx, tx, userID, eventID, reason)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func ledgerByEvent(ctx context.Context, tx pgx.Tx, userID int64, eventID, reason string) (domain.LedgerEntry, error) {
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
