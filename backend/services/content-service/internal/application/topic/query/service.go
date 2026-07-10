package query

import (
	"context"

	domain "content-service/internal/domain/topic"
)

type TopicView struct {
	Topic *domain.Topic
}

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func toViews(topics []*domain.Topic) []TopicView {
	out := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		out = append(out, TopicView{Topic: t})
	}
	return out
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (TopicView, error) {
	t, err := s.repo.FindTopicBySlug(ctx, slug)
	if err != nil {
		return TopicView{}, err
	}
	if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
		t.ViewCount = count
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (TopicView, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return TopicView{}, err
	}
	if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
		t.ViewCount = count
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) List(ctx context.Context, status domain.Status, typ domain.Type, tag string, authorID int64, categoryID int64, limit, offset int) ([]TopicView, error) {
	topics, err := s.repo.ListTopics(ctx, status, typ, tag, authorID, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	return toViews(topics), nil
}
