package grpc

import (
	"context"

	pb "admin/api/proto/adminpb"
	domain "admin/internal/domain/admin"
)

func (h *Handler) ListAnnouncements(ctx context.Context, req *pb.ListAnnouncementsRequest) (*pb.AnnouncementListResponse, error) {
	result, err := h.service.ListAnnouncements(ctx, toActor(req.GetActor()), domain.AnnouncementListFilter{
		Limit: req.GetLimit(), SinceID: req.GetSinceId(), UntilID: req.GetUntilId(), UserID: req.GetUserId(), Status: req.GetStatus(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AnnouncementListResponse{Items: toPbAnnouncements(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListPublicAnnouncements(ctx context.Context, req *pb.ListPublicAnnouncementsRequest) (*pb.AnnouncementListResponse, error) {
	var active *bool
	if req.GetActive() || req.Active != nil {
		value := req.GetActive()
		active = &value
	}
	result, err := h.service.ListPublicAnnouncements(ctx, req.GetUserId(), req.GetUserCreatedAt(), domain.PublicAnnouncementListFilter{
		Limit: req.GetLimit(), SinceID: req.GetSinceId(), UntilID: req.GetUntilId(), Active: active,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AnnouncementListResponse{Items: toPbAnnouncements(result.Items), Total: result.Total}, nil
}

func (h *Handler) GetPublicAnnouncement(ctx context.Context, req *pb.GetPublicAnnouncementRequest) (*pb.AnnouncementResponse, error) {
	item, err := h.service.GetPublicAnnouncement(ctx, req.GetUserId(), req.GetUserCreatedAt(), req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AnnouncementResponse{Success: true, Message: "ok", Announcement: toPbAnnouncement(item)}, nil
}

func (h *Handler) CreateAnnouncement(ctx context.Context, req *pb.CreateAnnouncementRequest) (*pb.AnnouncementResponse, error) {
	item, err := h.service.CreateAnnouncement(ctx, toActor(req.GetActor()), domain.CreateAnnouncementCommand{
		Title: req.GetTitle(), Text: req.GetText(), ImageURL: req.GetImageUrl(), Icon: req.GetIcon(), Display: req.GetDisplay(),
		ForExistingUsers: req.GetForExistingUsers(), ForRoles: req.GetForRoles(), Silence: req.GetSilence(),
		NeedConfirmationToRead: req.GetNeedConfirmationToRead(), Confetti: req.GetConfetti(), UserID: req.GetUserId(),
		Active: req.GetActive(), StartsAt: req.GetStartsAt(), EndsAt: req.GetEndsAt(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AnnouncementResponse{Success: true, Message: "ok", Announcement: toPbAnnouncement(item)}, nil
}

func (h *Handler) UpdateAnnouncement(ctx context.Context, req *pb.UpdateAnnouncementRequest) (*pb.AnnouncementResponse, error) {
	command := domain.UpdateAnnouncementCommand{
		ID: req.GetId(), Title: req.Title, Text: req.Text, ImageURL: req.ImageUrl, Icon: req.Icon, Display: req.Display,
		ForExistingUsers: req.ForExistingUsers, Silence: req.Silence, NeedConfirmationToRead: req.NeedConfirmationToRead,
		Confetti: req.Confetti, Active: req.Active, StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	}
	if req.GetForRoles() != nil {
		command.ForRolesSet = true
		command.ForRoles = req.GetForRoles().GetValues()
	}
	item, err := h.service.UpdateAnnouncement(ctx, toActor(req.GetActor()), command)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AnnouncementResponse{Success: true, Message: "ok", Announcement: toPbAnnouncement(item)}, nil
}

func (h *Handler) DeleteAnnouncement(ctx context.Context, req *pb.AnnouncementIDRequest) (*pb.SimpleResponse, error) {
	if err := h.service.DeleteAnnouncement(ctx, toActor(req.GetActor()), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) MarkAnnouncementRead(ctx context.Context, req *pb.ReadAnnouncementRequest) (*pb.SimpleResponse, error) {
	if err := h.service.MarkAnnouncementRead(ctx, req.GetUserId(), req.GetAnnouncementId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func toPbAnnouncement(item domain.Announcement) *pb.AnnouncementInfo {
	return &pb.AnnouncementInfo{
		Id: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Title: item.Title, Text: item.Text,
		ImageUrl: item.ImageURL, Icon: item.Icon, Display: item.Display, ForExistingUsers: item.ForExistingUsers,
		ForRoles: item.ForRoles, Silence: item.Silence, NeedConfirmationToRead: item.NeedConfirmationToRead,
		Confetti: item.Confetti, UserId: item.UserID, Active: item.Active, StartsAt: item.StartsAt, EndsAt: item.EndsAt,
		Reads: item.Reads, ForYou: item.ForYou, IsRead: item.IsRead,
	}
}

func toPbAnnouncements(items []domain.Announcement) []*pb.AnnouncementInfo {
	result := make([]*pb.AnnouncementInfo, 0, len(items))
	for _, item := range items {
		result = append(result, toPbAnnouncement(item))
	}
	return result
}
