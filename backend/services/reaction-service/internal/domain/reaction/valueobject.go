package reaction

import (
	"strings"
	"unicode/utf8"
)

type EntityType string

const (
	EntityArticle    EntityType = "article"
	EntityTopic      EntityType = "topic"
	EntityComment    EntityType = "comment"
	EntityCollection EntityType = "collection"
	EntityUser       EntityType = "user"
)

func (t EntityType) Valid() bool {
	switch t {
	case EntityArticle, EntityTopic, EntityComment, EntityCollection, EntityUser:
		return true
	default:
		return false
	}
}

type EntityRef struct {
	Type EntityType
	ID   int64
}

func NormalizeReaction(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 128 {
		return "", ErrInvalidReaction
	}
	return value, nil
}

func (r EntityRef) Validate() error {
	if !r.Type.Valid() {
		return ErrInvalidEntityType
	}
	if r.ID <= 0 {
		return ErrInvalidEntityID
	}
	return nil
}
