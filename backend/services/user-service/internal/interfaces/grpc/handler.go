package grpc

import (
	"context"
	"errors"
	"time"

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
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrUserListNotFound), errors.Is(err, domain.ErrUserListMemberNotFound), errors.Is(err, domain.ErrUserListFavoriteNotFound), errors.Is(err, domain.ErrPasskeyNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrUsernameExists), errors.Is(err, domain.ErrEmailExists), errors.Is(err, domain.ErrAlreadyFollowing), errors.Is(err, domain.ErrAlreadyBlocking), errors.Is(err, domain.ErrAlreadyMuted), errors.Is(err, domain.ErrUserListNameExists), errors.Is(err, domain.ErrUserListMemberExists), errors.Is(err, domain.ErrUserListFavoriteExists), errors.Is(err, domain.ErrPasskeyCredentialExists):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrInviteCodeExists):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrMuted), errors.Is(err, domain.ErrNotFollowing), errors.Is(err, domain.ErrNotBlocking), errors.Is(err, domain.ErrNotMuted), errors.Is(err, domain.ErrFollowBlocked), errors.Is(err, domain.ErrResetTokenExpired), errors.Is(err, domain.ErrEmailVerificationTokenExpired), errors.Is(err, domain.ErrInviteCodeExpired), errors.Is(err, domain.ErrInviteCodeUsed), errors.Is(err, domain.ErrInviteCodeRevoked), errors.Is(err, domain.ErrUserListLimitReached), errors.Is(err, domain.ErrUserListMemberLimitReached), errors.Is(err, domain.ErrUserListMemberBlocked), errors.Is(err, domain.ErrMFAEnrollmentNotStarted), errors.Is(err, domain.ErrMFAAlreadyEnabled), errors.Is(err, domain.ErrMFANotEnabled), errors.Is(err, domain.ErrPasskeyLimitReached), errors.Is(err, domain.ErrPasskeyPasswordlessUnavailable), errors.Is(err, domain.ErrPasskeyCredentialChanged), errors.Is(err, domain.ErrAccountSuspended), errors.Is(err, domain.ErrAccountDeletionPending), errors.Is(err, domain.ErrAccountAnonymized), errors.Is(err, domain.ErrAccountLifecycleChanged):
		code = codes.FailedPrecondition
	case errors.Is(err, domain.ErrMFACodeInvalid), errors.Is(err, domain.ErrMFACodeReplayed), errors.Is(err, domain.ErrMFAChallengeInvalid), errors.Is(err, domain.ErrMFAChallengeExpired), errors.Is(err, domain.ErrMFAChallengeAttemptsExceeded), errors.Is(err, domain.ErrPasskeyChallengeInvalid), errors.Is(err, domain.ErrPasskeyChallengeExpired), errors.Is(err, domain.ErrPasskeyChallengeAttemptsExceeded), errors.Is(err, domain.ErrPasskeyVerificationFailed):
		code = codes.Unauthenticated
	case errors.Is(err, domain.ErrInviteCodeNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrProfileThemeEntitlementRequired), errors.Is(err, domain.ErrProfileBackgroundEntitlementRequired), errors.Is(err, domain.ErrAccountProtected):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrOAuthSignupDisabled):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrSecurityEmailDeliveryUnavailable), errors.Is(err, domain.ErrSafetyRepositoryUnavailable), errors.Is(err, domain.ErrInviteRepositoryUnavailable), errors.Is(err, domain.ErrUserListRepositoryUnavailable), errors.Is(err, domain.ErrMFARepositoryUnavailable), errors.Is(err, domain.ErrMFAEncryptionUnavailable), errors.Is(err, domain.ErrPasskeyRepositoryUnavailable), errors.Is(err, domain.ErrPasskeyManagerUnavailable), errors.Is(err, domain.ErrAccountLifecycleRepositoryUnavailable):
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
		errors.Is(err, domain.ErrInvalidAccountState),
		errors.Is(err, domain.ErrInvalidProfileTheme),
		errors.Is(err, domain.ErrCannotFollowSelf),
		errors.Is(err, domain.ErrCannotRelateSelf),
		errors.Is(err, domain.ErrInviteCodeRequired),
		errors.Is(err, domain.ErrInviteCodeInvalid),
		errors.Is(err, domain.ErrInviteCountInvalid),
		errors.Is(err, domain.ErrInviteStatusInvalid),
		errors.Is(err, domain.ErrInviteExpiryInvalid),
		errors.Is(err, domain.ErrUserListNameRequired),
		errors.Is(err, domain.ErrUserListNameTooLong),
		errors.Is(err, domain.ErrPasskeyNameRequired),
		errors.Is(err, domain.ErrPasskeyNameTooLong):
		code = codes.InvalidArgument
	}
	return status.Error(code, err.Error())
}

func toAccountVerificationStatus(err error) error {
	mapped := toStatus(err)
	if status.Code(mapped) == codes.Unauthenticated {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return mapped
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
		AccountState:    string(domain.NormalizeAccountState(u.AccountState)),
	}
}

func toPbList(rows []*domain.User) []*pb.UserInfo {
	out := make([]*pb.UserInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPb(row))
	}
	return out
}

func toPbUserList(list *domain.UserList) *pb.UserListInfo {
	if list == nil {
		return nil
	}
	return &pb.UserListInfo{
		Id:            list.ID,
		OwnerId:       list.OwnerID,
		Name:          list.Name,
		IsPublic:      list.IsPublic,
		MemberCount:   list.MemberCount,
		FavoriteCount: list.FavoriteCount,
		IsFavorited:   list.IsFavorited,
		CreatedAt:     list.CreatedAt.UnixMilli(),
		UpdatedAt:     list.UpdatedAt.UnixMilli(),
	}
}

func toPbUserLists(rows []*domain.UserList) []*pb.UserListInfo {
	out := make([]*pb.UserListInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPbUserList(row))
	}
	return out
}

func (h *Handler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.Register(ctx, domain.RegisterCmd{
		Username:      req.GetUsername(),
		Email:         req.GetEmail(),
		Password:      req.GetPassword(),
		Nickname:      req.GetNickname(),
		InviteCode:    req.GetInviteCode(),
		RequireInvite: req.GetRequireInvite(),
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
	return toPbAuthResponse(u, token), nil
}

func toPbAuthResponse(u *domain.User, token command.AuthToken) *pb.AuthResponse {
	var expiresAt int64
	if !token.ExpiresAt.IsZero() {
		expiresAt = token.ExpiresAt.UnixMilli()
	}
	var mfaExpiresAt int64
	if !token.MFAChallengeExpiry.IsZero() {
		mfaExpiresAt = token.MFAChallengeExpiry.UnixMilli()
	}
	return &pb.AuthResponse{
		Success:      true,
		Message:      "ok",
		User:         toPb(u),
		AccessToken:  token.Value,
		ExpiresAt:    expiresAt,
		MfaRequired:  token.MFARequired,
		MfaChallenge: token.MFAChallenge,
		MfaExpiresAt: mfaExpiresAt,
	}
}

func (h *Handler) OAuthLogin(ctx context.Context, req *pb.OAuthLoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.OAuthLogin(ctx, domain.OAuthLoginCmd{
		Provider:       req.GetProvider(),
		ProviderUserID: req.GetProviderUserId(),
		Username:       req.GetUsername(),
		Email:          req.GetEmail(),
		Nickname:       req.GetNickname(),
		AvatarURL:      req.GetAvatarUrl(),
		ExistingOnly:   req.GetExistingOnly(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return toPbAuthResponse(u, token), nil
}

func toPbInvite(code domain.InviteCode, now time.Time) *pb.InviteCodeInfo {
	var usedByUserID, expiresAt, usedAt, revokedAt, revokedByAdminID int64
	if code.UsedByUserID != nil {
		usedByUserID = *code.UsedByUserID
	}
	if code.ExpiresAt != nil {
		expiresAt = code.ExpiresAt.Unix()
	}
	if code.UsedAt != nil {
		usedAt = code.UsedAt.Unix()
	}
	if code.RevokedAt != nil {
		revokedAt = code.RevokedAt.Unix()
	}
	if code.RevokedByAdminID != nil {
		revokedByAdminID = *code.RevokedByAdminID
	}
	return &pb.InviteCodeInfo{
		Id:               code.ID,
		Code:             code.Code,
		CreatedByAdminId: code.CreatedByAdminID,
		UsedByUserId:     usedByUserID,
		ExpiresAt:        expiresAt,
		UsedAt:           usedAt,
		RevokedAt:        revokedAt,
		RevokedByAdminId: revokedByAdminID,
		CreatedAt:        code.CreatedAt.Unix(),
		Status:           code.StatusAt(now),
	}
}

func toPbInvites(codes []domain.InviteCode) []*pb.InviteCodeInfo {
	now := time.Now()
	items := make([]*pb.InviteCodeInfo, 0, len(codes))
	for _, code := range codes {
		items = append(items, toPbInvite(code, now))
	}
	return items
}

func (h *Handler) CreateInviteCodes(ctx context.Context, req *pb.CreateInviteCodesRequest) (*pb.InviteCodeListResponse, error) {
	var expiresAt *time.Time
	if req.GetExpiresAt() != 0 {
		value := time.Unix(req.GetExpiresAt(), 0)
		expiresAt = &value
	}
	items, err := h.cmd.CreateInviteCodes(ctx, req.GetActorId(), int64(req.GetCount()), expiresAt)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.InviteCodeListResponse{Items: toPbInvites(items), Total: int64(len(items))}, nil
}

func (h *Handler) ListInviteCodes(ctx context.Context, req *pb.ListInviteCodesRequest) (*pb.InviteCodeListResponse, error) {
	items, total, err := h.cmd.ListInviteCodes(ctx, domain.InviteCodeListQuery{
		Status: req.GetStatus(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.InviteCodeListResponse{Items: toPbInvites(items), Total: total}, nil
}

func (h *Handler) RevokeInviteCode(ctx context.Context, req *pb.RevokeInviteCodeRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.RevokeInviteCode(ctx, req.GetActorId(), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
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

func (h *Handler) GetMFAStatus(ctx context.Context, req *pb.UserIDRequest) (*pb.MFAStatusResponse, error) {
	result, err := h.cmd.MFAStatus(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	var enabledAt int64
	if !result.EnabledAt.IsZero() {
		enabledAt = result.EnabledAt.UnixMilli()
	}
	return &pb.MFAStatusResponse{
		Enabled:                result.Enabled,
		RecoveryCodesRemaining: result.RecoveryCodesRemaining,
		EnabledAt:              enabledAt,
	}, nil
}

func (h *Handler) BeginTOTPEnrollment(ctx context.Context, req *pb.BeginTOTPEnrollmentRequest) (*pb.TOTPEnrollmentResponse, error) {
	enrollment, err := h.cmd.BeginTOTPEnrollment(ctx, req.GetUserId(), req.GetPassword(), req.GetCurrentCode())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.TOTPEnrollmentResponse{
		Secret:     enrollment.Secret,
		OtpauthUrl: enrollment.URL,
		QrDataUrl:  enrollment.QRDataURL,
		Issuer:     enrollment.Issuer,
		Account:    enrollment.Account,
	}, nil
}

func (h *Handler) ConfirmTOTPEnrollment(ctx context.Context, req *pb.ConfirmTOTPEnrollmentRequest) (*pb.MFARecoveryCodesResponse, error) {
	codes, err := h.cmd.ConfirmTOTPEnrollment(ctx, req.GetUserId(), req.GetCode())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.MFARecoveryCodesResponse{RecoveryCodes: codes}, nil
}

func (h *Handler) RegenerateMFARecoveryCodes(ctx context.Context, req *pb.MFAReauthenticateRequest) (*pb.MFARecoveryCodesResponse, error) {
	codes, err := h.cmd.RegenerateMFARecoveryCodes(ctx, req.GetUserId(), req.GetPassword(), req.GetCode())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.MFARecoveryCodesResponse{RecoveryCodes: codes}, nil
}

func (h *Handler) DisableTOTP(ctx context.Context, req *pb.MFAReauthenticateRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DisableTOTP(ctx, req.GetUserId(), req.GetPassword(), req.GetCode()); err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) CompleteMFALogin(ctx context.Context, req *pb.CompleteMFALoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.CompleteMFALogin(ctx, req.GetChallenge(), req.GetCode())
	if err != nil {
		return nil, toStatus(err)
	}
	return toPbAuthResponse(u, token), nil
}

func (h *Handler) ListPasskeys(ctx context.Context, req *pb.UserIDRequest) (*pb.PasskeyListResponse, error) {
	result, err := h.cmd.ListPasskeys(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.PasskeyInfo, 0, len(result.Credentials))
	for _, credential := range result.Credentials {
		items = append(items, toPBPasskey(credential))
	}
	return &pb.PasskeyListResponse{Items: items, PasswordlessEnabled: result.PasswordlessEnabled, Limit: domain.PasskeyMaxCredentials}, nil
}

func (h *Handler) BeginPasskeyRegistration(ctx context.Context, req *pb.BeginPasskeyRegistrationRequest) (*pb.PasskeyOptionsResponse, error) {
	result, err := h.cmd.BeginPasskeyRegistration(ctx, req.GetUserId(), req.GetName(), req.GetPassword(), req.GetCode())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return toPBPasskeyOptions(result), nil
}

func (h *Handler) FinishPasskeyRegistration(ctx context.Context, req *pb.FinishPasskeyRegistrationRequest) (*pb.PasskeyInfoResponse, error) {
	credential, err := h.cmd.FinishPasskeyRegistration(ctx, req.GetUserId(), req.GetChallenge(), req.GetCredentialJson())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.PasskeyInfoResponse{Success: true, Message: "passkey registered", Passkey: toPBPasskey(credential)}, nil
}

func (h *Handler) UpdatePasskey(ctx context.Context, req *pb.UpdatePasskeyRequest) (*pb.PasskeyInfoResponse, error) {
	credential, err := h.cmd.UpdatePasskeyName(ctx, req.GetUserId(), req.GetCredentialId(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.PasskeyInfoResponse{Success: true, Message: "passkey updated", Passkey: toPBPasskey(credential)}, nil
}

func (h *Handler) DeletePasskey(ctx context.Context, req *pb.DeletePasskeyRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeletePasskey(ctx, req.GetUserId(), req.GetCredentialId(), req.GetPassword(), req.GetCode()); err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "passkey deleted"}, nil
}

func (h *Handler) SetPasskeyPasswordless(ctx context.Context, req *pb.SetPasskeyPasswordlessRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.SetPasskeyPasswordless(ctx, req.GetUserId(), req.GetEnabled(), req.GetPassword(), req.GetCode()); err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "passkey passwordless setting updated"}, nil
}

func (h *Handler) BeginPasskeyMFALogin(ctx context.Context, req *pb.BeginPasskeyMFALoginRequest) (*pb.PasskeyOptionsResponse, error) {
	result, err := h.cmd.BeginPasskeyMFALogin(ctx, req.GetMfaChallenge())
	if err != nil {
		return nil, toStatus(err)
	}
	return toPBPasskeyOptions(result), nil
}

func (h *Handler) CompletePasskeyMFALogin(ctx context.Context, req *pb.CompletePasskeyLoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.CompletePasskeyMFALogin(ctx, req.GetChallenge(), req.GetCredentialJson())
	if err != nil {
		return nil, toStatus(err)
	}
	return toPbAuthResponse(u, token), nil
}

func (h *Handler) BeginPasswordlessPasskeyLogin(ctx context.Context, _ *pb.PasswordlessPasskeyOptionsRequest) (*pb.PasskeyOptionsResponse, error) {
	result, err := h.cmd.BeginPasswordlessPasskeyLogin(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	return toPBPasskeyOptions(result), nil
}

func (h *Handler) CompletePasswordlessPasskeyLogin(ctx context.Context, req *pb.CompletePasskeyLoginRequest) (*pb.AuthResponse, error) {
	u, token, err := h.cmd.CompletePasswordlessPasskeyLogin(ctx, req.GetChallenge(), req.GetCredentialJson())
	if err != nil {
		return nil, toStatus(err)
	}
	return toPbAuthResponse(u, token), nil
}

func (h *Handler) GetAccountLifecycle(ctx context.Context, req *pb.UserIDRequest) (*pb.AccountLifecycleResponse, error) {
	result, err := h.cmd.GetAccountLifecycle(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toPBAccountLifecycle(result), nil
}

func (h *Handler) RequestAccountDeletion(ctx context.Context, req *pb.RequestAccountDeletionRequest) (*pb.AccountLifecycleResponse, error) {
	result, err := h.cmd.RequestAccountDeletion(ctx, req.GetUserId(), req.GetPassword(), req.GetCode())
	if err != nil {
		return nil, toAccountVerificationStatus(err)
	}
	return toPBAccountLifecycle(result), nil
}

func toPBAccountLifecycle(result domain.AccountLifecycle) *pb.AccountLifecycleResponse {
	response := &pb.AccountLifecycleResponse{
		UserId: result.UserID, State: string(result.State), StateVersion: result.StateVersion, Protected: result.Protected,
	}
	if result.DeletionRequestedAt != nil {
		response.DeletionRequestedAt = result.DeletionRequestedAt.UnixMilli()
	}
	if result.DeletedAt != nil {
		response.DeletedAt = result.DeletedAt.UnixMilli()
	}
	if job := result.ActiveDeletionJob; job != nil {
		response.ActiveDeletionJob = &pb.AccountDeletionJobInfo{
			Id: job.ID, Status: string(job.Status), PolicyVersion: job.PolicyVersion,
			CompletedSteps: job.CompletedSteps, TotalSteps: job.TotalSteps,
			CreatedAt: job.CreatedAt.UnixMilli(), UpdatedAt: job.UpdatedAt.UnixMilli(),
		}
		if job.StartedAt != nil {
			response.ActiveDeletionJob.StartedAt = job.StartedAt.UnixMilli()
		}
		if job.CompletedAt != nil {
			response.ActiveDeletionJob.CompletedAt = job.CompletedAt.UnixMilli()
		}
	}
	return response
}

func toPBPasskeyOptions(result command.PasskeyOptionsResult) *pb.PasskeyOptionsResponse {
	return &pb.PasskeyOptionsResponse{Challenge: result.Challenge, OptionsJson: result.OptionsJSON, ExpiresAt: result.ExpiresAt.UnixMilli()}
}

func toPBPasskey(credential domain.PasskeyCredential) *pb.PasskeyInfo {
	var lastUsedAt int64
	if credential.LastUsedAt != nil {
		lastUsedAt = credential.LastUsedAt.UnixMilli()
	}
	return &pb.PasskeyInfo{
		CredentialId:   credential.CredentialID,
		Name:           credential.Name,
		BackupEligible: credential.BackupEligible,
		BackupState:    credential.BackupState,
		CreatedAt:      credential.CreatedAt.UnixMilli(),
		UpdatedAt:      credential.UpdatedAt.UnixMilli(),
		LastUsedAt:     lastUsedAt,
	}
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

func (h *Handler) Block(ctx context.Context, req *pb.UserRelationRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Block(ctx, req.GetActorId(), req.GetTargetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) Unblock(ctx context.Context, req *pb.UserRelationRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Unblock(ctx, req.GetActorId(), req.GetTargetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) Mute(ctx context.Context, req *pb.UserRelationRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Mute(ctx, req.GetActorId(), req.GetTargetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) Unmute(ctx context.Context, req *pb.UserRelationRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.Unmute(ctx, req.GetActorId(), req.GetTargetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) GetSafetyRelation(ctx context.Context, req *pb.UserRelationRequest) (*pb.SafetyRelationResponse, error) {
	relation, err := h.qry.GetSafetyRelation(ctx, req.GetActorId(), req.GetTargetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SafetyRelationResponse{Blocked: relation.Blocked, BlockedBy: relation.BlockedBy, Muted: relation.Muted}, nil
}

func (h *Handler) ListBlockedUsers(ctx context.Context, req *pb.ListUserRelationsRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListBlockedUsers(ctx, domain.FollowListQuery{UserID: req.GetActorId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize())})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListMutedUsers(ctx context.Context, req *pb.ListUserRelationsRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListMutedUsers(ctx, domain.FollowListQuery{UserID: req.GetActorId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize())})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) CreateUserList(ctx context.Context, req *pb.CreateUserListRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.cmd.CreateUserList(ctx, req.GetOwnerId(), req.GetName(), req.GetIsPublic())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}

func (h *Handler) UpdateUserList(ctx context.Context, req *pb.UpdateUserListRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.cmd.UpdateUserList(ctx, req.GetOwnerId(), req.GetListId(), req.GetName(), req.GetIsPublic())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}

func (h *Handler) DeleteUserList(ctx context.Context, req *pb.DeleteUserListRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeleteUserList(ctx, req.GetOwnerId(), req.GetListId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) GetUserList(ctx context.Context, req *pb.GetUserListRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.qry.GetUserList(ctx, req.GetViewerId(), req.GetListId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}

func (h *Handler) ListUserLists(ctx context.Context, req *pb.ListUserListsRequest) (*pb.UserListsResponse, error) {
	result, err := h.qry.ListUserLists(ctx, domain.UserListsQuery{
		ViewerID: req.GetViewerId(), OwnerID: req.GetOwnerId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListsResponse{Items: toPbUserLists(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListFavoriteUserLists(ctx context.Context, req *pb.ListFavoriteUserListsRequest) (*pb.UserListsResponse, error) {
	result, err := h.qry.ListFavoriteUserLists(ctx, domain.UserListFavoritesQuery{
		UserID: req.GetUserId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListsResponse{Items: toPbUserLists(result.Items), Total: result.Total}, nil
}

func (h *Handler) AddUserListMember(ctx context.Context, req *pb.UserListMemberRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.AddUserListMember(ctx, req.GetOwnerId(), req.GetListId(), req.GetUserId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) RemoveUserListMember(ctx context.Context, req *pb.UserListMemberRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.RemoveUserListMember(ctx, req.GetOwnerId(), req.GetListId(), req.GetUserId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListUserListMembers(ctx context.Context, req *pb.ListUserListMembersRequest) (*pb.UserListResponse, error) {
	result, err := h.qry.ListUserListMembers(ctx, domain.UserListMembersQuery{
		ViewerID: req.GetViewerId(), ListID: req.GetListId(), Page: int(req.GetPage()), PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) CopyUserList(ctx context.Context, req *pb.CopyUserListRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.cmd.CopyUserList(ctx, req.GetOwnerId(), req.GetSourceListId(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}

func (h *Handler) FavoriteUserList(ctx context.Context, req *pb.UserListFavoriteRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.cmd.FavoriteUserList(ctx, req.GetUserId(), req.GetListId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}

func (h *Handler) UnfavoriteUserList(ctx context.Context, req *pb.UserListFavoriteRequest) (*pb.UserListInfoResponse, error) {
	list, err := h.cmd.UnfavoriteUserList(ctx, req.GetUserId(), req.GetListId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UserListInfoResponse{Success: true, Message: "ok", UserList: toPbUserList(list)}, nil
}
