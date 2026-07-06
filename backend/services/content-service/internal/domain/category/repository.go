package category

import "context"

type Repository interface {
	FindCategoryByID(ctx context.Context, id int64) (*Category, error)
	ListCategories(ctx context.Context, status Status, limit, offset int) ([]*Category, error)
	CreateCategory(ctx context.Context, category *Category) error
	UpdateCategory(ctx context.Context, category *Category) error
	DeleteCategory(ctx context.Context, id int64) error
}
