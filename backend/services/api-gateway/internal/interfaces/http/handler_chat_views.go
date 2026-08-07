package http

import (
	"strconv"

	"api-gateway/api/proto/chatpb"
)

type chatRoomView struct {
	ID                  string `json:"id"`
	RoomNo              string `json:"room_no"`
	Name                string `json:"name"`
	CreatorID           string `json:"creator_id"`
	Announcement        string `json:"announcement"`
	AnnouncementVersion string `json:"announcement_version"`
	LastMessageSeq      string `json:"last_message_seq"`
	Status              int32  `json:"status"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type chatMembershipView struct {
	RoomID                      string  `json:"room_id"`
	UserID                      string  `json:"user_id"`
	Role                        int32   `json:"role"`
	RoleName                    string  `json:"role_name"`
	Status                      int32   `json:"status"`
	JoinedAtSeq                 string  `json:"joined_at_seq"`
	LastReadSeq                 string  `json:"last_read_seq"`
	LastSeenAnnouncementVersion string  `json:"last_seen_announcement_version"`
	GroupID                     string  `json:"group_id"`
	SortOrder                   int32   `json:"sort_order"`
	JoinedAt                    string  `json:"joined_at"`
	LeftAt                      string  `json:"left_at"`
	CreatedAt                   string  `json:"created_at"`
	UpdatedAt                   string  `json:"updated_at"`
	MutedUntil                  *string `json:"muted_until"`
}

type chatMessageView struct {
	ID              string `json:"id"`
	RoomID          string `json:"room_id"`
	Seq             string `json:"seq"`
	SenderID        string `json:"sender_id"`
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
	Status          int32  `json:"status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	DeletedAt       string `json:"deleted_at"`
}

type chatRoomGroupView struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type chatRoomDetailsView struct {
	Room        *chatRoomView       `json:"room"`
	Membership  *chatMembershipView `json:"membership"`
	MemberCount string              `json:"member_count"`
}

type chatSidebarRoomView struct {
	Room        *chatRoomView       `json:"room"`
	Membership  *chatMembershipView `json:"membership"`
	LastMessage *chatMessageView    `json:"last_message"`
	UnreadCount string              `json:"unread_count"`
}

type chatAdvanceReadResponse struct {
	Membership  *chatMembershipView `json:"membership"`
	LatestSeq   string              `json:"latest_seq"`
	UnreadCount string              `json:"unread_count"`
}

type chatGroupResponse struct {
	Group *chatRoomGroupView `json:"group"`
}

type chatMembershipResponse struct {
	Membership *chatMembershipView `json:"membership"`
}

func chatInt64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

func chatRoomViewFromProto(room *chatpb.Room) *chatRoomView {
	if room == nil {
		return nil
	}
	return &chatRoomView{
		ID: chatInt64String(room.GetId()), RoomNo: room.GetRoomNo(), Name: room.GetName(),
		CreatorID: chatInt64String(room.GetCreatorId()), Announcement: room.GetAnnouncement(),
		AnnouncementVersion: chatInt64String(room.GetAnnouncementVersion()),
		LastMessageSeq:      chatInt64String(room.GetLastMessageSeq()), Status: room.GetStatus(),
		CreatedAt: chatInt64String(room.GetCreatedAt()), UpdatedAt: chatInt64String(room.GetUpdatedAt()),
	}
}

func chatMembershipViewFromProto(membership *chatpb.Membership) *chatMembershipView {
	if membership == nil {
		return nil
	}
	return &chatMembershipView{
		RoomID: chatInt64String(membership.GetRoomId()), UserID: chatInt64String(membership.GetUserId()),
		Role: membership.GetRole(), RoleName: chatRoomRoleName(membership.GetRole()), Status: membership.GetStatus(),
		JoinedAtSeq: chatInt64String(membership.GetJoinedAtSeq()), LastReadSeq: chatInt64String(membership.GetLastReadSeq()),
		LastSeenAnnouncementVersion: chatInt64String(membership.GetLastSeenAnnouncementVersion()),
		GroupID:                     chatInt64String(membership.GetGroupId()), SortOrder: membership.GetSortOrder(),
		JoinedAt: chatInt64String(membership.GetJoinedAt()), LeftAt: chatInt64String(membership.GetLeftAt()),
		CreatedAt: chatInt64String(membership.GetCreatedAt()), UpdatedAt: chatInt64String(membership.GetUpdatedAt()),
		MutedUntil: chatOptionalInt64String(membership.GetMutedUntil()),
	}
}

func chatRoomRoleName(role int32) string {
	switch role {
	case 1:
		return "owner"
	case 2:
		return "member"
	case 3:
		return "manager"
	default:
		return ""
	}
}

func chatOptionalInt64String(value int64) *string {
	if value <= 0 {
		return nil
	}
	formatted := chatInt64String(value)
	return &formatted
}

func chatMessageViewFromProto(message *chatpb.ChatMessage) *chatMessageView {
	if message == nil {
		return nil
	}
	return &chatMessageView{
		ID: chatInt64String(message.GetId()), RoomID: chatInt64String(message.GetRoomId()),
		Seq: chatInt64String(message.GetSeq()), SenderID: chatInt64String(message.GetSenderId()),
		ClientMessageID: message.GetClientMessageId(), Body: message.GetBody(), Status: message.GetStatus(),
		CreatedAt: chatInt64String(message.GetCreatedAt()), UpdatedAt: chatInt64String(message.GetUpdatedAt()),
		DeletedAt: chatInt64String(message.GetDeletedAt()),
	}
}

func chatRoomGroupViewFromProto(group *chatpb.RoomGroup) *chatRoomGroupView {
	if group == nil {
		return nil
	}
	return &chatRoomGroupView{
		ID: chatInt64String(group.GetId()), UserID: chatInt64String(group.GetUserId()), Name: group.GetName(),
		SortOrder: group.GetSortOrder(), CreatedAt: chatInt64String(group.GetCreatedAt()), UpdatedAt: chatInt64String(group.GetUpdatedAt()),
	}
}

func chatRoomDetailsViewFromProto(details *chatpb.RoomDetails) *chatRoomDetailsView {
	if details == nil {
		return nil
	}
	return &chatRoomDetailsView{
		Room: chatRoomViewFromProto(details.GetRoom()), Membership: chatMembershipViewFromProto(details.GetMembership()),
		MemberCount: chatInt64String(details.GetMemberCount()),
	}
}

func chatRoomGroupViews(groups []*chatpb.RoomGroup) []*chatRoomGroupView {
	views := make([]*chatRoomGroupView, 0, len(groups))
	for _, group := range groups {
		if view := chatRoomGroupViewFromProto(group); view != nil {
			views = append(views, view)
		}
	}
	return views
}

func chatSidebarRoomViews(rooms []*chatpb.SidebarRoom) []*chatSidebarRoomView {
	views := make([]*chatSidebarRoomView, 0, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		views = append(views, &chatSidebarRoomView{
			Room: chatRoomViewFromProto(room.GetRoom()), Membership: chatMembershipViewFromProto(room.GetMembership()),
			LastMessage: chatMessageViewFromProto(room.GetLastMessage()), UnreadCount: chatInt64String(room.GetUnreadCount()),
		})
	}
	return views
}

func chatMessageViews(messages []*chatpb.ChatMessage) []*chatMessageView {
	views := make([]*chatMessageView, 0, len(messages))
	for _, message := range messages {
		if view := chatMessageViewFromProto(message); view != nil {
			views = append(views, view)
		}
	}
	return views
}
