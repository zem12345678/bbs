package command

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
)

func TestAcceptCommentResolvesQATopicAndPublishesEvent(t *testing.T) {
	repo := newFakeRepo()
	repo.topics[101] = mustQATopicWithBounty(t, 101, "如何排查回调？", 50)
	comments := &fakeCommentReader{items: map[int64]CommentRef{
		9001: {ID: 9001, EntityType: "topic", EntityID: 101, AuthorID: 22, Status: 1},
	}}
	publisher := &fakePublisher{}
	svc := NewService(repo, fakeIDGen{}, publisher, comments, nil)

	topic, err := svc.AcceptComment(context.Background(), 101, 9001)
	if err != nil {
		t.Fatal(err)
	}

	if topic.QAStatus != domain.QAStatusResolved || topic.AcceptedCommentID != 9001 || topic.AcceptedCommentAuthorID != 22 {
		t.Fatalf("topic acceptance = status:%q comment:%d author:%d", topic.QAStatus, topic.AcceptedCommentID, topic.AcceptedCommentAuthorID)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	event, ok := publisher.events[0].(domain.QAAcceptedEvent)
	if !ok {
		t.Fatalf("event = %T, want QAAcceptedEvent", publisher.events[0])
	}
	if event.ID != domain.QAAcceptedEventID(101, 9001) || event.AcceptedCommentAuthorID != 22 || event.RewardCredits != 50 {
		t.Fatalf("event = %+v", event)
	}
}

func TestAcceptCommentPublishesDefaultRewardWithoutBounty(t *testing.T) {
	repo := newFakeRepo()
	repo.topics[101] = mustTopic(t, 101, "qa", "如何排查回调？")
	comments := &fakeCommentReader{items: map[int64]CommentRef{
		9001: {ID: 9001, EntityType: "topic", EntityID: 101, AuthorID: 22, Status: 1},
	}}
	publisher := &fakePublisher{}
	svc := NewService(repo, fakeIDGen{}, publisher, comments, nil)

	if _, err := svc.AcceptComment(context.Background(), 101, 9001); err != nil {
		t.Fatal(err)
	}

	event := publisher.events[0].(domain.QAAcceptedEvent)
	if event.RewardCredits != domain.AcceptedAnswerRewardCredits {
		t.Fatalf("reward credits = %d, want %d", event.RewardCredits, domain.AcceptedAnswerRewardCredits)
	}
}

func TestAcceptCommentSameCommentIsIdempotent(t *testing.T) {
	repo := newFakeRepo()
	topic := mustTopic(t, 101, "qa", "如何排查回调？")
	if _, err := topic.AcceptComment(9001, 22); err != nil {
		t.Fatal(err)
	}
	repo.topics[101] = topic
	comments := &fakeCommentReader{items: map[int64]CommentRef{}}
	publisher := &fakePublisher{}
	svc := NewService(repo, fakeIDGen{}, publisher, comments, nil)

	accepted, err := svc.AcceptComment(context.Background(), 101, 9001)
	if err != nil {
		t.Fatal(err)
	}

	if accepted.AcceptedCommentID != 9001 || comments.calls != 0 {
		t.Fatalf("accepted=%d comment lookups=%d", accepted.AcceptedCommentID, comments.calls)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	event := publisher.events[0].(domain.QAAcceptedEvent)
	if event.ID != domain.QAAcceptedEventID(101, 9001) {
		t.Fatalf("event id = %q", event.ID)
	}
}

func TestAcceptCommentRejectsNonQATopic(t *testing.T) {
	repo := newFakeRepo()
	repo.topics[101] = mustTopic(t, 101, "topic", "普通话题")
	svc := NewService(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{}, nil)

	_, err := svc.AcceptComment(context.Background(), 101, 9001)
	if !errors.Is(err, domain.ErrNotQuestion) {
		t.Fatalf("err = %v, want ErrNotQuestion", err)
	}
}

func TestAcceptCommentReturnsCommentNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.topics[101] = mustTopic(t, 101, "qa", "如何排查回调？")
	svc := NewService(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{err: domain.ErrCommentNotFound}, nil)

	_, err := svc.AcceptComment(context.Background(), 101, 9001)
	if !errors.Is(err, domain.ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func TestAcceptCommentRejectsCommentFromAnotherTopic(t *testing.T) {
	repo := newFakeRepo()
	repo.topics[101] = mustTopic(t, 101, "qa", "如何排查回调？")
	comments := &fakeCommentReader{items: map[int64]CommentRef{
		9001: {ID: 9001, EntityType: "topic", EntityID: 102, AuthorID: 22, Status: 1},
	}}
	svc := NewService(repo, fakeIDGen{}, &fakePublisher{}, comments, nil)

	_, err := svc.AcceptComment(context.Background(), 101, 9001)
	if !errors.Is(err, domain.ErrCommentNotInTopic) {
		t.Fatalf("err = %v, want ErrCommentNotInTopic", err)
	}
}

func TestAcceptCommentRejectsDifferentAlreadyAcceptedComment(t *testing.T) {
	repo := newFakeRepo()
	topic := mustTopic(t, 101, "qa", "如何排查回调？")
	if _, err := topic.AcceptComment(9001, 22); err != nil {
		t.Fatal(err)
	}
	repo.topics[101] = topic
	svc := NewService(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{}, nil)

	_, err := svc.AcceptComment(context.Background(), 101, 9002)
	if !errors.Is(err, domain.ErrAlreadyAccepted) {
		t.Fatalf("err = %v, want ErrAlreadyAccepted", err)
	}
}

type fakeIDGen struct{}

func (fakeIDGen) Generate() int64 { return 1 }

type fakeRepo struct {
	topics map[int64]*domain.Topic
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{topics: map[int64]*domain.Topic{}}
}

func (r *fakeRepo) CreateTopic(_ context.Context, t *domain.Topic) error {
	r.topics[t.ID] = t
	return nil
}

func (r *fakeRepo) UpdateTopic(_ context.Context, t *domain.Topic) error {
	if _, ok := r.topics[t.ID]; !ok {
		return domain.ErrNotFound
	}
	r.topics[t.ID] = t
	return nil
}

func (r *fakeRepo) FindTopicBySlug(_ context.Context, slug string) (*domain.Topic, error) {
	for _, topic := range r.topics {
		if topic.Slug == slug {
			return topic, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) FindTopicByID(_ context.Context, id int64) (*domain.Topic, error) {
	topic, ok := r.topics[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return topic, nil
}

func (r *fakeRepo) ListTopics(context.Context, domain.Status, domain.Type, string, int64, int64, string, int, int) ([]*domain.Topic, error) {
	return nil, nil
}

func (r *fakeRepo) UpdateTopicStatus(_ context.Context, id int64, status domain.Status, publishedAt *time.Time) error {
	topic, ok := r.topics[id]
	if !ok {
		return domain.ErrNotFound
	}
	topic.Status = status
	topic.PublishedAt = publishedAt
	return nil
}

func (r *fakeRepo) AcceptTopicComment(_ context.Context, topicID, commentID, commentAuthorID int64, _ time.Time) (*domain.Topic, bool, error) {
	topic, ok := r.topics[topicID]
	if !ok {
		return nil, false, domain.ErrNotFound
	}
	changed, err := topic.AcceptComment(commentID, commentAuthorID)
	if err != nil {
		return nil, false, err
	}
	return topic, changed, nil
}

func (r *fakeRepo) IncrementTopicViewCount(context.Context, int64) (int64, error) {
	return 0, nil
}

type fakeCommentReader struct {
	items map[int64]CommentRef
	err   error
	calls int
}

func (r *fakeCommentReader) GetComment(_ context.Context, id int64) (CommentRef, error) {
	r.calls++
	if r.err != nil {
		return CommentRef{}, r.err
	}
	item, ok := r.items[id]
	if !ok {
		return CommentRef{}, domain.ErrCommentNotFound
	}
	return item, nil
}

type fakePublisher struct {
	events []messaging.DomainEvent
}

func (p *fakePublisher) PublishDomainEvents(_ context.Context, events []messaging.DomainEvent) error {
	p.events = append(p.events, events...)
	return nil
}

func mustTopic(t *testing.T, id int64, typ string, title string) *domain.Topic {
	t.Helper()
	topic, err := domain.New(id, domain.CreateCmd{
		Slug:     title,
		Type:     typ,
		Title:    title,
		Body:     "body",
		AuthorID: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return topic
}

func mustQATopicWithBounty(t *testing.T, id int64, title string, bountyScore int64) *domain.Topic {
	t.Helper()
	topic, err := domain.New(id, domain.CreateCmd{
		Slug:        title,
		Type:        "qa",
		Title:       title,
		Body:        "body",
		AuthorID:    10,
		BountyScore: bountyScore,
	})
	if err != nil {
		t.Fatal(err)
	}
	return topic
}
