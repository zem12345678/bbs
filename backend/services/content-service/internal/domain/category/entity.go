package category

import "time"

type Status int32

const (
	StatusDisabled Status = 1
	StatusEnabled  Status = 2
)

type Category struct {
	ID          int64
	Slug        string
	Name        string
	Description string
	Sort        int32
	Status      Status
	TopicCount  int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c *Category) CanReadPublicly() bool {
	return c != nil && c.Status == StatusEnabled
}
