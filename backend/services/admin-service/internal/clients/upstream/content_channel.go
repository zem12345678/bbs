package upstream

import (
	"context"

	"admin/api/proto/contentpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) ListChannels(ctx context.Context, query string, categoryID int64, archivedStatus int32, limit int32, offset int32) (domain.ChannelList, error) {
	resp, err := c.content.ListChannels(ctx, &contentpb.ListChannelsRequest{
		Query:           query,
		CategoryId:      categoryID,
		IncludeArchived: archivedStatus == 0,
		ArchivedStatus:  archivedStatus,
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		return domain.ChannelList{}, err
	}
	items := make([]domain.Channel, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainChannel(item))
	}
	return domain.ChannelList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) SetChannelFeatured(ctx context.Context, id int64, featured bool) (domain.Channel, error) {
	resp, err := c.content.SetChannelFeatured(ctx, &contentpb.SetChannelFeaturedRequest{Id: id, Featured: featured})
	if err != nil {
		return domain.Channel{}, err
	}
	return toDomainChannel(resp.GetChannel()), nil
}

func (c *Clients) SetChannelArchived(ctx context.Context, id int64, archived bool) (domain.Channel, error) {
	resp, err := c.content.SetChannelArchived(ctx, &contentpb.SetChannelArchivedRequest{Id: id, Archived: archived})
	if err != nil {
		return domain.Channel{}, err
	}
	return toDomainChannel(resp.GetChannel()), nil
}

func toDomainChannel(channel *contentpb.ChannelInfo) domain.Channel {
	if channel == nil {
		return domain.Channel{}
	}
	return domain.Channel{
		ID:             channel.GetId(),
		OwnerID:        channel.GetOwnerId(),
		CategoryID:     channel.GetCategoryId(),
		Name:           channel.GetName(),
		Description:    channel.GetDescription(),
		Color:          channel.GetColor(),
		IsArchived:     channel.GetIsArchived(),
		FollowersCount: channel.GetFollowersCount(),
		TopicsCount:    channel.GetTopicsCount(),
		LastPostedAt:   channel.GetLastPostedAt(),
		CreatedAt:      channel.GetCreatedAt(),
		UpdatedAt:      channel.GetUpdatedAt(),
		IsFeatured:     channel.GetIsFeatured(),
	}
}
