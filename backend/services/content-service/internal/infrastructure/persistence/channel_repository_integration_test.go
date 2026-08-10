package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	channelDomain "content-service/internal/domain/channel"
	topicDomain "content-service/internal/domain/topic"
	"content-service/migrations"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestChannelPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CONTENT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("BBS_CONTENT_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, filename := range []string{"0015_add_account_erasure.sql", "0016_create_channels.sql", "0017_add_channel_featured.sql"} {
		migration, readErr := migrations.Files.ReadFile(filename)
		require.NoError(t, readErr)
		require.NoError(t, db.Exec(string(migration)).Error)
	}

	ctx := context.Background()
	seed := time.Now().UnixNano()
	ownerID := seed
	viewerID := seed + 1
	channelID := seed + 100
	uncategorizedID := seed + 101
	topicID := seed + 200
	rejectedTopicID := seed + 201
	draftTopicID := seed + 202

	t.Cleanup(func() {
		cleanup := db.WithContext(context.Background())
		_ = cleanup.Where("id IN ?", []int64{topicID, rejectedTopicID, draftTopicID}).Delete(&topicPO{}).Error
		_ = cleanup.Where("id IN ?", []int64{channelID, uncategorizedID}).Delete(&channelPO{}).Error
		_ = cleanup.Where("user_id IN ?", []int64{ownerID, viewerID}).Delete(&contentErasedUserPO{}).Error
	})

	channels := NewChannelRepo(db)
	topics := NewTopicRepo(db)
	channel, err := channelDomain.New(channelID, channelDomain.CreateCmd{
		OwnerID: ownerID, CategoryID: 1, Name: "Engineering", Description: "Build things", Color: "#112233",
	})
	require.NoError(t, err)
	require.NoError(t, channels.CreateChannel(ctx, channel))
	uncategorized, err := channelDomain.New(uncategorizedID, channelDomain.CreateCmd{OwnerID: ownerID, Name: "General"})
	require.NoError(t, err)
	require.NoError(t, channels.CreateChannel(ctx, uncategorized))

	for range 2 {
		require.NoError(t, channels.FollowChannel(ctx, channelID, viewerID))
		require.NoError(t, channels.FavoriteChannel(ctx, channelID, viewerID))
	}
	published := integrationChannelTopic(t, topicID, ownerID, channelID, topicDomain.StatusPublished)
	require.NoError(t, topics.CreateTopic(ctx, published))
	draft := integrationChannelTopic(t, draftTopicID, ownerID, channelID, topicDomain.StatusDraft)
	require.NoError(t, topics.CreateTopic(ctx, draft))

	stored, err := channels.FindChannelByID(ctx, channelID, viewerID, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.FollowersCount)
	require.EqualValues(t, 1, stored.TopicsCount)
	require.NotNil(t, stored.LastPostedAt)
	require.True(t, stored.ViewerFollowing)
	require.True(t, stored.ViewerFavorited)

	listed, total, err := channels.ListChannels(ctx, channelDomain.ListFilter{
		Query: "build", CategoryID: 1, FollowerUserID: viewerID, FavoritedUserID: viewerID, ViewerID: viewerID,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, listed, 1)
	require.Equal(t, channelID, listed[0].ID)

	uncategorizedList, total, err := channels.ListChannels(ctx, channelDomain.ListFilter{Uncategorized: true, OwnerID: ownerID})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, uncategorizedList, 1)
	require.Equal(t, uncategorizedID, uncategorizedList[0].ID)

	require.NoError(t, channel.SetFeatured(true))
	require.NoError(t, channels.SetChannelFeatured(ctx, channel))
	featuredList, total, err := channels.ListChannels(ctx, channelDomain.ListFilter{
		OwnerID: ownerID, Featured: true, IncludeArchived: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, featuredList, 1)
	require.Equal(t, channelID, featuredList[0].ID)
	require.True(t, featuredList[0].IsFeatured)

	require.NoError(t, channel.SetArchived(true))
	require.NoError(t, channels.SetChannelArchived(ctx, channel))
	archivedList, total, err := channels.ListChannels(ctx, channelDomain.ListFilter{
		OwnerID: ownerID, ArchivedStatus: channelDomain.ArchivedStatusArchived,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, archivedList, 1)
	require.Equal(t, channelID, archivedList[0].ID)
	require.False(t, archivedList[0].IsFeatured)

	staleFeatured := *channel
	staleFeatured.IsFeatured = true
	require.ErrorIs(t, channels.SetChannelFeatured(ctx, &staleFeatured), channelDomain.ErrArchived)

	require.NoError(t, channel.SetArchived(false))
	require.NoError(t, channels.SetChannelArchived(ctx, channel))
	restored, err := channels.FindChannelByID(ctx, channelID, viewerID, false)
	require.NoError(t, err)
	require.False(t, restored.IsArchived)
	require.False(t, restored.IsFeatured)
	activeList, _, err := channels.ListChannels(ctx, channelDomain.ListFilter{ArchivedStatus: channelDomain.ArchivedStatusActive, IncludeArchived: true})
	require.NoError(t, err)
	for _, active := range activeList {
		require.False(t, active.IsArchived)
	}

	filteredTopics, total, err := topics.ListTopics(ctx, topicDomain.StatusPublished, "", "", 0, 0, channelID, "", 20, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, filteredTopics, 1)
	require.Equal(t, channelID, filteredTopics[0].ChannelID)

	aggregates, err := channels.ListChannelCategoryAggregates(ctx, false)
	require.NoError(t, err)
	var categoryOne *channelDomain.CategoryAggregate
	for i := range aggregates {
		if aggregates[i].CategoryID == 1 {
			categoryOne = &aggregates[i]
			break
		}
	}
	require.NotNil(t, categoryOne)
	require.GreaterOrEqual(t, categoryOne.ChannelCount, int64(1))
	require.GreaterOrEqual(t, categoryOne.FollowersCount, int64(1))
	require.GreaterOrEqual(t, categoryOne.TopicsCount, int64(1))

	require.NoError(t, channel.Archive())
	require.NoError(t, channels.ArchiveChannel(ctx, channel))
	require.ErrorIs(t, channels.FollowChannel(ctx, channelID, ownerID), channelDomain.ErrArchived)
	require.ErrorIs(t, channels.FavoriteChannel(ctx, channelID, ownerID), channelDomain.ErrArchived)
	require.NoError(t, channels.UnfollowChannel(ctx, channelID, viewerID))
	require.NoError(t, channels.UnfavoriteChannel(ctx, channelID, viewerID))
	_, err = channels.FindChannelByID(ctx, channelID, viewerID, false)
	require.ErrorIs(t, err, channelDomain.ErrNotFound)
	_, err = channels.FindChannelByID(ctx, channelID, viewerID, true)
	require.NoError(t, err)

	rejected := integrationChannelTopic(t, rejectedTopicID, ownerID, channelID, topicDomain.StatusDraft)
	require.ErrorIs(t, topics.CreateTopic(ctx, rejected), topicDomain.ErrChannelArchived)
	published.Title = "updated after archive"
	require.ErrorIs(t, topics.UpdateTopic(ctx, published), topicDomain.ErrChannelArchived)
	publishAt := time.Now().UTC()
	require.ErrorIs(t, topics.UpdateTopicStatus(ctx, draft.ID, topicDomain.StatusPublished, &publishAt), topicDomain.ErrChannelArchived)
}

func integrationChannelTopic(t *testing.T, id, authorID, channelID int64, status topicDomain.Status) *topicDomain.Topic {
	t.Helper()
	topic, err := topicDomain.New(id, topicDomain.CreateCmd{
		Slug: fmt.Sprintf("channel-topic-%d", id), Title: "topic", Body: "body", AuthorID: authorID, ChannelID: channelID,
	})
	require.NoError(t, err)
	topic.Status = status
	if status == topicDomain.StatusPublished {
		publishedAt := time.Now().UTC()
		topic.PublishedAt = &publishedAt
	}
	return topic
}
