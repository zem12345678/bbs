package search

import "errors"

var (
	ErrInvalidArticleID     = errors.New("SEARCH_INVALID_ARTICLE_ID")
	ErrInvalidUserID        = errors.New("SEARCH_INVALID_USER_ID")
	ErrInvalidDeletionJobID = errors.New("SEARCH_INVALID_DELETION_JOB_ID")
	ErrInvalidPolicyVersion = errors.New("SEARCH_INVALID_POLICY_VERSION")
	ErrKeywordRequired      = errors.New("SEARCH_KEYWORD_REQUIRED")
)
