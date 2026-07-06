package search

type ArticleDocument struct {
	ID             int64
	Title          string
	Summary        string
	ContentExcerpt string
	TagIDs         []string
	TagNames       []string
	AuthorID       int64
	AuthorNickname string
	Status         int32
	ViewCount      int64
	CommentCount   int64
	LikeCount      int64
	FavoriteCount  int64
	CreatedAt      int64
	UpdatedAt      int64
}

func (d ArticleDocument) Validate() error {
	if d.ID <= 0 {
		return ErrInvalidArticleID
	}
	return nil
}

type ArticleHit struct {
	Document ArticleDocument
	Score    float64
}

type TopicDocument struct {
	ID             int64
	Slug           string
	Type           string
	Title          string
	ContentExcerpt string
	TagNames       []string
	AuthorID       int64
	Status         int32
	CommentCount   int64
	LikeCount      int64
	FavoriteCount  int64
	CreatedAt      int64
	UpdatedAt      int64
}

func (d TopicDocument) Validate() error {
	if d.ID <= 0 {
		return ErrInvalidArticleID
	}
	return nil
}

type TopicHit struct {
	Document TopicDocument
	Score    float64
}
