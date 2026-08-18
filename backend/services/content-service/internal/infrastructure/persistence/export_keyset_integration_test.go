package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	articleDomain "content-service/internal/domain/article"
	topicDomain "content-service/internal/domain/topic"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestContentExportKeysetsPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CONTENT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("BBS_CONTENT_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })

	ctx := context.Background()
	baseID := time.Now().UnixNano()
	ownerID := baseID + 500
	otherOwnerID := ownerID + 1
	categoryID := baseID + 700
	now := time.Now().UTC()
	require.NoError(t, tx.Create(&categoryPO{
		ID: categoryID, Slug: fmt.Sprintf("export-keyset-%d", baseID), Name: "Export keyset", Status: 2,
		CreatedAt: now, UpdatedAt: now,
	}).Error)

	articles := make([]articlePO, 0, 102)
	for i := int64(1); i <= 101; i++ {
		articles = append(articles, articlePO{
			ID: baseID + i, Slug: fmt.Sprintf("export-article-%d-%d", baseID, i), Title: "article", Body: "body", Tags: "[]",
			AuthorID: ownerID, Status: int32(articleDomain.Status(i%4 + 1)), CreatedAt: now, UpdatedAt: now,
		})
	}
	articles = append(articles, articlePO{
		ID: baseID + 102, Slug: fmt.Sprintf("export-article-other-%d", baseID), Title: "other", Body: "body", Tags: "[]",
		AuthorID: otherOwnerID, Status: int32(articleDomain.StatusPublished), CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, tx.CreateInBatches(articles, 50).Error)

	articleRepo := NewRepo(tx)
	firstArticles, total, err := articleRepo.ListAfterID(ctx, 0, ownerID, 0, 100)
	require.NoError(t, err)
	require.EqualValues(t, 101, total)
	require.Len(t, firstArticles, 100)
	require.Equal(t, baseID+1, firstArticles[0].ID)
	require.Equal(t, baseID+100, firstArticles[99].ID)
	require.NoError(t, tx.Delete(&articlePO{}, baseID+1).Error)
	secondArticles, _, err := articleRepo.ListAfterID(ctx, 0, ownerID, firstArticles[99].ID, 100)
	require.NoError(t, err)
	require.Len(t, secondArticles, 1)
	require.Equal(t, baseID+101, secondArticles[0].ID)
	legacyArticles, _, err := articleRepo.List(ctx, 0, "", ownerID, "", 10, 1)
	require.NoError(t, err)
	require.Len(t, legacyArticles, 10)

	topics := []topicPO{
		{ID: baseID + 201, Slug: fmt.Sprintf("export-topic-%d-1", baseID), Type: string(topicDomain.TypeTopic), Title: "one", Body: "body", Tags: "[]", AuthorID: ownerID, CategoryID: categoryID, Status: int32(topicDomain.StatusDraft), CreatedAt: now, UpdatedAt: now},
		{ID: baseID + 202, Slug: fmt.Sprintf("export-topic-%d-2", baseID), Type: string(topicDomain.TypeTopic), Title: "two", Body: "body", Tags: "[]", AuthorID: ownerID, CategoryID: categoryID, Status: int32(topicDomain.StatusArchived), CreatedAt: now, UpdatedAt: now},
		{ID: baseID + 203, Slug: fmt.Sprintf("export-topic-%d-other", baseID), Type: string(topicDomain.TypeTopic), Title: "other", Body: "body", Tags: "[]", AuthorID: otherOwnerID, CategoryID: categoryID, Status: int32(topicDomain.StatusPublished), CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, tx.Create(&topics).Error)
	topicRepo := NewTopicRepo(tx)
	firstTopics, total, err := topicRepo.ListAfterID(ctx, 0, ownerID, 0, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, firstTopics, 1)
	require.Equal(t, baseID+201, firstTopics[0].ID)
	secondTopics, _, err := topicRepo.ListAfterID(ctx, 0, ownerID, firstTopics[0].ID, 100)
	require.NoError(t, err)
	require.Len(t, secondTopics, 1)
	require.Equal(t, baseID+202, secondTopics[0].ID)
}
