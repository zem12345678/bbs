package command

import (
	"context"

	domain "content-service/internal/domain/category"
)

type IDGenerator interface {
	Generate() int64
}

type Service struct {
	repo  domain.Repository
	idgen IDGenerator
}

func NewService(repo domain.Repository, idgen IDGenerator) *Service {
	return &Service{repo: repo, idgen: idgen}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Category, error) {
	category, err := domain.New(s.idgen.Generate(), cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *Service) Update(ctx context.Context, id int64, cmd domain.UpdateCmd) (*domain.Category, error) {
	category, err := s.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := category.Update(cmd); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return domain.ErrNotFound
	}
	return s.repo.DeleteCategory(ctx, id)
}
