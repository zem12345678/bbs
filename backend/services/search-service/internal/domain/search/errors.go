package search

import "errors"

var (
	ErrInvalidArticleID = errors.New("SEARCH_INVALID_ARTICLE_ID")
	ErrInvalidUserID    = errors.New("SEARCH_INVALID_USER_ID")
	ErrKeywordRequired  = errors.New("SEARCH_KEYWORD_REQUIRED")
)
