package grpc

import (
	"context"
	"errors"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	domain "user-service/internal/domain/user"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedUserServiceServer
	cmd *command.Service
	qry *query.Service
}

func NewHandler(cmd *command.Service, qry *query.Service) *Handler {
	return &Handler{cmd: cmd, qry: qry}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrUsernameExists), errors.Is(err, domain.ErrEmailExists), errors.Is(err, domain.ErrAlreadyFollowing):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrMuted), errors.Is(err, domain.ErrNotFollowing), errors.Is(err, domain.ErrResetTokenExpired), errors.Is(err, domain.ErrEmailVerificationTokenExpired):
		code = codes.FailedPrecondition
	case errors.Is(err, domain.ErrProfileThemeEntitlementRequired), errors.Is(err, domain.ErrProfileBackgroundEntitlementRequired):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrSecurityEmailDeliveryUnavailable):
		code = codes.Unavailable
	case errors.Is(err, domain.ErrInvalidID),
		errors.Is(err, domain.ErrUsernameRequired),
		errors.Is(err, domain.ErrUsernameInvalid),
		errors.Is(err, domain.ErrEmailRequired),
		errors.Is(err, domain.ErrEmailInvalid),
		errors.Is(err, domain.ErrPasswordRequired),
		errors.Is(err, domain.ErrPasswordTooShort),
		errors.Is(err, domain.ErrResetTokenInvalid),
		errors.Is(err, domain.ErrEmailVerificationTokenInvalid),
		errors.Is(err, domain.ErrNicknameRequired),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidOAuth),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidProfileTheme),
		errors.Is(err, domain.ErrCannotFollowSelf):
		code = codes.InvalidArgument
	}
	return status.Error(code, err.Error())
}

func toPb(u *domain.User) *pb.UserInfo {
	if u == nil {
		return nil
	}
	var lastLoginAt int64
	if u.LastLoginAt != nil {
		lastLoginAt = u.LastLoginAt.UnixMilli()
	}
	var emailVerifiedAt int64
	if u.EmailVerifiedAt != nil {
		emailVerifiedAt = u.EmailVerifiedAt.UnixMilli()
	}
	return &pb.UserInfo{
		Id:              u.ID,
		Username:        u.Username,
		Email:           u.Email,
		Nickname:        u.Nickname,
		AvatarUrl:       u.AvatarURL,
		BackgroundUrl:   u.BackgroundURL,
		ProfileTheme:    u.ProfileTheme,
		Bio:             u.Bio,
		Status:          int32(u.Status),
		FollowerCount:   u.FollowerCount,
		FollowingCount:  u.FollowingCount,
		CreatedAt:       u.CreatedAt.UnixMilli(),
		UpdatedAt:       u.UpdatedAt.UnixMilli(),
		LastLoginAt:     lastLoginAt,
		EmailVerified:   u.EmailVerifiedAt != nil,
		EmailVerifiedAt: emailVerifiedAt,
	}
}

func toPbList(rows []*domain.User) []*pb.UserInfo {
	out := make([]*pb.UserInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPb(row))
	}
	return out
}

func (h *Handler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.Register(ctx, domain.RegisterCmd{
		Username: req.GetUsername(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
		Nickname: req.GetNickname(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AuthResponse{Success: true, Message: "ok", User: toPb(u), AccessToken: token.Value, ExpiresAt: token.ExpiresAt.UnixMilli()}, nil
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.Login(ctx, req.GetAccount(), req.GetPassword())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AuthResponse{Success: true, Message: "ok", User: toPb(u), AccessToken: token.Value, ExpiresAt: token.ExpiresAt.UnixMilli()}, nil
}

func (h *Handler) OAuthLogin(ctx context.Context, req *pb.OAuthLoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       req.GetProvider(),
		ProviderUserID: req.GetProviderUserId(),
		Username:       req.GetUsername(),
		Email:          req.GetEmail(),
		Nickname:       req.GetNickname(),
		AvatarURL:      req.GetAvatarUrl(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AuthResponse{Success: true, Message: "ok", User: toPb(u), AccessToken: token.Value, ExpiresAt: token.ExpiresAt.UnixMilli()}, nil
}

func (h *Handler) WebmasterLogin(ctx context.Context, req *pb.WebmasterLoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.WebmasterLogin(ctx, domain.WebmasterLoginCmd{
		Username: req.GetUsername(),
		Password: req.GetPassword(),
		Email:    req.GetEmail(),
		Nickname: req.GetNickname(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AuthResponse{Success: true, Message: "ok", User: toPb(u), AccessToken: token.Value, ExpiresAt: token.ExpiresAt.UnixMilli()}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *pb.UserIDRequest) (*pb.UserResponse, error) {
	u, err := h.qry.Get(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPb(u)}, nil
}

func (h *Handler) GetUserByUsername(ctx context.Context, req *pb.UsernameRequest) (*pb.UserResponse, error) {
	u, err := h.qry.GetByUsername(ctx, req.GetUsername())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPb(u)}, nil
}

func (h *Handler) GetCredentialVersion(ctx context.Context, req *pb.UserIDRequest) (*pb.CredentialVersionResponse, error) {
	version, err := h.qry.GetCredentialVersion(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CredentialVersionResponse{UserId: req.GetId(), CredentialVersion: version}, nil
}

func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListUsers(ctx, domain.UserListQuery{
		Query:    req.GetQuery(),
		Status:   req.GetStatus(),
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
		IDs:      req.GetIds(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UserResponse, error) {
	u, err := h.cmd.UpdateProfile(ctx, req.GetId(), domain.UpdateProfileCmd{
		Nickname:      req.GetNickname(),
		AvatarURL:     req.GetAvatarUrl(),
		BackgroundURL: req.GetBackgroundUrl(),
		ProfileTheme:  req.GetProfileTheme(),
		Bio:           req.GetBio(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPb(u)}, nil
}

func (h *Handler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.ChangePassword(ctx, req.GetId(), req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) RequestPasswordReset(ctx context.Context, req *pb.PasswordResetRequest) (*pb.PasswordResetResponse, error) {
	result, err := h.cmd.RequestPasswordReset(ctx, req.GetEmail())
	if err != nil {
		return nil, toStatus(err)
	}
	// The raw reset token is intentionally retained only inside the command
	// service long enough to send the security email. It must never cross the
	// gRPC boundary, where it could be logged or exposed by another caller.
	return &pb.PasswordResetResponse{Accepted: result.Accepted, ExpiresAt: result.ExpiresAt.UnixMilli()}, nil
}

func (h *Handler) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.ResetPassword(ctx, req.GetToken(), req.GetNewPassword()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) RequestEmailVerification(ctx context.Context, req *pb.EmailVerificationRequest) (*pb.EmailVerificationResponse, error) {
	result, err := h.cmd.RequestEmailVerification(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.EmailVerificationResponse{
		Accepted:          result.Accepted,
		VerificationToken: result.VerificationToken,
		ExpiresAt:         result.ExpiresAt.UnixMilli(),
		AlreadyVerified:   result.AlreadyVerified,
	}, nil
}

func (h *Handler) VerifyEmail(ctx context.Context, req *pb.VerifyEmailRequest) (*pb.UserResponse, error) {
	u, err := h.cmd.VerifyEmail(ctx, req.GetToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPb(u)}, nil
}

func (h *Handler) UpdateStatus(ctx context.Context, req *pb.UpdateStatusRequest) (*pb.UserResponse, error) {
	u, err := h.cmd.UpdateStatus(ctx, req.GetId(), domain.Status(req.GetStatus()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserResponse{Success: true, Message: "ok", User: toPb(u)}, nil
}

func (h *Handler) Follow(ctx context.Context, req *pb.FollowRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Follow(ctx, req.GetFollowerId(), req.GetFolloweeId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) Unfollow(ctx context.Context, req *pb.FollowRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Unfollow(ctx, req.GetFollowerId(), req.GetFolloweeId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) IsFollowing(ctx context.Context, req *pb.FollowRequest) (*pb.IsFollowingResponse, error) {
	ok, err := h.qry.IsFollowing(ctx, req.GetFollowerId(), req.GetFolloweeId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.IsFollowingResponse{Following: ok}, nil
}

func (h *Handler) ListFollowers(ctx context.Context, req *pb.ListFollowsRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListFollowers(ctx, domain.FollowListQuery{UserID: req.GetUserId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize())})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListFollowing(ctx context.Context, req *pb.ListFollowsRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListFollowing(ctx, domain.FollowListQuery{UserID: req.GetUserId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize())})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}
