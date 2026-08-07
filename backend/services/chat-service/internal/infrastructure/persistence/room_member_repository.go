package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	domain "chat-service/internal/domain/chat"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ListRoomMembers(ctx context.Context, roomNo string, requesterID int64, query domain.RoomMemberQuery) (domain.RoomMemberPage, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.RoomMemberPage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var roomID int64
	var roomStatus int16
	var requesterRole, requesterStatus sql.NullInt16
	err = tx.QueryRow(ctx, `
SELECT r.id, r.status, m.role, m.status
FROM chat_rooms r
LEFT JOIN chat_room_members m
  ON m.room_id = r.id AND m.user_id = $2
WHERE r.room_no = $1
`, roomNo, requesterID).Scan(&roomID, &roomStatus, &requesterRole, &requesterStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoomMemberPage{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RoomMemberPage{}, err
	}
	if roomStatus != domain.RoomStatusActive {
		return domain.RoomMemberPage{}, domain.ErrRoomClosed
	}
	if !requesterRole.Valid || !requesterStatus.Valid || requesterStatus.Int16 != domain.MemberStatusJoined {
		return domain.RoomMemberPage{}, domain.ErrNotMember
	}

	var page domain.RoomMemberPage
	err = tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM chat_room_members
WHERE room_id = $1 AND status = $2
  AND ($3::SMALLINT = 0 OR role = $3)
  AND ($4::BIGINT = 0 OR user_id = $4)
`, roomID, domain.MemberStatusJoined, query.Role, query.UserID).Scan(&page.Total)
	if err != nil {
		return domain.RoomMemberPage{}, err
	}

	rows, err := tx.Query(ctx, `
SELECT `+membershipColumns+`
FROM chat_room_members
WHERE room_id = $1 AND status = $2
  AND ($3::SMALLINT = 0 OR role = $3)
  AND ($4::BIGINT = 0 OR user_id = $4)
ORDER BY joined_at, user_id
LIMIT $5 OFFSET $6
`, roomID, domain.MemberStatusJoined, query.Role, query.UserID, query.Limit, query.Offset)
	if err != nil {
		return domain.RoomMemberPage{}, err
	}
	defer rows.Close()
	page.Members = make([]domain.Membership, 0)
	for rows.Next() {
		var member domain.Membership
		if err := scanMembership(rows, &member); err != nil {
			return domain.RoomMemberPage{}, err
		}
		page.Members = append(page.Members, member)
	}
	if err := rows.Err(); err != nil {
		return domain.RoomMemberPage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RoomMemberPage{}, err
	}
	return page, nil
}

func (r *PostgresRepository) UpdateRoomMemberRole(ctx context.Context, roomNo string, actorID, userID int64, role int16, eventID string) (domain.Membership, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureChatUserActive(ctx, tx, actorID); err != nil {
		return domain.Membership{}, err
	}
	room, actor, target, err := lockRoomAndGovernanceMembers(ctx, tx, roomNo, actorID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	if actor.Role != domain.MemberRoleOwner {
		return domain.Membership{}, domain.ErrNotOwner
	}
	if target.Role == domain.MemberRoleOwner {
		return domain.Membership{}, domain.ErrMemberActionDenied
	}

	err = scanMembership(tx.QueryRow(ctx, `
UPDATE chat_room_members
SET role = $3, updated_at = NOW()
WHERE room_id = $1 AND user_id = $2 AND status = $4
RETURNING `+membershipColumns+`
`, room.ID, userID, role, domain.MemberStatusJoined), &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.membership.role_updated.v1", room.ID, actorID, map[string]any{
		"roomId": room.ID, "roomNo": room.RoomNo, "userId": userID, "role": role,
	}); err != nil {
		return domain.Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return target, nil
}

func (r *PostgresRepository) MuteRoomMember(ctx context.Context, roomNo string, actorID, userID int64, mutedUntil time.Time, eventID string) (domain.Membership, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureChatUserActive(ctx, tx, actorID); err != nil {
		return domain.Membership{}, err
	}
	room, actor, target, err := lockRoomAndGovernanceMembers(ctx, tx, roomNo, actorID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	if !canModerate(actor.Role, target.Role) {
		return domain.Membership{}, domain.ErrMemberActionDenied
	}

	err = scanMembership(tx.QueryRow(ctx, `
UPDATE chat_room_members
SET muted_until = $3, updated_at = NOW()
WHERE room_id = $1 AND user_id = $2 AND status = $4
RETURNING `+membershipColumns+`
`, room.ID, userID, mutedUntil, domain.MemberStatusJoined), &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.membership.muted.v1", room.ID, actorID, map[string]any{
		"roomId": room.ID, "roomNo": room.RoomNo, "userId": userID, "mutedUntil": mutedUntil.UnixMilli(),
	}); err != nil {
		return domain.Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return target, nil
}

func (r *PostgresRepository) UnmuteRoomMember(ctx context.Context, roomNo string, actorID, userID int64, eventID string) (domain.Membership, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureChatUserActive(ctx, tx, actorID); err != nil {
		return domain.Membership{}, err
	}
	room, actor, target, err := lockRoomAndGovernanceMembers(ctx, tx, roomNo, actorID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	if !canModerate(actor.Role, target.Role) {
		return domain.Membership{}, domain.ErrMemberActionDenied
	}

	err = scanMembership(tx.QueryRow(ctx, `
UPDATE chat_room_members
SET muted_until = NULL, updated_at = NOW()
WHERE room_id = $1 AND user_id = $2 AND status = $3
RETURNING `+membershipColumns+`
`, room.ID, userID, domain.MemberStatusJoined), &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.membership.unmuted.v1", room.ID, actorID, map[string]any{
		"roomId": room.ID, "roomNo": room.RoomNo, "userId": userID,
	}); err != nil {
		return domain.Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return target, nil
}

func lockRoomAndGovernanceMembers(ctx context.Context, tx pgx.Tx, roomNo string, actorID, userID int64) (domain.Room, domain.Membership, domain.Membership, error) {
	room, actor, found, err := lockRoomThenMemberForUpdate(ctx, tx, roomNo, actorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, err
	}
	if room.Status != domain.RoomStatusActive {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, domain.ErrRoomClosed
	}
	if !found || actor.Status != domain.MemberStatusJoined {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, domain.ErrNotMember
	}
	if actorID == userID {
		return room, actor, actor, nil
	}

	var target domain.Membership
	err = scanMembership(tx.QueryRow(ctx, `
SELECT `+membershipColumns+`
FROM chat_room_members
WHERE room_id = $1 AND user_id = $2 AND status = $3
FOR UPDATE
`, room.ID, userID, domain.MemberStatusJoined), &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Room{}, domain.Membership{}, domain.Membership{}, err
	}
	return room, actor, target, nil
}

func canModerate(actorRole, targetRole int16) bool {
	if targetRole == domain.MemberRoleOwner {
		return false
	}
	if actorRole == domain.MemberRoleOwner {
		return targetRole == domain.MemberRoleMember || targetRole == domain.MemberRoleManager
	}
	return actorRole == domain.MemberRoleManager && targetRole == domain.MemberRoleMember
}

var _ domain.Repository = (*PostgresRepository)(nil)
