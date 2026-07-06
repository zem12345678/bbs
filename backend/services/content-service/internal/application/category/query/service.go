package query

import (
	"context"

	domain "content-service/internal/domain/category"
)

type CategoryView struct {
	Category *domain.Category
}

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(ctx context.Context, id int64) (CategoryView, error) {
	category, err := s.repo.FindCategoryByID(ctx, id)
	if err != nil {
		return CategoryView{}, err
	}
	return CategoryView{Category: category}, nil
}

func (s *Service) List(ctx context.Context, status domain.Status, limit, offset int) ([]CategoryView, error) {
	categories, err := s.repo.ListCategories(ctx, status, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryView, 0, len(categories))
	for _, category := range categories {
		out = append(out, CategoryView{Category: category})
	}
	return out, nil
}
