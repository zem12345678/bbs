package channel

import "errors"

var (
	ErrNotFound           = errors.New("channel not found")
	ErrOwnerRequired      = errors.New("channel owner required")
	ErrNameRequired       = errors.New("channel name required")
	ErrNameTooLong        = errors.New("channel name too long")
	ErrDescriptionTooLong = errors.New("channel description too long")
	ErrColorInvalid       = errors.New("channel color invalid")
	ErrCategoryDisabled   = errors.New("channel category disabled")
	ErrForbidden          = errors.New("channel operation forbidden")
	ErrArchived           = errors.New("channel archived")
)
