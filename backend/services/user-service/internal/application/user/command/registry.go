package command

import (
	"context"

	domain "user-service/internal/domain/user"
)

func (s *Service) SetRegistryItem(ctx context.Context, userID int64, itemDomain *string, scope []string, key string, value []byte) (*domain.RegistryItem, error) {
	repo, err := s.registryRepository(userID)
	if err != nil {
		return nil, err
	}
	item, err := domain.NewRegistryItem(s.idgen.Generate(), userID, itemDomain, scope, key, value)
	if err != nil {
		return nil, err
	}
	if err := repo.SetRegistryItem(ctx, item); err != nil {
		return nil, err
	}
	return repo.GetRegistryItem(ctx, userID, item.Domain, item.Scope, item.Key)
}

func (s *Service) RemoveRegistryItem(ctx context.Context, userID int64, itemDomain *string, scope []string, key string) error {
	repo, err := s.registryRepository(userID)
	if err != nil {
		return err
	}
	return repo.RemoveRegistryItem(ctx, userID, itemDomain, scope, key)
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
