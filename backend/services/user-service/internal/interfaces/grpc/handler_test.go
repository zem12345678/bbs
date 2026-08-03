package grpc

import (
	"context"
	"time"
	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	domain "user-service/internal/domain/user"
	"user-service/pkg/logger"

	pb "user-service/api/proto/userpb"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"testing"
)

func TestGetCredentialVersionReturnsDurableValue(t *testing.T) {
	repo := credentialVersionRepo{versions: map[int64]string{42: "rotated-version"}}
	h := NewHandler(command.NewService(repo, nil, nil, nil, "test-secret", 0, 8, nil, nil, nil), query.NewService(repo, nil))

	response, err := h.GetCredentialVersion(context.Background(), &pb.UserIDRequest{Id: 42})
	if err != nil {
		t.Fatalf("GetCredentialVersion() error = %v", err)
	}
	if response.GetUserId() != 42 || response.GetCredentialVersion() != "rotated-version" {
		t.Fatalf("response = %+v", response)
	}
}

type credentialVersionRepo struct {
	domain.Repository
	versions map[int64]string
}

func (r credentialVersionRepo) GetCredentialVersion(_ context.Context, userID int64) (string, error) {
	version, ok := r.versions[userID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return version, nil
}

func TestToStatusMapsProfileThemeEntitlementRequired(t *testing.T) {
	err := toStatus(domain.ErrProfileThemeEntitlementRequired)
	if grpcstatus.Code(err) != codes.PermissionDenied {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.PermissionDenied)
	}
}

func TestToStatusMapsSecurityEmailDeliveryUnavailable(t *testing.T) {
	err := toStatus(domain.ErrSecurityEmailDeliveryUnavailable)
	if grpcstatus.Code(err) != codes.Unavailable {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.Unavailable)
	}
}

func TestAccountVerificationErrorsDoNotInvalidateBearerAuthentication(t *testing.T) {
	for _, err := range []error{
		domain.ErrMFACodeInvalid,
		domain.ErrPasskeyChallengeExpired,
		domain.ErrPasskeyVerificationFailed,
	} {
		if got := grpcstatus.Code(toAccountVerificationStatus(err)); got != codes.FailedPrecondition {
			t.Fatalf("toAccountVerificationStatus(%v) code = %v, want %v", err, got, codes.FailedPrecondition)
		}
	}
	if got := grpcstatus.Code(toStatus(domain.ErrPasskeyVerificationFailed)); got != codes.Unauthenticated {
		t.Fatalf("login passkey verification code = %v, want %v", got, codes.Unauthenticated)
	}
}

func TestToStatusMapsSafetyRelationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "self relation", err: domain.ErrCannotRelateSelf, code: codes.InvalidArgument},
		{name: "already blocking", err: domain.ErrAlreadyBlocking, code: codes.AlreadyExists},
		{name: "already muted", err: domain.ErrAlreadyMuted, code: codes.AlreadyExists},
		{name: "not blocking", err: domain.ErrNotBlocking, code: codes.FailedPrecondition},
		{name: "not muted", err: domain.ErrNotMuted, code: codes.FailedPrecondition},
		{name: "follow blocked", err: domain.ErrFollowBlocked, code: codes.FailedPrecondition},
		{name: "repository unavailable", err: domain.ErrSafetyRepositoryUnavailable, code: codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := grpcstatus.Code(toStatus(tt.err)); got != tt.code {
				t.Fatalf("status code = %v, want %v", got, tt.code)
			}
		})
	}
}

func TestToStatusMapsInviteAndOAuthErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invite required", err: domain.ErrInviteCodeRequired, code: codes.InvalidArgument},
		{name: "invite invalid", err: domain.ErrInviteCodeInvalid, code: codes.InvalidArgument},
		{name: "invite expired", err: domain.ErrInviteCodeExpired, code: codes.FailedPrecondition},
		{name: "invite used", err: domain.ErrInviteCodeUsed, code: codes.FailedPrecondition},
		{name: "invite revoked", err: domain.ErrInviteCodeRevoked, code: codes.FailedPrecondition},
		{name: "invite not found", err: domain.ErrInviteCodeNotFound, code: codes.NotFound},
		{name: "invite repository unavailable", err: domain.ErrInviteRepositoryUnavailable, code: codes.Unavailable},
		{name: "oauth signup disabled", err: domain.ErrOAuthSignupDisabled, code: codes.PermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := grpcstatus.Code(toStatus(tt.err)); got != tt.code {
				t.Fatalf("status code = %v, want %v", got, tt.code)
			}
		})
	}
}

func TestToPbAuthResponseCarriesMFAChallengeWithoutToken(t *testing.T) {
	expiresAt := time.UnixMilli(1_784_025_300_000)
	response := toPbAuthResponse(&domain.User{ID: 42}, command.AuthToken{
		MFARequired:        true,
		MFAChallenge:       "oauth-mfa-challenge",
		MFAChallengeExpiry: expiresAt,
	})
	if !response.GetMfaRequired() || response.GetMfaChallenge() != "oauth-mfa-challenge" || response.GetMfaExpiresAt() != expiresAt.UnixMilli() {
		t.Fatalf("MFA response = %+v", response)
	}
	if response.GetAccessToken() != "" || response.GetExpiresAt() != 0 {
		t.Fatalf("MFA challenge exposed token fields: %+v", response)
	}
}

func TestToStatusMapsUserListErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "list not found", err: domain.ErrUserListNotFound, code: codes.NotFound},
		{name: "member not found", err: domain.ErrUserListMemberNotFound, code: codes.NotFound},
		{name: "favorite not found", err: domain.ErrUserListFavoriteNotFound, code: codes.NotFound},
		{name: "name required", err: domain.ErrUserListNameRequired, code: codes.InvalidArgument},
		{name: "name too long", err: domain.ErrUserListNameTooLong, code: codes.InvalidArgument},
		{name: "name exists", err: domain.ErrUserListNameExists, code: codes.AlreadyExists},
		{name: "member exists", err: domain.ErrUserListMemberExists, code: codes.AlreadyExists},
		{name: "favorite exists", err: domain.ErrUserListFavoriteExists, code: codes.AlreadyExists},
		{name: "list limit", err: domain.ErrUserListLimitReached, code: codes.FailedPrecondition},
		{name: "member limit", err: domain.ErrUserListMemberLimitReached, code: codes.FailedPrecondition},
		{name: "member blocked", err: domain.ErrUserListMemberBlocked, code: codes.FailedPrecondition},
		{name: "repository unavailable", err: domain.ErrUserListRepositoryUnavailable, code: codes.Unavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := grpcstatus.Code(toStatus(tt.err)); got != tt.code {
				t.Fatalf("status code = %v, want %v", got, tt.code)
			}
		})
	}
}

func TestUserListHandlersMapRequestsAndResponses(t *testing.T) {
	now := time.Unix(1_700_000_000, 123_000_000)
	repo := &userListHandlerRepo{
		users: map[int64]*domain.User{
			1: {ID: 1},
			2: {ID: 2, Username: "member", Nickname: "Member", CreatedAt: now, UpdatedAt: now},
		},
		lists: map[int64]*domain.UserList{
			10: {ID: 10, OwnerID: 2, Name: "Source", IsPublic: true, CreatedAt: now, UpdatedAt: now},
		},
	}
	h := NewHandler(
		command.NewService(repo, &handlerIDGen{next: 19}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
		query.NewService(repo, nil),
	)
	ctx := context.Background()

	created, err := h.CreateUserList(ctx, &pb.CreateUserListRequest{OwnerId: 1, Name: "  Team  ", IsPublic: true})
	if err != nil {
		t.Fatalf("CreateUserList() error = %v", err)
	}
	if list := created.GetUserList(); list.GetId() != 20 || list.GetName() != "Team" || !list.GetIsPublic() {
		t.Fatalf("created response = %+v", list)
	}

	updated, err := h.UpdateUserList(ctx, &pb.UpdateUserListRequest{OwnerId: 1, ListId: 20, Name: "Editors"})
	if err != nil || updated.GetUserList().GetName() != "Editors" || updated.GetUserList().GetIsPublic() {
		t.Fatalf("UpdateUserList() response = %+v, error = %v", updated, err)
	}
	got, err := h.GetUserList(ctx, &pb.GetUserListRequest{ViewerId: 1, ListId: 20})
	if err != nil || got.GetUserList().GetId() != 20 {
		t.Fatalf("GetUserList() response = %+v, error = %v", got, err)
	}

	lists, err := h.ListUserLists(ctx, &pb.ListUserListsRequest{ViewerId: 1, OwnerId: 1, Page: 2, PageSize: 3})
	if err != nil || lists.GetTotal() != 1 || repo.listQuery != (domain.UserListsQuery{ViewerID: 1, OwnerID: 1, Page: 2, PageSize: 3}) {
		t.Fatalf("ListUserLists() response = %+v, query = %+v, error = %v", lists, repo.listQuery, err)
	}

	if _, err := h.AddUserListMember(ctx, &pb.UserListMemberRequest{OwnerId: 1, ListId: 20, UserId: 2}); err != nil {
		t.Fatalf("AddUserListMember() error = %v", err)
	}
	members, err := h.ListUserListMembers(ctx, &pb.ListUserListMembersRequest{ViewerId: 1, ListId: 20, Page: 3, PageSize: 4})
	if err != nil || members.GetTotal() != 1 || members.GetItems()[0].GetId() != 2 {
		t.Fatalf("ListUserListMembers() response = %+v, error = %v", members, err)
	}
	if repo.membersQuery != (domain.UserListMembersQuery{ViewerID: 1, ListID: 20, Page: 3, PageSize: 4}) {
		t.Fatalf("member query = %+v", repo.membersQuery)
	}
	if _, err := h.RemoveUserListMember(ctx, &pb.UserListMemberRequest{OwnerId: 1, ListId: 20, UserId: 2}); err != nil {
		t.Fatalf("RemoveUserListMember() error = %v", err)
	}

	copied, err := h.CopyUserList(ctx, &pb.CopyUserListRequest{OwnerId: 1, SourceListId: 10, Name: "Copied"})
	if err != nil || copied.GetUserList().GetId() != 21 || copied.GetUserList().GetIsPublic() {
		t.Fatalf("CopyUserList() response = %+v, error = %v", copied, err)
	}
	favorited, err := h.FavoriteUserList(ctx, &pb.UserListFavoriteRequest{UserId: 1, ListId: 10})
	if err != nil || !favorited.GetUserList().GetIsFavorited() {
		t.Fatalf("FavoriteUserList() response = %+v, error = %v", favorited, err)
	}
	favorites, err := h.ListFavoriteUserLists(ctx, &pb.ListFavoriteUserListsRequest{UserId: 1, Page: 4, PageSize: 5})
	if err != nil || favorites.GetTotal() != 1 || !favorites.GetItems()[0].GetIsFavorited() {
		t.Fatalf("ListFavoriteUserLists() response = %+v, error = %v", favorites, err)
	}
	if repo.favoritesQuery != (domain.UserListFavoritesQuery{UserID: 1, Page: 4, PageSize: 5}) {
		t.Fatalf("favorite query = %+v", repo.favoritesQuery)
	}
	unfavorited, err := h.UnfavoriteUserList(ctx, &pb.UserListFavoriteRequest{UserId: 1, ListId: 10})
	if err != nil || unfavorited.GetUserList().GetIsFavorited() {
		t.Fatalf("UnfavoriteUserList() response = %+v, error = %v", unfavorited, err)
	}
	if _, err := h.DeleteUserList(ctx, &pb.DeleteUserListRequest{OwnerId: 1, ListId: 20}); err != nil {
		t.Fatalf("DeleteUserList() error = %v", err)
	}
}

func TestToPbInviteUsesUnixSeconds(t *testing.T) {
	createdAt := time.Unix(1_700_000_000, 987_000_000)
	expiresAt := createdAt.Add(time.Hour)
	usedAt := createdAt.Add(time.Minute)
	usedBy := int64(77)
	got := toPbInvite(domain.InviteCode{
		ID: 1, Code: "INVITE", CreatedByAdminID: 42, UsedByUserID: &usedBy,
		ExpiresAt: &expiresAt, UsedAt: &usedAt, CreatedAt: createdAt,
	}, createdAt)
	if got.GetCreatedAt() != createdAt.Unix() || got.GetExpiresAt() != expiresAt.Unix() || got.GetUsedAt() != usedAt.Unix() {
		t.Fatalf("invite timestamps = created:%d expires:%d used:%d", got.GetCreatedAt(), got.GetExpiresAt(), got.GetUsedAt())
	}
	if got.GetStatus() != domain.InviteStatusUsed {
		t.Fatalf("invite status = %q, want %q", got.GetStatus(), domain.InviteStatusUsed)
	}
}

func TestCreateInviteCodesRejectsNegativeExpiry(t *testing.T) {
	h := NewHandler(command.NewService(nil, nil, nil, nil, "test-secret", 0, 8, nil, nil, nil), nil)

	_, err := h.CreateInviteCodes(context.Background(), &pb.CreateInviteCodesRequest{
		ActorId:   42,
		Count:     1,
		ExpiresAt: -1,
	})
	if got := grpcstatus.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %v, want %v", got, codes.InvalidArgument)
	}
}

func TestRequestPasswordResetRedactsTokenFromGRPCResponse(t *testing.T) {
	repo := &passwordResetHandlerRepo{user: &domain.User{
		ID:     42,
		Email:  "member@example.com",
		Status: domain.StatusActive,
	}}
	emails := &passwordResetHandlerEmails{}
	cmd := command.NewService(repo, nil, nil, logger.NewNopLogger(), "test-secret", 0, 8, nil, emails, nil)
	h := NewHandler(cmd, query.NewService(repo, nil))

	response, err := h.RequestPasswordReset(context.Background(), &pb.PasswordResetRequest{Email: repo.user.Email})
	if err != nil {
		t.Fatalf("RequestPasswordReset() error = %v", err)
	}
	if !response.GetAccepted() || response.GetExpiresAt() == 0 {
		t.Fatalf("response = %+v", response)
	}
	if response.ProtoReflect().Descriptor().Fields().ByName("reset_token") != nil {
		t.Fatal("gRPC password reset response still exposes a reset_token field")
	}
	if emails.token == "" {
		t.Fatal("password reset email did not receive a raw token")
	}
	if repo.token.TokenHash == "" || repo.token.TokenHash == emails.token {
		t.Fatal("password reset token was not stored as a hash")
	}
}

type passwordResetHandlerRepo struct {
	domain.Repository
	user  *domain.User
	token domain.PasswordResetToken
}

func (r *passwordResetHandlerRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if email != r.user.Email {
		return nil, domain.ErrNotFound
	}
	return r.user, nil
}

func (r *passwordResetHandlerRepo) CreatePasswordResetToken(_ context.Context, token domain.PasswordResetToken) error {
	r.token = token
	return nil
}

type passwordResetHandlerEmails struct {
	token string
}

func (*passwordResetHandlerEmails) Ready() bool { return true }

func (e *passwordResetHandlerEmails) SendPasswordReset(_ context.Context, _ string, token string, _ time.Time) error {
	e.token = token
	return nil
}

func (*passwordResetHandlerEmails) SendEmailVerification(context.Context, string, string, time.Time) error {
	return nil
}

type handlerIDGen struct {
	next int64
}

func (g *handlerIDGen) Generate() int64 {
	g.next++
	return g.next
}

type userListHandlerRepo struct {
	domain.Repository
	users          map[int64]*domain.User
	lists          map[int64]*domain.UserList
	members        map[int64][]int64
	favorites      map[int64]map[int64]bool
	listQuery      domain.UserListsQuery
	membersQuery   domain.UserListMembersQuery
	favoritesQuery domain.UserListFavoritesQuery
}

func (r *userListHandlerRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (r *userListHandlerRepo) CreateUserList(_ context.Context, list *domain.UserList) error {
	r.lists[list.ID] = cloneHandlerList(list)
	return nil
}

func (r *userListHandlerRepo) UpdateUserList(_ context.Context, list *domain.UserList) error {
	if _, ok := r.lists[list.ID]; !ok {
		return domain.ErrUserListNotFound
	}
	r.lists[list.ID] = cloneHandlerList(list)
	return nil
}

func (r *userListHandlerRepo) DeleteUserList(_ context.Context, ownerID, listID int64) error {
	list, ok := r.lists[listID]
	if !ok || list.OwnerID != ownerID {
		return domain.ErrUserListNotFound
	}
	delete(r.lists, listID)
	return nil
}

func (r *userListHandlerRepo) GetUserList(_ context.Context, viewerID, listID int64) (*domain.UserList, error) {
	list, ok := r.lists[listID]
	if !ok || (!list.IsPublic && list.OwnerID != viewerID) {
		return nil, domain.ErrUserListNotFound
	}
	copy := cloneHandlerList(list)
	copy.MemberCount = int64(len(r.members[listID]))
	if r.favorites[viewerID] != nil && r.favorites[viewerID][listID] {
		copy.IsFavorited = true
	}
	for _, favorites := range r.favorites {
		if favorites[listID] {
			copy.FavoriteCount++
		}
	}
	return copy, nil
}

func (r *userListHandlerRepo) ListUserLists(ctx context.Context, q domain.UserListsQuery) ([]*domain.UserList, int64, error) {
	r.listQuery = q
	items := make([]*domain.UserList, 0)
	for id, list := range r.lists {
		if list.OwnerID == q.OwnerID && (list.IsPublic || q.ViewerID == q.OwnerID) {
			item, _ := r.GetUserList(ctx, q.ViewerID, id)
			items = append(items, item)
		}
	}
	return items, int64(len(items)), nil
}

func (r *userListHandlerRepo) ListFavoriteUserLists(ctx context.Context, q domain.UserListFavoritesQuery) ([]*domain.UserList, int64, error) {
	r.favoritesQuery = q
	items := make([]*domain.UserList, 0)
	for listID := range r.favorites[q.UserID] {
		if list, ok := r.lists[listID]; ok && list.IsPublic {
			item, _ := r.GetUserList(ctx, q.UserID, listID)
			items = append(items, item)
		}
	}
	return items, int64(len(items)), nil
}

func (r *userListHandlerRepo) AddUserListMember(_ context.Context, _ int64, membership domain.UserListMembership) error {
	if r.members == nil {
		r.members = make(map[int64][]int64)
	}
	r.members[membership.ListID] = append(r.members[membership.ListID], membership.UserID)
	return nil
}

func (r *userListHandlerRepo) RemoveUserListMember(_ context.Context, _, listID, userID int64) error {
	for i, id := range r.members[listID] {
		if id == userID {
			r.members[listID] = append(r.members[listID][:i], r.members[listID][i+1:]...)
			return nil
		}
	}
	return domain.ErrUserListMemberNotFound
}

func (r *userListHandlerRepo) ListUserListMembers(_ context.Context, q domain.UserListMembersQuery) ([]*domain.User, int64, error) {
	r.membersQuery = q
	items := make([]*domain.User, 0, len(r.members[q.ListID]))
	for _, id := range r.members[q.ListID] {
		items = append(items, r.users[id])
	}
	return items, int64(len(items)), nil
}

func (r *userListHandlerRepo) CopyUserList(_ context.Context, sourceListID int64, target *domain.UserList) error {
	if _, ok := r.lists[sourceListID]; !ok {
		return domain.ErrUserListNotFound
	}
	r.lists[target.ID] = cloneHandlerList(target)
	r.members[target.ID] = append([]int64(nil), r.members[sourceListID]...)
	return nil
}

func (r *userListHandlerRepo) FavoriteUserList(_ context.Context, favorite domain.UserListFavorite) error {
	if r.favorites == nil {
		r.favorites = make(map[int64]map[int64]bool)
	}
	if r.favorites[favorite.UserID] == nil {
		r.favorites[favorite.UserID] = make(map[int64]bool)
	}
	r.favorites[favorite.UserID][favorite.ListID] = true
	return nil
}

func (r *userListHandlerRepo) UnfavoriteUserList(_ context.Context, userID, listID int64) error {
	delete(r.favorites[userID], listID)
	return nil
}

func cloneHandlerList(list *domain.UserList) *domain.UserList {
	copy := *list
	return &copy
}
