package comment

import "strings"

const MaxContentRunes = 2000

type EntityType string

const (
	EntityArticle EntityType = "article"
	EntityTopic   EntityType = "topic"
)

func ParseEntityType(value string) (EntityType, error) {
	switch EntityType(strings.TrimSpace(value)) {
	case EntityArticle:
		return EntityArticle, nil
	case EntityTopic:
		return EntityTopic, nil
	default:
		return "", ErrInvalidEntityType
	}
}

type Status int32

const (
	StatusHidden  Status = 0
	StatusVisible Status = 1
)

func (s Status) IsValid() bool {
	return s == StatusHidden || s == StatusVisible
}

func (s Status) CanTransitionTo(target Status) bool {
	if !target.IsValid() {
		return false
	}
	if s == target {
		return true
	}
	return s == StatusVisible && target == StatusHidden
}
