package query

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestGetUserListSupportsAnonymousViewerAndValidatesAuthenticatedViewer(t *testing.T) {
	repo := newUserListQueryRepo(1)
	repo.lists[10] = &domain.UserList{ID: 10, OwnerID: 1, Name: "Public", IsPublic: true}
	svc := NewService(repo, nil)

	got, err := svc.GetUserList(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("GetUserList() anonymous error = %v", err)
	}
	if got.ID != 10 || repo.getViewerID != 0 {
		t.Fatalf("anonymous result = %+v, viewer = %d", got, repo.getViewerID)
	}
	if _, err := svc.GetUserList(context.Background(), 2, 10); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing viewer error = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetUserList(context.Background(), -1, 10); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatalf("negative viewer error = %v, want ErrInvalidID", err)
	}
}

func TestListUserListsValidatesUsersAndReturnsRepositoryPage(t *testing.T) {
	repo := newUserListQueryRepo(1, 2)
	repo.listed = []*domain.UserList{{ID: 10, OwnerID: 2, Name: "Public", IsPublic: true}}
	repo.total = 1
	svc := NewService(repo, nil)
	query := domain.UserListsQuery{ViewerID: 1, OwnerID: 2, Page: 3, PageSize: 7}

	got, err := svc.ListUserLists(context.Background(), query)
	if err != nil {
		t.Fatalf("ListUserLists() error = %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != 10 {
		t.Fatalf("list result = %+v", got)
	}
	if repo.listQuery != query {
		t.Fatalf("repository query = %+v, want %+v", repo.listQuery, query)
	}

	if _, err := svc.ListUserLists(context.Background(), domain.UserListsQuery{ViewerID: 1, OwnerID: 3}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing owner error = %v, want ErrNotFound", err)
	}
}

func TestListUserListMembersShapesProfiles(t *testing.T) {
	repo := newUserListQueryRepo()
	repo.members = []*domain.User{{
		ID:            2,
		BackgroundURL: "https://example.com/background.webp",
		ProfileTheme:  domain.ProfileThemePro,
	}}
	repo.membersTotal = 1
	svc := NewService(repo, nil)
	query := domain.UserListMembersQuery{ViewerID: 0, ListID: 10, Page: 2, PageSize: 5}

	got, err := svc.ListUserListMembers(context.Background(), query)
	if err != nil {
		t.Fatalf("ListUserListMembers() error = %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("members result = %+v", got)
	}
	if got.Items[0].BackgroundURL != "" || got.Items[0].ProfileTheme != domain.ProfileThemeDefault {
		t.Fatalf("member profile = %+v, want premium fields hidden", got.Items[0])
	}
	if repo.membersQuery != query {
		t.Fatalf("repository query = %+v, want %+v", repo.membersQuery, query)
	}
}

func TestListFavoriteUserListsValidatesUserAndReturnsRepositoryPage(t *testing.T) {
	repo := newUserListQueryRepo(1)
	repo.favorites = []*domain.UserList{{ID: 10, OwnerID: 2, Name: "Public", IsPublic: true, IsFavorited: true}}
	repo.favoritesTotal = 1
	svc := NewService(repo, nil)
	query := domain.UserListFavoritesQuery{UserID: 1, Page: 2, PageSize: 4}

	got, err := svc.ListFavoriteUserLists(context.Background(), query)
	if err != nil {
		t.Fatalf("ListFavoriteUserLists() error = %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != 10 || !got.Items[0].IsFavorited {
		t.Fatalf("favorite lists result = %+v", got)
	}
	if repo.favoritesQuery != query {
		t.Fatalf("repository query = %+v, want %+v", repo.favoritesQuery, query)
	}
	if _, err := svc.ListFavoriteUserLists(context.Background(), domain.UserListFavoritesQuery{UserID: 2}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user error = %v, want ErrNotFound", err)
	}
	if _, err := svc.ListFavoriteUserLists(context.Background(), domain.UserListFavoritesQuery{}); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatalf("invalid user error = %v, want ErrInvalidID", err)
	}
}

func TestUserListQueriesFailClosedWithoutOptionalRepository(t *testing.T) {
	repo := &repoStub{users: map[int64]*domain.User{1: {ID: 1}}}
	svc := NewService(repo, nil)

	if _, err := svc.GetUserList(context.Background(), 1, 10); !errors.Is(err, domain.ErrUserListRepositoryUnavailable) {
		t.Fatalf("GetUserList() error = %v, want ErrUserListRepositoryUnavailable", err)
	}
}

type userListQueryRepo struct {
	domain.Repository
	users          map[int64]*domain.User
	lists          map[int64]*domain.UserList
	listed         []*domain.UserList
	total          int64
	members        []*domain.User
	membersTotal   int64
	favorites      []*domain.UserList
	favoritesTotal int64
	getViewerID    int64
	listQuery      domain.UserListsQuery
	membersQuery   domain.UserListMembersQuery
	favoritesQuery domain.UserListFavoritesQuery
}

func newUserListQueryRepo(userIDs ...int64) *userListQueryRepo {
	repo := &userListQueryRepo{
		users: make(map[int64]*domain.User, len(userIDs)),
		lists: make(map[int64]*domain.UserList),
	}
	for _, id := range userIDs {
		repo.users[id] = &domain.User{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	return repo
}

func (r *userListQueryRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (r *userListQueryRepo) CreateUserList(context.Context, *domain.UserList) error { return nil }
func (r *userListQueryRepo) UpdateUserList(context.Context, *domain.UserList) error { return nil }
func (r *userListQueryRepo) DeleteUserList(context.Context, int64, int64) error     { return nil }

func (r *userListQueryRepo) GetUserList(_ context.Context, viewerID, listID int64) (*domain.UserList, error) {
	r.getViewerID = viewerID
	list, ok := r.lists[listID]
	if !ok {
		return nil, domain.ErrUserListNotFound
	}
	copy := *list
	return &copy, nil
}

func (r *userListQueryRepo) ListUserLists(_ context.Context, q domain.UserListsQuery) ([]*domain.UserList, int64, error) {
	r.listQuery = q
	return r.listed, r.total, nil
}

func (r *userListQueryRepo) ListFavoriteUserLists(_ context.Context, q domain.UserListFavoritesQuery) ([]*domain.UserList, int64, error) {
	r.favoritesQuery = q
	return r.favorites, r.favoritesTotal, nil
}

func (r *userListQueryRepo) AddUserListMember(context.Context, int64, domain.UserListMembership) error {
	return nil
}

func (r *userListQueryRepo) RemoveUserListMember(context.Context, int64, int64, int64) error {
	return nil
}

func (r *userListQueryRepo) ListUserListMembers(_ context.Context, q domain.UserListMembersQuery) ([]*domain.User, int64, error) {
	r.membersQuery = q
	return r.members, r.membersTotal, nil
}

func (r *userListQueryRepo) CopyUserList(context.Context, int64, *domain.UserList) error {
	return nil
}

func (r *userListQueryRepo) FavoriteUserList(context.Context, domain.UserListFavorite) error {
	return nil
}

func (r *userListQueryRepo) UnfavoriteUserList(context.Context, int64, int64) error {
	return nil
}
