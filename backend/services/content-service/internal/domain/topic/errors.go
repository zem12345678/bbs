package topic

import "errors"

var (
	ErrSlugRequired                  = errors.New("TOPIC_SLUG_REQUIRED")
	ErrTitleRequired                 = errors.New("TOPIC_TITLE_REQUIRED")
	ErrBodyRequired                  = errors.New("TOPIC_BODY_REQUIRED")
	ErrAuthorRequired                = errors.New("TOPIC_AUTHOR_REQUIRED")
	ErrBountyInvalid                 = errors.New("TOPIC_BOUNTY_INVALID")
	ErrInvalidComment                = errors.New("TOPIC_ACCEPTED_COMMENT_INVALID")
	ErrNotQuestion                   = errors.New("TOPIC_NOT_QUESTION")
	ErrMembershipEntitlementRequired = errors.New("TOPIC_MEMBERSHIP_ENTITLEMENT_REQUIRED")
	ErrCommentNotFound               = errors.New("TOPIC_COMMENT_NOT_FOUND")
	ErrCommentNotInTopic             = errors.New("TOPIC_COMMENT_NOT_IN_TOPIC")
	ErrAlreadyAccepted               = errors.New("TOPIC_COMMENT_ALREADY_ACCEPTED")
	ErrCannotAcceptOwnComment        = errors.New("TOPIC_CANNOT_ACCEPT_OWN_COMMENT")
	ErrNotFound                      = errors.New("TOPIC_NOT_FOUND")
	ErrSlugExists                    = errors.New("TOPIC_SLUG_EXISTS")
	ErrAlreadyPublished              = errors.New("TOPIC_ALREADY_PUBLISHED")
	ErrNotPublished                  = errors.New("TOPIC_NOT_PUBLISHED")
	ErrArchived                      = errors.New("TOPIC_ARCHIVED")
)
