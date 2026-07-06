package reaction

import "errors"

var (
	ErrInvalidEntityType   = errors.New("REACTION_INVALID_ENTITY_TYPE")
	ErrInvalidEntityID     = errors.New("REACTION_INVALID_ENTITY_ID")
	ErrInvalidUserID       = errors.New("REACTION_INVALID_USER_ID")
	ErrInvalidReportID     = errors.New("REACTION_INVALID_REPORT_ID")
	ErrInvalidReportReason = errors.New("REACTION_INVALID_REPORT_REASON")
	ErrInvalidReportStatus = errors.New("REACTION_INVALID_REPORT_STATUS")
	ErrReportNotFound      = errors.New("REACTION_REPORT_NOT_FOUND")
)
