package upstream

import (
	"context"

	"admin/api/proto/userpb"
	domain "admin/internal/domain/admin"
)

func (c *Clients) ListUsers(ctx context.Context, query string, status int32, page int32, pageSize int32) (domain.UserList, error) {
	resp, err := c.user.ListUsers(ctx, &userpb.ListUsersRequest{Query: query, Status: status, Page: page, PageSize: pageSize})
	if err != nil {
		return domain.UserList{}, err
	}
	items := make([]domain.User, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainUser(item))
	}
	return domain.UserList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) UpdateStatus(ctx context.Context, userID int64, status int32) (domain.User, error) {
	resp, err := c.user.UpdateStatus(ctx, &userpb.UpdateStatusRequest{Id: userID, Status: status})
	if err != nil {
		return domain.User{}, err
	}
	return toDomainUser(resp.GetUser()), nil
}

func toDomainUser(u *userpb.UserInfo) domain.User {
	if u == nil {
		return domain.User{}
	}
	return domain.User{
		ID:             u.GetId(),
		Username:       u.GetUsername(),
		Email:          u.GetEmail(),
		Nickname:       u.GetNickname(),
		AvatarURL:      u.GetAvatarUrl(),
		Bio:            u.GetBio(),
		Status:         u.GetStatus(),
		FollowerCount:  u.GetFollowerCount(),
		FollowingCount: u.GetFollowingCount(),
		CreatedAt:      u.GetCreatedAt(),
		UpdatedAt:      u.GetUpdatedAt(),
		LastLoginAt:    u.GetLastLoginAt(),
	}
}
