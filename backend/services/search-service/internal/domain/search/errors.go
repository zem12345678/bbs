package search

import "errors"

var (
	ErrInvalidArticleID = errors.New("SEARCH_INVALID_ARTICLE_ID")
	ErrKeywordRequired  = errors.New("SEARCH_KEYWORD_REQUIRED")
)
