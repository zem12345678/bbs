package article

import (
	"strings"
	"time"
)

type Article struct {
	ID          int64
	Slug        string
	Title       string
	Summary     string
	Body        string
	CoverURL    string
	Tags        []string
	AuthorID    int64
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
}

type CreateCmd struct {
	Slug     string
	Title    string
	Summary  string
	Body     string
	CoverURL string
	Tags     []string
	AuthorID int64
}

type UpdateCmd struct {
	Title    string
	Summary  string
	Body     string
	CoverURL string
	Tags     []string
}

func New(id int64, cmd CreateCmd) (*Article, error) {
	cmd.Slug = strings.TrimSpace(cmd.Slug)
	cmd.Title = strings.TrimSpace(cmd.Title)
	cmd.Body = strings.TrimSpace(cmd.Body)

	if cmd.Slug == "" {
		return nil, ErrSlugRequired
	}
	if cmd.Title == "" {
		return nil, ErrTitleRequired
	}
	if cmd.Body == "" {
		return nil, ErrBodyRequired
	}
	if cmd.AuthorID <= 0 {
		return nil, ErrAuthorRequired
	}

	now := time.Now()
	return &Article{
		ID:        id,
		Slug:      cmd.Slug,
		Title:     cmd.Title,
		Summary:   strings.TrimSpace(cmd.Summary),
		Body:      cmd.Body,
		CoverURL:  strings.TrimSpace(cmd.CoverURL),
		Tags:      normalizeTags(cmd.Tags),
		AuthorID:  cmd.AuthorID,
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (a *Article) Update(cmd UpdateCmd) error {
	if strings.TrimSpace(cmd.Title) == "" {
		return ErrTitleRequired
	}
	if strings.TrimSpace(cmd.Body) == "" {
		return ErrBodyRequired
	}
	a.Title = strings.TrimSpace(cmd.Title)
	a.Summary = strings.TrimSpace(cmd.Summary)
	a.Body = strings.TrimSpace(cmd.Body)
	a.CoverURL = strings.TrimSpace(cmd.CoverURL)
	a.Tags = normalizeTags(cmd.Tags)
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Article) Publish() error {
	switch a.Status {
	case StatusPublished:
		return ErrAlreadyPublished
	case StatusArchived:
		return ErrArchived
	}
	a.Status = StatusPublished
	a.UpdatedAt = time.Now()
	if a.PublishedAt == nil {
		t := a.UpdatedAt
		a.PublishedAt = &t
	}
	return nil
}

func (a *Article) Hide() error {
	if a.Status != StatusPublished {
		return ErrNotPublished
	}
	a.Status = StatusHidden
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Article) Archive() error {
	if a.Status == StatusArchived {
		return ErrArchived
	}
	a.Status = StatusArchived
	a.UpdatedAt = time.Now()
	return nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}
