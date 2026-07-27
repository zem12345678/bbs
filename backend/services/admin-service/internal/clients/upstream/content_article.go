package upstream

import (
	"context"

	"admin/api/proto/contentpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) ListArticles(ctx context.Context, status int32, tag string, authorID int64, limit int32, offset int32) (domain.ArticleList, error) {
	resp, err := c.content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status:   status,
		Tag:      tag,
		AuthorId: authorID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return domain.ArticleList{}, err
	}
	items := make([]domain.Article, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainArticle(item))
	}
	return domain.ArticleList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) HideArticle(ctx context.Context, id int64) (domain.Article, error) {
	resp, err := c.content.HideArticle(ctx, &contentpb.ArticleIDRequest{Id: id})
	if err != nil {
		return domain.Article{}, err
	}
	return toDomainArticle(resp.GetArticle()), nil
}

func (c *Clients) PublishArticle(ctx context.Context, id int64) (domain.Article, error) {
	resp, err := c.content.PublishArticle(ctx, &contentpb.ArticleIDRequest{Id: id})
	if err != nil {
		return domain.Article{}, err
	}
	return toDomainArticle(resp.GetArticle()), nil
}

func (c *Clients) ArchiveArticle(ctx context.Context, id int64) (domain.Article, error) {
	resp, err := c.content.ArchiveArticle(ctx, &contentpb.ArticleIDRequest{Id: id})
	if err != nil {
		return domain.Article{}, err
	}
	return toDomainArticle(resp.GetArticle()), nil
}

func toDomainArticle(a *contentpb.ArticleInfo) domain.Article {
	if a == nil {
		return domain.Article{}
	}
	return domain.Article{
		ID:          a.GetId(),
		Slug:        a.GetSlug(),
		Title:       a.GetTitle(),
		Summary:     a.GetSummary(),
		Body:        a.GetBody(),
		CoverURL:    a.GetCoverUrl(),
		Tags:        a.GetTags(),
		AuthorID:    a.GetAuthorId(),
		Status:      a.GetStatus(),
		CreatedAt:   a.GetCreatedAt(),
		UpdatedAt:   a.GetUpdatedAt(),
		PublishedAt: a.GetPublishedAt(),
	}
}
