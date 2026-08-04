package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "chat-service/internal/domain/chat"
	"chat-service/internal/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountErasurePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_CHAT_TEST_DSN to run the PostgreSQL account-erasure integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrations.Run(ctx, pool); err != nil {
		t.Fatalf("run chat migrations: %v", err)
	}

	base := time.Now().UnixMilli() * 1000
	userID, successorID := base+1, base+2
	transferRoomID, closedRoomID, groupID := base+10, base+11, base+20
	messageID := base + 30
	transferRoomNo, closedRoomNo := integrationRoomNo(), integrationRoomNo()
	eventID := uuid.NewString()
	clientMessageID := uuid.NewString()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_outbox WHERE event_id = $1`, eventID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_messages WHERE room_id = ANY($1)`, []int64{transferRoomID, closedRoomID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_room_members WHERE room_id = ANY($1)`, []int64{transferRoomID, closedRoomID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_rooms WHERE id = ANY($1)`, []int64{transferRoomID, closedRoomID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_room_groups WHERE id = $1`, groupID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_erased_users WHERE user_id = $1`, userID)
	})

	if _, err := pool.Exec(ctx, `
INSERT INTO chat_rooms(id, room_no, name, creator_id, announcement, announcement_version, last_message_seq, status)
VALUES ($1, $2, 'transfer room', $3, 'private announcement', 4, 1, $4),
       ($5, $6, 'closed room', $3, 'private announcement', 7, 0, $4)
`, transferRoomID, transferRoomNo, userID, domain.RoomStatusActive, closedRoomID, closedRoomNo); err != nil {
		t.Fatalf("seed rooms: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO chat_room_groups(id, user_id, name, sort_order) VALUES($1, $2, 'erased group', 0)`, groupID, userID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO chat_room_members(room_id, user_id, role, status, group_id, joined_at)
VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '2 minutes'),
       ($1, $6, $7, $4, NULL, NOW() - INTERVAL '1 minute'),
       ($8, $2, $3, $4, NULL, NOW() - INTERVAL '2 minutes')
`, transferRoomID, userID, domain.MemberRoleOwner, domain.MemberStatusJoined, groupID,
		successorID, domain.MemberRoleMember, closedRoomID); err != nil {
		t.Fatalf("seed memberships: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO chat_messages(id, room_id, seq, sender_id, client_message_id, body, status)
VALUES($1, $2, 1, $3, $4, 'private message', $5)
`, messageID, transferRoomID, userID, clientMessageID, domain.MessageStatusPublished); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO chat_outbox(event_id, aggregate_type, aggregate_id, event_type, partition_key, payload)
VALUES($1::UUID, 'chat.room', $2::BIGINT::TEXT, 'chat.message.created.v1', $2::BIGINT::TEXT,
       jsonb_build_object('eventId', $1::TEXT, 'eventType', 'chat.message.created.v1',
                          'occurredAt', 1, 'producer', 'chat-service', 'actorId', $3::BIGINT,
                          'version', 1, 'payload', jsonb_build_object('body', 'private message')))
`, eventID, transferRoomID, userID); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}

	repository := NewPostgresRepository(pool)
	result, err := repository.EraseUserData(ctx, userID, base+100, 1)
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	want := domain.AccountErasureResult{
		RedactedMessages: 1, DeletedMemberships: 2, DeletedGroups: 1,
		TransferredRooms: 1, ClosedRooms: 1, SuppressedOutboxEvents: 1,
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("EraseUserData() = %+v, want %+v", result, want)
	}

	var creatorID int64
	var announcement string
	var announcementVersion int64
	var roomStatus int16
	if err := pool.QueryRow(ctx, `SELECT creator_id, announcement, announcement_version, status FROM chat_rooms WHERE id = $1`, transferRoomID).
		Scan(&creatorID, &announcement, &announcementVersion, &roomStatus); err != nil {
		t.Fatalf("load transferred room: %v", err)
	}
	if creatorID != successorID || announcement != "" || announcementVersion != 5 || roomStatus != domain.RoomStatusActive {
		t.Fatalf("transferred room = creator %d announcement %q version %d status %d", creatorID, announcement, announcementVersion, roomStatus)
	}
	var successorRole int16
	if err := pool.QueryRow(ctx, `SELECT role FROM chat_room_members WHERE room_id = $1 AND user_id = $2`, transferRoomID, successorID).Scan(&successorRole); err != nil {
		t.Fatalf("load successor membership: %v", err)
	}
	if successorRole != domain.MemberRoleOwner {
		t.Fatalf("successor role = %d, want owner", successorRole)
	}
	if err := pool.QueryRow(ctx, `SELECT creator_id, announcement, announcement_version, status FROM chat_rooms WHERE id = $1`, closedRoomID).
		Scan(&creatorID, &announcement, &announcementVersion, &roomStatus); err != nil {
		t.Fatalf("load closed room: %v", err)
	}
	if creatorID != 0 || announcement != "" || announcementVersion != 8 || roomStatus != domain.RoomStatusClosed {
		t.Fatalf("closed room = creator %d announcement %q version %d status %d", creatorID, announcement, announcementVersion, roomStatus)
	}

	var senderID int64
	var body string
	var messageStatus int16
	var deletedAt sql.NullTime
	if err := pool.QueryRow(ctx, `SELECT sender_id, body, status, deleted_at FROM chat_messages WHERE id = $1`, messageID).
		Scan(&senderID, &body, &messageStatus, &deletedAt); err != nil {
		t.Fatalf("load redacted message: %v", err)
	}
	if senderID != 0 || body != "" || messageStatus != domain.MessageStatusDeleted || !deletedAt.Valid {
		t.Fatalf("redacted message = sender %d body %q status %d deleted %t", senderID, body, messageStatus, deletedAt.Valid)
	}
	var actorID, erased, outboxStatus string
	if err := pool.QueryRow(ctx, `SELECT payload->>'actorId', payload->'payload'->>'erased', status FROM chat_outbox WHERE event_id = $1`, eventID).
		Scan(&actorID, &erased, &outboxStatus); err != nil {
		t.Fatalf("load suppressed outbox event: %v", err)
	}
	if actorID != "0" || erased != "true" || outboxStatus != "published" {
		t.Fatalf("suppressed outbox = actor %q erased %q status %q", actorID, erased, outboxStatus)
	}

	var erasedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT erased_at FROM chat_erased_users WHERE user_id = $1`, userID).Scan(&erasedAt); err != nil {
		t.Fatalf("load erasure receipt: %v", err)
	}
	replayed, err := repository.EraseUserData(ctx, userID, base+100, 1)
	if err != nil || !reflect.DeepEqual(replayed, want) {
		t.Fatalf("replayed EraseUserData() = %+v, %v", replayed, err)
	}
	upgraded, err := repository.EraseUserData(ctx, userID, base+101, 2)
	if err != nil || !reflect.DeepEqual(upgraded, want) {
		t.Fatalf("upgraded EraseUserData() = %+v, %v", upgraded, err)
	}
	var upgradedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT erased_at FROM chat_erased_users WHERE user_id = $1`, userID).Scan(&upgradedAt); err != nil {
		t.Fatalf("load upgraded receipt: %v", err)
	}
	if !upgradedAt.Equal(erasedAt) {
		t.Fatalf("erased_at changed from %s to %s", erasedAt, upgradedAt)
	}

	_, err = repository.CreateGroup(ctx, domain.Group{ID: base + 200, UserID: userID, Name: "late group"})
	if !errors.Is(err, domain.ErrUserErased) {
		t.Fatalf("CreateGroup() after erasure error = %v, want ErrUserErased", err)
	}
	_, _, err = repository.SendMessage(ctx, transferRoomNo, userID, domain.Message{
		ID: base + 201, ClientMessageID: uuid.NewString(), Body: "late message", Status: domain.MessageStatusPublished,
	}, uuid.NewString())
	if !errors.Is(err, domain.ErrUserErased) {
		t.Fatalf("SendMessage() after erasure error = %v, want ErrUserErased", err)
	}
}

func integrationRoomNo() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12]
}
