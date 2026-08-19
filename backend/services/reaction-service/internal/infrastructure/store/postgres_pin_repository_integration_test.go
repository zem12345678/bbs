package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresPinRepositoryIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_REACTION_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_REACTION_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	repo := NewPostgresPinRepository(db)
	require.NoError(t, repo.EnsureSchema(ctx))

	userID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.WithContext(context.Background()).Where("user_id = ?", userID).Delete(&pinPO{}).Error
		_ = db.WithContext(context.Background()).Where("user_id = ?", userID).Delete(&reactionErasedUserPO{}).Error
	})
	for index := int64(1); index <= domain.MaxPinsPerUser; index++ {
		count, changed, err := repo.Pin(ctx, domain.EntityRef{Type: domain.EntityArticle, ID: userID + index}, userID)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, index, count)
	}

	_, _, err = repo.Pin(ctx, domain.EntityRef{Type: domain.EntityTopic, ID: userID + 10}, userID)
	require.ErrorIs(t, err, domain.ErrPinLimitExceeded)
	_, _, err = repo.Pin(ctx, domain.EntityRef{Type: domain.EntityArticle, ID: userID + 1}, userID)
	require.ErrorIs(t, err, domain.ErrAlreadyPinned)

	items, total, err := repo.ListPins(ctx, userID, 100, 0)
	require.NoError(t, err)
	require.Equal(t, int64(domain.MaxPinsPerUser), total)
	require.Len(t, items, domain.MaxPinsPerUser)
	require.Greater(t, items[0].ID, items[len(items)-1].ID)

	count, changed, err := repo.Unpin(ctx, domain.EntityRef{Type: domain.EntityArticle, ID: userID + 1}, userID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, int64(domain.MaxPinsPerUser-1), count)
	_, changed, err = repo.Unpin(ctx, domain.EntityRef{Type: domain.EntityArticle, ID: userID + 1}, userID)
	require.NoError(t, err)
	require.False(t, changed)
}
