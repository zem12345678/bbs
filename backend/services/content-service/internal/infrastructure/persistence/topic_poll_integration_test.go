package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	topicDomain "content-service/internal/domain/topic"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTopicPollPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CONTENT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("BBS_CONTENT_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	repo := NewTopicRepo(db)
	ctx := context.Background()
	baseID := time.Now().UnixNano()
	defer db.WithContext(ctx).Where("id IN ?", []int64{baseID, baseID + 1, baseID + 2}).Delete(&topicPO{})

	published := integrationPollTopic(t, baseID, "published", topicDomain.StatusPublished, time.Now().Add(time.Hour))
	require.NoError(t, repo.CreateTopic(ctx, published))
	published.Title = "published updated"
	require.NoError(t, repo.UpdateTopic(ctx, published))
	preservedPoll, err := repo.FindTopicPoll(ctx, published.ID, 0)
	require.NoError(t, err)
	require.Len(t, preservedPoll.Choices, 2)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, voteErr := repo.VoteTopicPoll(ctx, published.ID, 7001, []int32{1}, time.Now())
			errs <- voteErr
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var successCount, duplicateCount int
	for voteErr := range errs {
		switch {
		case voteErr == nil:
			successCount++
		case errors.Is(voteErr, topicDomain.ErrPollAlreadyVoted):
			duplicateCount++
		default:
			t.Fatalf("unexpected concurrent vote error: %v", voteErr)
		}
	}
	require.Equal(t, 1, successCount)
	require.Equal(t, 1, duplicateCount)

	poll, err := repo.FindTopicPoll(ctx, published.ID, 7001)
	require.NoError(t, err)
	require.EqualValues(t, 1, poll.TotalVoters)
	require.Equal(t, []int32{1}, poll.MyChoices)
	require.EqualValues(t, 0, poll.Choices[0].Votes)
	require.EqualValues(t, 1, poll.Choices[1].Votes)

	published.Title = "must roll back"
	err = repo.UpdateTopicWithPoll(ctx, published, &topicDomain.PollInput{Enabled: false})
	require.ErrorIs(t, err, topicDomain.ErrPollLocked)
	stored, err := repo.FindTopicByID(ctx, published.ID)
	require.NoError(t, err)
	require.Equal(t, "published updated", stored.Title)

	expired := integrationPollTopic(t, baseID+1, "expired", topicDomain.StatusPublished, time.Now().Add(time.Hour))
	expired.Poll.ExpiresAt = timePointerForPollTest(time.Now().Add(-time.Minute))
	require.NoError(t, repo.CreateTopic(ctx, expired))
	_, err = repo.VoteTopicPoll(ctx, expired.ID, 7002, []int32{0}, time.Now())
	require.ErrorIs(t, err, topicDomain.ErrPollExpired)

	draft := integrationPollTopic(t, baseID+2, "draft", topicDomain.StatusDraft, time.Now().Add(time.Hour))
	require.NoError(t, repo.CreateTopic(ctx, draft))
	_, err = repo.VoteTopicPoll(ctx, draft.ID, 7003, []int32{0}, time.Now())
	require.ErrorIs(t, err, topicDomain.ErrNotPublished)
}

func integrationPollTopic(t *testing.T, id int64, title string, status topicDomain.Status, expiresAt time.Time) *topicDomain.Topic {
	t.Helper()
	topic, err := topicDomain.New(id, topicDomain.CreateCmd{
		Slug:     fmt.Sprintf("poll-integration-%d", id),
		Title:    title,
		Body:     "body",
		AuthorID: 6001,
		Poll: &topicDomain.PollInput{
			Enabled:   true,
			Multiple:  false,
			Choices:   []string{"first", "second"},
			ExpiresAt: &expiresAt,
		},
	})
	require.NoError(t, err)
	topic.Status = status
	if status == topicDomain.StatusPublished {
		publishedAt := time.Now()
		topic.PublishedAt = &publishedAt
	}
	return topic
}

func timePointerForPollTest(value time.Time) *time.Time { return &value }
