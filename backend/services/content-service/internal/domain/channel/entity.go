package channel

import (
	"regexp"
	"strings"
	"time"
)

const DefaultColor = "#3b82f6"

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Channel struct {
	ID              int64
	OwnerID         int64
	CategoryID      int64
	Name            string
	Description     string
	Color           string
	IsArchived      bool
	IsFeatured      bool
	FollowersCount  int64
	TopicsCount     int64
	LastPostedAt    *time.Time
	ViewerFollowing bool
	ViewerFavorited bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ArchivedStatus int32

const (
	ArchivedStatusLegacy   ArchivedStatus = 0
	ArchivedStatusActive   ArchivedStatus = 1
	ArchivedStatusArchived ArchivedStatus = 2
)

type CreateCmd struct {
	OwnerID     int64
	CategoryID  int64
	Name        string
	Description string
	Color       string
}

type UpdateCmd struct {
	CategoryID  int64
	Name        string
	Description string
	Color       string
}

type ListFilter struct {
	Query           string
	Category        string
	CategoryID      int64
	Uncategorized   bool
	OwnerID         int64
	ViewerID        int64
	FollowerUserID  int64
	FavoritedUserID int64
	Featured        bool
	IncludeArchived bool
	ArchivedStatus  ArchivedStatus
	Limit           int
	Offset          int
}

type CategoryAggregate struct {
	CategoryID     int64
	Slug           string
	Name           string
	ChannelCount   int64
	FollowersCount int64
	TopicsCount    int64
	LastPostedAt   *time.Time
}

func New(id int64, cmd CreateCmd) (*Channel, error) {
	now := time.Now()
	channel := &Channel{
		ID:          id,
		OwnerID:     cmd.OwnerID,
		CategoryID:  normalizeCategoryID(cmd.CategoryID),
		Name:        strings.TrimSpace(cmd.Name),
		Description: strings.TrimSpace(cmd.Description),
		Color:       normalizeColor(cmd.Color),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := channel.Validate(); err != nil {
		return nil, err
	}
	return channel, nil
}

func (c *Channel) Validate() error {
	if c == nil || c.ID <= 0 {
		return ErrNotFound
	}
	if c.OwnerID <= 0 {
		return ErrOwnerRequired
	}
	if strings.TrimSpace(c.Name) == "" {
		return ErrNameRequired
	}
	if len([]rune(c.Name)) > 128 {
		return ErrNameTooLong
	}
	if len([]rune(c.Description)) > 2048 {
		return ErrDescriptionTooLong
	}
	if !colorPattern.MatchString(c.Color) {
		return ErrColorInvalid
	}
	return nil
}

func (c *Channel) Update(cmd UpdateCmd) error {
	if c == nil {
		return ErrNotFound
	}
	if c.IsArchived {
		return ErrArchived
	}
	c.CategoryID = normalizeCategoryID(cmd.CategoryID)
	c.Name = strings.TrimSpace(cmd.Name)
	c.Description = strings.TrimSpace(cmd.Description)
	c.Color = normalizeColor(cmd.Color)
	c.UpdatedAt = time.Now()
	return c.Validate()
}

func (c *Channel) Archive() error {
	if c == nil {
		return ErrNotFound
	}
	if c.IsArchived {
		return ErrArchived
	}
	c.IsArchived = true
	c.IsFeatured = false
	c.UpdatedAt = time.Now()
	return nil
}

func (c *Channel) SetFeatured(featured bool) error {
	if c == nil {
		return ErrNotFound
	}
	if featured && c.IsArchived {
		return ErrArchived
	}
	c.IsFeatured = featured
	c.UpdatedAt = time.Now()
	return nil
}

func (c *Channel) SetArchived(archived bool) error {
	if c == nil {
		return ErrNotFound
	}
	wasArchived := c.IsArchived
	c.IsArchived = archived
	if archived || wasArchived {
		c.IsFeatured = false
	}
	c.UpdatedAt = time.Now()
	return nil
}

func normalizeCategoryID(id int64) int64 {
	if id <= 0 {
		return 0
	}
	return id
}

func normalizeColor(color string) string {
	color = strings.TrimSpace(color)
	if color == "" {
		return DefaultColor
	}
	return color
}
