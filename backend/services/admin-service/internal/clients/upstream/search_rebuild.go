package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"admin/api/proto/contentpb"
	"admin/api/proto/searchpb"
	"admin/api/proto/userpb"
	domain "admin/internal/domain/admin"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	searchRebuildLockTTL      = 15 * time.Minute
	searchRebuildHeartbeatTTL = time.Minute
	searchRebuildStatusTTL    = 7 * 24 * time.Hour
	searchRebuildPageSize     = int32(100)
	searchRebuildRPCTimeout   = 15 * time.Second

	searchRebuildStateQueued    = "queued"
	searchRebuildStateRunning   = "running"
	searchRebuildStateCompleted = "completed"
	searchRebuildStateFailed    = "failed"
)

const (
	searchRebuildLockKey      = "bbs:admin:search-rebuild:lock"
	searchRebuildHeartbeatKey = "bbs:admin:search-rebuild:heartbeat"
	searchRebuildStatusKey    = "bbs:admin:search-rebuild:status"
)

type searchRebuildStore interface {
	Claim(ctx context.Context, jobID string) (bool, error)
	Load(ctx context.Context) (domain.SearchRebuildStatus, bool, error)
	SaveOwned(ctx context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error)
	MarkFailedIfWorkerInactive(ctx context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error)
	Refresh(ctx context.Context, jobID string) (bool, error)
	Release(ctx context.Context, jobID string) error
}

// SearchRebuilder reindexes currently published content and active users without deleting unmatched index documents.
type SearchRebuilder struct {
	clients *Clients
	store   searchRebuildStore
}

func NewRedisSearchRebuilder(clients *Clients, client redis.Cmdable) *SearchRebuilder {
	return newSearchRebuilder(clients, newRedisSearchRebuildStore(client))
}

func newSearchRebuilder(clients *Clients, store searchRebuildStore) *SearchRebuilder {
	return &SearchRebuilder{clients: clients, store: store}
}

func (r *SearchRebuilder) StartSearchRebuild(ctx context.Context, requestedBy int64) (domain.SearchRebuildStatus, error) {
	if r == nil || r.clients == nil || r.clients.content == nil || r.clients.search == nil || r.clients.user == nil || r.store == nil {
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildUnavailable
	}
	if _, err := r.recoverInactiveJob(ctx); err != nil {
		return domain.SearchRebuildStatus{}, err
	}
	jobID := uuid.NewString()
	claimed, err := r.store.Claim(ctx, jobID)
	if err != nil {
		return domain.SearchRebuildStatus{}, searchRebuildUnavailable(err)
	}
	if !claimed {
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildInProgress
	}
	now := time.Now().UnixMilli()
	status := domain.SearchRebuildStatus{
		JobID:       jobID,
		State:       searchRebuildStateQueued,
		RequestedBy: requestedBy,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	saved, err := r.store.SaveOwned(ctx, jobID, status)
	if err != nil {
		_ = r.store.Release(context.Background(), jobID)
		return domain.SearchRebuildStatus{}, searchRebuildUnavailable(err)
	}
	if !saved {
		_ = r.store.Release(context.Background(), jobID)
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildInProgress
	}
	go r.run(jobID, status)
	return status, nil
}

func (r *SearchRebuilder) GetSearchRebuildStatus(ctx context.Context) (domain.SearchRebuildStatus, error) {
	if r == nil || r.store == nil {
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildUnavailable
	}
	return r.recoverInactiveJob(ctx)
}

func (r *SearchRebuilder) recoverInactiveJob(ctx context.Context) (domain.SearchRebuildStatus, error) {
	status, found, err := r.store.Load(ctx)
	if err != nil {
		return domain.SearchRebuildStatus{}, searchRebuildUnavailable(err)
	}
	if !found {
		return domain.SearchRebuildStatus{State: "idle"}, nil
	}
	if status.State == searchRebuildStateQueued || status.State == searchRebuildStateRunning {
		stale := status
		stale.State = searchRebuildStateFailed
		stale.Error = "search rebuild lease expired; worker may have stopped"
		stale.UpdatedAt = time.Now().UnixMilli()
		stale.CompletedAt = stale.UpdatedAt
		updated, err := r.store.MarkFailedIfWorkerInactive(ctx, status.JobID, stale)
		if err != nil {
			return domain.SearchRebuildStatus{}, searchRebuildUnavailable(err)
		}
		if updated {
			return stale, nil
		}
		latest, found, err := r.store.Load(ctx)
		if err != nil {
			return domain.SearchRebuildStatus{}, searchRebuildUnavailable(err)
		}
		if found {
			return latest, nil
		}
		return domain.SearchRebuildStatus{State: "idle"}, nil
	}
	return status, nil
}

func (r *SearchRebuilder) run(jobID string, status domain.SearchRebuildStatus) {
	ctx := context.Background()
	defer func() { _ = r.store.Release(context.Background(), jobID) }()

	status.State = searchRebuildStateRunning
	status.UpdatedAt = time.Now().UnixMilli()
	if err := r.save(ctx, jobID, status); err != nil {
		r.persistFailed(ctx, jobID, &status, err)
		return
	}

	err := r.rebuild(ctx, jobID, &status)
	status.UpdatedAt = time.Now().UnixMilli()
	status.CompletedAt = status.UpdatedAt
	if err != nil {
		status.State = searchRebuildStateFailed
		status.Error = truncateSearchRebuildError(err.Error())
	} else {
		status.State = searchRebuildStateCompleted
		status.Error = ""
	}
	if err := r.save(ctx, jobID, status); err != nil {
		r.persistFailed(ctx, jobID, &status, err)
	}
}

func (r *SearchRebuilder) rebuild(ctx context.Context, jobID string, status *domain.SearchRebuildStatus) error {
	if err := r.refresh(ctx, jobID); err != nil {
		return err
	}
	if err := r.ensureArticleIndex(ctx); err != nil {
		return err
	}
	if err := r.ensureTopicIndex(ctx); err != nil {
		return err
	}
	if err := r.ensureUserIndex(ctx); err != nil {
		return err
	}
	if err := r.indexArticles(ctx, jobID, status); err != nil {
		return err
	}
	if err := r.indexTopics(ctx, jobID, status); err != nil {
		return err
	}
	return r.indexUsers(ctx, jobID, status)
}

func (r *SearchRebuilder) ensureArticleIndex(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
	defer cancel()
	if _, err := r.clients.search.EnsureArticleIndex(callCtx, &searchpb.EnsureArticleIndexRequest{}); err != nil {
		return fmt.Errorf("ensure article index: %w", err)
	}
	return nil
}

func (r *SearchRebuilder) ensureTopicIndex(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
	defer cancel()
	if _, err := r.clients.search.EnsureTopicIndex(callCtx, &searchpb.EnsureTopicIndexRequest{}); err != nil {
		return fmt.Errorf("ensure topic index: %w", err)
	}
	return nil
}

func (r *SearchRebuilder) ensureUserIndex(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
	defer cancel()
	if _, err := r.clients.search.EnsureUserIndex(callCtx, &searchpb.EnsureUserIndexRequest{}); err != nil {
		return fmt.Errorf("ensure user index: %w", err)
	}
	return nil
}

func (r *SearchRebuilder) indexArticles(ctx context.Context, jobID string, status *domain.SearchRebuildStatus) error {
	for offset := int32(0); ; {
		if err := r.refresh(ctx, jobID); err != nil {
			return err
		}
		callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
		page, err := r.clients.content.ListArticles(callCtx, &contentpb.ListArticlesRequest{
			Status: 2,
			Limit:  searchRebuildPageSize,
			Offset: offset,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("list published articles: %w", err)
		}
		status.ArticleTotal = page.GetTotal()
		for _, article := range page.GetItems() {
			if err := r.refresh(ctx, jobID); err != nil {
				return err
			}
			callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
			_, err := r.clients.search.ReindexArticle(callCtx, &searchpb.IndexArticleRequest{Article: searchArticleDocument(article)})
			cancel()
			if err != nil {
				return fmt.Errorf("index article %d: %w", article.GetId(), err)
			}
			status.ArticleIndexed++
		}
		if err := r.save(ctx, jobID, *status); err != nil {
			return err
		}
		count := int32(len(page.GetItems()))
		if count == 0 || int64(offset)+int64(count) >= page.GetTotal() {
			return nil
		}
		offset += count
	}
}

func (r *SearchRebuilder) indexTopics(ctx context.Context, jobID string, status *domain.SearchRebuildStatus) error {
	for offset := int32(0); ; {
		if err := r.refresh(ctx, jobID); err != nil {
			return err
		}
		callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
		page, err := r.clients.content.ListTopics(callCtx, &contentpb.ListTopicsRequest{
			Status: 2,
			Limit:  searchRebuildPageSize,
			Offset: offset,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("list published topics: %w", err)
		}
		status.TopicTotal = page.GetTotal()
		for _, topic := range page.GetItems() {
			if err := r.refresh(ctx, jobID); err != nil {
				return err
			}
			callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
			_, err := r.clients.search.ReindexTopic(callCtx, &searchpb.IndexTopicRequest{Topic: searchTopicDocument(topic)})
			cancel()
			if err != nil {
				return fmt.Errorf("index topic %d: %w", topic.GetId(), err)
			}
			status.TopicIndexed++
		}
		if err := r.save(ctx, jobID, *status); err != nil {
			return err
		}
		count := int32(len(page.GetItems()))
		if count == 0 || int64(offset)+int64(count) >= page.GetTotal() {
			return nil
		}
		offset += count
	}
}

func (r *SearchRebuilder) indexUsers(ctx context.Context, jobID string, status *domain.SearchRebuildStatus) error {
	for pageNumber := int32(1); ; pageNumber++ {
		if err := r.refresh(ctx, jobID); err != nil {
			return err
		}
		callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
		page, err := r.clients.user.ListUsers(callCtx, &userpb.ListUsersRequest{
			Status:   domain.UserStatusActive,
			Page:     pageNumber,
			PageSize: searchRebuildPageSize,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("list active users: %w", err)
		}
		status.UserTotal = page.GetTotal()
		for _, user := range page.GetItems() {
			if err := r.refresh(ctx, jobID); err != nil {
				return err
			}
			callCtx, cancel := context.WithTimeout(ctx, searchRebuildRPCTimeout)
			_, err := r.clients.search.ReindexUser(callCtx, &searchpb.IndexUserRequest{User: searchUserDocument(user)})
			cancel()
			if err != nil {
				return fmt.Errorf("index user %d: %w", user.GetId(), err)
			}
			status.UserIndexed++
		}
		if err := r.save(ctx, jobID, *status); err != nil {
			return err
		}
		count := int64(len(page.GetItems()))
		if count == 0 || int64(pageNumber)*int64(searchRebuildPageSize) >= page.GetTotal() {
			return nil
		}
	}
}

func (r *SearchRebuilder) save(ctx context.Context, jobID string, status domain.SearchRebuildStatus) error {
	status.UpdatedAt = time.Now().UnixMilli()
	saved, err := r.store.SaveOwned(ctx, jobID, status)
	if err != nil {
		return searchRebuildUnavailable(err)
	}
	if !saved {
		return fmt.Errorf("search rebuild lock lost")
	}
	return nil
}

func (r *SearchRebuilder) persistFailed(ctx context.Context, jobID string, status *domain.SearchRebuildStatus, cause error) {
	status.State = searchRebuildStateFailed
	status.Error = truncateSearchRebuildError(cause.Error())
	status.UpdatedAt = time.Now().UnixMilli()
	status.CompletedAt = status.UpdatedAt
	if saved, err := r.store.SaveOwned(ctx, jobID, *status); err == nil && saved {
		return
	}
	_, _ = r.store.MarkFailedIfWorkerInactive(ctx, jobID, *status)
}

func (r *SearchRebuilder) refresh(ctx context.Context, jobID string) error {
	ok, err := r.store.Refresh(ctx, jobID)
	if err != nil {
		return searchRebuildUnavailable(err)
	}
	if !ok {
		return fmt.Errorf("search rebuild lock lost")
	}
	return nil
}

func searchArticleDocument(article *contentpb.ArticleInfo) *searchpb.ArticleDocument {
	return &searchpb.ArticleDocument{
		Id:             article.GetId(),
		Title:          article.GetTitle(),
		Summary:        article.GetSummary(),
		ContentExcerpt: searchRebuildExcerpt(article.GetBody()),
		TagNames:       article.GetTags(),
		AuthorId:       article.GetAuthorId(),
		Status:         article.GetStatus(),
		ViewCount:      article.GetViewCount(),
		CreatedAt:      article.GetCreatedAt(),
		UpdatedAt:      article.GetUpdatedAt(),
	}
}

func searchTopicDocument(topic *contentpb.TopicInfo) *searchpb.TopicDocument {
	return &searchpb.TopicDocument{
		Id:             topic.GetId(),
		Slug:           topic.GetSlug(),
		Type:           topic.GetType(),
		Title:          topic.GetTitle(),
		ContentExcerpt: searchRebuildExcerpt(topic.GetBody()),
		TagNames:       topic.GetTags(),
		AuthorId:       topic.GetAuthorId(),
		Status:         topic.GetStatus(),
		ViewCount:      topic.GetViewCount(),
		CategoryId:     topic.GetCategoryId(),
		CreatedAt:      topic.GetCreatedAt(),
		UpdatedAt:      topic.GetUpdatedAt(),
	}
}

func searchUserDocument(user *userpb.UserInfo) *searchpb.UserDocument {
	return &searchpb.UserDocument{
		Id:        user.GetId(),
		Username:  user.GetUsername(),
		Nickname:  user.GetNickname(),
		Status:    user.GetStatus(),
		CreatedAt: user.GetCreatedAt(),
		UpdatedAt: user.GetUpdatedAt(),
	}
}

func searchRebuildExcerpt(value string) string {
	runes := []rune(value)
	if len(runes) <= 512 {
		return value
	}
	return string(runes[:512])
}

func truncateSearchRebuildError(value string) string {
	runes := []rune(value)
	if len(runes) <= 2048 {
		return value
	}
	return string(runes[:2048])
}

func searchRebuildUnavailable(err error) error {
	return fmt.Errorf("%w: %v", domain.ErrSearchRebuildUnavailable, err)
}

type redisSearchRebuildStore struct {
	client redis.Cmdable
}

func newRedisSearchRebuildStore(client redis.Cmdable) searchRebuildStore {
	if client == nil {
		return nil
	}
	return &redisSearchRebuildStore{client: client}
}

func (s *redisSearchRebuildStore) Claim(ctx context.Context, jobID string) (bool, error) {
	result, err := claimSearchRebuild.Run(ctx, s.client, []string{searchRebuildLockKey, searchRebuildHeartbeatKey}, jobID, searchRebuildLockTTL.Milliseconds(), searchRebuildHeartbeatTTL.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisSearchRebuildStore) Load(ctx context.Context) (domain.SearchRebuildStatus, bool, error) {
	value, err := s.client.Get(ctx, searchRebuildStatusKey).Result()
	if err == redis.Nil {
		return domain.SearchRebuildStatus{}, false, nil
	}
	if err != nil {
		return domain.SearchRebuildStatus{}, false, err
	}
	var status domain.SearchRebuildStatus
	if err := json.Unmarshal([]byte(value), &status); err != nil {
		return domain.SearchRebuildStatus{}, false, err
	}
	return status, true, nil
}

func (s *redisSearchRebuildStore) SaveOwned(ctx context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error) {
	payload, err := json.Marshal(status)
	if err != nil {
		return false, err
	}
	result, err := saveOwnedSearchRebuildStatus.Run(ctx, s.client, []string{searchRebuildLockKey, searchRebuildStatusKey, searchRebuildHeartbeatKey}, jobID, searchRebuildLockTTL.Milliseconds(), searchRebuildHeartbeatTTL.Milliseconds(), payload, searchRebuildStatusTTL.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisSearchRebuildStore) MarkFailedIfWorkerInactive(ctx context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error) {
	payload, err := json.Marshal(status)
	if err != nil {
		return false, err
	}
	result, err := markSearchRebuildFailedIfWorkerInactive.Run(ctx, s.client, []string{searchRebuildLockKey, searchRebuildStatusKey, searchRebuildHeartbeatKey}, jobID, payload, searchRebuildStatusTTL.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisSearchRebuildStore) Refresh(ctx context.Context, jobID string) (bool, error) {
	result, err := refreshSearchRebuildLock.Run(ctx, s.client, []string{searchRebuildLockKey, searchRebuildHeartbeatKey}, jobID, searchRebuildLockTTL.Milliseconds(), searchRebuildHeartbeatTTL.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *redisSearchRebuildStore) Release(ctx context.Context, jobID string) error {
	_, err := releaseSearchRebuildLock.Run(ctx, s.client, []string{searchRebuildLockKey, searchRebuildHeartbeatKey}, jobID).Result()
	return err
}

var claimSearchRebuild = redis.NewScript(`
if redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2]) then
  redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[3])
  return 1
end
local owner = redis.call("GET", KEYS[1])
if owner and redis.call("GET", KEYS[2]) ~= owner then
  redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
  redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[3])
  return 1
end
return 0
`)

var saveOwnedSearchRebuildStatus = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call("PEXPIRE", KEYS[1], ARGV[2])
redis.call("SET", KEYS[3], ARGV[1], "PX", ARGV[3])
redis.call("SET", KEYS[2], ARGV[4], "PX", ARGV[5])
return 1
`)

var markSearchRebuildFailedIfWorkerInactive = redis.NewScript(`
local owner = redis.call("GET", KEYS[1])
if owner and owner ~= ARGV[1] then
  return 0
end
if owner == ARGV[1] and redis.call("GET", KEYS[3]) == ARGV[1] then
  return 0
end
local raw = redis.call("GET", KEYS[2])
if not raw then
  return 0
end
local ok, current = pcall(cjson.decode, raw)
if not ok or current["job_id"] ~= ARGV[1] then
  return 0
end
if current["state"] ~= "queued" and current["state"] ~= "running" then
  return 0
end
if owner == ARGV[1] then
  redis.call("DEL", KEYS[1])
end
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
return 1
`)

var refreshSearchRebuildLock = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
  redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[3])
  return 1
end
return 0
`)

var releaseSearchRebuildLock = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  redis.call("DEL", KEYS[1])
  if redis.call("GET", KEYS[2]) == ARGV[1] then
    redis.call("DEL", KEYS[2])
  end
  return 1
end
return 0
`)
