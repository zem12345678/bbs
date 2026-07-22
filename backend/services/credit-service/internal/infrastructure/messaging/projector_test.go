package messaging

import (
	"context"
	"testing"
	"time"

	app "credit-service/internal/application/credit"
	domain "credit-service/internal/domain/credit"
)

func TestHandleArticleProjectsPublishedTopicForFirstTopicTask(t *testing.T) {
	t.Parallel()

	repo := &topicProjectionRepo{}
	projector := NewProjector(app.NewService(repo))
	occurredAt := time.Date(2026, time.July, 20, 10, 30, 0, 0, time.UTC)
	err := projector.HandleArticle(context.Background(), eventEnvelope{
		EventType:  "topic.published.v1",
		OccurredAt: occurredAt,
		Payload:    []byte(`{"topic_id":501,"author_id":42,"title":"首个话题"}`),
	})
	if err != nil {
		t.Fatalf("project topic published event: %v", err)
	}
	if repo.topic.ID != 501 || repo.topic.AuthorID != 42 || repo.topic.Title != "首个话题" || !repo.publishedAt.Equal(occurredAt) {
		t.Fatalf("projected topic = %+v at %s", repo.topic, repo.publishedAt)
	}
}

func TestHandleCommentProjectsTopicCommentForFirstCommentTask(t *testing.T) {
	t.Parallel()

	repo := &topicProjectionRepo{}
	projector := NewProjector(app.NewService(repo))
	occurredAt := time.Date(2026, time.July, 20, 10, 45, 0, 0, time.UTC)
	err := projector.HandleComment(context.Background(), eventEnvelope{
		EventID:    "comment-created:601",
		EventType:  "comment.created",
		OccurredAt: occurredAt,
		Payload:    []byte(`{"comment_id":601,"entity_type":"topic","entity_id":501,"author_id":42}`),
	})
	if err != nil {
		t.Fatalf("project topic comment event: %v", err)
	}
	if repo.comment.ID != 601 || repo.comment.AuthorID != 42 || repo.comment.EntityType != "topic" || repo.comment.EntityID != 501 || !repo.commentCreatedAt.Equal(occurredAt) {
		t.Fatalf("projected comment = %+v at %s", repo.comment, repo.commentCreatedAt)
	}
}

type topicProjectionRepo struct {
	topic            domain.TopicPublicationRef
	publishedAt      time.Time
	comment          domain.CommentCreationRef
	commentCreatedAt time.Time
}

func (*topicProjectionRepo) EnsureSchema(context.Context) error { return nil }
func (*topicProjectionRepo) SaveArticle(context.Context, domain.ArticleRef, time.Time) error {
	return nil
}
func (*topicProjectionRepo) GetArticle(context.Context, int64) (domain.ArticleRef, error) {
	return domain.ArticleRef{}, nil
}
func (r *topicProjectionRepo) SavePublishedTopic(_ context.Context, topic domain.TopicPublicationRef, publishedAt time.Time) error {
	r.topic = topic
	r.publishedAt = publishedAt
	return nil
}
func (*topicProjectionRepo) HasPublishedTopic(context.Context, int64) (bool, error) {
	return false, nil
}
func (r *topicProjectionRepo) SaveCreatedComment(_ context.Context, comment domain.CommentCreationRef, createdAt time.Time) error {
	r.comment = comment
	r.commentCreatedAt = createdAt
	return nil
}
func (*topicProjectionRepo) HasCreatedComment(context.Context, int64) (bool, error) {
	return false, nil
}
func (*topicProjectionRepo) AddCredit(context.Context, domain.LedgerEntry) error { return nil }
func (*topicProjectionRepo) AdjustCredit(context.Context, domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	return domain.LedgerEntry{}, domain.Balance{}, false, nil
}
func (*topicProjectionRepo) DebitCredit(context.Context, domain.LedgerEntry) (domain.LedgerEntry, domain.Balance, bool, error) {
	return domain.LedgerEntry{}, domain.Balance{}, false, nil
}
func (*topicProjectionRepo) ReserveCredit(context.Context, domain.CreditReservation, domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	return domain.CreditReservation{}, domain.Balance{}, false, nil
}
func (*topicProjectionRepo) ReleaseCredit(context.Context, domain.CreditReservation, domain.LedgerEntry) (domain.CreditReservation, domain.Balance, bool, error) {
	return domain.CreditReservation{}, domain.Balance{}, false, nil
}
func (*topicProjectionRepo) GetCheckIn(context.Context, int64) (domain.CheckIn, error) {
	return domain.CheckIn{}, nil
}
func (*topicProjectionRepo) GetTaskClaimSnapshot(context.Context, int64, []domain.TaskClaimLedgerLookup) (domain.TaskClaimSnapshot, error) {
	return domain.TaskClaimSnapshot{}, nil
}
func (*topicProjectionRepo) RecordCheckIn(context.Context, domain.CheckIn, domain.LedgerEntry) (domain.CheckIn, domain.LedgerEntry, domain.Balance, bool, error) {
	return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, nil
}
func (*topicProjectionRepo) SettleCreditReservation(context.Context, domain.CreditReservation, domain.LedgerEntry) error {
	return nil
}
func (*topicProjectionRepo) ReverseQAAcceptance(context.Context, domain.QAAcceptanceReversal) (bool, error) {
	return false, nil
}
func (*topicProjectionRepo) TransferCredit(context.Context, domain.LedgerEntry, domain.LedgerEntry) error {
	return nil
}
func (*topicProjectionRepo) SavePendingArticleCredit(context.Context, string, string, int64, int64, int64, string, int64, time.Time) error {
	return nil
}
func (*topicProjectionRepo) FlushPendingArticleCredits(context.Context, domain.ArticleRef) error {
	return nil
}
func (*topicProjectionRepo) GetBalance(context.Context, int64) (domain.Balance, error) {
	return domain.Balance{}, nil
}
func (*topicProjectionRepo) ListLedger(context.Context, int64, int32, int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	return nil, 0, domain.Balance{}, nil
}
func (*topicProjectionRepo) ListLeaderboard(context.Context, int32) ([]domain.LeaderboardEntry, error) {
	return nil, nil
}
func (*topicProjectionRepo) GetLedgerEntry(context.Context, int64, string, string) (domain.LedgerEntry, bool, error) {
	return domain.LedgerEntry{}, false, nil
}

var _ domain.Repository = (*topicProjectionRepo)(nil)
