package channel

import "context"

type Repository interface {
	CreateChannel(ctx context.Context, channel *Channel) error
	UpdateChannel(ctx context.Context, channel *Channel) error
	ArchiveChannel(ctx context.Context, channel *Channel) error
	SetChannelFeatured(ctx context.Context, channel *Channel) error
	SetChannelArchived(ctx context.Context, channel *Channel) error
	FindChannelByID(ctx context.Context, id, viewerID int64, includeArchived bool) (*Channel, error)
	ListChannels(ctx context.Context, filter ListFilter) ([]*Channel, int64, error)
	FollowChannel(ctx context.Context, channelID, userID int64) error
	UnfollowChannel(ctx context.Context, channelID, userID int64) error
	FavoriteChannel(ctx context.Context, channelID, userID int64) error
	UnfavoriteChannel(ctx context.Context, channelID, userID int64) error
	ListChannelCategoryAggregates(ctx context.Context, includeArchived bool) ([]CategoryAggregate, error)
}
