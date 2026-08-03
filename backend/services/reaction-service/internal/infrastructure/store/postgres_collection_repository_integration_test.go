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

func TestPostgresCollectionRepositoryIntegration(t *testing.T) {
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
	repo := NewPostgresCollectionRepository(db)
	require.NoError(t, repo.EnsureSchema(ctx))

	ownerID := time.Now().UnixNano()
	foreignOwnerID := ownerID + 1
	t.Cleanup(func() {
		_ = db.WithContext(context.Background()).Where("user_id IN ?", []int64{ownerID, foreignOwnerID}).Delete(&collectionPO{}).Error
	})

	reading := &domain.Collection{UserID: ownerID, Name: "Reading", Description: "Later", IsPublic: false}
	require.NoError(t, repo.CreateCollection(ctx, reading))
	require.Positive(t, reading.ID)
	require.Equal(t, int64(0), reading.ItemCount)

	duplicate := &domain.Collection{UserID: ownerID, Name: "reading"}
	require.ErrorIs(t, repo.CreateCollection(ctx, duplicate), domain.ErrCollectionNameExists)

	entity := domain.EntityRef{Type: domain.EntityArticle, ID: 9_007_199_254_740_999}
	changed, err := repo.AddCollectionItem(ctx, ownerID, reading.ID, entity)
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = repo.AddCollectionItem(ctx, ownerID, reading.ID, entity)
	require.NoError(t, err)
	require.False(t, changed)

	archive := &domain.Collection{UserID: ownerID, Name: "Archive"}
	require.NoError(t, repo.CreateCollection(ctx, archive))
	changed, err = repo.AddCollectionItem(ctx, ownerID, archive.ID, entity)
	require.NoError(t, err)
	require.True(t, changed)
	var legacyFavorites int64
	require.NoError(t, db.WithContext(ctx).Model(&favoritePO{}).
		Where("user_id = ? AND entity_type = ? AND entity_id = ? AND deleted_at IS NULL", ownerID, string(entity.Type), entity.ID).
		Count(&legacyFavorites).Error)
	require.Zero(t, legacyFavorites)
	changed, err = repo.RemoveCollectionItem(ctx, ownerID, archive.ID, entity)
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = repo.RemoveCollectionItem(ctx, ownerID, archive.ID, entity)
	require.NoError(t, err)
	require.False(t, changed)

	collections, total, err := repo.ListCollections(ctx, ownerID, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, collections, 2)
	var readingCount int64
	for _, collection := range collections {
		if collection.ID == reading.ID {
			readingCount = collection.ItemCount
		}
	}
	require.Equal(t, int64(1), readingCount)

	items, total, err := repo.ListCollectionItems(ctx, ownerID, reading.ID, domain.EntityArticle, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, entity, items[0].Entity)

	_, _, err = repo.ListCollectionItems(ctx, foreignOwnerID, reading.ID, "", 20, 0)
	require.ErrorIs(t, err, domain.ErrCollectionNotFound)
	_, err = repo.AddCollectionItem(ctx, foreignOwnerID, reading.ID, entity)
	require.ErrorIs(t, err, domain.ErrCollectionNotFound)
	_, err = repo.RemoveCollectionItem(ctx, foreignOwnerID, reading.ID, entity)
	require.ErrorIs(t, err, domain.ErrCollectionNotFound)
	require.ErrorIs(t, repo.DeleteCollection(ctx, foreignOwnerID, reading.ID), domain.ErrCollectionNotFound)

	updated, err := repo.UpdateCollection(ctx, ownerID, reading.ID, "  Research  ", "  References  ", true)
	require.NoError(t, err)
	require.Equal(t, "Research", updated.Name)
	require.Equal(t, "References", updated.Description)
	require.True(t, updated.IsPublic)
	require.Equal(t, int64(1), updated.ItemCount)

	require.NoError(t, repo.DeleteCollection(ctx, ownerID, reading.ID))
	var remainingItems int64
	require.NoError(t, db.WithContext(ctx).Model(&collectionItemPO{}).Where("collection_id = ?", reading.ID).Count(&remainingItems).Error)
	require.Zero(t, remainingItems)
	require.ErrorIs(t, repo.DeleteCollection(ctx, ownerID, reading.ID), domain.ErrCollectionNotFound)
	require.NoError(t, repo.DeleteCollection(ctx, ownerID, archive.ID))
}
