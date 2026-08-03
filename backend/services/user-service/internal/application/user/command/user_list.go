package command

import (
	"context"
	"time"

	domain "user-service/internal/domain/user"
)

func (s *Service) CreateUserList(ctx context.Context, ownerID int64, name string, isPublic bool) (*domain.UserList, error) {
	if ownerID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return nil, err
	}
	list, err := domain.NewUserList(s.idgen.Generate(), ownerID, name, isPublic)
	if err != nil {
		return nil, err
	}
	if err := repo.CreateUserList(ctx, list); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, ownerID, list.ID)
}

func (s *Service) UpdateUserList(ctx context.Context, ownerID, listID int64, name string, isPublic bool) (*domain.UserList, error) {
	if ownerID <= 0 || listID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return nil, err
	}
	list, err := domain.NewUserList(listID, ownerID, name, isPublic)
	if err != nil {
		return nil, err
	}
	if err := repo.UpdateUserList(ctx, list); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, ownerID, listID)
}

func (s *Service) DeleteUserList(ctx context.Context, ownerID, listID int64) error {
	if ownerID <= 0 || listID <= 0 {
		return domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return err
	}
	return repo.DeleteUserList(ctx, ownerID, listID)
}

func (s *Service) AddUserListMember(ctx context.Context, ownerID, listID, userID int64) error {
	if ownerID <= 0 || listID <= 0 || userID <= 0 {
		return domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return err
	}
	if ownerID != userID {
		if err := s.ensureUserListActor(ctx, userID); err != nil {
			return err
		}
	}
	return repo.AddUserListMember(ctx, ownerID, domain.UserListMembership{
		ListID:    listID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
}

func (s *Service) RemoveUserListMember(ctx context.Context, ownerID, listID, userID int64) error {
	if ownerID <= 0 || listID <= 0 || userID <= 0 {
		return domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return err
	}
	if ownerID != userID {
		if err := s.ensureUserListActor(ctx, userID); err != nil {
			return err
		}
	}
	return repo.RemoveUserListMember(ctx, ownerID, listID, userID)
}

func (s *Service) CopyUserList(ctx context.Context, ownerID, sourceListID int64, name string) (*domain.UserList, error) {
	if ownerID <= 0 || sourceListID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListActor(ctx, ownerID); err != nil {
		return nil, err
	}
	target, err := domain.NewUserList(s.idgen.Generate(), ownerID, name, false)
	if err != nil {
		return nil, err
	}
	if err := repo.CopyUserList(ctx, sourceListID, target); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, ownerID, target.ID)
}

func (s *Service) FavoriteUserList(ctx context.Context, userID, listID int64) (*domain.UserList, error) {
	if userID <= 0 || listID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListActor(ctx, userID); err != nil {
		return nil, err
	}
	if err := repo.FavoriteUserList(ctx, domain.UserListFavorite{
		ListID:    listID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, userID, listID)
}

func (s *Service) UnfavoriteUserList(ctx context.Context, userID, listID int64) (*domain.UserList, error) {
	if userID <= 0 || listID <= 0 {
		return nil, domain.ErrInvalidID
	}
	repo, err := s.userListRepository()
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserListActor(ctx, userID); err != nil {
		return nil, err
	}
	if err := repo.UnfavoriteUserList(ctx, userID, listID); err != nil {
		return nil, err
	}
	return repo.GetUserList(ctx, userID, listID)
}

func (s *Service) userListRepository() (domain.UserListRepository, error) {
	repo, ok := s.repo.(domain.UserListRepository)
	if !ok {
		return nil, domain.ErrUserListRepositoryUnavailable
	}
	return repo, nil
}

func (s *Service) ensureUserListActor(ctx context.Context, userID int64) error {
	_, err := s.repo.FindByID(ctx, userID)
	return err
}
