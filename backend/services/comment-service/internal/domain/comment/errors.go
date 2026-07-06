package comment

import "errors"

var (
	ErrNotFound            = errors.New("comment not found")
	ErrInvalidID           = errors.New("invalid comment id")
	ErrInvalidEntityType   = errors.New("invalid comment entity type")
	ErrInvalidEntityID     = errors.New("invalid comment entity id")
	ErrInvalidAuthorID     = errors.New("invalid comment author id")
	ErrContentRequired     = errors.New("comment content required")
	ErrContentTooLong      = errors.New("comment content too long")
	ErrInvalidParent       = errors.New("invalid parent comment")
	ErrInvalidStatus       = errors.New("invalid comment status")
	ErrAlreadyHidden       = errors.New("comment already hidden")
	ErrInvalidStatusChange = errors.New("invalid comment status change")
	ErrPermissionDenied    = errors.New("comment permission denied")
)
