package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetRegistryItem(ctx context.Context, userID int64, itemDomain *string, scope []string, key string) (*domain.RegistryItem, error) {
	repo, err := s.registryRepository(userID)
	if err != nil {
		return nil, err
	}
	return repo.GetRegistryItem(ctx, userID, itemDomain, scope, key)
}

func (s *Service) ListRegistryItems(ctx context.Context, userID int64, itemDomain *string, scope []string) ([]*domain.RegistryItem, error) {
	repo, err := s.registryRepository(userID)
	if err != nil {
		return nil, err
	}
	return repo.ListRegistryItems(ctx, userID, itemDomain, scope)
}

func (s *Service) ListRegistryScopeDomains(ctx context.Context, userID int64) ([]domain.RegistryScopeDomain, error) {
	repo, err := s.registryRepository(userID)
	if err != nil {
		return nil, err
	}
	return repo.ListRegistryScopeDomains(ctx, userID)
}

func (s *Service) registryRepository(userID int64) (domain.RegistryRepository, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, ok := s.repo.(domain.RegistryRepository)
	if !ok {
		return nil, domain.ErrRegistryRepositoryUnavailable
	}
	return repo, nil
}
