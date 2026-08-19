package reaction

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxCollectionNameRunes        = 100
	MaxCollectionDescriptionRunes = 2048
)

// ValidCollectionEntityType intentionally narrows the general reaction entity
// set to the content types that can be placed in a user collection.
func ValidCollectionEntityType(entityType EntityType) bool {
	return entityType == EntityArticle || entityType == EntityTopic
}

func (r EntityRef) ValidateForCollection() error {
	if !ValidCollectionEntityType(r.Type) {
		return ErrInvalidCollectionEntityType
	}
	if r.ID <= 0 {
		return ErrInvalidEntityID
	}
	return nil
}

type Collection struct {
	ID            int64
	UserID        int64
	Name          string
	Description   string
	IsPublic      bool
	ItemCount     int64
	LastClippedAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CollectionItem struct {
	ID           int64
	CollectionID int64
	Entity       EntityRef
	CreatedAt    time.Time
}

func NormalizeCollectionName(name string) string {
	return strings.TrimSpace(name)
}

func NormalizeCollectionDescription(description string) string {
	return strings.TrimSpace(description)
}

func ValidateCollectionFields(userID int64, name, description string) (string, string, error) {
	if userID <= 0 {
		return "", "", ErrInvalidUserID
	}
	name = NormalizeCollectionName(name)
	if name == "" || utf8.RuneCountInString(name) > MaxCollectionNameRunes {
		return "", "", ErrInvalidCollectionName
	}
	description = NormalizeCollectionDescription(description)
	if utf8.RuneCountInString(description) > MaxCollectionDescriptionRunes {
		return "", "", ErrInvalidCollectionDescription
	}
	return name, description, nil
}
