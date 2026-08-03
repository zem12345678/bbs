package user

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxUserListsPerOwner = 20
	MaxUserListMembers   = 100
	MaxUserListNameRunes = 100
)

type UserList struct {
	ID            int64
	OwnerID       int64
	Name          string
	IsPublic      bool
	MemberCount   int64
	FavoriteCount int64
	IsFavorited   bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserListMembership struct {
	ListID    int64
	UserID    int64
	CreatedAt time.Time
}

type UserListFavorite struct {
	ListID    int64
	UserID    int64
	CreatedAt time.Time
}

type UserListsQuery struct {
	ViewerID int64
	OwnerID  int64
	Page     int
	PageSize int
}

type UserListMembersQuery struct {
	ViewerID int64
	ListID   int64
	Page     int
	PageSize int
}

type UserListFavoritesQuery struct {
	UserID   int64
	Page     int
	PageSize int
}

func NewUserList(id, ownerID int64, name string, isPublic bool) (*UserList, error) {
	list := &UserList{
		ID:        id,
		OwnerID:   ownerID,
		Name:      NormalizeUserListName(name),
		IsPublic:  isPublic,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := list.Validate(); err != nil {
		return nil, err
	}
	return list, nil
}

func (l *UserList) Update(name string, isPublic bool) error {
	if l == nil || l.ID <= 0 || l.OwnerID <= 0 {
		return ErrInvalidID
	}
	name = NormalizeUserListName(name)
	if err := validateUserListName(name); err != nil {
		return err
	}
	l.Name = name
	l.IsPublic = isPublic
	l.UpdatedAt = time.Now()
	return nil
}

func (l *UserList) Validate() error {
	if l == nil || l.ID <= 0 || l.OwnerID <= 0 {
		return ErrInvalidID
	}
	l.Name = NormalizeUserListName(l.Name)
	return validateUserListName(l.Name)
}

func NormalizeUserListName(name string) string {
	return strings.TrimSpace(name)
}

func validateUserListName(name string) error {
	if name == "" {
		return ErrUserListNameRequired
	}
	if utf8.RuneCountInString(name) > MaxUserListNameRunes {
		return ErrUserListNameTooLong
	}
	return nil
}
