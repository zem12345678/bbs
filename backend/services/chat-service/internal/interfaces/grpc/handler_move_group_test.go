package grpc

import (
	"context"
	"testing"

	"chat-service/api/proto/chatpb"
	chatapp "chat-service/internal/application/chat"
	domain "chat-service/internal/domain/chat"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type moveGroupRepository struct {
	userID           int64
	groupID          int64
	direction        int32
	deletedRoomNo    string
	deletedUserID    int64
	deletedMessageID int64
	deleteErr        error
}

func (r *moveGroupRepository) CreateRoom(context.Context, domain.Room, domain.Membership) (domain.RoomDetails, error) {
	return domain.RoomDetails{}, nil
}
func (r *moveGroupRepository) LookupRoom(context.Context, string, int64) (domain.RoomDetails, error) {
	return domain.RoomDetails{}, nil
}
func (r *moveGroupRepository) JoinRoom(context.Context, string, int64, string) (domain.RoomDetails, error) {
	return domain.RoomDetails{}, nil
}
func (r *moveGroupRepository) ListSidebar(context.Context, int64) (domain.Sidebar, error) {
	return domain.Sidebar{}, nil
}
func (r *moveGroupRepository) ListMessages(context.Context, string, int64, domain.MessageQuery) (domain.MessagePage, error) {
	return domain.MessagePage{}, nil
}
func (r *moveGroupRepository) SendMessage(context.Context, string, int64, domain.Message, string) (domain.Message, int64, error) {
	return domain.Message{}, 0, nil
}
func (r *moveGroupRepository) DeleteMessage(_ context.Context, roomNo string, userID, messageID int64, _ string) (domain.Message, error) {
	r.deletedRoomNo, r.deletedUserID, r.deletedMessageID = roomNo, userID, messageID
	if r.deleteErr != nil {
		return domain.Message{}, r.deleteErr
	}
	return domain.Message{ID: messageID, SenderID: userID, Status: domain.MessageStatusDeleted}, nil
}
func (r *moveGroupRepository) AdvanceRead(context.Context, string, int64, int64, string) (domain.Membership, int64, error) {
	return domain.Membership{}, 0, nil
}
func (r *moveGroupRepository) CreateGroup(context.Context, domain.Group) (domain.Group, error) {
	return domain.Group{}, nil
}
func (r *moveGroupRepository) UpdateGroup(context.Context, domain.Group) (domain.Group, error) {
	return domain.Group{}, nil
}
func (r *moveGroupRepository) DeleteGroup(context.Context, int64, int64) error { return nil }
func (r *moveGroupRepository) MoveGroup(_ context.Context, userID, groupID int64, direction int32) error {
	r.userID, r.groupID, r.direction = userID, groupID, direction
	return nil
}
func (r *moveGroupRepository) PlaceRoom(context.Context, string, int64, domain.Placement) (domain.Membership, error) {
	return domain.Membership{}, nil
}
func (r *moveGroupRepository) UpdateAnnouncement(context.Context, string, int64, string, string) (domain.Room, error) {
	return domain.Room{}, nil
}
func (r *moveGroupRepository) MarkAnnouncementSeen(context.Context, string, int64, int64) (domain.Membership, error) {
	return domain.Membership{}, nil
}
func (r *moveGroupRepository) ValidateMemberships(context.Context, int64, []string) ([]string, error) {
	return nil, nil
}

func TestMoveGroupHandler(t *testing.T) {
	repo := &moveGroupRepository{}
	handler := NewHandler(chatapp.NewService(repo, nil))

	response, err := handler.MoveGroup(context.Background(), &chatpb.MoveGroupRequest{UserId: 42, GroupId: 9, Direction: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !response.GetSuccess() {
		t.Fatal("move response is not successful")
	}
	if repo.userID != 42 || repo.groupID != 9 || repo.direction != 1 {
		t.Fatalf("move arguments = user %d, group %d, direction %d", repo.userID, repo.groupID, repo.direction)
	}

	_, err = handler.MoveGroup(context.Background(), &chatpb.MoveGroupRequest{UserId: 42, GroupId: 9, Direction: 0})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid direction gRPC code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestDeleteMessageHandler(t *testing.T) {
	repo := &moveGroupRepository{}
	handler := NewHandler(chatapp.NewService(repo, nil))

	response, err := handler.DeleteMessage(context.Background(), &chatpb.DeleteMessageRequest{RoomNo: "AB12CD3E", UserId: 42, MessageId: 9})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetMessage().GetId() != 9 || response.GetMessage().GetStatus() != int32(domain.MessageStatusDeleted) {
		t.Fatalf("delete response = %#v", response.GetMessage())
	}
	if repo.deletedRoomNo != "AB12CD3E" || repo.deletedUserID != 42 || repo.deletedMessageID != 9 {
		t.Fatalf("delete arguments = room %q, user %d, message %d", repo.deletedRoomNo, repo.deletedUserID, repo.deletedMessageID)
	}

	repo.deleteErr = domain.ErrNotMessageAuthor
	_, err = handler.DeleteMessage(context.Background(), &chatpb.DeleteMessageRequest{RoomNo: "AB12CD3E", UserId: 7, MessageId: 9})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("non-author gRPC code = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
}
