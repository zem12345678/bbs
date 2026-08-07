package chat

import "time"

const (
	RoomStatusActive int16 = 1
	RoomStatusClosed int16 = 2

	MemberRoleOwner   int16 = 1
	MemberRoleMember  int16 = 2
	MemberRoleManager int16 = 3

	MemberStatusJoined int16 = 1
	MemberStatusLeft   int16 = 2

	MessageStatusPublished int16 = 1
	MessageStatusDeleted   int16 = 2
)

type Room struct {
	ID                  int64
	RoomNo              string
	Name                string
	CreatorID           int64
	Announcement        string
	AnnouncementVersion int64
	LastMessageSeq      int64
	Status              int16
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Membership struct {
	RoomID                      int64
	UserID                      int64
	Role                        int16
	Status                      int16
	JoinedAtSeq                 int64
	LastReadSeq                 int64
	LastSeenAnnouncementVersion int64
	GroupID                     int64
	SortOrder                   int32
	JoinedAt                    time.Time
	LeftAt                      *time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	MutedUntil                  *time.Time
}

type Message struct {
	ID              int64
	RoomID          int64
	Seq             int64
	SenderID        int64
	ClientMessageID string
	Body            string
	Status          int16
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type Group struct {
	ID        int64
	UserID    int64
	Name      string
	SortOrder int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoomDetails struct {
	Room        Room
	Membership  *Membership
	MemberCount int64
}

type SidebarRoom struct {
	Room        Room
	Membership  Membership
	LastMessage *Message
	UnreadCount int64
}

type Sidebar struct {
	Groups []Group
	Rooms  []SidebarRoom
}

type MessageQuery struct {
	AnchorSeq    int64
	Before       int32
	After        int32
	BeforeSeq    int64
	AfterSeq     int64
	BeforeSeqSet bool
	AfterSeqSet  bool
	Limit        int32
}

type MessagePage struct {
	Messages  []Message
	LatestSeq int64
	AnchorSeq int64
	HasOlder  bool
	HasNewer  bool
}

type Placement struct {
	GroupID   int64
	SortOrder int32
}

type RoomSubscription struct {
	RoomID int64
	RoomNo string
}

type RoomMemberQuery struct {
	Limit  int32
	Offset int32
	Role   int16
	UserID int64
}

type RoomMemberPage struct {
	Members []Membership
	Total   int64
}
