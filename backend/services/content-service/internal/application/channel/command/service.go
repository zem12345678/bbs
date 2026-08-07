package command

import (
	"context"

	categoryDomain "content-service/internal/domain/category"
	domain "content-service/internal/domain/channel"
)

type IDGenerator interface {
	Generate() int64
}

type CategoryReader interface {
	FindCategoryByID(ctx context.Context, id int64) (*categoryDomain.Category, error)
}

type Service struct {
	repo       domain.Repository
	idgen      IDGenerator
	categories CategoryReader
}

func NewService(repo domain.Repository, idgen IDGenerator, categories CategoryReader) *Service {
	return &Service{repo: repo, idgen: idgen, categories: categories}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Channel, error) {
	channel, err := domain.New(s.idgen.Generate(), cmd)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCategoryEnabled(ctx, channel.CategoryID); err != nil {
		return nil, err
	}
	if err := s.repo.CreateChannel(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *Service) Update(ctx context.Context, id, ownerID int64, cmd domain.UpdateCmd) (*domain.Channel, error) {
	channel, err := s.ownedChannel(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if err := channel.Update(cmd); err != nil {
		return nil, err
	}
	if err := s.ensureCategoryEnabled(ctx, channel.CategoryID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateChannel(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *Service) ensureCategoryEnabled(ctx context.Context, categoryID int64) error {
	if categoryID <= 0 {
		return nil
	}
	category, err := s.categories.FindCategoryByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if !category.CanReadPublicly() {
		return domain.ErrCategoryDisabled
	}
	return nil
}

func (s *Service) Archive(ctx context.Context, id, ownerID int64) (*domain.Channel, error) {
	channel, err := s.ownedChannel(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if err := channel.Archive(); err != nil {
		return nil, err
	}
	if err := s.repo.ArchiveChannel(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (s *Service) Follow(ctx context.Context, channelID, userID int64) error {
	return s.repo.FollowChannel(ctx, channelID, userID)
}

func (s *Service) Unfollow(ctx context.Context, channelID, userID int64) error {
	return s.repo.UnfollowChannel(ctx, channelID, userID)
}

func (s *Service) Favorite(ctx context.Context, channelID, userID int64) error {
	return s.repo.FavoriteChannel(ctx, channelID, userID)
}

func (s *Service) Unfavorite(ctx context.Context, channelID, userID int64) error {
	return s.repo.UnfavoriteChannel(ctx, channelID, userID)
}

func (s *Service) ownedChannel(ctx context.Context, id, ownerID int64) (*domain.Channel, error) {
	channel, err := s.repo.FindChannelByID(ctx, id, ownerID, true)
	if err != nil {
		return nil, err
	}
	if ownerID <= 0 || channel.OwnerID != ownerID {
		return nil, domain.ErrForbidden
	}
	return channel, nil
}
