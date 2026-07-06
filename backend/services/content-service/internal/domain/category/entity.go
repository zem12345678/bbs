package category

import (
	"strings"
	"time"
)

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

type CreateCmd struct {
	Slug        string
	Name        string
	Description string
	Sort        int32
	Status      Status
}

type UpdateCmd struct {
	Slug        string
	Name        string
	Description string
	Sort        int32
	Status      Status
}

func New(id int64, cmd CreateCmd) (*Category, error) {
	category := &Category{
		ID:          id,
		Slug:        strings.TrimSpace(cmd.Slug),
		Name:        strings.TrimSpace(cmd.Name),
		Description: strings.TrimSpace(cmd.Description),
		Sort:        cmd.Sort,
		Status:      normalizeStatus(cmd.Status),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := category.Validate(); err != nil {
		return nil, err
	}
	return category, nil
}

func (c *Category) Update(cmd UpdateCmd) error {
	c.Slug = strings.TrimSpace(cmd.Slug)
	c.Name = strings.TrimSpace(cmd.Name)
	c.Description = strings.TrimSpace(cmd.Description)
	c.Sort = cmd.Sort
	c.Status = normalizeStatus(cmd.Status)
	c.UpdatedAt = time.Now()
	return c.Validate()
}

func (c *Category) Validate() error {
	if c.Slug == "" {
		return ErrSlugRequired
	}
	if c.Name == "" {
		return ErrNameRequired
	}
	return nil
}

func (c *Category) CanReadPublicly() bool {
	return c != nil && c.Status == StatusEnabled
}

func normalizeStatus(status Status) Status {
	switch status {
	case StatusDisabled, StatusEnabled:
		return status
	default:
		return StatusEnabled
	}
}
