package category

import "errors"

var (
	ErrNotFound     = errors.New("CATEGORY_NOT_FOUND")
	ErrSlugRequired = errors.New("CATEGORY_SLUG_REQUIRED")
	ErrNameRequired = errors.New("CATEGORY_NAME_REQUIRED")
	ErrSlugExists   = errors.New("CATEGORY_SLUG_EXISTS")
	ErrInUse        = errors.New("CATEGORY_IN_USE")
)
