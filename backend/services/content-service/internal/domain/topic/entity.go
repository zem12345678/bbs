package topic

import (
	"strings"
	"time"
)

type Topic struct {
	ID                      int64
	Slug                    string
	Type                    Type
	Title                   string
	Body                    string
	Tags                    []string
	AuthorID                int64
	CategoryID              int64
	ChannelID               int64
	BountyScore             int64
	QAStatus                QAStatus
	AcceptedCommentID       int64
	AcceptedCommentAuthorID int64
	QAAcceptanceCycle       int64
	Status                  Status
	CreatedAt               time.Time
	UpdatedAt               time.Time
	PublishedAt             *time.Time
	ViewCount               int64
	Poll                    *Poll
}

type CreateCmd struct {
	Slug        string
	Type        string
	Title       string
	Body        string
	Tags        []string
	AuthorID    int64
	CategoryID  int64
	ChannelID   int64
	BountyScore int64
	Poll        *PollInput
}

type UpdateCmd struct {
	Title       string
	Body        string
	Tags        []string
	CategoryID  int64
	ChannelID   int64
	BountyScore int64
	Poll        *PollInput
}

func New(id int64, cmd CreateCmd) (*Topic, error) {
	poll, err := normalizePollInput(cmd.Poll, time.Now())
	if err != nil {
		return nil, err
	}
	t := &Topic{
		ID:          id,
		Slug:        strings.TrimSpace(cmd.Slug),
		Type:        NormalizeType(strings.TrimSpace(cmd.Type)),
		Title:       strings.TrimSpace(cmd.Title),
		Body:        strings.TrimSpace(cmd.Body),
		Tags:        normalizeTags(cmd.Tags),
		AuthorID:    cmd.AuthorID,
		CategoryID:  normalizeCategoryID(cmd.CategoryID),
		ChannelID:   normalizeChannelID(cmd.ChannelID),
		BountyScore: cmd.BountyScore,
		Status:      StatusDraft,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Poll:        poll,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Topic) Validate() error {
	if t.Slug == "" {
		return ErrSlugRequired
	}
	if t.Type == "" {
		t.Type = TypeTopic
	}
	if (t.Type == TypeTopic || t.Type == TypeQA) && strings.TrimSpace(t.Title) == "" {
		return ErrTitleRequired
	}
	if strings.TrimSpace(t.Body) == "" {
		return ErrBodyRequired
	}
	if t.AuthorID <= 0 {
		return ErrAuthorRequired
	}
	if t.BountyScore < 0 {
		return ErrBountyInvalid
	}
	if t.Type == TypeQA {
		if t.QAStatus == "" {
			t.QAStatus = QAStatusOpen
		}
	} else {
		t.BountyScore = 0
		t.QAStatus = ""
		t.AcceptedCommentID = 0
		t.AcceptedCommentAuthorID = 0
	}
	return nil
}

func (t *Topic) Update(cmd UpdateCmd) error {
	switch t.Status {
	case StatusArchiving:
		return ErrNotPublished
	case StatusArchived:
		return ErrArchived
	}
	if t.Type == TypeQA && t.AcceptedCommentID > 0 {
		cmd.BountyScore = t.BountyScore
	}
	if t.Type == TypeQA && t.PublishedAt != nil && t.BountyScore > 0 {
		cmd.BountyScore = t.BountyScore
	}
	if cmd.Poll != nil {
		poll, err := normalizePollInput(cmd.Poll, time.Now())
		if err != nil {
			return err
		}
		t.Poll = poll
	}
	t.Title = strings.TrimSpace(cmd.Title)
	t.Body = strings.TrimSpace(cmd.Body)
	t.Tags = normalizeTags(cmd.Tags)
	t.CategoryID = normalizeCategoryID(cmd.CategoryID)
	t.ChannelID = normalizeChannelID(cmd.ChannelID)
	t.BountyScore = cmd.BountyScore
	t.UpdatedAt = time.Now()
	return t.Validate()
}

func (t *Topic) Publish() error {
	switch t.Status {
	case StatusPublished:
		return ErrAlreadyPublished
	case StatusArchiving:
		return ErrNotPublished
	case StatusArchived:
		return ErrArchived
	}
	t.Status = StatusPublished
	t.UpdatedAt = time.Now()
	if t.PublishedAt == nil {
		publishedAt := t.UpdatedAt
		t.PublishedAt = &publishedAt
	}
	return nil
}

func (t *Topic) Hide() error {
	if t.Status != StatusPublished {
		return ErrNotPublished
	}
	t.Status = StatusHidden
	t.UpdatedAt = time.Now()
	return nil
}

func (t *Topic) Archive() error {
	if t.Status == StatusArchived {
		return ErrArchived
	}
	t.Status = StatusArchived
	t.UpdatedAt = time.Now()
	return nil
}

func (t *Topic) BeginArchive() error {
	if t.Status == StatusArchived {
		return ErrArchived
	}
	if t.Status == StatusArchiving {
		return nil
	}
	t.Status = StatusArchiving
	t.UpdatedAt = time.Now()
	return nil
}

func (t *Topic) AcceptComment(commentID, commentAuthorID int64) (bool, error) {
	if t == nil || t.ID <= 0 {
		return false, ErrNotFound
	}
	if t.Type != TypeQA {
		return false, ErrNotQuestion
	}
	if t.Status != StatusPublished {
		return false, ErrNotPublished
	}
	if commentID <= 0 || commentAuthorID <= 0 {
		return false, ErrInvalidComment
	}
	if commentAuthorID == t.AuthorID {
		return false, ErrCannotAcceptOwnComment
	}
	if t.AcceptedCommentID > 0 {
		if t.AcceptedCommentID != commentID {
			return false, ErrAlreadyAccepted
		}
		changed := t.QAStatus != QAStatusResolved || t.AcceptedCommentAuthorID != commentAuthorID
		t.QAStatus = QAStatusResolved
		t.AcceptedCommentAuthorID = commentAuthorID
		if changed {
			t.UpdatedAt = time.Now()
		}
		return changed, nil
	}
	t.QAStatus = QAStatusResolved
	t.AcceptedCommentID = commentID
	t.AcceptedCommentAuthorID = commentAuthorID
	t.UpdatedAt = time.Now()
	return true, nil
}

func (t *Topic) UnacceptComment(commentID int64) (bool, error) {
	if t == nil || t.ID <= 0 {
		return false, ErrNotFound
	}
	if t.Type != TypeQA {
		return false, ErrNotQuestion
	}
	if t.Status != StatusPublished {
		return false, ErrNotPublished
	}
	if commentID <= 0 || t.QAStatus != QAStatusResolved || t.AcceptedCommentID != commentID || t.AcceptedCommentAuthorID <= 0 {
		return false, ErrNotAccepted
	}
	t.QAStatus = QAStatusOpen
	t.AcceptedCommentID = 0
	t.AcceptedCommentAuthorID = 0
	t.QAAcceptanceCycle++
	t.UpdatedAt = time.Now()
	return true, nil
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

func normalizeCategoryID(id int64) int64 {
	if id <= 0 {
		return 1
	}
	return id
}

func normalizeChannelID(id int64) int64 {
	if id <= 0 {
		return 0
	}
	return id
}
