package persistence

import (
	"context"
	"errors"
	"time"

	domain "chat-service/internal/domain/chat"

	"github.com/jackc/pgx/v5"
)

type accountErasureReceipt struct {
	UserID                 int64
	DeletionJobID          int64
	PolicyVersion          int32
	RedactedMessages       int64
	DeletedMemberships     int64
	DeletedGroups          int64
	TransferredRooms       int64
	ClosedRooms            int64
	SuppressedOutboxEvents int64
	ErasedAt               time.Time
}

func (r *PostgresRepository) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	if r == nil || r.pool == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, domain.ErrInvalidErasure
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockChatUser(ctx, tx, userID); err != nil {
		return domain.AccountErasureResult{}, err
	}

	receipt, found, err := loadAccountErasureReceipt(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	if found && policyVersion <= receipt.PolicyVersion {
		if err := tx.Commit(ctx); err != nil {
			return domain.AccountErasureResult{}, err
		}
		return receipt.result(), nil
	}
	if !found {
		receipt = accountErasureReceipt{UserID: userID, DeletionJobID: deletionJobID, PolicyVersion: policyVersion, ErasedAt: time.Now().UTC()}
		if _, err := tx.Exec(ctx, `
INSERT INTO chat_erased_users(user_id, deletion_job_id, policy_version, erased_at)
VALUES($1, $2, $3, $4)
`, userID, deletionJobID, policyVersion, receipt.ErasedAt); err != nil {
			return domain.AccountErasureResult{}, err
		}
	} else {
		receipt.DeletionJobID = deletionJobID
		receipt.PolicyVersion = policyVersion
		if _, err := tx.Exec(ctx, `
UPDATE chat_erased_users
SET deletion_job_id = $2, policy_version = $3
WHERE user_id = $1
`, userID, deletionJobID, policyVersion); err != nil {
			return domain.AccountErasureResult{}, err
		}
	}

	transferred, closed, err := transferOrCloseOwnedRooms(ctx, tx, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	redactedMessages, err := tx.Exec(ctx, `
UPDATE chat_messages
SET sender_id = 0,
    body = '',
    status = $2,
    deleted_at = COALESCE(deleted_at, NOW()),
    updated_at = NOW()
WHERE sender_id = $1
`, userID, domain.MessageStatusDeleted)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	deletedMemberships, err := tx.Exec(ctx, `DELETE FROM chat_room_members WHERE user_id = $1`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	deletedGroups, err := tx.Exec(ctx, `DELETE FROM chat_room_groups WHERE user_id = $1`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}
	suppressedOutbox, err := tx.Exec(ctx, `
UPDATE chat_outbox
SET payload = jsonb_build_object(
      'eventId', payload->'eventId',
      'eventType', payload->'eventType',
      'occurredAt', payload->'occurredAt',
      'producer', 'chat-service',
      'actorId', 0,
      'version', COALESCE(payload->'version', '1'::jsonb),
      'payload', jsonb_build_object('erased', TRUE)
    ),
    status = 'published',
    published_at = COALESCE(published_at, NOW()),
    lease_owner = '',
    lease_expires_at = NULL,
    last_error = '',
    updated_at = NOW()
WHERE payload->>'actorId' = ($1::BIGINT)::TEXT
`, userID)
	if err != nil {
		return domain.AccountErasureResult{}, err
	}

	receipt.RedactedMessages += redactedMessages.RowsAffected()
	receipt.DeletedMemberships += deletedMemberships.RowsAffected()
	receipt.DeletedGroups += deletedGroups.RowsAffected()
	receipt.TransferredRooms += transferred
	receipt.ClosedRooms += closed
	receipt.SuppressedOutboxEvents += suppressedOutbox.RowsAffected()
	if _, err := tx.Exec(ctx, `
UPDATE chat_erased_users
SET redacted_messages = $2,
    deleted_memberships = $3,
    deleted_groups = $4,
    transferred_rooms = $5,
    closed_rooms = $6,
    suppressed_outbox_events = $7
WHERE user_id = $1
`, userID, receipt.RedactedMessages, receipt.DeletedMemberships, receipt.DeletedGroups,
		receipt.TransferredRooms, receipt.ClosedRooms, receipt.SuppressedOutboxEvents); err != nil {
		return domain.AccountErasureResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccountErasureResult{}, err
	}
	return receipt.result(), nil
}

func loadAccountErasureReceipt(ctx context.Context, tx pgx.Tx, userID int64) (accountErasureReceipt, bool, error) {
	var receipt accountErasureReceipt
	err := tx.QueryRow(ctx, `
SELECT user_id, deletion_job_id, policy_version, redacted_messages,
       deleted_memberships, deleted_groups, transferred_rooms, closed_rooms,
       suppressed_outbox_events, erased_at
FROM chat_erased_users
WHERE user_id = $1
FOR UPDATE
`, userID).Scan(&receipt.UserID, &receipt.DeletionJobID, &receipt.PolicyVersion, &receipt.RedactedMessages,
		&receipt.DeletedMemberships, &receipt.DeletedGroups, &receipt.TransferredRooms, &receipt.ClosedRooms,
		&receipt.SuppressedOutboxEvents, &receipt.ErasedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return accountErasureReceipt{}, false, nil
	}
	return receipt, err == nil, err
}

func transferOrCloseOwnedRooms(ctx context.Context, tx pgx.Tx, userID int64) (int64, int64, error) {
	rows, err := tx.Query(ctx, `
SELECT id
FROM chat_rooms
WHERE creator_id = $1
ORDER BY id
FOR UPDATE
`, userID)
	if err != nil {
		return 0, 0, err
	}
	roomIDs := make([]int64, 0)
	for rows.Next() {
		var roomID int64
		if err := rows.Scan(&roomID); err != nil {
			rows.Close()
			return 0, 0, err
		}
		roomIDs = append(roomIDs, roomID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, err
	}

	var transferred, closed int64
	for _, roomID := range roomIDs {
		wasTransferred, err := transferOrCloseRoom(ctx, tx, roomID, userID)
		if err != nil {
			return transferred, closed, err
		}
		if wasTransferred {
			transferred++
		} else {
			closed++
		}
	}
	return transferred, closed, nil
}

func transferOrCloseRoom(ctx context.Context, tx pgx.Tx, roomID, ownerID int64) (bool, error) {
	var successorID int64
	err := tx.QueryRow(ctx, `
SELECT member.user_id
FROM chat_room_members member
WHERE member.room_id = $1
  AND member.user_id <> $2
  AND member.status = $3
  AND NOT EXISTS (SELECT 1 FROM chat_erased_users erased WHERE erased.user_id = member.user_id)
ORDER BY member.joined_at, member.user_id
LIMIT 1
FOR UPDATE
`, roomID, ownerID, domain.MemberStatusJoined).Scan(&successorID)
	switch {
	case err == nil:
		if _, err := tx.Exec(ctx, `
UPDATE chat_room_members
SET role = $3,
    muted_until = NULL,
    updated_at = NOW()
WHERE room_id = $1 AND user_id = $2
`, roomID, successorID, domain.MemberRoleOwner); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE chat_rooms
SET creator_id = $2,
    announcement = '',
    announcement_version = announcement_version + 1,
    updated_at = NOW()
WHERE id = $1
`, roomID, successorID); err != nil {
			return false, err
		}
		return true, nil
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx, `
UPDATE chat_rooms
SET creator_id = 0,
    announcement = '',
    announcement_version = announcement_version + 1,
    status = $2,
    updated_at = NOW()
WHERE id = $1
`, roomID, domain.RoomStatusClosed); err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, err
	}
}

func (r accountErasureReceipt) result() domain.AccountErasureResult {
	return domain.AccountErasureResult{
		RedactedMessages: r.RedactedMessages, DeletedMemberships: r.DeletedMemberships,
		DeletedGroups: r.DeletedGroups, TransferredRooms: r.TransferredRooms,
		ClosedRooms: r.ClosedRooms, SuppressedOutboxEvents: r.SuppressedOutboxEvents,
	}
}

func lockChatUser(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID)
	return err
}

func ensureChatUserActive(ctx context.Context, tx pgx.Tx, userID int64) error {
	if err := lockChatUser(ctx, tx, userID); err != nil {
		return err
	}
	var erased bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM chat_erased_users WHERE user_id = $1)`, userID).Scan(&erased); err != nil {
		return err
	}
	if erased {
		return domain.ErrUserErased
	}
	return nil
}

var _ domain.AccountErasureRepository = (*PostgresRepository)(nil)
