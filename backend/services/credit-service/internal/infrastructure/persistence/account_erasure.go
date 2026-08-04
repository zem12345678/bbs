package persistence

import (
	"context"
	"database/sql"
	"errors"

	domain "credit-service/internal/domain/credit"

	"github.com/jackc/pgx/v5"
)

type accountErasureReceipt struct {
	AnonymizedLedgerEntries int64
	AnonymizedReservations  int64
	DeletedCheckIns         int64
	PolicyVersion           int32
	CompletedAt             sql.NullTime
}

func (r *PostgresRepository) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	if r == nil || r.pool == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, domain.ErrInvalidAccountErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('bbs-credit-user:' || $1::BIGINT::TEXT, 0))`, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL bbs.credit_erasure = 'on'`); err != nil {
		return domain.AccountErasureResult{}, err
	}

	var receipt accountErasureReceipt
	var found bool
	var existingAnonymizedUserID int64
	err = tx.QueryRow(ctx, `
SELECT anonymized_user_id, anonymized_ledger_entries, anonymized_reservations, deleted_check_ins, policy_version, completed_at
FROM credit_erased_users WHERE user_id = $1 FOR UPDATE
`, userID).Scan(&existingAnonymizedUserID, &receipt.AnonymizedLedgerEntries, &receipt.AnonymizedReservations, &receipt.DeletedCheckIns, &receipt.PolicyVersion, &receipt.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		found = false
	} else if err != nil {
		return domain.AccountErasureResult{}, err
	} else {
		found = true
	}
	if found && receipt.CompletedAt.Valid && policyVersion <= receipt.PolicyVersion {
		if err := tx.Commit(ctx); err != nil {
			return domain.AccountErasureResult{}, err
		}
		return receipt.result(), nil
	}

	var anonymizedUserID int64
	if !found {
		if err := tx.QueryRow(ctx, `
INSERT INTO credit_erased_users(user_id, deletion_job_id, policy_version, anonymized_user_id)
VALUES($1, $2, $3, nextval('credit_erased_user_id_seq'))
RETURNING anonymized_user_id
`, userID, deletionJobID, policyVersion).Scan(&anonymizedUserID); err != nil {
			return domain.AccountErasureResult{}, err
		}
		receipt.PolicyVersion = policyVersion
	} else {
		anonymizedUserID = existingAnonymizedUserID
		if policyVersion > receipt.PolicyVersion {
			if _, err := tx.Exec(ctx, `UPDATE credit_erased_users SET deletion_job_id = $2, policy_version = $3, completed_at = NULL WHERE user_id = $1`, userID, deletionJobID, policyVersion); err != nil {
				return domain.AccountErasureResult{}, err
			}
			receipt.PolicyVersion = policyVersion
		}
	}

	ledgerResult, err := tx.Exec(ctx, `UPDATE credit_ledger
SET user_id = $2, description = 'account-erased', source_event_id = 'account-erased:' || id::TEXT, source_type = 'account_erasure', source_id = 0
WHERE user_id = $1
	`, userID, anonymizedUserID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	reservationCount, err := tx.Exec(ctx, `
UPDATE credit_reservations
SET user_id = $2, description = 'account-erased', source_event_id = 'account-erased-reservation:' || id::TEXT,
    source_type = 'account_erasure', source_id = 0,
    status = CASE WHEN status = 'ACTIVE' THEN 'RELEASED' ELSE status END,
    settled_at = COALESCE(settled_at, NOW()), updated_at = NOW()
WHERE user_id = $1
`, userID, anonymizedUserID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	checkInCount, err := tx.Exec(ctx, `UPDATE check_ins SET user_id = $2 WHERE user_id = $1`, userID, anonymizedUserID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE credit_balances SET user_id = $2, total = 0, updated_at = NOW() WHERE user_id = $1`, userID, anonymizedUserID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pending_article_credits WHERE actor_id = $1 OR article_id IN (SELECT article_id FROM article_authors WHERE author_id = $1)`, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}
	for _, statement := range []string{
		`DELETE FROM article_authors WHERE author_id = $1`,
		`DELETE FROM published_topics WHERE author_id = $1`,
		`DELETE FROM created_comments WHERE author_id = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, userID); err != nil {
			return domain.AccountErasureResult{}, err
		}
	}
	ledgerCount := ledgerResult.RowsAffected()
	reservationTotal := reservationCount.RowsAffected()
	checkInTotal := checkInCount.RowsAffected()
	if found {
		ledgerCount += receipt.AnonymizedLedgerEntries
		reservationTotal += receipt.AnonymizedReservations
		checkInTotal += receipt.DeletedCheckIns
	}
	if _, err := tx.Exec(ctx, `UPDATE credit_erased_users SET anonymized_ledger_entries = $2, anonymized_reservations = $3, deleted_check_ins = $4, completed_at = NOW() WHERE user_id = $1`, userID, ledgerCount, reservationTotal, checkInTotal); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if r.leaderboardCache != nil {
		_ = r.leaderboardCache.Remove(ctx, userID)
	}
	return domain.AccountErasureResult{AnonymizedLedgerEntries: ledgerCount, AnonymizedReservations: reservationTotal, DeletedCheckIns: checkInTotal}, nil
}

func (r accountErasureReceipt) result() domain.AccountErasureResult {
	return domain.AccountErasureResult{AnonymizedLedgerEntries: r.AnonymizedLedgerEntries, AnonymizedReservations: r.AnonymizedReservations, DeletedCheckIns: r.DeletedCheckIns}
}

var _ domain.AccountErasureRepository = (*PostgresRepository)(nil)
