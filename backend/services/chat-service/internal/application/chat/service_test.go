package chat

import (
	"context"
	"errors"
	"testing"

	domain "chat-service/internal/domain/chat"
)

type fixedIDs struct {
	next int64
}

func (g *fixedIDs) Next() (int64, error) {
	g.next++
	return g.next, nil
}

type recordingRepository struct {
	createRoom       func(domain.Room, domain.Membership) (domain.RoomDetails, error)
	lookupRoom       func(string, int64) (domain.RoomDetails, error)
	validateRooms    func(int64, []string) ([]string, error)
	sentMessage      domain.Message
	deletedRoomNo    string
	deletedUserID    int64
	deletedMessageID int64
	deleteEventID    string
	deleteMessage    domain.Message
	deleteMessageErr error
	roomsValidated   []string
	messageQuery     domain.MessageQuery
	movedUserID      int64
	movedGroupID     int64
	movedDirection   int32
	moveGroupErr     error
	placedRoomNo     string
	placedUserID     int64
	placedPlacement  domain.Placement
	placeRoomErr     error
	leftRoomNo       string
	leftUserID       int64
	leaveEventID     string
	leftMembership   domain.Membership
	leaveRoomErr     error
	createRoomCalls  int
}

func (r *recordingRepository) CreateRoom(_ context.Context, room domain.Room, owner domain.Membership) (domain.RoomDetails, error) {
	r.createRoomCalls++
	if r.createRoom == nil {
		return domain.RoomDetails{}, nil
	}
	return r.createRoom(room, owner)
}

func (r *recordingRepository) LookupRoom(_ context.Context, roomNo string, userID int64) (domain.RoomDetails, error) {
	if r.lookupRoom == nil {
		return domain.RoomDetails{}, nil
	}
	return r.lookupRoom(roomNo, userID)
}

func (r *recordingRepository) JoinRoom(context.Context, string, int64, string) (domain.RoomDetails, error) {
	return domain.RoomDetails{}, nil
}

func (r *recordingRepository) LeaveRoom(_ context.Context, roomNo string, userID int64, eventID string) (domain.Membership, error) {
	r.leftRoomNo = roomNo
	r.leftUserID = userID
	r.leaveEventID = eventID
	return r.leftMembership, r.leaveRoomErr
}

func (r *recordingRepository) ListSidebar(context.Context, int64) (domain.Sidebar, error) {
	return domain.Sidebar{}, nil
}

func (r *recordingRepository) ListMessages(_ context.Context, _ string, _ int64, query domain.MessageQuery) (domain.MessagePage, error) {
	r.messageQuery = query
	return domain.MessagePage{}, nil
}

func (r *recordingRepository) SendMessage(_ context.Context, _ string, _ int64, message domain.Message, _ string) (domain.Message, int64, error) {
	r.sentMessage = message
	message.Seq = 1
	return message, message.Seq, nil
}

func (r *recordingRepository) DeleteMessage(_ context.Context, roomNo string, userID, messageID int64, eventID string) (domain.Message, error) {
	r.deletedRoomNo = roomNo
	r.deletedUserID = userID
	r.deletedMessageID = messageID
	r.deleteEventID = eventID
	return r.deleteMessage, r.deleteMessageErr
}

func (r *recordingRepository) AdvanceRead(context.Context, string, int64, int64, string) (domain.Membership, int64, error) {
	return domain.Membership{}, 0, nil
}

func (r *recordingRepository) CreateGroup(context.Context, domain.Group) (domain.Group, error) {
	return domain.Group{}, nil
}

func (r *recordingRepository) UpdateGroup(context.Context, domain.Group) (domain.Group, error) {
	return domain.Group{}, nil
}

func (r *recordingRepository) DeleteGroup(context.Context, int64, int64) error {
	return nil
}

func (r *recordingRepository) MoveGroup(_ context.Context, userID, groupID int64, direction int32) error {
	r.movedUserID = userID
	r.movedGroupID = groupID
	r.movedDirection = direction
	return r.moveGroupErr
}

func (r *recordingRepository) PlaceRoom(_ context.Context, roomNo string, userID int64, placement domain.Placement) (domain.Membership, error) {
	r.placedRoomNo = roomNo
	r.placedUserID = userID
	r.placedPlacement = placement
	return domain.Membership{}, r.placeRoomErr
}

func (r *recordingRepository) UpdateAnnouncement(context.Context, string, int64, string, string) (domain.Room, error) {
	return domain.Room{}, nil
}

func (r *recordingRepository) MarkAnnouncementSeen(context.Context, string, int64, int64) (domain.Membership, error) {
	return domain.Membership{}, nil
}

func (r *recordingRepository) ValidateMemberships(_ context.Context, userID int64, roomNumbers []string) ([]string, error) {
	r.roomsValidated = append([]string(nil), roomNumbers...)
	if r.validateRooms != nil {
		return r.validateRooms(userID, roomNumbers)
	}
	return roomNumbers, nil
}

func newTestService(repo *recordingRepository) *Service {
	service := NewService(repo, &fixedIDs{})
	service.newEventID = func() string { return "00000000-0000-0000-0000-000000000001" }
	return service
}

func TestCreateRoomRetriesRoomNumberConflict(t *testing.T) {
	repo := &recordingRepository{}
	repo.createRoom = func(room domain.Room, owner domain.Membership) (domain.RoomDetails, error) {
		if repo.createRoomCalls == 1 {
			return domain.RoomDetails{}, domain.ErrRoomNumberConflict
		}
		return domain.RoomDetails{Room: room, Membership: &owner}, nil
	}
	service := newTestService(repo)
	numbers := []string{"AAAAAAAA", "BBBBBBBB"}
	service.newRoomNumber = func() (string, error) {
		value := numbers[repo.createRoomCalls]
		return value, nil
	}

	details, err := service.CreateRoom(context.Background(), 42, "  Release room  ")
	if err != nil {
		t.Fatal(err)
	}
	if details.Room.RoomNo != "BBBBBBBB" || details.Room.Name != "Release room" {
		t.Fatalf("unexpected room: %#v", details.Room)
	}
	if repo.createRoomCalls != 2 {
		t.Fatalf("create calls = %d, want 2", repo.createRoomCalls)
	}
}

func TestSendMessageNormalizesBodyAndClientID(t *testing.T) {
	repo := &recordingRepository{}
	service := newTestService(repo)
	message, _, err := service.SendMessage(
		context.Background(),
		" ab12cd3e ",
		42,
		"550e8400-e29b-41d4-a716-446655440000",
		"  hello  ",
	)
	if err != nil {
		t.Fatal(err)
	}
	if message.Body != "hello" || repo.sentMessage.ClientMessageID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected message: %#v", message)
	}
}

func TestListMessagesPreservesZeroAfterSequence(t *testing.T) {
	repo := &recordingRepository{}
	service := newTestService(repo)
	_, err := service.ListMessages(context.Background(), "AB12CD3E", 42, domain.MessageQuery{
		AfterSeq: 0, AfterSeqSet: true, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.messageQuery.AfterSeqSet || repo.messageQuery.AfterSeq != 0 || repo.messageQuery.Limit != 100 {
		t.Fatalf("message query = %#v", repo.messageQuery)
	}
}

func TestLookupRoomHidesAnnouncementForNonMember(t *testing.T) {
	repo := &recordingRepository{}
	repo.lookupRoom = func(_ string, userID int64) (domain.RoomDetails, error) {
		details := domain.RoomDetails{Room: domain.Room{Announcement: "private"}}
		if userID == 42 {
			details.Membership = &domain.Membership{UserID: userID}
		}
		return details, nil
	}
	service := newTestService(repo)

	preview, err := service.LookupRoom(context.Background(), "ab12cd3e", 7)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Room.Announcement != "" {
		t.Fatalf("non-member preview exposed announcement %q", preview.Room.Announcement)
	}
	member, err := service.LookupRoom(context.Background(), "ab12cd3e", 42)
	if err != nil {
		t.Fatal(err)
	}
	if member.Room.Announcement != "private" {
		t.Fatalf("member announcement = %q", member.Room.Announcement)
	}
}

func TestValidateMembershipsDeduplicatesRoomNumbers(t *testing.T) {
	repo := &recordingRepository{}
	service := newTestService(repo)
	rooms, err := service.ValidateMemberships(context.Background(), 42, []string{"ab12cd3e", "AB12CD3E", "xy34z567"})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.roomsValidated) != 2 || repo.roomsValidated[0] != "AB12CD3E" || repo.roomsValidated[1] != "XY34Z567" {
		t.Fatalf("validated rooms = %#v", repo.roomsValidated)
	}
	if len(rooms) != 2 {
		t.Fatalf("result rooms = %#v", rooms)
	}
}

func TestSendMessageRejectsInvalidInput(t *testing.T) {
	service := newTestService(&recordingRepository{})
	_, _, err := service.SendMessage(context.Background(), "AB12CD3E", 42, "not-a-uuid", "hello")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

func TestDeleteMessageValidatesAndDelegates(t *testing.T) {
	repo := &recordingRepository{deleteMessage: domain.Message{ID: 9, Status: domain.MessageStatusDeleted}}
	service := newTestService(repo)

	message, err := service.DeleteMessage(context.Background(), " ab12cd3e ", 42, 9)
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != 9 || message.Status != domain.MessageStatusDeleted {
		t.Fatalf("deleted message = %#v", message)
	}
	if repo.deletedRoomNo != "AB12CD3E" || repo.deletedUserID != 42 || repo.deletedMessageID != 9 || repo.deleteEventID == "" {
		t.Fatalf("delete arguments = room %q, user %d, message %d, event %q", repo.deletedRoomNo, repo.deletedUserID, repo.deletedMessageID, repo.deleteEventID)
	}

	if _, err := service.DeleteMessage(context.Background(), "AB12CD3E", 42, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("missing message id error = %v, want invalid input", err)
	}
}

func TestLeaveRoomValidatesAndDelegates(t *testing.T) {
	repo := &recordingRepository{leftMembership: domain.Membership{RoomID: 9, UserID: 42, Status: domain.MemberStatusLeft}}
	service := newTestService(repo)

	membership, err := service.LeaveRoom(context.Background(), " ab12cd3e ", 42)
	if err != nil {
		t.Fatal(err)
	}
	if membership.Status != domain.MemberStatusLeft {
		t.Fatalf("membership status = %d, want left", membership.Status)
	}
	if repo.leftRoomNo != "AB12CD3E" || repo.leftUserID != 42 || repo.leaveEventID == "" {
		t.Fatalf("leave arguments = room %q, user %d, event %q", repo.leftRoomNo, repo.leftUserID, repo.leaveEventID)
	}
	if _, err := service.LeaveRoom(context.Background(), "AB12CD3E", 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("missing user error = %v, want invalid input", err)
	}
}

func TestMoveGroupValidatesAndDelegates(t *testing.T) {
	repo := &recordingRepository{}
	service := newTestService(repo)
	if err := service.MoveGroup(context.Background(), 42, 9, -1); err != nil {
		t.Fatal(err)
	}
	if repo.movedUserID != 42 || repo.movedGroupID != 9 || repo.movedDirection != -1 {
		t.Fatalf("move arguments = user %d, group %d, direction %d", repo.movedUserID, repo.movedGroupID, repo.movedDirection)
	}

	if err := service.MoveGroup(context.Background(), 42, 9, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid direction error = %v, want invalid input", err)
	}
}

func TestPlaceRoomValidatesAndDelegates(t *testing.T) {
	repo := &recordingRepository{}
	service := newTestService(repo)
	if _, err := service.PlaceRoom(context.Background(), " ab12cd3e ", 42, 9, 2); err != nil {
		t.Fatal(err)
	}
	if repo.placedRoomNo != "AB12CD3E" || repo.placedUserID != 42 || repo.placedPlacement != (domain.Placement{GroupID: 9, SortOrder: 2}) {
		t.Fatalf("place arguments = room %q, user %d, placement %#v", repo.placedRoomNo, repo.placedUserID, repo.placedPlacement)
	}
	if _, err := service.PlaceRoom(context.Background(), "AB12CD3E", 42, 9, -1); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("negative sort order error = %v, want invalid input", err)
	}
}
