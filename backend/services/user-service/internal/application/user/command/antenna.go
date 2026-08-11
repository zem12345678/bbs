package command

import (
	"context"

	domain "user-service/internal/domain/user"
)

type AntennaInput struct {
	Name, Source                                   string
	UserListID                                     int64
	Keywords, ExcludeKeywords                      [][]string
	Users                                          []string
	CaseSensitive, LocalOnly, ExcludeBots          bool
	WithReplies, WithFile, ExcludeSensitiveChannel bool
}

func (s *Service) CreateAntenna(ctx context.Context, ownerID int64, input AntennaInput) (*domain.Antenna, error) {
	repo, err := s.antennaRepository(ownerID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.FindByID(ctx, ownerID); err != nil {
		return nil, err
	}
	antenna, err := domain.NewAntenna(s.idgen.Generate(), ownerID, input.Name, input.Source, input.UserListID, input.Keywords, input.ExcludeKeywords, input.Users, input.CaseSensitive, input.LocalOnly, input.ExcludeBots, input.WithReplies, input.WithFile, input.ExcludeSensitiveChannel)
	if err != nil {
		return nil, err
	}
	if err := repo.CreateAntenna(ctx, antenna); err != nil {
		return nil, err
	}
	return repo.GetAntenna(ctx, ownerID, antenna.ID)
}

func (s *Service) UpdateAntenna(ctx context.Context, ownerID, antennaID int64, input AntennaInput) (*domain.Antenna, error) {
	repo, err := s.antennaRepository(ownerID)
	if err != nil || antennaID <= 0 {
		if err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidID
	}
	existing, err := repo.GetAntenna(ctx, ownerID, antennaID)
	if err != nil {
		return nil, err
	}
	updated, err := domain.NewAntenna(antennaID, ownerID, input.Name, input.Source, input.UserListID, input.Keywords, input.ExcludeKeywords, input.Users, input.CaseSensitive, input.LocalOnly, input.ExcludeBots, input.WithReplies, input.WithFile, input.ExcludeSensitiveChannel)
	if err != nil {
		return nil, err
	}
	updated.CreatedAt = existing.CreatedAt
	if err := repo.UpdateAntenna(ctx, updated); err != nil {
		return nil, err
	}
	return repo.GetAntenna(ctx, ownerID, antennaID)
}

func (s *Service) DeleteAntenna(ctx context.Context, ownerID, antennaID int64) error {
	repo, err := s.antennaRepository(ownerID)
	if err != nil || antennaID <= 0 {
		if err != nil {
			return err
		}
		return domain.ErrInvalidID
	}
	return repo.DeleteAntenna(ctx, ownerID, antennaID)
}

func (s *Service) antennaRepository(ownerID int64) (domain.AntennaRepository, error) {
	if ownerID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, ok := s.repo.(domain.AntennaRepository)
	if !ok {
		return nil, domain.ErrAntennaRepositoryUnavailable
	}
	return repo, nil
}
