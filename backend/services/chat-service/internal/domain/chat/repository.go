package chat

import (
	"context"
	"errors"
)

var (
	ErrInvalidInput       = errors.New("invalid chat input")
	ErrNotFound           = errors.New("chat resource not found")
	ErrNotMember          = errors.New("chat membership required")
	ErrNotOwner           = errors.New("chat room owner required")
	ErrNotMessageAuthor   = errors.New("chat message author required")
	ErrRoomClosed         = errors.New("chat room is closed")
	ErrRoomNumberConflict = errors.New("chat room number already exists")
	ErrGroupNameConflict  = errors.New("chat group name already exists")
	ErrUserErased         = errors.New("chat user erased")
	ErrInvalidErasure     = errors.New("invalid chat account erasure")
)

type Repository interface {
	CreateRoom(context.Context, Room, Membership) (RoomDetails, error)
	LookupRoom(context.Context, string, int64) (RoomDetails, error)
	JoinRoom(context.Context, string, int64, string) (RoomDetails, error)
	LeaveRoom(context.Context, string, int64, string) (Membership, error)
	ListSidebar(context.Context, int64) (Sidebar, error)
	ListMessages(context.Context, string, int64, MessageQuery) (MessagePage, error)
	SendMessage(context.Context, string, int64, Message, string) (Message, int64, error)
	DeleteMessage(context.Context, string, int64, int64, string) (Message, error)
	AdvanceRead(context.Context, string, int64, int64, string) (Membership, int64, error)
	CreateGroup(context.Context, Group) (Group, error)
	UpdateGroup(context.Context, Group) (Group, error)
	DeleteGroup(context.Context, int64, int64) error
	MoveGroup(context.Context, int64, int64, int32) error
	PlaceRoom(context.Context, string, int64, Placement) (Membership, error)
	UpdateAnnouncement(context.Context, string, int64, string, string) (Room, error)
	MarkAnnouncementSeen(context.Context, string, int64, int64) (Membership, error)
	ValidateMemberships(context.Context, int64, []string) ([]string, error)
}

type AccountErasureResult struct {
	RedactedMessages       int64
	DeletedMemberships     int64
	DeletedGroups          int64
	TransferredRooms       int64
	ClosedRooms            int64
	SuppressedOutboxEvents int64
}

type AccountErasureRepository interface {
	EraseUserData(context.Context, int64, int64, int32) (AccountErasureResult, error)
}
