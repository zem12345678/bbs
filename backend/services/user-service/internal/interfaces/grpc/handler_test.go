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
