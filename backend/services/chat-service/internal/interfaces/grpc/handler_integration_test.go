//go:build integration

package grpc_test

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"chat-service/api/proto/chatpb"
	chatapp "chat-service/internal/application/chat"
	"chat-service/internal/infrastructure/persistence"
	interfacesgrpc "chat-service/internal/interfaces/grpc"
	platformpostgres "chat-service/internal/platform/postgres"
	"chat-service/pkg/snowflake"

	"github.com/google/uuid"
	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestChatPostgresGRPCIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_CHAT_TEST_DSN to run the PostgreSQL gRPC integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := platformpostgres.Open(ctx, dsn, 4)
	if err != nil {
		t.Fatalf("open test PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ids, err := snowflake.New(31)
	if err != nil {
		t.Fatalf("create test ID generator: %v", err)
	}
	server := stdgrpc.NewServer()
	interfacesgrpc.Register(server, interfacesgrpc.NewHandler(chatapp.NewService(persistence.NewPostgresRepository(pool), ids)))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for test gRPC server: %v", err)
	}
	defer listener.Close()
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	connection, err := stdgrpc.DialContext(ctx, listener.Addr().String(), stdgrpc.WithBlock(), stdgrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial test gRPC server: %v", err)
	}
	defer connection.Close()
	client := chatpb.NewChatServiceClient(connection)

	seed := time.Now().UnixMilli() * 10
	ownerID := seed + 1
	memberID := seed + 2
	visitorID := seed + 3
	var roomID, groupID int64
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if roomID != 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_outbox WHERE aggregate_id = $1`, strconv.FormatInt(roomID, 10))
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_messages WHERE room_id = $1`, roomID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_room_members WHERE room_id = $1`, roomID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_rooms WHERE id = $1`, roomID)
		}
		if groupID != 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM chat_room_groups WHERE id = $1 AND user_id = $2`, groupID, memberID)
		}
	})

	created, err := client.CreateRoom(ctx, &chatpb.CreateRoomRequest{CreatorId: ownerID, Name: "integration room"})
	requireNoError(t, err)
	room := created.GetDetails().GetRoom()
	roomID = room.GetId()
	roomNo := room.GetRoomNo()
	requireEqual(t, created.GetDetails().GetMemberCount(), int64(1), "owner member count")

	joined, err := client.JoinRoom(ctx, &chatpb.JoinRoomRequest{RoomNo: roomNo, UserId: memberID})
	requireNoError(t, err)
	requireEqual(t, joined.GetDetails().GetMembership().GetLastReadSeq(), int64(0), "new member read cursor")
	joinedAgain, err := client.JoinRoom(ctx, &chatpb.JoinRoomRequest{RoomNo: roomNo, UserId: memberID})
	requireNoError(t, err)
	requireEqual(t, joinedAgain.GetDetails().GetMemberCount(), int64(2), "idempotent join member count")

	group, err := client.CreateGroup(ctx, &chatpb.CreateGroupRequest{UserId: memberID, Name: "integration", SortOrder: 3})
	requireNoError(t, err)
	groupID = group.GetGroup().GetId()
	placed, err := client.PlaceRoom(ctx, &chatpb.PlaceRoomRequest{RoomNo: roomNo, UserId: memberID, GroupId: groupID, SortOrder: 7})
	requireNoError(t, err)
	requireEqual(t, placed.GetMembership().GetGroupId(), groupID, "group placement")
	requireEqual(t, placed.GetMembership().GetSortOrder(), int32(7), "room sort order")

	announcement, err := client.UpdateAnnouncement(ctx, &chatpb.UpdateAnnouncementRequest{RoomNo: roomNo, UserId: ownerID, Announcement: "integration announcement"})
	requireNoError(t, err)
	requireEqual(t, announcement.GetRoom().GetAnnouncementVersion(), int64(1), "announcement version")
	preview, err := client.LookupRoom(ctx, &chatpb.LookupRoomRequest{RoomNo: roomNo, UserId: visitorID})
	requireNoError(t, err)
	if preview.GetDetails().GetMembership() != nil {
		t.Fatal("non-member preview unexpectedly includes membership")
	}
	requireEqual(t, preview.GetDetails().GetRoom().GetAnnouncement(), "", "non-member announcement visibility")
	seen, err := client.MarkAnnouncementSeen(ctx, &chatpb.MarkAnnouncementSeenRequest{RoomNo: roomNo, UserId: memberID, AnnouncementVersion: 1})
	requireNoError(t, err)
	requireEqual(t, seen.GetMembership().GetLastSeenAnnouncementVersion(), int64(1), "announcement seen version")

	first, err := client.SendMessage(ctx, &chatpb.SendMessageRequest{RoomNo: roomNo, UserId: ownerID, ClientMessageId: uuid.NewString(), Body: "first"})
	requireNoError(t, err)
	requireEqual(t, first.GetMessage().GetSeq(), int64(1), "first message sequence")
	sidebarBeforeReply, err := client.ListSidebar(ctx, &chatpb.ListSidebarRequest{UserId: memberID})
	requireNoError(t, err)
	roomBeforeReply := findSidebarRoom(sidebarBeforeReply.GetRooms(), roomNo)
	if roomBeforeReply == nil {
		t.Fatalf("room %s missing from sidebar", roomNo)
	}
	requireEqual(t, roomBeforeReply.GetUnreadCount(), int64(1), "incoming unread count")
	partialRead, err := client.AdvanceRead(ctx, &chatpb.AdvanceReadRequest{RoomNo: roomNo, UserId: memberID, ReadSeq: 1})
	requireNoError(t, err)
	requireEqual(t, partialRead.GetMembership().GetLastReadSeq(), int64(1), "partial read cursor")
	requireEqual(t, partialRead.GetUnreadCount(), int64(0), "partial unread count")

	clientMessageID := uuid.NewString()
	second, err := client.SendMessage(ctx, &chatpb.SendMessageRequest{RoomNo: roomNo, UserId: memberID, ClientMessageId: clientMessageID, Body: "second"})
	requireNoError(t, err)
	requireEqual(t, second.GetMessage().GetSeq(), int64(2), "second message sequence")
	retry, err := client.SendMessage(ctx, &chatpb.SendMessageRequest{RoomNo: roomNo, UserId: memberID, ClientMessageId: clientMessageID, Body: "must not duplicate"})
	requireNoError(t, err)
	requireEqual(t, retry.GetMessage().GetId(), second.GetMessage().GetId(), "idempotent message ID")
	requireEqual(t, retry.GetMessage().GetSeq(), int64(2), "idempotent message sequence")

	sidebar, err := client.ListSidebar(ctx, &chatpb.ListSidebarRequest{UserId: memberID})
	requireNoError(t, err)
	sidebarRoom := findSidebarRoom(sidebar.GetRooms(), roomNo)
	if sidebarRoom == nil {
		t.Fatalf("room %s missing from sidebar", roomNo)
	}
	requireEqual(t, sidebarRoom.GetUnreadCount(), int64(0), "self-read unread count")
	requireEqual(t, sidebarRoom.GetLastMessage().GetSeq(), int64(2), "sidebar latest message")

	messages, err := client.ListMessages(ctx, &chatpb.ListMessagesRequest{RoomNo: roomNo, UserId: memberID, AnchorSeq: 0, Before: 2, After: 5})
	requireNoError(t, err)
	requireEqual(t, len(messages.GetMessages()), 2, "anchor message count")
	requireEqual(t, messages.GetMessages()[0].GetSeq(), int64(1), "first anchored message")
	requireEqual(t, messages.GetMessages()[1].GetSeq(), int64(2), "second anchored message")

	regressedRead, err := client.AdvanceRead(ctx, &chatpb.AdvanceReadRequest{RoomNo: roomNo, UserId: memberID, ReadSeq: 0})
	requireNoError(t, err)
	requireEqual(t, regressedRead.GetMembership().GetLastReadSeq(), int64(2), "monotonic read cursor")
	fullRead, err := client.AdvanceRead(ctx, &chatpb.AdvanceReadRequest{RoomNo: roomNo, UserId: memberID, ReadSeq: 99})
	requireNoError(t, err)
	requireEqual(t, fullRead.GetMembership().GetLastReadSeq(), int64(2), "capped read cursor")
	requireEqual(t, fullRead.GetUnreadCount(), int64(0), "full read unread count")

	validated, err := client.ValidateRoomSubscriptions(ctx, &chatpb.ValidateRoomSubscriptionsRequest{UserId: memberID, RoomNumbers: []string{roomNo, roomNo, "ZZZZZZZZ"}})
	requireNoError(t, err)
	requireEqual(t, len(validated.GetRoomNumbers()), 1, "validated room count")
	requireEqual(t, validated.GetRoomNumbers()[0], roomNo, "validated room number")
}

func findSidebarRoom(rooms []*chatpb.SidebarRoom, roomNo string) *chatpb.SidebarRoom {
	for _, room := range rooms {
		if room.GetRoom().GetRoomNo() == roomNo {
			return room
		}
	}
	return nil
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func requireEqual[T comparable](t *testing.T, actual, expected T, label string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("%s = %v, want %v", label, actual, expected)
	}
}
