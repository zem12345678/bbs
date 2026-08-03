package query

import (
	"context"

	domain "user-service/internal/domain/user"
)

type UserListsResult struct {
	Items []*domain.UserList
	Total int64
}

func (s *Service) GetUserList(ctx context.Context, viewerID, listID int64) (*domain.UserList, error) {
	if viewerID < 0 || listID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListViewer(ctx, viewerID); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, viewerID, listID)
}

func (s *Service) ListUserLists(ctx context.Context, q domain.UserListsQuery) (UserListsResult, error) {
	if q.ViewerID < 0 || q.OwnerID <= 0 {
		return UserListsResult{}, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return UserListsResult{}, err
	}
	if err := s.ensureUserListViewer(ctx, q.ViewerID); err != nil {
		return UserListsResult{}, err
	}
	if q.ViewerID != q.OwnerID {
		if _, err := s.repo.FindByID(ctx, q.OwnerID); err != nil {
			return UserListsResult{}, err
		}
	}
	items, total, err := repo.ListUserLists(ctx, q)
	if err != nil {
		return UserListsResult{}, err
	}
	return UserListsResult{Items: items, Total: total}, nil
}

func (s *Service) ListFavoriteUserLists(ctx context.Context, q domain.UserListFavoritesQuery) (UserListsResult, error) {
	if q.UserID <= 0 {
		return UserListsResult{}, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return UserListsResult{}, err
	}
	if _, err := s.repo.FindByID(ctx, q.UserID); err != nil {
		return UserListsResult{}, err
	}
	items, total, err := repo.ListFavoriteUserLists(ctx, q)
	if err != nil {
		return UserListsResult{}, err
	}
	return UserListsResult{Items: items, Total: total}, nil
}

func (s *Service) ListUserListMembers(ctx context.Context, q domain.UserListMembersQuery) (UserListResult, error) {
	if q.ViewerID < 0 || q.ListID <= 0 {
		return UserListResult{}, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return UserListResult{}, err
	}
	if err := s.ensureUserListViewer(ctx, q.ViewerID); err != nil {
		return UserListResult{}, err
	}
	items, total, err := repo.ListUserListMembers(ctx, q)
	if err != nil {
		return UserListResult{}, err
	}
	return UserListResult{Items: s.profilesForResponse(ctx, items), Total: total}, nil
}

func (s *Service) userListRepository() (domain.UserListRepository, error) {
	repo, ok := s.repo.(domain.UserListRepository)
	if !ok {
		return nil, domain.ErrUserListRepositoryUnavailable
	}
	return repo, nil
}

func (s *Service) ensureUserListViewer(ctx context.Context, viewerID int64) error {
	if viewerID == 0 {
		return nil
	}
	_, err := s.repo.FindByID(ctx, viewerID)
	return err
}
