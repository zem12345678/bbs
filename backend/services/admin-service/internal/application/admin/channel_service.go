package admin

import (
	"context"

	domain "admin/internal/domain/admin"
)

func (s *Service) ListChannels(ctx context.Context, actor domain.Actor, query string, categoryID int64, archivedStatus int32, limit int32, offset int32) (domain.ChannelList, error) {
	if err := actor.Validate(); err != nil {
		return domain.ChannelList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListChannels); err != nil {
		return domain.ChannelList{}, err
	}
	return s.content.ListChannels(ctx, query, categoryID, archivedStatus, limit, offset)
}

func (s *Service) SetChannelFeatured(ctx context.Context, actor domain.Actor, id int64, featured bool) (domain.Channel, error) {
	return s.updateChannel(ctx, actor, id, domain.ActionFeatureChannel, func(ctx context.Context, id int64) (domain.Channel, error) {
		return s.content.SetChannelFeatured(ctx, id, featured)
	})
}

func (s *Service) SetChannelArchived(ctx context.Context, actor domain.Actor, id int64, archived bool) (domain.Channel, error) {
	action := domain.ActionRestoreChannel
	if archived {
		action = domain.ActionArchiveChannel
	}
	return s.updateChannel(ctx, actor, id, action, func(ctx context.Context, id int64) (domain.Channel, error) {
		return s.content.SetChannelArchived(ctx, id, archived)
	})
}

func (s *Service) updateChannel(ctx context.Context, actor domain.Actor, id int64, action domain.Action, update func(context.Context, int64) (domain.Channel, error)) (domain.Channel, error) {
	if err := actor.Validate(); err != nil {
		return domain.Channel{}, err
	}
	if id <= 0 {
		return domain.Channel{}, domain.ErrInvalidChannelID
	}
	if err := s.auth.Authorize(ctx, actor, action); err != nil {
		return domain.Channel{}, err
	}
	return update(ctx, id)
}
