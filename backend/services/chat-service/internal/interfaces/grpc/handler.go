package grpc

import (
	"context"
	"errors"
	"time"

	"chat-service/api/proto/chatpb"
	chatapp "chat-service/internal/application/chat"
	domain "chat-service/internal/domain/chat"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	chatpb.UnimplementedChatServiceServer
	service *chatapp.Service
}

func NewHandler(service *chatapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateRoom(ctx context.Context, request *chatpb.CreateRoomRequest) (*chatpb.RoomDetailsResponse, error) {
	details, err := h.service.CreateRoom(ctx, request.GetCreatorId(), request.GetName())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.RoomDetailsResponse{Details: toRoomDetails(details)}, nil
}

func (h *Handler) LookupRoom(ctx context.Context, request *chatpb.LookupRoomRequest) (*chatpb.RoomDetailsResponse, error) {
	details, err := h.service.LookupRoom(ctx, request.GetRoomNo(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.RoomDetailsResponse{Details: toRoomDetails(details)}, nil
}

func (h *Handler) JoinRoom(ctx context.Context, request *chatpb.JoinRoomRequest) (*chatpb.RoomDetailsResponse, error) {
	details, err := h.service.JoinRoom(ctx, request.GetRoomNo(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.RoomDetailsResponse{Details: toRoomDetails(details)}, nil
}

func (h *Handler) LeaveRoom(ctx context.Context, request *chatpb.LeaveRoomRequest) (*chatpb.MembershipResponse, error) {
	membership, err := h.service.LeaveRoom(ctx, request.GetRoomNo(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.MembershipResponse{Membership: toMembership(membership)}, nil
}

func (h *Handler) ListSidebar(ctx context.Context, request *chatpb.ListSidebarRequest) (*chatpb.SidebarResponse, error) {
	sidebar, err := h.service.ListSidebar(ctx, request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	response := &chatpb.SidebarResponse{
		Groups: make([]*chatpb.RoomGroup, 0, len(sidebar.Groups)),
		Rooms:  make([]*chatpb.SidebarRoom, 0, len(sidebar.Rooms)),
	}
	for _, group := range sidebar.Groups {
		response.Groups = append(response.Groups, toGroup(group))
	}
	for _, room := range sidebar.Rooms {
		item := &chatpb.SidebarRoom{
			Room:        toRoom(room.Room),
			Membership:  toMembership(room.Membership),
			UnreadCount: room.UnreadCount,
		}
		if room.LastMessage != nil {
			item.LastMessage = toMessage(*room.LastMessage)
		}
		response.Rooms = append(response.Rooms, item)
	}
	return response, nil
}

func (h *Handler) ListMessages(ctx context.Context, request *chatpb.ListMessagesRequest) (*chatpb.MessagePageResponse, error) {
	page, err := h.service.ListMessages(ctx, request.GetRoomNo(), request.GetUserId(), domain.MessageQuery{
		AnchorSeq:    request.GetAnchorSeq(),
		Before:       request.GetBefore(),
		After:        request.GetAfter(),
		BeforeSeq:    request.GetBeforeSeq(),
		AfterSeq:     request.GetAfterSeq(),
		BeforeSeqSet: request.BeforeSeq != nil,
		AfterSeqSet:  request.AfterSeq != nil,
		Limit:        request.GetLimit(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	response := &chatpb.MessagePageResponse{
		Messages:  make([]*chatpb.ChatMessage, 0, len(page.Messages)),
		LatestSeq: page.LatestSeq,
		AnchorSeq: page.AnchorSeq,
		HasOlder:  page.HasOlder,
		HasNewer:  page.HasNewer,
	}
	for _, message := range page.Messages {
		response.Messages = append(response.Messages, toMessage(message))
	}
	return response, nil
}

func (h *Handler) SendMessage(ctx context.Context, request *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {
	message, latestSeq, err := h.service.SendMessage(
		ctx,
		request.GetRoomNo(),
		request.GetUserId(),
		request.GetClientMessageId(),
		request.GetBody(),
	)
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.SendMessageResponse{Message: toMessage(message), LatestSeq: latestSeq}, nil
}

func (h *Handler) DeleteMessage(ctx context.Context, request *chatpb.DeleteMessageRequest) (*chatpb.DeleteMessageResponse, error) {
	message, err := h.service.DeleteMessage(ctx, request.GetRoomNo(), request.GetUserId(), request.GetMessageId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.DeleteMessageResponse{Message: toMessage(message)}, nil
}

func (h *Handler) AdvanceRead(ctx context.Context, request *chatpb.AdvanceReadRequest) (*chatpb.AdvanceReadResponse, error) {
	membership, latestSeq, err := h.service.AdvanceRead(ctx, request.GetRoomNo(), request.GetUserId(), request.GetReadSeq())
	if err != nil {
		return nil, grpcError(err)
	}
	unread := latestSeq - membership.LastReadSeq
	if unread < 0 {
		unread = 0
	}
	return &chatpb.AdvanceReadResponse{
		Membership:  toMembership(membership),
		LatestSeq:   latestSeq,
		UnreadCount: unread,
	}, nil
}

func (h *Handler) CreateGroup(ctx context.Context, request *chatpb.CreateGroupRequest) (*chatpb.GroupResponse, error) {
	group, err := h.service.CreateGroup(ctx, request.GetUserId(), request.GetName(), request.GetSortOrder())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.GroupResponse{Group: toGroup(group)}, nil
}

func (h *Handler) UpdateGroup(ctx context.Context, request *chatpb.UpdateGroupRequest) (*chatpb.GroupResponse, error) {
	group, err := h.service.UpdateGroup(ctx, request.GetUserId(), request.GetGroupId(), request.GetName(), request.GetSortOrder())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.GroupResponse{Group: toGroup(group)}, nil
}

func (h *Handler) DeleteGroup(ctx context.Context, request *chatpb.DeleteGroupRequest) (*chatpb.SimpleResponse, error) {
	if err := h.service.DeleteGroup(ctx, request.GetUserId(), request.GetGroupId()); err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.SimpleResponse{Success: true}, nil
}

func (h *Handler) MoveGroup(ctx context.Context, request *chatpb.MoveGroupRequest) (*chatpb.SimpleResponse, error) {
	if err := h.service.MoveGroup(ctx, request.GetUserId(), request.GetGroupId(), request.GetDirection()); err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.SimpleResponse{Success: true}, nil
}

func (h *Handler) PlaceRoom(ctx context.Context, request *chatpb.PlaceRoomRequest) (*chatpb.MembershipResponse, error) {
	membership, err := h.service.PlaceRoom(
		ctx,
		request.GetRoomNo(),
		request.GetUserId(),
		request.GetGroupId(),
		request.GetSortOrder(),
	)
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.MembershipResponse{Membership: toMembership(membership)}, nil
}

func (h *Handler) UpdateAnnouncement(ctx context.Context, request *chatpb.UpdateAnnouncementRequest) (*chatpb.RoomResponse, error) {
	room, err := h.service.UpdateAnnouncement(ctx, request.GetRoomNo(), request.GetUserId(), request.GetAnnouncement())
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.RoomResponse{Room: toRoom(room)}, nil
}

func (h *Handler) MarkAnnouncementSeen(ctx context.Context, request *chatpb.MarkAnnouncementSeenRequest) (*chatpb.MembershipResponse, error) {
	membership, err := h.service.MarkAnnouncementSeen(
		ctx,
		request.GetRoomNo(),
		request.GetUserId(),
		request.GetAnnouncementVersion(),
	)
	if err != nil {
		return nil, grpcError(err)
	}
	return &chatpb.MembershipResponse{Membership: toMembership(membership)}, nil
}

func (h *Handler) ValidateRoomSubscriptions(ctx context.Context, request *chatpb.ValidateRoomSubscriptionsRequest) (*chatpb.ValidateRoomSubscriptionsResponse, error) {
	subscriptions, err := h.service.ValidateRoomSubscriptions(ctx, request.GetUserId(), request.GetRoomNumbers())
	if err != nil {
		return nil, grpcError(err)
	}
	response := &chatpb.ValidateRoomSubscriptionsResponse{
		RoomNumbers:   make([]string, 0, len(subscriptions)),
		Subscriptions: make([]*chatpb.RoomSubscription, 0, len(subscriptions)),
	}
	for _, subscription := range subscriptions {
		response.RoomNumbers = append(response.RoomNumbers, subscription.RoomNo)
		response.Subscriptions = append(response.Subscriptions, &chatpb.RoomSubscription{
			RoomId: subscription.RoomID,
			RoomNo: subscription.RoomNo,
		})
	}
	return response, nil
}

func toRoomDetails(details domain.RoomDetails) *chatpb.RoomDetails {
	response := &chatpb.RoomDetails{Room: toRoom(details.Room), MemberCount: details.MemberCount}
	if details.Membership != nil {
		response.Membership = toMembership(*details.Membership)
	}
	return response
}

func toRoom(room domain.Room) *chatpb.Room {
	return &chatpb.Room{
		Id: room.ID, RoomNo: room.RoomNo, Name: room.Name, CreatorId: room.CreatorID,
		Announcement: room.Announcement, AnnouncementVersion: room.AnnouncementVersion,
		LastMessageSeq: room.LastMessageSeq, Status: int32(room.Status),
		CreatedAt: milliseconds(room.CreatedAt), UpdatedAt: milliseconds(room.UpdatedAt),
	}
}

func toMembership(member domain.Membership) *chatpb.Membership {
	response := &chatpb.Membership{
		RoomId: member.RoomID, UserId: member.UserID, Role: int32(member.Role),
		Status: int32(member.Status), JoinedAtSeq: member.JoinedAtSeq,
		LastReadSeq:                 member.LastReadSeq,
		LastSeenAnnouncementVersion: member.LastSeenAnnouncementVersion,
		GroupId:                     member.GroupID, SortOrder: member.SortOrder,
		JoinedAt: milliseconds(member.JoinedAt), CreatedAt: milliseconds(member.CreatedAt),
		UpdatedAt: milliseconds(member.UpdatedAt),
	}
	if member.LeftAt != nil {
		response.LeftAt = milliseconds(*member.LeftAt)
	}
	return response
}

func toMessage(message domain.Message) *chatpb.ChatMessage {
	response := &chatpb.ChatMessage{
		Id: message.ID, RoomId: message.RoomID, Seq: message.Seq, SenderId: message.SenderID,
		ClientMessageId: message.ClientMessageID, Body: message.Body, Status: int32(message.Status),
		CreatedAt: milliseconds(message.CreatedAt), UpdatedAt: milliseconds(message.UpdatedAt),
	}
	if message.DeletedAt != nil {
		response.DeletedAt = milliseconds(*message.DeletedAt)
	}
	return response
}

func toGroup(group domain.Group) *chatpb.RoomGroup {
	return &chatpb.RoomGroup{
		Id: group.ID, UserId: group.UserID, Name: group.Name, SortOrder: group.SortOrder,
		CreatedAt: milliseconds(group.CreatedAt), UpdatedAt: milliseconds(group.UpdatedAt),
	}
}

func milliseconds(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrNotMember), errors.Is(err, domain.ErrNotOwner), errors.Is(err, domain.ErrNotMessageAuthor):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrRoomClosed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrGroupNameConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrRoomNumberConflict):
		return status.Error(codes.Aborted, err.Error())
	default:
		return status.Error(codes.Internal, "chat service internal error")
	}
}
