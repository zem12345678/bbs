package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetAntenna(ctx context.Context, ownerID, antennaID int64) (*domain.Antenna, error) {
	repo, err := s.antennaRepository(ownerID)
	if err != nil || antennaID <= 0 {
		if err != nil {
			return nil, err
		}
		return nil, domain.ErrInvalidID
	}
	return repo.GetAntenna(ctx, ownerID, antennaID)
}

func (s *Service) ListAntennas(ctx context.Context, ownerID int64) ([]*domain.Antenna, error) {
	repo, err := s.antennaRepository(ownerID)
	if err != nil {
		return nil, err
	}
	return repo.ListAntennas(ctx, ownerID)
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
