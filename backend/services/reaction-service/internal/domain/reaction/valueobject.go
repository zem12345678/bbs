package reaction

type EntityType string

const (
	EntityArticle EntityType = "article"
	EntityTopic   EntityType = "topic"
	EntityComment EntityType = "comment"
)

func (t EntityType) Valid() bool {
	switch t {
	case EntityArticle, EntityTopic, EntityComment:
		return true
	default:
		return false
	}
}

type EntityRef struct {
	Type EntityType
	ID   int64
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
