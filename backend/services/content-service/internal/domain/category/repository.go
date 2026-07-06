package category

import "context"

type Repository interface {
	FindCategoryByID(ctx context.Context, id int64) (*Category, error)
	ListCategories(ctx context.Context, status Status, limit, offset int) ([]*Category, error)
}
