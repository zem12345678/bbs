package upstream

import (
	"context"

	"admin/api/proto/contentpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) ListTopics(ctx context.Context, status int32, typ string, tag string, authorID int64, categoryID int64, limit int32, offset int32) (domain.TopicList, error) {
	resp, err := c.content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status:     status,
		Type:       typ,
		Tag:        tag,
		AuthorId:   authorID,
		CategoryId: categoryID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return domain.TopicList{}, err
	}
	items := make([]domain.Topic, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainTopic(item))
	}
	return domain.TopicList{Items: items, Total: int64(len(items))}, nil
}

func (c *Clients) HideTopic(ctx context.Context, id int64) (domain.Topic, error) {
	resp, err := c.content.HideTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		return domain.Topic{}, err
	}
	return toDomainTopic(resp.GetTopic()), nil
}

func (c *Clients) PublishTopic(ctx context.Context, id int64) (domain.Topic, error) {
	resp, err := c.content.PublishTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		return domain.Topic{}, err
	}
	return toDomainTopic(resp.GetTopic()), nil
}

func (c *Clients) ArchiveTopic(ctx context.Context, id int64) (domain.Topic, error) {
	resp, err := c.content.ArchiveTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		return domain.Topic{}, err
	}
	return toDomainTopic(resp.GetTopic()), nil
}

func (c *Clients) ListCategories(ctx context.Context, status int32, limit int32, offset int32) (domain.CategoryList, error) {
	resp, err := c.content.ListCategories(ctx, &contentpb.ListCategoriesRequest{Status: status, Limit: limit, Offset: offset})
	if err != nil {
		return domain.CategoryList{}, err
	}
	items := make([]domain.Category, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainCategory(item))
	}
	return domain.CategoryList{Items: items, Total: int64(len(items))}, nil
}

func (c *Clients) CreateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error) {
	resp, err := c.content.CreateCategory(ctx, toContentCategoryRequest(command))
	if err != nil {
		return domain.Category{}, err
	}
	return toDomainCategory(resp.GetCategory()), nil
}

func (c *Clients) UpdateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error) {
	resp, err := c.content.UpdateCategory(ctx, toContentCategoryRequest(command))
	if err != nil {
		return domain.Category{}, err
	}
	return toDomainCategory(resp.GetCategory()), nil
}

func (c *Clients) DeleteCategory(ctx context.Context, id int64) error {
	_, err := c.content.DeleteCategory(ctx, &contentpb.CategoryIDRequest{Id: id})
	return err
}

func toDomainTopic(t *contentpb.TopicInfo) domain.Topic {
	if t == nil {
		return domain.Topic{}
	}
	return domain.Topic{
		ID:          t.GetId(),
		Slug:        t.GetSlug(),
		Type:        t.GetType(),
		Title:       t.GetTitle(),
		Body:        t.GetBody(),
		Tags:        t.GetTags(),
		AuthorID:    t.GetAuthorId(),
		CategoryID:  t.GetCategoryId(),
		Status:      t.GetStatus(),
		CreatedAt:   t.GetCreatedAt(),
		UpdatedAt:   t.GetUpdatedAt(),
		PublishedAt: t.GetPublishedAt(),
	}
}

func toDomainCategory(c *contentpb.CategoryInfo) domain.Category {
	if c == nil {
		return domain.Category{}
	}
	return domain.Category{
		ID:          c.GetId(),
		Slug:        c.GetSlug(),
		Name:        c.GetName(),
		Description: c.GetDescription(),
		Sort:        c.GetSort(),
		Status:      c.GetStatus(),
		TopicCount:  c.GetTopicCount(),
		CreatedAt:   c.GetCreatedAt(),
		UpdatedAt:   c.GetUpdatedAt(),
	}
}

func toContentCategoryRequest(command domain.UpsertCategoryCommand) *contentpb.UpsertCategoryRequest {
	return &contentpb.UpsertCategoryRequest{
		Id:          command.ID,
		Slug:        command.Slug,
		Name:        command.Name,
		Description: command.Description,
		Sort:        command.Sort,
		Status:      command.Status,
	}
}
