package search

import "context"

type Repository interface {
	EnsureArticleIndex(ctx context.Context) error
	EnsureTopicIndex(ctx context.Context) error
	IndexArticle(ctx context.Context, doc ArticleDocument) error
	IndexTopic(ctx context.Context, doc TopicDocument) error
	ReindexArticle(ctx context.Context, doc ArticleDocument) error
	ReindexTopic(ctx context.Context, doc TopicDocument) error
	DeleteArticle(ctx context.Context, id int64) error
	DeleteTopic(ctx context.Context, id int64) error
	SearchArticles(ctx context.Context, keyword string, page, pageSize int32) ([]ArticleHit, int64, error)
	SearchTopics(ctx context.Context, keyword string, page, pageSize int32) ([]TopicHit, int64, error)
	IncrementArticleCommentCount(ctx context.Context, id int64, delta int64) error
	IncrementTopicCommentCount(ctx context.Context, id int64, delta int64) error
	SetArticleLikeCount(ctx context.Context, id int64, count int64) error
	SetTopicLikeCount(ctx context.Context, id int64, count int64) error
	SetArticleFavoriteCount(ctx context.Context, id int64, count int64) error
	SetTopicFavoriteCount(ctx context.Context, id int64, count int64) error
	SetArticleViewCount(ctx context.Context, id int64, count int64) error
	SetTopicViewCount(ctx context.Context, id int64, count int64) error
}
