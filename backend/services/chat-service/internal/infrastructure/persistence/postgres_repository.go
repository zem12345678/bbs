package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	domain "chat-service/internal/domain/chat"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const roomColumns = `
  id, room_no, name, creator_id, announcement, announcement_version,
  last_message_seq, status, created_at, updated_at`

const membershipColumns = `
  room_id, user_id, role, status, joined_at_seq, last_read_seq,
  last_seen_announcement_version, group_id, sort_order, joined_at, left_at,
  created_at, updated_at`

const messageColumns = `
  id, room_id, seq, sender_id, client_message_id::text,
  CASE WHEN status = 2 THEN '' ELSE body END,
  status, created_at, updated_at, deleted_at`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateRoom(ctx context.Context, room domain.Room, owner domain.Membership) (domain.RoomDetails, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoomDetails{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, owner.UserID); err != nil {
		return domain.RoomDetails{}, err
	}
	members, err := listUserRoomPlacementsForUpdate(ctx, tx, owner.UserID)
	if err != nil {
		return domain.RoomDetails{}, err
	}

	err = tx.QueryRow(ctx, `
INSERT INTO chat_rooms(id, room_no, name, creator_id, status)
VALUES($1, $2, $3, $4, $5)
RETURNING created_at, updated_at
`, room.ID, room.RoomNo, room.Name, room.CreatorID, room.Status).Scan(&room.CreatedAt, &room.UpdatedAt)
	if err != nil {
		return domain.RoomDetails{}, mapPostgresError(err)
	}

	owner.RoomID = room.ID
	placementUpdates := appendRoomPlacement(members, &owner)
	err = tx.QueryRow(ctx, `
INSERT INTO chat_room_members(
  room_id, user_id, role, status, joined_at_seq, last_read_seq,
  last_seen_announcement_version, sort_order
)
VALUES($1, $2, $3, $4, 0, 0, 0, $5)
RETURNING joined_at, created_at, updated_at
`, room.ID, owner.UserID, owner.Role, owner.Status, owner.SortOrder).Scan(&owner.JoinedAt, &owner.CreatedAt, &owner.UpdatedAt)
	if err != nil {
		return domain.RoomDetails{}, mapPostgresError(err)
	}
	if err := setUserRoomPlacementOrders(ctx, tx, owner.UserID, placementUpdates); err != nil {
		return domain.RoomDetails{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RoomDetails{}, err
	}
	return domain.RoomDetails{Room: room, Membership: &owner, MemberCount: 1}, nil
}

func (r *PostgresRepository) LookupRoom(ctx context.Context, roomNo string, userID int64) (domain.RoomDetails, error) {
	var details domain.RoomDetails
	err := scanRoom(r.pool.QueryRow(ctx, `
SELECT `+roomColumns+`
FROM chat_rooms
WHERE room_no = $1
`, roomNo), &details.Room)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoomDetails{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RoomDetails{}, err
	}
	if err := r.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM chat_room_members
WHERE room_id = $1 AND status = $2
`, details.Room.ID, domain.MemberStatusJoined).Scan(&details.MemberCount); err != nil {
		return domain.RoomDetails{}, err
	}
	if userID <= 0 {
		return details, nil
	}
	var member domain.Membership
	err = scanMembership(r.pool.QueryRow(ctx, `
SELECT `+membershipColumns+`
FROM chat_room_members
WHERE room_id = $1 AND user_id = $2 AND status = $3
`, details.Room.ID, userID, domain.MemberStatusJoined), &member)
	if err == nil {
		details.Membership = &member
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return domain.RoomDetails{}, err
	}
	return details, nil
}

func (r *PostgresRepository) JoinRoom(ctx context.Context, roomNo string, userID int64, eventID string) (domain.RoomDetails, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RoomDetails{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, userID); err != nil {
		return domain.RoomDetails{}, err
	}

	room, member, memberFound, err := lockRoomThenMemberForUpdate(ctx, tx, roomNo, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoomDetails{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RoomDetails{}, err
	}
	if room.Status != domain.RoomStatusActive {
		return domain.RoomDetails{}, domain.ErrRoomClosed
	}

	if memberFound && member.Status == domain.MemberStatusJoined {
		if err := tx.Commit(ctx); err != nil {
			return domain.RoomDetails{}, err
		}
		return r.LookupRoom(ctx, roomNo, userID)
	}
	members, err := listUserRoomPlacementsForUpdate(ctx, tx, userID)
	if err != nil {
		return domain.RoomDetails{}, err
	}

	if !memberFound {
		member = domain.Membership{
			RoomID:      room.ID,
			UserID:      userID,
			Role:        domain.MemberRoleMember,
			Status:      domain.MemberStatusJoined,
			JoinedAtSeq: room.LastMessageSeq,
			LastReadSeq: room.LastMessageSeq,
		}
	}
	placementUpdates := appendRoomPlacement(members, &member)
	if !memberFound {
		err = scanMembership(tx.QueryRow(ctx, `
INSERT INTO chat_room_members(
  room_id, user_id, role, status, joined_at_seq, last_read_seq,
  last_seen_announcement_version, sort_order
)
VALUES($1, $2, $3, $4, $5, $5, 0, $6)
RETURNING `+membershipColumns+`
`, room.ID, userID, domain.MemberRoleMember, domain.MemberStatusJoined, room.LastMessageSeq, member.SortOrder), &member)
	} else {
		err = scanMembership(tx.QueryRow(ctx, `
UPDATE chat_room_members
SET status = $3,
    joined_at_seq = $4,
    last_read_seq = $4,
    sort_order = $5,
    joined_at = NOW(),
    left_at = NULL,
    updated_at = NOW()
WHERE room_id = $1 AND user_id = $2
RETURNING `+membershipColumns+`
`, room.ID, userID, domain.MemberStatusJoined, room.LastMessageSeq, member.SortOrder), &member)
	}
	if err != nil {
		return domain.RoomDetails{}, mapPostgresError(err)
	}
	if err := setUserRoomPlacementOrders(ctx, tx, userID, placementUpdates); err != nil {
		return domain.RoomDetails{}, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.membership.joined.v1", room.ID, userID, map[string]any{
		"roomId": room.ID, "roomNo": room.RoomNo, "userId": userID,
	}); err != nil {
		return domain.RoomDetails{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RoomDetails{}, err
	}
	return r.LookupRoom(ctx, roomNo, userID)
}

func (r *PostgresRepository) ListSidebar(ctx context.Context, userID int64) (domain.Sidebar, error) {
	groups, err := r.listGroups(ctx, userID)
	if err != nil {
		return domain.Sidebar{}, err
	}
	rows, err := r.pool.Query(ctx, `
SELECT
  r.id, r.room_no, r.name, r.creator_id, r.announcement, r.announcement_version,
  r.last_message_seq, r.status, r.created_at, r.updated_at,
  m.room_id, m.user_id, m.role, m.status, m.joined_at_seq, m.last_read_seq,
  m.last_seen_announcement_version, m.group_id, m.sort_order, m.joined_at,
  m.left_at, m.created_at, m.updated_at,
  lm.id, lm.room_id, lm.seq, lm.sender_id, lm.client_message_id::text,
  CASE WHEN lm.status = 2 THEN '' ELSE lm.body END,
  lm.status, lm.created_at, lm.updated_at, lm.deleted_at,
  GREATEST(0, r.last_message_seq - m.last_read_seq)
FROM chat_room_members m
JOIN chat_rooms r ON r.id = m.room_id
LEFT JOIN LATERAL (
  SELECT id, room_id, seq, sender_id, client_message_id, body, status,
         created_at, updated_at, deleted_at
  FROM chat_messages
  WHERE room_id = r.id
  ORDER BY seq DESC
  LIMIT 1
) lm ON TRUE
WHERE m.user_id = $1 AND m.status = $2 AND r.status = $3
ORDER BY m.group_id NULLS FIRST, m.sort_order, r.updated_at DESC, r.id DESC
`, userID, domain.MemberStatusJoined, domain.RoomStatusActive)
	if err != nil {
		return domain.Sidebar{}, err
	}
	defer rows.Close()

	rooms := make([]domain.SidebarRoom, 0)
	for rows.Next() {
		var item domain.SidebarRoom
		var groupID sql.NullInt64
		var leftAt sql.NullTime
		var messageID sql.NullInt64
		var messageRoomID sql.NullInt64
		var messageSeq sql.NullInt64
		var senderID sql.NullInt64
		var clientMessageID sql.NullString
		var body sql.NullString
		var messageStatus sql.NullInt16
		var messageCreated sql.NullTime
		var messageUpdated sql.NullTime
		var messageDeleted sql.NullTime
		err := rows.Scan(
			&item.Room.ID, &item.Room.RoomNo, &item.Room.Name, &item.Room.CreatorID,
			&item.Room.Announcement, &item.Room.AnnouncementVersion, &item.Room.LastMessageSeq,
			&item.Room.Status, &item.Room.CreatedAt, &item.Room.UpdatedAt,
			&item.Membership.RoomID, &item.Membership.UserID, &item.Membership.Role,
			&item.Membership.Status, &item.Membership.JoinedAtSeq, &item.Membership.LastReadSeq,
			&item.Membership.LastSeenAnnouncementVersion, &groupID, &item.Membership.SortOrder,
			&item.Membership.JoinedAt, &leftAt, &item.Membership.CreatedAt, &item.Membership.UpdatedAt,
			&messageID, &messageRoomID, &messageSeq, &senderID, &clientMessageID, &body,
			&messageStatus, &messageCreated, &messageUpdated, &messageDeleted,
			&item.UnreadCount,
		)
		if err != nil {
			return domain.Sidebar{}, err
		}
		applyNullableMembership(&item.Membership, groupID, leftAt)
		if messageID.Valid {
			message := domain.Message{
				ID: messageID.Int64, RoomID: messageRoomID.Int64, Seq: messageSeq.Int64,
				SenderID: senderID.Int64, ClientMessageID: clientMessageID.String,
				Body: body.String, Status: messageStatus.Int16,
				CreatedAt: messageCreated.Time, UpdatedAt: messageUpdated.Time,
			}
			if messageDeleted.Valid {
				message.DeletedAt = &messageDeleted.Time
			}
			item.LastMessage = &message
		}
		rooms = append(rooms, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Sidebar{}, err
	}
	return domain.Sidebar{Groups: groups, Rooms: rooms}, nil
}

func (r *PostgresRepository) ListMessages(ctx context.Context, roomNo string, userID int64, query domain.MessageQuery) (domain.MessagePage, error) {
	var roomID, latestSeq int64
	err := r.pool.QueryRow(ctx, `
SELECT r.id, r.last_message_seq
FROM chat_rooms r
JOIN chat_room_members m ON m.room_id = r.id
WHERE r.room_no = $1 AND r.status = $2
  AND m.user_id = $3 AND m.status = $4
`, roomNo, domain.RoomStatusActive, userID, domain.MemberStatusJoined).Scan(&roomID, &latestSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MessagePage{}, domain.ErrNotMember
	}
	if err != nil {
		return domain.MessagePage{}, err
	}

	page := domain.MessagePage{LatestSeq: latestSeq}
	switch {
	case query.BeforeSeqSet:
		messages, more, err := r.loadDirectionalMessages(ctx, roomID, "seq < $2", query.BeforeSeq, query.Limit, true)
		if err != nil {
			return domain.MessagePage{}, err
		}
		page.Messages = messages
		page.HasOlder = more
		page.HasNewer = len(messages) > 0 && messages[len(messages)-1].Seq < latestSeq
	case query.AfterSeqSet:
		messages, more, err := r.loadDirectionalMessages(ctx, roomID, "seq > $2", query.AfterSeq, query.Limit, false)
		if err != nil {
			return domain.MessagePage{}, err
		}
		page.Messages = messages
		page.HasOlder = len(messages) > 0 && messages[0].Seq > 1
		page.HasNewer = more
	default:
		anchor := query.AnchorSeq
		if anchor > latestSeq {
			anchor = latestSeq
		}
		if anchor < 0 {
			anchor = 0
		}
		page.AnchorSeq = anchor
		older, olderMore, err := r.loadAnchorMessages(ctx, roomID, anchor, query.Before, true)
		if err != nil {
			return domain.MessagePage{}, err
		}
		newer, newerMore, err := r.loadAnchorMessages(ctx, roomID, anchor, query.After, false)
		if err != nil {
			return domain.MessagePage{}, err
		}
		page.Messages = append(older, newer...)
		page.HasOlder = olderMore
		page.HasNewer = newerMore
	}
	return page, nil
}

func (r *PostgresRepository) SendMessage(ctx context.Context, roomNo string, userID int64, message domain.Message, eventID string) (domain.Message, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Message{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	room, member, memberFound, err := lockRoomThenMemberForUpdate(ctx, tx, roomNo, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, 0, domain.ErrNotMember
	}
	if err != nil {
		return domain.Message{}, 0, err
	}
	if !memberFound || member.Status != domain.MemberStatusJoined {
		return domain.Message{}, 0, domain.ErrNotMember
	}
	if room.Status != domain.RoomStatusActive {
		return domain.Message{}, 0, domain.ErrRoomClosed
	}
	roomID, latestSeq := room.ID, room.LastMessageSeq

	var existing domain.Message
	err = scanMessage(tx.QueryRow(ctx, `
SELECT `+messageColumns+`
FROM chat_messages
WHERE room_id = $1 AND sender_id = $2 AND client_message_id = $3::uuid
`, roomID, userID, message.ClientMessageID), &existing)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return domain.Message{}, 0, err
		}
		return existing, latestSeq, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, 0, err
	}

	err = tx.QueryRow(ctx, `
UPDATE chat_rooms
SET last_message_seq = last_message_seq + 1, updated_at = NOW()
WHERE id = $1 AND status = $2
RETURNING last_message_seq
`, roomID, domain.RoomStatusActive).Scan(&message.Seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, 0, domain.ErrRoomClosed
	}
	if err != nil {
		return domain.Message{}, 0, err
	}
	message.RoomID = roomID
	err = tx.QueryRow(ctx, `
INSERT INTO chat_messages(id, room_id, seq, sender_id, client_message_id, body, status)
VALUES($1, $2, $3, $4, $5::uuid, $6, $7)
RETURNING created_at, updated_at
	`, message.ID, roomID, message.Seq, userID, message.ClientMessageID, message.Body, message.Status).Scan(&message.CreatedAt, &message.UpdatedAt)
	if err != nil {
		return domain.Message{}, 0, mapPostgresError(err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE chat_room_members
SET last_read_seq = GREATEST(last_read_seq, $3), updated_at = NOW()
WHERE room_id = $1 AND user_id = $2
`, roomID, userID, message.Seq); err != nil {
		return domain.Message{}, 0, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.message.created.v1", roomID, userID, map[string]any{
		"messageId": message.ID, "roomId": roomID, "roomNo": roomNo,
		"seq": message.Seq, "senderId": userID, "clientMessageId": message.ClientMessageID,
		"body": message.Body, "status": message.Status, "createdAt": message.CreatedAt.UnixMilli(),
	}); err != nil {
		return domain.Message{}, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Message{}, 0, err
	}
	return message, message.Seq, nil
}

func (r *PostgresRepository) DeleteMessage(ctx context.Context, roomNo string, userID, messageID int64, eventID string) (domain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Message{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	room, member, memberFound, err := lockRoomThenMemberForUpdate(ctx, tx, roomNo, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, domain.ErrNotMember
	}
	if err != nil {
		return domain.Message{}, err
	}
	if !memberFound || member.Status != domain.MemberStatusJoined {
		return domain.Message{}, domain.ErrNotMember
	}
	if room.Status != domain.RoomStatusActive {
		return domain.Message{}, domain.ErrRoomClosed
	}

	var message domain.Message
	err = scanMessage(tx.QueryRow(ctx, `
SELECT `+messageColumns+`
FROM chat_messages
WHERE room_id = $1 AND id = $2
FOR UPDATE
`, room.ID, messageID), &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Message{}, err
	}
	if message.SenderID != userID {
		return domain.Message{}, domain.ErrNotMessageAuthor
	}
	if message.Status == domain.MessageStatusDeleted {
		if err := tx.Commit(ctx); err != nil {
			return domain.Message{}, err
		}
		return message, nil
	}

	err = scanMessage(tx.QueryRow(ctx, `
UPDATE chat_messages
SET status = $3,
    deleted_at = COALESCE(deleted_at, NOW()),
    updated_at = NOW()
WHERE room_id = $1 AND id = $2 AND status = $4
RETURNING `+messageColumns+`
`, room.ID, messageID, domain.MessageStatusDeleted, domain.MessageStatusPublished), &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Message{}, err
	}
	deletedAt := int64(0)
	if message.DeletedAt != nil {
		deletedAt = message.DeletedAt.UnixMilli()
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.message.deleted.v1", room.ID, userID, map[string]any{
		"messageId": message.ID, "roomId": room.ID, "roomNo": roomNo,
		"seq": message.Seq, "senderId": message.SenderID, "body": "",
		"status": message.Status, "updatedAt": message.UpdatedAt.UnixMilli(), "deletedAt": deletedAt,
	}); err != nil {
		return domain.Message{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func (r *PostgresRepository) AdvanceRead(ctx context.Context, roomNo string, userID, readSeq int64, eventID string) (domain.Membership, int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var roomID, latestSeq int64
	var member domain.Membership
	var groupID sql.NullInt64
	var leftAt sql.NullTime
	err = tx.QueryRow(ctx, `
SELECT r.id, r.last_message_seq,
       m.room_id, m.user_id, m.role, m.status, m.joined_at_seq, m.last_read_seq,
       m.last_seen_announcement_version, m.group_id, m.sort_order, m.joined_at,
       m.left_at, m.created_at, m.updated_at
FROM chat_rooms r
JOIN chat_room_members m ON m.room_id = r.id
WHERE r.room_no = $1 AND r.status = $2
  AND m.user_id = $3 AND m.status = $4
FOR UPDATE OF m
`, roomNo, domain.RoomStatusActive, userID, domain.MemberStatusJoined).Scan(
		&roomID, &latestSeq,
		&member.RoomID, &member.UserID, &member.Role, &member.Status,
		&member.JoinedAtSeq, &member.LastReadSeq, &member.LastSeenAnnouncementVersion,
		&groupID, &member.SortOrder, &member.JoinedAt, &leftAt, &member.CreatedAt, &member.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, 0, domain.ErrNotMember
	}
	if err != nil {
		return domain.Membership{}, 0, err
	}
	applyNullableMembership(&member, groupID, leftAt)
	target := readSeq
	if target > latestSeq {
		target = latestSeq
	}
	if target > member.LastReadSeq {
		err = scanMembership(tx.QueryRow(ctx, `
UPDATE chat_room_members
SET last_read_seq = $3, updated_at = NOW()
WHERE room_id = $1 AND user_id = $2
RETURNING `+membershipColumns+`
`, roomID, userID, target), &member)
		if err != nil {
			return domain.Membership{}, 0, err
		}
		if err := insertOutbox(ctx, tx, eventID, "chat.read.advanced.v1", roomID, userID, map[string]any{
			"roomId": roomID, "roomNo": roomNo, "userId": userID,
			"lastReadSeq": member.LastReadSeq, "latestSeq": latestSeq,
		}); err != nil {
			return domain.Membership{}, 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, 0, err
	}
	return member, latestSeq, nil
}

func (r *PostgresRepository) CreateGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Group{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, group.UserID); err != nil {
		return domain.Group{}, err
	}
	err = tx.QueryRow(ctx, `
INSERT INTO chat_room_groups(id, user_id, name, sort_order)
VALUES($1, $2, $3, $4)
RETURNING created_at, updated_at
`, group.ID, group.UserID, group.Name, group.SortOrder).Scan(&group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		return domain.Group{}, mapPostgresError(err)
	}
	if err := normalizeUserGroupSortOrders(ctx, tx, group.UserID); err != nil {
		return domain.Group{}, err
	}
	if err := tx.QueryRow(ctx, `
SELECT sort_order, created_at, updated_at
FROM chat_room_groups
WHERE id = $1 AND user_id = $2
`, group.ID, group.UserID).Scan(&group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return domain.Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Group{}, err
	}
	return group, nil
}

func (r *PostgresRepository) UpdateGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Group{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, group.UserID); err != nil {
		return domain.Group{}, err
	}
	err = tx.QueryRow(ctx, `
UPDATE chat_room_groups
SET name = $3, sort_order = $4, updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING created_at, updated_at
`, group.ID, group.UserID, group.Name, group.SortOrder).Scan(&group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Group{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Group{}, mapPostgresError(err)
	}
	if err := normalizeUserGroupSortOrders(ctx, tx, group.UserID); err != nil {
		return domain.Group{}, err
	}
	if err := tx.QueryRow(ctx, `
SELECT sort_order, created_at, updated_at
FROM chat_room_groups
WHERE id = $1 AND user_id = $2
`, group.ID, group.UserID).Scan(&group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return domain.Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Group{}, err
	}
	return group, nil
}

func (r *PostgresRepository) DeleteGroup(ctx context.Context, userID, groupID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, userID); err != nil {
		return err
	}

	var id int64
	err = tx.QueryRow(ctx, `
SELECT id FROM chat_room_groups WHERE id = $1 AND user_id = $2 FOR UPDATE
`, groupID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	members, err := listUserRoomPlacementsForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE chat_room_members
SET group_id = NULL, updated_at = NOW()
WHERE user_id = $1 AND group_id = $2
`, userID, groupID); err != nil {
		return err
	}
	if err := setUserRoomPlacementOrders(ctx, tx, userID, releaseGroupRoomPlacements(members, groupID)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_room_groups WHERE id = $1 AND user_id = $2`, groupID, userID); err != nil {
		return err
	}
	if err := normalizeUserGroupSortOrders(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) MoveGroup(ctx context.Context, userID, groupID int64, direction int32) error {
	if direction != -1 && direction != 1 {
		return domain.ErrInvalidInput
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, userID); err != nil {
		return err
	}

	groups, err := listUserGroupsForUpdate(ctx, tx, userID)
	if err != nil {
		return err
	}
	_, err = moveGroupInOrder(groups, groupID, direction)
	if err != nil {
		return err
	}
	if err := setUserGroupSortOrders(ctx, tx, userID, groups); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func moveGroupInOrder(groups []domain.Group, groupID int64, direction int32) (bool, error) {
	if direction != -1 && direction != 1 {
		return false, domain.ErrInvalidInput
	}
	index := -1
	for candidate, group := range groups {
		if group.ID == groupID {
			index = candidate
			break
		}
	}
	if index < 0 {
		return false, domain.ErrNotFound
	}
	target := index + int(direction)
	if target < 0 || target >= len(groups) {
		return false, nil
	}
	groups[index], groups[target] = groups[target], groups[index]
	return true, nil
}

func lockUserGroupWrites(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, userID)
	return err
}

func normalizeUserGroupSortOrders(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
WITH ordered AS (
  SELECT id, ROW_NUMBER() OVER (ORDER BY sort_order, id) - 1 AS next_sort_order
  FROM chat_room_groups
  WHERE user_id = $1
)
UPDATE chat_room_groups AS groups
SET sort_order = ordered.next_sort_order,
    updated_at = NOW()
FROM ordered
WHERE groups.id = ordered.id
  AND groups.sort_order IS DISTINCT FROM ordered.next_sort_order
`, userID)
	return err
}

func listUserGroupsForUpdate(ctx context.Context, tx pgx.Tx, userID int64) ([]domain.Group, error) {
	rows, err := tx.Query(ctx, `
SELECT id, user_id, name, sort_order, created_at, updated_at
FROM chat_room_groups
WHERE user_id = $1
ORDER BY sort_order, id
FOR UPDATE
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]domain.Group, 0)
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.UserID, &group.Name, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func setUserGroupSortOrders(ctx context.Context, tx pgx.Tx, userID int64, groups []domain.Group) error {
	ids := make([]int64, len(groups))
	sortOrders := make([]int32, len(groups))
	for index, group := range groups {
		ids[index] = group.ID
		sortOrders[index] = int32(index)
	}
	_, err := tx.Exec(ctx, `
UPDATE chat_room_groups AS groups
SET sort_order = positions.sort_order,
    updated_at = NOW()
FROM UNNEST($2::BIGINT[], $3::INTEGER[]) AS positions(id, sort_order)
WHERE groups.user_id = $1
  AND groups.id = positions.id
  AND groups.sort_order IS DISTINCT FROM positions.sort_order
`, userID, ids, sortOrders)
	return err
}

func (r *PostgresRepository) PlaceRoom(ctx context.Context, roomNo string, userID int64, placement domain.Placement) (domain.Membership, error) {
	if placement.GroupID < 0 || placement.SortOrder < 0 {
		return domain.Membership{}, domain.ErrInvalidInput
	}
	roomNo = strings.TrimSpace(roomNo)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockUserGroupWrites(ctx, tx, userID); err != nil {
		return domain.Membership{}, err
	}

	if placement.GroupID > 0 {
		var owned int64
		err := tx.QueryRow(ctx, `
SELECT id FROM chat_room_groups WHERE id = $1 AND user_id = $2 FOR KEY SHARE
`, placement.GroupID, userID).Scan(&owned)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Membership{}, domain.ErrNotFound
		}
		if err != nil {
			return domain.Membership{}, err
		}
	}

	members, err := listUserRoomPlacementsForUpdate(ctx, tx, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	updates, err := placeRoomInOrder(members, roomNo, placement)
	if err != nil {
		return domain.Membership{}, err
	}
	if err := setUserRoomPlacementOrders(ctx, tx, userID, updates); err != nil {
		return domain.Membership{}, err
	}

	var member domain.Membership
	err = scanMembership(tx.QueryRow(ctx, `
SELECT `+membershipColumns+`
FROM chat_room_members
WHERE room_id = $1 AND user_id = $2 AND status = $3
`, roomIDForRoomNo(members, roomNo), userID, domain.MemberStatusJoined), &member)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotMember
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return member, nil
}

type roomPlacementMember struct {
	RoomNo     string
	Membership domain.Membership
}

// listUserRoomPlacementsForUpdate takes the same per-user lock used by group
// writes before locking the active memberships in a stable order. This keeps
// cross-device moves from producing duplicate or drifting sort positions.
func listUserRoomPlacementsForUpdate(ctx context.Context, tx pgx.Tx, userID int64) ([]roomPlacementMember, error) {
	rows, err := tx.Query(ctx, `
SELECT r.room_no,
       m.room_id, m.user_id, m.role, m.status, m.joined_at_seq, m.last_read_seq,
       m.last_seen_announcement_version, m.group_id, m.sort_order, m.joined_at,
       m.left_at, m.created_at, m.updated_at
FROM chat_room_members m
JOIN chat_rooms r ON r.id = m.room_id
WHERE m.user_id = $1 AND m.status = $2 AND r.status = $3
ORDER BY m.group_id NULLS FIRST, m.sort_order, m.room_id
FOR UPDATE OF m
`, userID, domain.MemberStatusJoined, domain.RoomStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]roomPlacementMember, 0)
	for rows.Next() {
		var member roomPlacementMember
		if err := scanRoomPlacementMember(rows, &member); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// placeRoomInOrder treats Placement.SortOrder as the requested zero-based
// position in the destination group. It renumbers both affected groups so the
// sidebar has a durable, gap-free order after any move.
func placeRoomInOrder(members []roomPlacementMember, roomNo string, placement domain.Placement) ([]domain.Membership, error) {
	if placement.GroupID < 0 || placement.SortOrder < 0 {
		return nil, domain.ErrInvalidInput
	}

	roomNo = strings.TrimSpace(roomNo)
	var target roomPlacementMember
	found := false
	for _, member := range members {
		if member.RoomNo == roomNo {
			target = member
			found = true
			break
		}
	}
	if !found {
		return nil, domain.ErrNotMember
	}

	sourceGroupID := target.Membership.GroupID
	source := roomPlacementMembersForGroup(members, sourceGroupID)
	source = removeRoomPlacementMember(source, target.Membership.RoomID)
	target.Membership.GroupID = placement.GroupID

	if sourceGroupID == placement.GroupID {
		ordered := insertRoomPlacementMember(source, target, placement.SortOrder)
		return renumberRoomPlacementMembers(ordered), nil
	}

	destination := roomPlacementMembersForGroup(members, placement.GroupID)
	destination = insertRoomPlacementMember(destination, target, placement.SortOrder)
	updates := renumberRoomPlacementMembers(source)
	updates = append(updates, renumberRoomPlacementMembers(destination)...)
	return updates, nil
}

func roomPlacementMembersForGroup(members []roomPlacementMember, groupID int64) []roomPlacementMember {
	result := make([]roomPlacementMember, 0)
	for _, member := range members {
		if member.Membership.GroupID == groupID {
			result = append(result, member)
		}
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Membership.SortOrder != result[right].Membership.SortOrder {
			return result[left].Membership.SortOrder < result[right].Membership.SortOrder
		}
		return result[left].Membership.RoomID < result[right].Membership.RoomID
	})
	return result
}

// releaseGroupRoomPlacements appends a deleted group's rooms after the
// existing ungrouped rooms while retaining both prior orders.
func releaseGroupRoomPlacements(members []roomPlacementMember, groupID int64) []domain.Membership {
	ungrouped := roomPlacementMembersForGroup(members, 0)
	released := roomPlacementMembersForGroup(members, groupID)
	for index := range released {
		released[index].Membership.GroupID = 0
	}
	return renumberRoomPlacementMembers(append(ungrouped, released...))
}

// appendRoomPlacement adds a newly active membership after its existing group
// peers and makes that group's sort positions continuous.
func appendRoomPlacement(members []roomPlacementMember, member *domain.Membership) []domain.Membership {
	group := roomPlacementMembersForGroup(members, member.GroupID)
	member.SortOrder = int32(len(group))
	group = append(group, roomPlacementMember{Membership: *member})
	return renumberRoomPlacementMembers(group)
}

func removeRoomPlacementMember(members []roomPlacementMember, roomID int64) []roomPlacementMember {
	result := make([]roomPlacementMember, 0, len(members))
	for _, member := range members {
		if member.Membership.RoomID != roomID {
			result = append(result, member)
		}
	}
	return result
}

func insertRoomPlacementMember(members []roomPlacementMember, target roomPlacementMember, position int32) []roomPlacementMember {
	index := int(position)
	if index > len(members) {
		index = len(members)
	}
	result := make([]roomPlacementMember, 0, len(members)+1)
	result = append(result, members[:index]...)
	result = append(result, target)
	result = append(result, members[index:]...)
	return result
}

func renumberRoomPlacementMembers(members []roomPlacementMember) []domain.Membership {
	updates := make([]domain.Membership, 0, len(members))
	for index, member := range members {
		member.Membership.SortOrder = int32(index)
		updates = append(updates, member.Membership)
	}
	return updates
}

func roomIDForRoomNo(members []roomPlacementMember, roomNo string) int64 {
	for _, member := range members {
		if member.RoomNo == roomNo {
			return member.Membership.RoomID
		}
	}
	return 0
}

func setUserRoomPlacementOrders(ctx context.Context, tx pgx.Tx, userID int64, members []domain.Membership) error {
	if len(members) == 0 {
		return nil
	}
	roomIDs := make([]int64, 0, len(members))
	groupIDs := make([]int64, 0, len(members))
	sortOrders := make([]int32, 0, len(members))
	for _, member := range members {
		roomIDs = append(roomIDs, member.RoomID)
		groupIDs = append(groupIDs, member.GroupID)
		sortOrders = append(sortOrders, member.SortOrder)
	}
	_, err := tx.Exec(ctx, `
UPDATE chat_room_members AS members
SET group_id = NULLIF(positions.group_id, 0),
    sort_order = positions.sort_order,
    updated_at = NOW()
FROM UNNEST($2::BIGINT[], $3::BIGINT[], $4::INTEGER[]) AS positions(room_id, group_id, sort_order)
WHERE members.user_id = $1
  AND members.room_id = positions.room_id
  AND members.status = $5
  AND (
    members.group_id IS DISTINCT FROM NULLIF(positions.group_id, 0)
    OR members.sort_order IS DISTINCT FROM positions.sort_order
  )
`, userID, roomIDs, groupIDs, sortOrders, domain.MemberStatusJoined)
	return err
}

func (r *PostgresRepository) UpdateAnnouncement(ctx context.Context, roomNo string, userID int64, announcement, eventID string) (domain.Room, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Room{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var room domain.Room
	err = scanRoom(tx.QueryRow(ctx, `
UPDATE chat_rooms
SET announcement = $3,
    announcement_version = announcement_version + 1,
    updated_at = NOW()
WHERE room_no = $1 AND creator_id = $2 AND status = $4
RETURNING `+roomColumns+`
`, roomNo, userID, announcement, domain.RoomStatusActive), &room)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Room{}, r.classifyOwnerFailure(ctx, roomNo, userID)
	}
	if err != nil {
		return domain.Room{}, err
	}
	if err := insertOutbox(ctx, tx, eventID, "chat.announcement.updated.v1", room.ID, userID, map[string]any{
		"roomId": room.ID, "roomNo": room.RoomNo, "creatorId": userID,
		"announcement": room.Announcement, "announcementVersion": room.AnnouncementVersion,
	}); err != nil {
		return domain.Room{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Room{}, err
	}
	return room, nil
}

func (r *PostgresRepository) MarkAnnouncementSeen(ctx context.Context, roomNo string, userID, version int64) (domain.Membership, error) {
	var member domain.Membership
	err := scanMembership(r.pool.QueryRow(ctx, `
UPDATE chat_room_members m
SET last_seen_announcement_version = GREATEST(
      m.last_seen_announcement_version,
      LEAST($3, r.announcement_version)
    ),
    updated_at = NOW()
FROM chat_rooms r
WHERE r.id = m.room_id AND r.room_no = $1
  AND m.user_id = $2 AND m.status = $4
RETURNING m.room_id, m.user_id, m.role, m.status, m.joined_at_seq, m.last_read_seq,
          m.last_seen_announcement_version, m.group_id, m.sort_order, m.joined_at,
          m.left_at, m.created_at, m.updated_at
`, roomNo, userID, version, domain.MemberStatusJoined), &member)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotMember
	}
	return member, err
}

func (r *PostgresRepository) ValidateMemberships(ctx context.Context, userID int64, roomNumbers []string) ([]string, error) {
	subscriptions, err := r.ValidateMembershipsDetailed(ctx, userID, roomNumbers)
	if err != nil {
		return nil, err
	}
	valid := make([]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		valid = append(valid, subscription.RoomNo)
	}
	return valid, nil
}

func (r *PostgresRepository) ValidateMembershipsDetailed(ctx context.Context, userID int64, roomNumbers []string) ([]domain.RoomSubscription, error) {
	if len(roomNumbers) == 0 {
		return []domain.RoomSubscription{}, nil
	}
	rows, err := r.pool.Query(ctx, `
SELECT r.id, r.room_no
FROM chat_rooms r
JOIN chat_room_members m ON m.room_id = r.id
WHERE m.user_id = $1 AND m.status = $2 AND r.status = $3
  AND r.room_no = ANY($4)
`, userID, domain.MemberStatusJoined, domain.RoomStatusActive, roomNumbers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	validSet := make(map[string]int64, len(roomNumbers))
	for rows.Next() {
		var roomID int64
		var roomNo string
		if err := rows.Scan(&roomID, &roomNo); err != nil {
			return nil, err
		}
		validSet[roomNo] = roomID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	valid := make([]domain.RoomSubscription, 0, len(validSet))
	for _, roomNo := range roomNumbers {
		if roomID, exists := validSet[roomNo]; exists {
			valid = append(valid, domain.RoomSubscription{RoomID: roomID, RoomNo: roomNo})
		}
	}
	return valid, nil
}

func (r *PostgresRepository) ClaimPendingOutboxEvents(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		return []domain.OutboxEvent{}, nil
	}
	leaseMilliseconds := leaseDuration.Milliseconds()
	if leaseMilliseconds <= 0 {
		leaseMilliseconds = 1
	}
	rows, err := r.pool.Query(ctx, `
WITH candidates AS (
  SELECT candidate.event_id
  FROM chat_outbox candidate
  WHERE (
      (candidate.status IN ('pending', 'failed') AND candidate.next_attempt_at <= NOW())
      OR (candidate.status = 'publishing' AND candidate.lease_expires_at <= NOW())
    )
    AND NOT EXISTS (
      SELECT 1
      FROM chat_outbox predecessor
      WHERE predecessor.partition_key = candidate.partition_key
        AND predecessor.dispatch_id < candidate.dispatch_id
        AND predecessor.status <> 'published'
    )
  ORDER BY candidate.dispatch_id
  LIMIT $1
  FOR UPDATE SKIP LOCKED
)
UPDATE chat_outbox event
SET status = 'publishing',
    attempts = event.attempts + 1,
    lease_owner = $2,
    lease_expires_at = NOW() + ($3 * INTERVAL '1 millisecond'),
    last_error = '',
    updated_at = NOW()
FROM candidates
WHERE event.event_id = candidates.event_id
RETURNING event.dispatch_id, event.event_id::text, event.aggregate_type,
          event.aggregate_id, event.event_type, event.event_version,
          event.partition_key, event.payload::text, event.attempts,
          event.created_at
`, limit, owner, leaseMilliseconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.OutboxEvent, 0, limit)
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(
			&event.DispatchID,
			&event.EventID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&event.EventVersion,
			&event.PartitionKey,
			&event.Payload,
			&event.Attempt,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *PostgresRepository) MarkOutboxEventPublished(ctx context.Context, eventID, owner string) error {
	result, err := r.pool.Exec(ctx, `
UPDATE chat_outbox
SET status = 'published',
    published_at = NOW(),
    lease_owner = '',
    lease_expires_at = NULL,
    last_error = '',
    updated_at = NOW()
WHERE event_id = $1::uuid AND status = 'publishing' AND lease_owner = $2
`, eventID, owner)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrOutboxLeaseLost
	}
	return nil
}

func (r *PostgresRepository) MarkOutboxEventFailed(ctx context.Context, eventID, owner, reason string, retryAt time.Time) error {
	result, err := r.pool.Exec(ctx, `
UPDATE chat_outbox
SET status = 'failed',
    next_attempt_at = $3,
    lease_owner = '',
    lease_expires_at = NULL,
    last_error = $4,
    updated_at = NOW()
WHERE event_id = $1::uuid AND status = 'publishing' AND lease_owner = $2
`, eventID, owner, retryAt, reason)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrOutboxLeaseLost
	}
	return nil
}

func (r *PostgresRepository) listGroups(ctx context.Context, userID int64) ([]domain.Group, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, user_id, name, sort_order, created_at, updated_at
FROM chat_room_groups
WHERE user_id = $1
ORDER BY sort_order, id
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]domain.Group, 0)
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.UserID, &group.Name, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *PostgresRepository) loadDirectionalMessages(ctx context.Context, roomID int64, predicate string, cursor int64, limit int32, descending bool) ([]domain.Message, bool, error) {
	order := "ASC"
	if descending {
		order = "DESC"
	}
	rows, err := r.pool.Query(ctx, `
SELECT `+messageColumns+`
FROM chat_messages
WHERE room_id = $1 AND `+predicate+`
ORDER BY seq `+order+`
LIMIT $3
`, roomID, cursor, limit+1)
	if err != nil {
		return nil, false, err
	}
	messages, err := scanMessages(rows)
	if err != nil {
		return nil, false, err
	}
	more := len(messages) > int(limit)
	if more {
		messages = messages[:limit]
	}
	if descending {
		reverseMessages(messages)
	}
	return messages, more, nil
}

func (r *PostgresRepository) loadAnchorMessages(ctx context.Context, roomID, anchor int64, limit int32, older bool) ([]domain.Message, bool, error) {
	predicate := "seq > $2"
	order := "ASC"
	if older {
		predicate = "seq <= $2"
		order = "DESC"
	}
	rows, err := r.pool.Query(ctx, `
SELECT `+messageColumns+`
FROM chat_messages
WHERE room_id = $1 AND `+predicate+`
ORDER BY seq `+order+`
LIMIT $3
`, roomID, anchor, limit+1)
	if err != nil {
		return nil, false, err
	}
	messages, err := scanMessages(rows)
	if err != nil {
		return nil, false, err
	}
	more := len(messages) > int(limit)
	if more {
		messages = messages[:limit]
	}
	if older {
		reverseMessages(messages)
	}
	return messages, more, nil
}

func (r *PostgresRepository) classifyOwnerFailure(ctx context.Context, roomNo string, userID int64) error {
	var creatorID int64
	var status int16
	err := r.pool.QueryRow(ctx, `SELECT creator_id, status FROM chat_rooms WHERE room_no = $1`, roomNo).Scan(&creatorID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != domain.RoomStatusActive {
		return domain.ErrRoomClosed
	}
	if creatorID != userID {
		return domain.ErrNotOwner
	}
	return domain.ErrNotFound
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// lockRoomThenMemberForUpdate is shared by joins and sends so concurrent
// commands always acquire the room lock before the membership lock.
func lockRoomThenMemberForUpdate(ctx context.Context, query rowQuerier, roomNo string, userID int64) (domain.Room, domain.Membership, bool, error) {
	var room domain.Room
	err := scanRoom(query.QueryRow(ctx, `
SELECT `+roomColumns+`
FROM chat_rooms
WHERE room_no = $1
FOR UPDATE
`, roomNo), &room)
	if err != nil {
		return domain.Room{}, domain.Membership{}, false, err
	}

	var member domain.Membership
	err = scanMembership(query.QueryRow(ctx, `
SELECT `+membershipColumns+`
FROM chat_room_members
WHERE room_id = $1 AND user_id = $2
FOR UPDATE
`, room.ID, userID), &member)
	if errors.Is(err, pgx.ErrNoRows) {
		return room, domain.Membership{}, false, nil
	}
	if err != nil {
		return domain.Room{}, domain.Membership{}, false, err
	}
	return room, member, true, nil
}

func insertOutbox(ctx context.Context, tx pgx.Tx, eventID, eventType string, roomID, actorID int64, payload map[string]any) error {
	envelope := map[string]any{
		"eventId": eventID, "eventType": eventType, "occurredAt": time.Now().UnixMilli(),
		"producer": "chat-service", "actorId": actorID, "version": 1, "payload": payload,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO chat_outbox(
  event_id, aggregate_type, aggregate_id, event_type, event_version,
  partition_key, payload
)
VALUES($1::uuid, 'room', $2, $3, 1, $2, $4::jsonb)
`, eventID, strconv.FormatInt(roomID, 10), eventType, encoded)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRoom(row scanner, room *domain.Room) error {
	return row.Scan(
		&room.ID, &room.RoomNo, &room.Name, &room.CreatorID, &room.Announcement,
		&room.AnnouncementVersion, &room.LastMessageSeq, &room.Status,
		&room.CreatedAt, &room.UpdatedAt,
	)
}

func scanMembership(row scanner, member *domain.Membership) error {
	var groupID sql.NullInt64
	var leftAt sql.NullTime
	err := row.Scan(
		&member.RoomID, &member.UserID, &member.Role, &member.Status,
		&member.JoinedAtSeq, &member.LastReadSeq, &member.LastSeenAnnouncementVersion,
		&groupID, &member.SortOrder, &member.JoinedAt, &leftAt, &member.CreatedAt, &member.UpdatedAt,
	)
	if err == nil {
		applyNullableMembership(member, groupID, leftAt)
	}
	return err
}

func scanRoomPlacementMember(row scanner, member *roomPlacementMember) error {
	var groupID sql.NullInt64
	var leftAt sql.NullTime
	err := row.Scan(
		&member.RoomNo,
		&member.Membership.RoomID, &member.Membership.UserID, &member.Membership.Role, &member.Membership.Status,
		&member.Membership.JoinedAtSeq, &member.Membership.LastReadSeq, &member.Membership.LastSeenAnnouncementVersion,
		&groupID, &member.Membership.SortOrder, &member.Membership.JoinedAt, &leftAt,
		&member.Membership.CreatedAt, &member.Membership.UpdatedAt,
	)
	if err == nil {
		applyNullableMembership(&member.Membership, groupID, leftAt)
	}
	return err
}

func applyNullableMembership(member *domain.Membership, groupID sql.NullInt64, leftAt sql.NullTime) {
	if groupID.Valid {
		member.GroupID = groupID.Int64
	} else {
		member.GroupID = 0
	}
	if leftAt.Valid {
		member.LeftAt = &leftAt.Time
	} else {
		member.LeftAt = nil
	}
}

func scanMessage(row scanner, message *domain.Message) error {
	var deletedAt sql.NullTime
	err := row.Scan(
		&message.ID, &message.RoomID, &message.Seq, &message.SenderID,
		&message.ClientMessageID, &message.Body, &message.Status,
		&message.CreatedAt, &message.UpdatedAt, &deletedAt,
	)
	if deletedAt.Valid {
		message.DeletedAt = &deletedAt.Time
	}
	return err
}

func scanMessages(rows pgx.Rows) ([]domain.Message, error) {
	defer rows.Close()
	messages := make([]domain.Message, 0)
	for rows.Next() {
		var message domain.Message
		if err := scanMessage(rows, &message); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func reverseMessages(messages []domain.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func mapPostgresError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		constraint := strings.ToLower(pgErr.ConstraintName)
		switch {
		case strings.Contains(constraint, "room_no"):
			return domain.ErrRoomNumberConflict
		case strings.Contains(constraint, "groups_user_name"):
			return domain.ErrGroupNameConflict
		}
	}
	return err
}

var _ domain.Repository = (*PostgresRepository)(nil)
