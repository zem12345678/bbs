package topic

import "errors"

var (
	ErrSlugRequired     = errors.New("TOPIC_SLUG_REQUIRED")
	ErrTitleRequired    = errors.New("TOPIC_TITLE_REQUIRED")
	ErrBodyRequired     = errors.New("TOPIC_BODY_REQUIRED")
	ErrAuthorRequired   = errors.New("TOPIC_AUTHOR_REQUIRED")
	ErrNotFound         = errors.New("TOPIC_NOT_FOUND")
	ErrSlugExists       = errors.New("TOPIC_SLUG_EXISTS")
	ErrAlreadyPublished = errors.New("TOPIC_ALREADY_PUBLISHED")
	ErrNotPublished     = errors.New("TOPIC_NOT_PUBLISHED")
	ErrArchived         = errors.New("TOPIC_ARCHIVED")
)
