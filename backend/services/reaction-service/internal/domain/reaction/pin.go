package reaction

import "time"

const MaxPinsPerUser = 5

func ValidPinnedEntityType(entityType EntityType) bool {
	return entityType == EntityArticle || entityType == EntityTopic
}

func (r EntityRef) ValidateForPin() error {
	if !ValidPinnedEntityType(r.Type) {
		return ErrInvalidPinnedEntityType
	}
	if r.ID <= 0 {
		return ErrInvalidEntityID
	}
	return nil
}

type Pin struct {
	ID        int64
	UserID    int64
	Entity    EntityRef
	CreatedAt time.Time
}
