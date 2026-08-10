package grpc

import (
	"context"

	pb "admin/api/proto/adminpb"
	domain "admin/internal/domain/admin"
)

func (h *Handler) ListChannels(ctx context.Context, req *pb.ListChannelsRequest) (*pb.ChannelListResponse, error) {
	result, err := h.service.ListChannels(ctx, toActor(req.GetActor()), req.GetQuery(), req.GetCategoryId(), req.GetArchivedStatus(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ChannelListResponse{Items: toPbChannels(result.Items), Total: result.Total}, nil
}

func (h *Handler) SetChannelFeatured(ctx context.Context, req *pb.ChannelFeaturedRequest) (*pb.ChannelResponse, error) {
	channel, err := h.service.SetChannelFeatured(ctx, toActor(req.GetActor()), req.GetId(), req.GetFeatured())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ChannelResponse{Success: true, Message: "ok", Channel: toPbChannel(channel)}, nil
}

func (h *Handler) SetChannelArchived(ctx context.Context, req *pb.ChannelArchivedRequest) (*pb.ChannelResponse, error) {
	channel, err := h.service.SetChannelArchived(ctx, toActor(req.GetActor()), req.GetId(), req.GetArchived())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ChannelResponse{Success: true, Message: "ok", Channel: toPbChannel(channel)}, nil
}

func toPbChannels(channels []domain.Channel) []*pb.ChannelInfo {
	items := make([]*pb.ChannelInfo, 0, len(channels))
	for _, channel := range channels {
		items = append(items, toPbChannel(channel))
	}
	return items
}

func toPbChannel(channel domain.Channel) *pb.ChannelInfo {
	return &pb.ChannelInfo{
		Id:             channel.ID,
		OwnerId:        channel.OwnerID,
		CategoryId:     channel.CategoryID,
		Name:           channel.Name,
		Description:    channel.Description,
		Color:          channel.Color,
		IsArchived:     channel.IsArchived,
		FollowersCount: channel.FollowersCount,
		TopicsCount:    channel.TopicsCount,
		LastPostedAt:   channel.LastPostedAt,
		CreatedAt:      channel.CreatedAt,
		UpdatedAt:      channel.UpdatedAt,
		IsFeatured:     channel.IsFeatured,
	}
}
