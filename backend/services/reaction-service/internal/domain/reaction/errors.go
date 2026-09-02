package reaction

import "errors"

var (
	ErrInvalidEntityType             = errors.New("REACTION_INVALID_ENTITY_TYPE")
	ErrInvalidEntityID               = errors.New("REACTION_INVALID_ENTITY_ID")
	ErrInvalidUserID                 = errors.New("REACTION_INVALID_USER_ID")
	ErrInvalidReaction               = errors.New("REACTION_INVALID_REACTION")
	ErrReactionAlreadyExists         = errors.New("REACTION_ALREADY_EXISTS")
	ErrReactionNotFound              = errors.New("REACTION_NOT_FOUND")
	ErrReactionRepositoryUnavailable = errors.New("REACTION_REACTION_REPOSITORY_UNAVAILABLE")
	ErrInvalidReactionCursor         = errors.New("REACTION_INVALID_REACTION_CURSOR")
	ErrInvalidReportID               = errors.New("REACTION_INVALID_REPORT_ID")
	ErrInvalidReportReason           = errors.New("REACTION_INVALID_REPORT_REASON")
	ErrInvalidReportStatus           = errors.New("REACTION_INVALID_REPORT_STATUS")
	ErrInvalidReportNote             = errors.New("REACTION_INVALID_REPORT_NOTE")
	ErrInvalidReportAction           = errors.New("REACTION_INVALID_REPORT_ACTION")
	ErrReportNotFound                = errors.New("REACTION_REPORT_NOT_FOUND")
	ErrInvalidFavoriteCursor         = errors.New("REACTION_INVALID_FAVORITE_CURSOR")
	ErrInvalidPinnedEntityType       = errors.New("REACTION_INVALID_PINNED_ENTITY_TYPE")
	ErrPinLimitExceeded              = errors.New("REACTION_PIN_LIMIT_EXCEEDED")
	ErrAlreadyPinned                 = errors.New("REACTION_ALREADY_PINNED")
	ErrPinRepositoryUnavailable      = errors.New("REACTION_PIN_REPOSITORY_UNAVAILABLE")

	ErrInvalidCollectionID             = errors.New("REACTION_INVALID_COLLECTION_ID")
	ErrInvalidCollectionName           = errors.New("REACTION_INVALID_COLLECTION_NAME")
	ErrInvalidCollectionDescription    = errors.New("REACTION_INVALID_COLLECTION_DESCRIPTION")
	ErrInvalidCollectionEntityType     = errors.New("REACTION_INVALID_COLLECTION_ENTITY_TYPE")
	ErrInvalidCollectionCursor         = errors.New("REACTION_INVALID_COLLECTION_CURSOR")
	ErrCollectionNotFound              = errors.New("REACTION_COLLECTION_NOT_FOUND")
	ErrCollectionNameExists            = errors.New("REACTION_COLLECTION_NAME_EXISTS")
	ErrCollectionRepositoryUnavailable = errors.New("REACTION_COLLECTION_REPOSITORY_UNAVAILABLE")
)
