package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestCreateUserListUsesGeneratedIDAndReturnsRepositoryView(t *testing.T) {
	repo := newUserListCommandRepo(1)
	svc := NewService(repo, &fakeIDGen{next: 899}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	got, err := svc.CreateUserList(context.Background(), 1, "  Close friends  ", true)
	if err != nil {
		t.Fatalf("CreateUserList() error = %v", err)
	}
	if got.ID != 900 || got.OwnerID != 1 || got.Name != "Close friends" || !got.IsPublic {
		t.Fatalf("created list = %+v", got)
	}
	if got.MemberCount != 4 || got.FavoriteCount != 2 || !got.IsFavorited {
		t.Fatalf("returned repository view = %+v", got)
	}
	if repo.created == nil || repo.created.ID != 900 || repo.getViewerID != 1 || repo.getListID != 900 {
		t.Fatalf("repository calls = created:%+v get:(%d,%d)", repo.created, repo.getViewerID, repo.getListID)
	}
}

func TestUserListMemberCommandsRequireExistingUsers(t *testing.T) {
	repo := newUserListCommandRepo(1, 2)
	repo.lists[10] = &domain.UserList{ID: 10, OwnerID: 1, Name: "Friends"}
	svc := NewService(repo, &fakeIDGen{next: 100}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if err := svc.AddUserListMember(context.Background(), 1, 10, 2); err != nil {
		t.Fatalf("AddUserListMember() error = %v", err)
	}
	if repo.added == nil || repo.added.ListID != 10 || repo.added.UserID != 2 || repo.added.CreatedAt.IsZero() {
		t.Fatalf("added membership = %+v", repo.added)
	}

	repo.added = nil
	if err := svc.AddUserListMember(context.Background(), 1, 10, 3); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing target error = %v, want ErrNotFound", err)
	}
	if repo.added != nil {
		t.Fatalf("repository called for missing target: %+v", repo.added)
	}

	if err := svc.RemoveUserListMember(context.Background(), 0, 10, 2); !errors.Is(err, domain.ErrInvalidID) {
		t.Fatalf("invalid owner error = %v, want ErrInvalidID", err)
	}
}

func TestCopyAndFavoriteUserListReturnFreshViews(t *testing.T) {
	repo := newUserListCommandRepo(1)
	repo.lists[20] = &domain.UserList{ID: 20, OwnerID: 9, Name: "Source", IsPublic: true, FavoriteCount: 2}
	svc := NewService(repo, &fakeIDGen{next: 999}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	copied, err := svc.CopyUserList(context.Background(), 1, 20, "  Copied  ")
	if err != nil {
		t.Fatalf("CopyUserList() error = %v", err)
	}
	if copied.ID != 1000 || copied.OwnerID != 1 || copied.Name != "Copied" || copied.IsPublic {
		t.Fatalf("copied list = %+v", copied)
	}
	if repo.copiedSourceID != 20 || copied.MemberCount != 4 {
		t.Fatalf("copy repository state = source:%d list:%+v", repo.copiedSourceID, copied)
	}

	favorited, err := svc.FavoriteUserList(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("FavoriteUserList() error = %v", err)
	}
	if !favorited.IsFavorited || favorited.FavoriteCount != 3 || repo.favorited == nil {
		t.Fatalf("favorited list = %+v, favorite = %+v", favorited, repo.favorited)
	}
	unfavorited, err := svc.UnfavoriteUserList(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("UnfavoriteUserList() error = %v", err)
	}
	if unfavorited.IsFavorited || unfavorited.FavoriteCount != 2 {
		t.Fatalf("unfavorited list = %+v", unfavorited)
	}
}

func TestUserListCommandsFailClosedWithoutOptionalRepository(t *testing.T) {
	repo := struct{ domain.Repository }{}
	svc := NewService(repo, &fakeIDGen{next: 1}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, err := svc.CreateUserList(context.Background(), 1, "Friends", false); !errors.Is(err, domain.ErrUserListRepositoryUnavailable) {
		t.Fatalf("CreateUserList() error = %v, want ErrUserListRepositoryUnavailable", err)
	}
}

func TestUserListCommandsValidateNamesBeforeRepositoryWrites(t *testing.T) {
	repo := newUserListCommandRepo(1)
	repo.lists[10] = &domain.UserList{ID: 10, OwnerID: 1, Name: "Existing"}
	svc := NewService(repo, &fakeIDGen{next: 100}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)

	if _, err := svc.CreateUserList(context.Background(), 1, " \t ", false); !errors.Is(err, domain.ErrUserListNameRequired) {
		t.Fatalf("blank create name error = %v, want ErrUserListNameRequired", err)
	}
	if repo.created != nil {
		t.Fatalf("repository received invalid create: %+v", repo.created)
	}
	if _, err := svc.UpdateUserList(context.Background(), 1, 10, strings.Repeat("界", domain.MaxUserListNameRunes+1), false); !errors.Is(err, domain.ErrUserListNameTooLong) {
		t.Fatalf("long update name error = %v, want ErrUserListNameTooLong", err)
	}
	if repo.updated != nil {
		t.Fatalf("repository received invalid update: %+v", repo.updated)
	}
}

type userListCommandRepo struct {
	domain.Repository
	users          map[int64]*domain.User
	lists          map[int64]*domain.UserList
	created        *domain.UserList
	updated        *domain.UserList
	added          *domain.UserListMembership
	favorited      *domain.UserListFavorite
	copiedSourceID int64
	getViewerID    int64
	getListID      int64
}

func newUserListCommandRepo(userIDs ...int64) *userListCommandRepo {
	repo := &userListCommandRepo{
		users: make(map[int64]*domain.User, len(userIDs)),
		lists: make(map[int64]*domain.UserList),
	}
	for _, id := range userIDs {
		repo.users[id] = &domain.User{ID: id}
	}
	return repo
}

func (r *userListCommandRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (r *userListCommandRepo) CreateUserList(_ context.Context, list *domain.UserList) error {
	r.created = cloneUserList(list)
	stored := cloneUserList(list)
	stored.MemberCount = 4
	stored.FavoriteCount = 2
	stored.IsFavorited = true
	r.lists[list.ID] = stored
	return nil
}

func (r *userListCommandRepo) UpdateUserList(_ context.Context, list *domain.UserList) error {
	r.updated = cloneUserList(list)
	r.lists[list.ID] = cloneUserList(list)
	return nil
}

func (r *userListCommandRepo) DeleteUserList(_ context.Context, _, listID int64) error {
	delete(r.lists, listID)
	return nil
}

func (r *userListCommandRepo) GetUserList(_ context.Context, viewerID, listID int64) (*domain.UserList, error) {
	r.getViewerID = viewerID
	r.getListID = listID
	list, ok := r.lists[listID]
	if !ok {
		return nil, domain.ErrUserListNotFound
	}
	return cloneUserList(list), nil
}

func (r *userListCommandRepo) ListUserLists(context.Context, domain.UserListsQuery) ([]*domain.UserList, int64, error) {
	return nil, 0, nil
}

func (r *userListCommandRepo) ListFavoriteUserLists(context.Context, domain.UserListFavoritesQuery) ([]*domain.UserList, int64, error) {
	return nil, 0, nil
}

func (r *userListCommandRepo) AddUserListMember(_ context.Context, _ int64, membership domain.UserListMembership) error {
	r.added = &membership
	return nil
}

func (r *userListCommandRepo) RemoveUserListMember(context.Context, int64, int64, int64) error {
	return nil
}

func (r *userListCommandRepo) ListUserListMembers(context.Context, domain.UserListMembersQuery) ([]*domain.User, int64, error) {
	return nil, 0, nil
}

func (r *userListCommandRepo) CopyUserList(_ context.Context, sourceListID int64, target *domain.UserList) error {
	r.copiedSourceID = sourceListID
	stored := cloneUserList(target)
	stored.MemberCount = 4
	stored.FavoriteCount = 2
	r.lists[target.ID] = stored
	return nil
}

func (r *userListCommandRepo) FavoriteUserList(_ context.Context, favorite domain.UserListFavorite) error {
	r.favorited = &favorite
	list := r.lists[favorite.ListID]
	list.FavoriteCount++
	list.IsFavorited = true
	return nil
}

func (r *userListCommandRepo) UnfavoriteUserList(_ context.Context, _, listID int64) error {
	list := r.lists[listID]
	list.FavoriteCount--
	list.IsFavorited = false
	return nil
}

func cloneUserList(list *domain.UserList) *domain.UserList {
	if list == nil {
		return nil
	}
	copy := *list
	return &copy
}
