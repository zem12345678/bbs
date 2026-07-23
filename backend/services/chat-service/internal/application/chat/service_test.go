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
	createRoom      func(domain.Room, domain.Membership) (domain.RoomDetails, error)
	lookupRoom      func(string, int64) (domain.RoomDetails, error)
	validateRooms   func(int64, []string) ([]string, error)
	sentMessage     domain.Message
	roomsValidated  []string
	messageQuery    domain.MessageQuery
	createRoomCalls int
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

func (r *recordingRepository) PlaceRoom(context.Context, string, int64, domain.Placement) (domain.Membership, error) {
	return domain.Membership{}, nil
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
