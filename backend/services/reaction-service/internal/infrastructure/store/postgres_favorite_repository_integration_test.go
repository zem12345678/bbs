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

func TestPostgresFavoriteRepositoryKeysetIntegration(t *testing.T) {
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
	repo := NewPostgresFavoriteRepository(db)
	require.NoError(t, repo.EnsureSchema(ctx))

	userID := time.Now().UnixNano()
	t.Cleanup(func() {
		_ = db.WithContext(context.Background()).Where("user_id = ?", userID).Delete(&favoritePO{}).Error
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	rows := []favoritePO{
		{UserID: userID, EntityType: "article", EntityID: 101, CreatedAt: now, UpdatedAt: now},
		{UserID: userID, EntityType: "topic", EntityID: 201, CreatedAt: now, UpdatedAt: now},
		{UserID: userID, EntityType: "article", EntityID: 102, CreatedAt: now, UpdatedAt: now},
		{UserID: userID, EntityType: "collection", EntityID: 301, CreatedAt: now, UpdatedAt: now},
		{UserID: userID, EntityType: "article", EntityID: 103, CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.WithContext(ctx).Create(&rows).Error)

	first, total, err := repo.ListFavoritesAfterID(ctx, userID, domain.EntityArticle, 0, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Equal(t, []int64{rows[0].ID, rows[2].ID}, favoriteRelationIDs(first))

	deletedAt := now.Add(time.Minute)
	require.NoError(t, db.WithContext(ctx).Model(&favoritePO{}).Where("id = ?", rows[0].ID).Update("deleted_at", &deletedAt).Error)
	second, total, err := repo.ListFavoritesAfterID(ctx, userID, domain.EntityArticle, rows[2].ID, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int64{rows[4].ID}, favoriteRelationIDs(second))

	legacy, _, err := repo.ListFavorites(ctx, userID, domain.EntityTopic, 20, 0)
	require.NoError(t, err)
	require.Equal(t, []int64{rows[1].ID}, favoriteRelationIDs(legacy))
}

func favoriteRelationIDs(rows []*domain.Favorite) []int64 {
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result
}
