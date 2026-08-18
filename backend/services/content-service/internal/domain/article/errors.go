package article

import "errors"

var (
	ErrSlugRequired      = errors.New("ARTICLE_SLUG_REQUIRED")
	ErrTitleRequired     = errors.New("ARTICLE_TITLE_REQUIRED")
	ErrBodyRequired      = errors.New("ARTICLE_BODY_REQUIRED")
	ErrAuthorRequired    = errors.New("ARTICLE_AUTHOR_REQUIRED")
	ErrNotFound          = errors.New("ARTICLE_NOT_FOUND")
	ErrSlugExists        = errors.New("ARTICLE_SLUG_EXISTS")
	ErrAlreadyPublished  = errors.New("ARTICLE_ALREADY_PUBLISHED")
	ErrNotPublished      = errors.New("ARTICLE_NOT_PUBLISHED")
	ErrArchived          = errors.New("ARTICLE_ARCHIVED")
	ErrInvalidCursor     = errors.New("ARTICLE_INVALID_CURSOR")
	ErrKeysetUnavailable = errors.New("ARTICLE_KEYSET_UNAVAILABLE")
)
