package http

import (
	"context"
	"errors"
	"testing"

	realtimechat "api-gateway/internal/realtime/chat"

	"github.com/stretchr/testify/require"
)

func TestChatSessionValidatorRejectsRevokedAndRotatedCredentials(t *testing.T) {
	revocations := &fakeTokenRevocationStore{}
	versions := &fakeCredentialVersionStore{version: "credential-v2"}
	handler := NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores(
		nil, "Authorization", "Bearer", testJWTSecret, nil, nil, nil, nil, revocations, versions,
	)
	ticket := realtimechat.Ticket{
		UserID:                 42,
		TokenFingerprint:       tokenRevocationFingerprint("access-token"),
		CredentialVersion:      "credential-v2",
		CredentialVersionClaim: true,
	}

	require.NoError(t, handler.ValidateChatSession(context.Background(), ticket))
	revocations.revokedToken = "access-token"
	require.ErrorIs(t, handler.ValidateChatSession(context.Background(), ticket), realtimechat.ErrSessionInvalid)

	revocations.revokedToken = ""
	versions.version = "credential-v3"
	require.ErrorIs(t, handler.ValidateChatSession(context.Background(), ticket), realtimechat.ErrSessionInvalid)

	versions.err = errors.New("credential authority unavailable")
	require.ErrorIs(t, handler.ValidateChatSession(context.Background(), ticket), realtimechat.ErrSessionValidationUnavailable)
}
