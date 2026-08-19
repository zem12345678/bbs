package command

import (
	"context"
	"testing"

	domain "reaction-service/internal/domain/reaction"

	"github.com/stretchr/testify/require"
)

func TestPinServiceRestrictsPinsToContent(t *testing.T) {
	repo := &pinRepositoryStub{}
	service := NewService(nil, nil, nil, nil, repo, nil, nil, nil)

	_, err := service.Pin(context.Background(), domain.EntityRef{Type: domain.EntityComment, ID: 7}, 42)

	require.ErrorIs(t, err, domain.ErrInvalidPinnedEntityType)
	require.False(t, repo.pinCalled)
}

func TestPinServiceDelegatesPinAndIdempotentUnpin(t *testing.T) {
	repo := &pinRepositoryStub{pinCount: 1, unpinCount: 0}
	service := NewService(nil, nil, nil, nil, repo, nil, nil, nil)
	ref := domain.EntityRef{Type: domain.EntityArticle, ID: 7}

	pinned, err := service.Pin(context.Background(), ref, 42)
	require.NoError(t, err)
	require.True(t, pinned.Changed)
	require.Equal(t, int64(1), pinned.Count)

	unpinned, err := service.Unpin(context.Background(), ref, 42)
	require.NoError(t, err)
	require.False(t, unpinned.Changed)
	require.Equal(t, int64(0), unpinned.Count)
	require.True(t, repo.pinCalled)
	require.True(t, repo.unpinCalled)
}

type pinRepositoryStub struct {
	domain.PinRepository
	pinCount    int64
	unpinCount  int64
	pinCalled   bool
	unpinCalled bool
}

func (r *pinRepositoryStub) Pin(_ context.Context, _ domain.EntityRef, _ int64) (int64, bool, error) {
	r.pinCalled = true
	return r.pinCount, true, nil
}

func (r *pinRepositoryStub) Unpin(_ context.Context, _ domain.EntityRef, _ int64) (int64, bool, error) {
	r.unpinCalled = true
	return r.unpinCount, false, nil
}
