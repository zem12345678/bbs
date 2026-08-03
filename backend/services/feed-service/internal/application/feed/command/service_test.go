package command

import (
	"context"
	"errors"
	"testing"

	domain "feed-service/internal/domain/feed"

	"github.com/stretchr/testify/require"
)

func TestServicePurgeAccountFeedValidatesIdentifiers(t *testing.T) {
	testCases := []struct {
		name          string
		userID        int64
		deletionJobID int64
		policyVersion int32
	}{
		{name: "missing user", userID: 0, deletionJobID: 2, policyVersion: 3},
		{name: "missing deletion job", userID: 1, deletionJobID: 0, policyVersion: 3},
		{name: "missing policy version", userID: 1, deletionJobID: 2, policyVersion: 0},
		{name: "negative user", userID: -1, deletionJobID: 2, policyVersion: 3},
		{name: "negative deletion job", userID: 1, deletionJobID: -2, policyVersion: 3},
		{name: "negative policy version", userID: 1, deletionJobID: 2, policyVersion: -3},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &purgeRepositoryStub{}
			service := NewService(repo)

			count, err := service.PurgeAccountFeed(context.Background(), testCase.userID, testCase.deletionJobID, testCase.policyVersion)

			require.Error(t, err)
			require.Zero(t, count)
			require.Zero(t, repo.calls)
		})
	}
}

func TestServicePurgeAccountFeedDelegatesToRepository(t *testing.T) {
	expectedErr := errors.New("purge failed")
	repo := &purgeRepositoryStub{count: 7, err: expectedErr}
	service := NewService(repo)
	ctx := context.Background()

	count, err := service.PurgeAccountFeed(ctx, 42, 1001, 4)

	require.Equal(t, int64(7), count)
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, int64(42), repo.userID)
	require.Equal(t, ctx, repo.ctx)
}

type purgeRepositoryStub struct {
	domain.Repository
	ctx    context.Context
	userID int64
	count  int64
	err    error
	calls  int
}

func (r *purgeRepositoryStub) PurgeByAuthor(ctx context.Context, userID int64) (int64, error) {
	r.ctx = ctx
	r.userID = userID
	r.calls++
	return r.count, r.err
}
