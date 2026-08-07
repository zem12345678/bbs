package grpc

import (
	"context"
	"testing"
	"time"

	pb "content-service/api/proto/contentpb"
	channelcommand "content-service/internal/application/channel/command"
	channelquery "content-service/internal/application/channel/query"
	categoryDomain "content-service/internal/domain/category"
	channelDomain "content-service/internal/domain/channel"
	topicDomain "content-service/internal/domain/topic"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestChannelHandlerCreatesAndMapsChannel(t *testing.T) {
	repo := &handlerChannelRepo{}
	handler := channelTestHandler(repo)

	response, err := handler.CreateChannel(context.Background(), &pb.CreateChannelRequest{
		OwnerId: 42, CategoryId: 7, Name: "Engineering", Description: "Build notes", Color: "#123abc",
	})

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.EqualValues(t, 7001, repo.created.ID)
	require.EqualValues(t, 42, repo.created.OwnerID)
	require.EqualValues(t, 7, repo.created.CategoryID)
	require.Equal(t, "Engineering", response.GetChannel().GetName())
	require.Equal(t, "#123abc", response.GetChannel().GetColor())
}

func TestChannelHandlerListsWithIndependentRelationAndViewerIDs(t *testing.T) {
	lastPostedAt := time.UnixMilli(1_720_000_000_000)
	repo := &handlerChannelRepo{
		listed: []*channelDomain.Channel{{
			ID: 7001, OwnerID: 42, Name: "Engineering", FollowersCount: 3, TopicsCount: 5,
			LastPostedAt: &lastPostedAt, ViewerFollowing: true, ViewerFavorited: true,
		}},
		total: 1,
	}
	handler := channelTestHandler(repo)

	response, err := handler.ListChannels(context.Background(), &pb.ListChannelsRequest{
		Query: "engine", CategoryId: 7, Uncategorized: true, OwnerId: 42,
		FollowerUserId: 50, FavoritedUserId: 60, ViewerUserId: 70,
		Featured: true, IncludeArchived: true, Limit: 8, Offset: 3,
	})

	require.NoError(t, err)
	require.Equal(t, channelDomain.ListFilter{
		Query: "engine", CategoryID: 7, Uncategorized: true, OwnerID: 42,
		FollowerUserID: 50, FavoritedUserID: 60, ViewerID: 70,
		Featured: true, IncludeArchived: true, Limit: 8, Offset: 3,
	}, repo.filter)
	require.EqualValues(t, 1, response.GetTotal())
	require.EqualValues(t, lastPostedAt.UnixMilli(), response.GetItems()[0].GetLastPostedAt())
	require.True(t, response.GetItems()[0].GetIsFollowing())
	require.True(t, response.GetItems()[0].GetIsFavorited())
}

func TestChannelHandlerMapsActionsAndArchivedError(t *testing.T) {
	repo := &handlerChannelRepo{}
	handler := channelTestHandler(repo)

	response, err := handler.FavoriteChannel(context.Background(), &pb.ChannelUserRequest{ChannelId: 7001, UserId: 42})
	require.NoError(t, err)
	require.True(t, response.GetSuccess())
	require.Equal(t, "favorite", repo.action)
	require.EqualValues(t, 7001, repo.actionChannelID)
	require.EqualValues(t, 42, repo.actionUserID)

	repo.actionErr = channelDomain.ErrArchived
	_, err = handler.FollowChannel(context.Background(), &pb.ChannelUserRequest{ChannelId: 7001, UserId: 42})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestChannelHandlerMapsCategoryAggregatesAndTopicChannel(t *testing.T) {
	lastPostedAt := time.UnixMilli(1_720_000_000_000)
	repo := &handlerChannelRepo{categories: []channelDomain.CategoryAggregate{{
		CategoryID: 7, Slug: "engineering", Name: "Engineering", ChannelCount: 2,
		FollowersCount: 3, TopicsCount: 5, LastPostedAt: &lastPostedAt,
	}}}
	handler := channelTestHandler(repo)

	response, err := handler.ListChannelCategories(context.Background(), &pb.ListChannelCategoriesRequest{})
	require.NoError(t, err)
	require.Len(t, response.GetItems(), 1)
	require.EqualValues(t, 2, response.GetItems()[0].GetChannelsCount())
	require.EqualValues(t, lastPostedAt.UnixMilli(), response.GetItems()[0].GetLastPostedAt())
	require.EqualValues(t, 7001, toPbTopic(&topicDomain.Topic{ChannelID: 7001}).GetChannelId())
}

func channelTestHandler(repo channelDomain.Repository) *Handler {
	commandService := channelcommand.NewService(repo, handlerChannelIDGenerator(7001), handlerCategoryReader{})
	queryService := channelquery.NewService(repo)
	return NewHandlerWithChannels(nil, nil, nil, nil, nil, nil, nil, commandService, queryService)
}

type handlerChannelIDGenerator int64

func (g handlerChannelIDGenerator) Generate() int64 { return int64(g) }

type handlerCategoryReader struct{}

func (handlerCategoryReader) FindCategoryByID(context.Context, int64) (*categoryDomain.Category, error) {
	return &categoryDomain.Category{ID: 7, Status: categoryDomain.StatusEnabled}, nil
}

type handlerChannelRepo struct {
	created         *channelDomain.Channel
	listed          []*channelDomain.Channel
	total           int64
	filter          channelDomain.ListFilter
	categories      []channelDomain.CategoryAggregate
	action          string
	actionChannelID int64
	actionUserID    int64
	actionErr       error
}

func (r *handlerChannelRepo) CreateChannel(_ context.Context, channel *channelDomain.Channel) error {
	r.created = channel
	return nil
}

func (r *handlerChannelRepo) UpdateChannel(context.Context, *channelDomain.Channel) error { return nil }
func (r *handlerChannelRepo) ArchiveChannel(context.Context, *channelDomain.Channel) error {
	return nil
}

func (r *handlerChannelRepo) FindChannelByID(context.Context, int64, int64, bool) (*channelDomain.Channel, error) {
	return &channelDomain.Channel{ID: 7001, OwnerID: 42, Name: "Engineering"}, nil
}

func (r *handlerChannelRepo) ListChannels(_ context.Context, filter channelDomain.ListFilter) ([]*channelDomain.Channel, int64, error) {
	r.filter = filter
	return r.listed, r.total, nil
}

func (r *handlerChannelRepo) FollowChannel(_ context.Context, channelID, userID int64) error {
	return r.recordAction("follow", channelID, userID)
}

func (r *handlerChannelRepo) UnfollowChannel(_ context.Context, channelID, userID int64) error {
	return r.recordAction("unfollow", channelID, userID)
}

func (r *handlerChannelRepo) FavoriteChannel(_ context.Context, channelID, userID int64) error {
	return r.recordAction("favorite", channelID, userID)
}

func (r *handlerChannelRepo) UnfavoriteChannel(_ context.Context, channelID, userID int64) error {
	return r.recordAction("unfavorite", channelID, userID)
}

func (r *handlerChannelRepo) ListChannelCategoryAggregates(context.Context, bool) ([]channelDomain.CategoryAggregate, error) {
	return r.categories, nil
}

func (r *handlerChannelRepo) recordAction(action string, channelID, userID int64) error {
	r.action = action
	r.actionChannelID = channelID
	r.actionUserID = userID
	return r.actionErr
}
