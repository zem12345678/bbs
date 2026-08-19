package search

import "errors"

var (
	ErrInvalidArticleID     = errors.New("SEARCH_INVALID_ARTICLE_ID")
	ErrInvalidUserID        = errors.New("SEARCH_INVALID_USER_ID")
	ErrInvalidDeletionJobID = errors.New("SEARCH_INVALID_DELETION_JOB_ID")
	ErrInvalidPolicyVersion = errors.New("SEARCH_INVALID_POLICY_VERSION")
	ErrKeywordRequired      = errors.New("SEARCH_KEYWORD_REQUIRED")
	ErrTagQueryRequired     = errors.New("SEARCH_TAG_QUERY_REQUIRED")
	ErrTagQueryInvalid      = errors.New("SEARCH_TAG_QUERY_INVALID")
	ErrTagFilterUnsupported = errors.New("SEARCH_TAG_FILTER_UNSUPPORTED")
)
