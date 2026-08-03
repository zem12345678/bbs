package upstream

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"admin/api/proto/contentpb"
	"admin/api/proto/searchpb"
	"admin/api/proto/userpb"
	domain "admin/internal/domain/admin"

	"google.golang.org/grpc"
)

func TestSearchRebuilderIndexesPublishedContentAndRejectsConcurrentRun(t *testing.T) {
	store := &memorySearchRebuildStore{}
	content := &rebuildContentClient{
		articles: &contentpb.ArticleListResponse{Items: []*contentpb.ArticleInfo{{
			Id: 11, Title: "Article", Summary: "Summary", Body: strings.Repeat("a", 513), Tags: []string{"go"}, AuthorId: 7, Status: 2, ViewCount: 12, CreatedAt: 101, UpdatedAt: 102,
		}}, Total: 1},
		topics: &contentpb.TopicListResponse{Items: []*contentpb.TopicInfo{{
			Id: 22, Slug: "topic", Type: "discussion", Title: "Topic", Body: "Body", Tags: []string{"redis"}, AuthorId: 8, Status: 2, ViewCount: 9, CategoryId: 3, CreatedAt: 201, UpdatedAt: 202,
		}}, Total: 1},
		started: make(chan struct{}),
		block:   make(chan struct{}),
	}
	users := &rebuildUserClient{users: &userpb.UserListResponse{Items: []*userpb.UserInfo{{
		Id: 33, Username: "alice", Nickname: "Alice", Status: domain.UserStatusActive, CreatedAt: 301, UpdatedAt: 302,
	}}, Total: 1}}
	search := &rebuildSearchClient{}
	rebuilder := newSearchRebuilder(&Clients{content: content, search: search, user: users}, store)

	started, err := rebuilder.StartSearchRebuild(t.Context(), 99)
	if err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	if started.State != searchRebuildStateQueued || started.RequestedBy != 99 {
		t.Fatalf("start status = %#v", started)
	}
	select {
	case <-content.started:
	case <-time.After(time.Second):
		t.Fatal("rebuild did not begin listing articles")
	}
	if _, err := rebuilder.StartSearchRebuild(t.Context(), 100); !errors.Is(err, domain.ErrSearchRebuildInProgress) {
		t.Fatalf("second StartSearchRebuild() error = %v, want in progress", err)
	}
	close(content.block)

	status := waitForSearchRebuildState(t, rebuilder, searchRebuildStateCompleted)
	if status.ArticleTotal != 1 || status.ArticleIndexed != 1 || status.TopicTotal != 1 || status.TopicIndexed != 1 || status.UserTotal != 1 || status.UserIndexed != 1 {
		t.Fatalf("completed status = %#v", status)
	}
	if len(search.articles) != 1 || len(search.topics) != 1 || len(search.users) != 1 {
		t.Fatalf("indexed articles/topics/users = %d/%d/%d", len(search.articles), len(search.topics), len(search.users))
	}
	if got := len([]rune(search.articles[0].GetContentExcerpt())); got != 512 {
		t.Fatalf("article excerpt length = %d, want 512", got)
	}
	if got := search.topics[0]; got.GetViewCount() != 9 || got.GetCategoryId() != 3 {
		t.Fatalf("topic document = %#v", got)
	}
	if got := search.users[0]; got.GetUsername() != "alice" || got.GetStatus() != domain.UserStatusActive {
		t.Fatalf("user document = %#v", got)
	}
	if len(users.requests) != 1 || users.requests[0].GetStatus() != domain.UserStatusActive || users.requests[0].GetPage() != 1 || users.requests[0].GetPageSize() != searchRebuildPageSize {
		t.Fatalf("list active users request = %#v", users.requests)
	}
}

func TestSearchRebuilderPersistsFailure(t *testing.T) {
	store := &memorySearchRebuildStore{}
	content := &rebuildContentClient{articles: &contentpb.ArticleListResponse{Items: []*contentpb.ArticleInfo{{Id: 11, Status: 2}}, Total: 1}, topics: &contentpb.TopicListResponse{}}
	search := &rebuildSearchClient{indexArticleErr: errors.New("elasticsearch unavailable")}
	rebuilder := newSearchRebuilder(&Clients{content: content, search: search, user: &rebuildUserClient{}}, store)

	if _, err := rebuilder.StartSearchRebuild(t.Context(), 99); err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	status := waitForSearchRebuildState(t, rebuilder, searchRebuildStateFailed)
	if !strings.Contains(status.Error, "elasticsearch unavailable") {
		t.Fatalf("failure error = %q", status.Error)
	}
}

func TestSearchRebuilderIndexesActiveUsersAcrossPages(t *testing.T) {
	firstPage := make([]*userpb.UserInfo, 0, searchRebuildPageSize)
	for id := int64(1); id <= int64(searchRebuildPageSize); id++ {
		firstPage = append(firstPage, &userpb.UserInfo{Id: id, Username: "member", Status: domain.UserStatusActive})
	}
	users := &rebuildUserClient{pages: map[int32]*userpb.UserListResponse{
		1: {Items: firstPage, Total: int64(searchRebuildPageSize) + 1},
		2: {Items: []*userpb.UserInfo{{Id: int64(searchRebuildPageSize) + 1, Username: "member", Status: domain.UserStatusActive}}, Total: int64(searchRebuildPageSize) + 1},
	}}
	search := &rebuildSearchClient{}
	rebuilder := newSearchRebuilder(&Clients{
		content: &rebuildContentClient{
			articles: &contentpb.ArticleListResponse{},
			topics:   &contentpb.TopicListResponse{},
		},
		search: search,
		user:   users,
	}, &memorySearchRebuildStore{})

	if _, err := rebuilder.StartSearchRebuild(t.Context(), 99); err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	status := waitForSearchRebuildState(t, rebuilder, searchRebuildStateCompleted)
	if status.State != searchRebuildStateCompleted {
		t.Fatalf("status = %#v", status)
	}
	if status.UserTotal != int64(searchRebuildPageSize)+1 || status.UserIndexed != int64(searchRebuildPageSize)+1 {
		t.Fatalf("user rebuild status = %#v", status)
	}
	if len(users.requests) != 2 || users.requests[0].GetPage() != 1 || users.requests[1].GetPage() != 2 {
		t.Fatalf("user page requests = %#v", users.requests)
	}
	if len(search.users) != int(searchRebuildPageSize)+1 {
		t.Fatalf("indexed users = %d", len(search.users))
	}
}

func TestSearchRebuilderMarksRunningStateWriteFailureAsFailed(t *testing.T) {
	store := &memorySearchRebuildStore{saveOwnedErrAtCall: 2, saveOwnedErr: errors.New("redis write failed")}
	rebuilder := newSearchRebuilder(&Clients{content: &rebuildContentClient{}, search: &rebuildSearchClient{}, user: &rebuildUserClient{}}, store)

	if _, err := rebuilder.StartSearchRebuild(t.Context(), 99); err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	status := waitForSearchRebuildState(t, rebuilder, searchRebuildStateFailed)
	if !strings.Contains(status.Error, "redis write failed") {
		t.Fatalf("failure error = %q", status.Error)
	}
}

func TestSearchRebuilderDoesNotOverwriteStatusAfterLosingClaim(t *testing.T) {
	replacement := domain.SearchRebuildStatus{JobID: "newer-job", State: searchRebuildStateQueued, RequestedBy: 100}
	store := &memorySearchRebuildStore{
		loseOwnershipOnSaveOwnedAtCall: 1,
		replacementStatus:              replacement,
	}
	rebuilder := newSearchRebuilder(&Clients{content: &rebuildContentClient{}, search: &rebuildSearchClient{}, user: &rebuildUserClient{}}, store)

	if _, err := rebuilder.StartSearchRebuild(t.Context(), 99); !errors.Is(err, domain.ErrSearchRebuildInProgress) {
		t.Fatalf("StartSearchRebuild() error = %v, want in progress", err)
	}
	status, found, err := store.Load(t.Context())
	if err != nil || !found || status != replacement {
		t.Fatalf("status = %#v, found = %v, err = %v; want %#v", status, found, err, replacement)
	}
}

func TestSearchRebuilderMarksLostWorkerStatusAsFailed(t *testing.T) {
	store := &memorySearchRebuildStore{
		found: true,
		status: domain.SearchRebuildStatus{
			JobID: "abandoned-job",
			State: searchRebuildStateRunning,
		},
	}
	rebuilder := newSearchRebuilder(nil, store)

	status, err := rebuilder.GetSearchRebuildStatus(t.Context())
	if err != nil {
		t.Fatalf("GetSearchRebuildStatus() error = %v", err)
	}
	if status.State != searchRebuildStateFailed || !strings.Contains(status.Error, "lease expired") {
		t.Fatalf("stale status = %#v", status)
	}
}

func TestSearchRebuilderMarksMissingHeartbeatAsFailedAndReleasesLock(t *testing.T) {
	store := &memorySearchRebuildStore{
		owner: "abandoned-job",
		found: true,
		status: domain.SearchRebuildStatus{
			JobID: "abandoned-job",
			State: searchRebuildStateRunning,
		},
	}
	rebuilder := newSearchRebuilder(nil, store)

	status, err := rebuilder.GetSearchRebuildStatus(t.Context())
	if err != nil {
		t.Fatalf("GetSearchRebuildStatus() error = %v", err)
	}
	if status.State != searchRebuildStateFailed || !strings.Contains(status.Error, "lease expired") {
		t.Fatalf("stale status = %#v", status)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.owner != "" {
		t.Fatalf("lock owner = %q, want released", store.owner)
	}
}

func TestSearchRebuilderStartReclaimsLockAfterHeartbeatExpires(t *testing.T) {
	store := &memorySearchRebuildStore{
		owner: "abandoned-job",
		found: true,
		status: domain.SearchRebuildStatus{
			JobID: "abandoned-job",
			State: searchRebuildStateRunning,
		},
	}
	rebuilder := newSearchRebuilder(&Clients{
		content: &rebuildContentClient{
			articles: &contentpb.ArticleListResponse{},
			topics:   &contentpb.TopicListResponse{},
		},
		search: &rebuildSearchClient{},
		user:   &rebuildUserClient{},
	}, store)

	started, err := rebuilder.StartSearchRebuild(t.Context(), 99)
	if err != nil {
		t.Fatalf("StartSearchRebuild() error = %v", err)
	}
	if started.JobID == "abandoned-job" {
		t.Fatalf("started job ID = %q, want a replacement job", started.JobID)
	}
	status := waitForSearchRebuildState(t, rebuilder, searchRebuildStateCompleted)
	if status.JobID != started.JobID {
		t.Fatalf("completed job ID = %q, want %q", status.JobID, started.JobID)
	}
}

func TestSearchRebuilderDoesNotOverwriteCompletedStatusDuringInactiveRecovery(t *testing.T) {
	store := &memorySearchRebuildStore{
		owner: "job-1",
		found: true,
		status: domain.SearchRebuildStatus{
			JobID: "job-1",
			State: searchRebuildStateRunning,
		},
		beforeMarkInactive: func(store *memorySearchRebuildStore) {
			store.owner = ""
			store.heartbeat = false
			store.status.State = searchRebuildStateCompleted
		},
	}
	rebuilder := newSearchRebuilder(nil, store)

	status, err := rebuilder.GetSearchRebuildStatus(t.Context())
	if err != nil {
		t.Fatalf("GetSearchRebuildStatus() error = %v", err)
	}
	if status.State != searchRebuildStateCompleted {
		t.Fatalf("status state = %q, want %q", status.State, searchRebuildStateCompleted)
	}
}

func TestSearchRebuildStoreFencesOldOwnerAfterHeartbeatExpiry(t *testing.T) {
	store := &memorySearchRebuildStore{owner: "old-job"}
	claimed, err := store.Claim(t.Context(), "new-job")
	if err != nil || !claimed {
		t.Fatalf("Claim() = %v, %v; want true, nil", claimed, err)
	}
	if saved, err := store.SaveOwned(t.Context(), "old-job", domain.SearchRebuildStatus{JobID: "old-job", State: searchRebuildStateRunning}); err != nil || saved {
		t.Fatalf("old SaveOwned() = %v, %v; want false, nil", saved, err)
	}
	if refreshed, err := store.Refresh(t.Context(), "old-job"); err != nil || refreshed {
		t.Fatalf("old Refresh() = %v, %v; want false, nil", refreshed, err)
	}
	if err := store.Release(t.Context(), "old-job"); err != nil {
		t.Fatalf("old Release() error = %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.owner != "new-job" || !store.heartbeat {
		t.Fatalf("store owner/heartbeat = %q/%v, want new-job/true", store.owner, store.heartbeat)
	}
}

func waitForSearchRebuildState(t *testing.T, rebuilder *SearchRebuilder, want string) domain.SearchRebuildStatus {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status, err := rebuilder.GetSearchRebuildStatus(t.Context())
		if err != nil {
			t.Fatalf("GetSearchRebuildStatus() error = %v", err)
		}
		if status.State == want {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	status, _ := rebuilder.GetSearchRebuildStatus(t.Context())
	t.Fatalf("status state = %q, want %q; status=%#v", status.State, want, status)
	return domain.SearchRebuildStatus{}
}

type rebuildContentClient struct {
	contentpb.ContentServiceClient
	articles *contentpb.ArticleListResponse
	topics   *contentpb.TopicListResponse
	started  chan struct{}
	block    chan struct{}
	once     sync.Once
}

func (c *rebuildContentClient) ListArticles(_ context.Context, _ *contentpb.ListArticlesRequest, _ ...grpc.CallOption) (*contentpb.ArticleListResponse, error) {
	if c.started != nil {
		c.once.Do(func() { close(c.started) })
	}
	if c.block != nil {
		<-c.block
	}
	return c.articles, nil
}

func (c *rebuildContentClient) ListTopics(context.Context, *contentpb.ListTopicsRequest, ...grpc.CallOption) (*contentpb.TopicListResponse, error) {
	return c.topics, nil
}

type rebuildSearchClient struct {
	searchpb.SearchServiceClient
	articles        []*searchpb.ArticleDocument
	topics          []*searchpb.TopicDocument
	users           []*searchpb.UserDocument
	indexArticleErr error
}

func (*rebuildSearchClient) EnsureArticleIndex(context.Context, *searchpb.EnsureArticleIndexRequest, ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	return &searchpb.SimpleResponse{Success: true}, nil
}

func (*rebuildSearchClient) EnsureTopicIndex(context.Context, *searchpb.EnsureTopicIndexRequest, ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	return &searchpb.SimpleResponse{Success: true}, nil
}

func (*rebuildSearchClient) EnsureUserIndex(context.Context, *searchpb.EnsureUserIndexRequest, ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	return &searchpb.SimpleResponse{Success: true}, nil
}

func (c *rebuildSearchClient) ReindexArticle(_ context.Context, request *searchpb.IndexArticleRequest, _ ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	if c.indexArticleErr != nil {
		return nil, c.indexArticleErr
	}
	c.articles = append(c.articles, request.GetArticle())
	return &searchpb.SimpleResponse{Success: true}, nil
}

func (c *rebuildSearchClient) ReindexTopic(_ context.Context, request *searchpb.IndexTopicRequest, _ ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	c.topics = append(c.topics, request.GetTopic())
	return &searchpb.SimpleResponse{Success: true}, nil
}

func (c *rebuildSearchClient) ReindexUser(_ context.Context, request *searchpb.IndexUserRequest, _ ...grpc.CallOption) (*searchpb.SimpleResponse, error) {
	c.users = append(c.users, request.GetUser())
	return &searchpb.SimpleResponse{Success: true}, nil
}

type rebuildUserClient struct {
	userpb.UserServiceClient
	users    *userpb.UserListResponse
	pages    map[int32]*userpb.UserListResponse
	requests []*userpb.ListUsersRequest
}

func (c *rebuildUserClient) ListUsers(_ context.Context, request *userpb.ListUsersRequest, _ ...grpc.CallOption) (*userpb.UserListResponse, error) {
	c.requests = append(c.requests, request)
	if c.pages != nil {
		return c.pages[request.GetPage()], nil
	}
	return c.users, nil
}

type memorySearchRebuildStore struct {
	mu                             sync.Mutex
	owner                          string
	heartbeat                      bool
	status                         domain.SearchRebuildStatus
	found                          bool
	saveOwnedCalls                 int
	saveOwnedErrAtCall             int
	saveOwnedErr                   error
	loseOwnershipOnSaveOwnedAtCall int
	replacementStatus              domain.SearchRebuildStatus
	beforeMarkInactive             func(*memorySearchRebuildStore)
}

func (s *memorySearchRebuildStore) Claim(_ context.Context, jobID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != "" && s.heartbeat {
		return false, nil
	}
	s.owner = jobID
	s.heartbeat = true
	return true, nil
}

func (s *memorySearchRebuildStore) Load(context.Context) (domain.SearchRebuildStatus, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status, s.found, nil
}

func (s *memorySearchRebuildStore) SaveOwned(_ context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveOwnedCalls++
	if s.saveOwnedErrAtCall == s.saveOwnedCalls {
		return false, s.saveOwnedErr
	}
	if s.loseOwnershipOnSaveOwnedAtCall == s.saveOwnedCalls {
		s.owner = s.replacementStatus.JobID
		s.status = s.replacementStatus
		s.found = true
		return false, nil
	}
	if s.owner != jobID {
		return false, nil
	}
	s.heartbeat = true
	s.status = status
	s.found = true
	return true, nil
}

func (s *memorySearchRebuildStore) MarkFailedIfWorkerInactive(_ context.Context, jobID string, status domain.SearchRebuildStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.beforeMarkInactive != nil {
		beforeMarkInactive := s.beforeMarkInactive
		s.beforeMarkInactive = nil
		beforeMarkInactive(s)
	}
	if (s.owner != "" && s.owner != jobID) || (s.owner == jobID && s.heartbeat) || !s.found || s.status.JobID != jobID || (s.status.State != searchRebuildStateQueued && s.status.State != searchRebuildStateRunning) {
		return false, nil
	}
	if s.owner == jobID {
		s.owner = ""
	}
	s.status = status
	return true, nil
}

func (s *memorySearchRebuildStore) Refresh(_ context.Context, jobID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != jobID {
		return false, nil
	}
	s.heartbeat = true
	return true, nil
}

func (s *memorySearchRebuildStore) Release(_ context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner == jobID {
		s.owner = ""
		s.heartbeat = false
	}
	return nil
}
