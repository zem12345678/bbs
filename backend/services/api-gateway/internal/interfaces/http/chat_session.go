package http

import (
	"context"
	"errors"
	"fmt"
	"strings"

	realtimechat "api-gateway/internal/realtime/chat"
)

// ValidateChatSession implements the realtime chat session validator. The
// ticket carries only a one-way token fingerprint, so an already-connected
// socket can be invalidated after logout without retaining a bearer token.
func (h *Handler) ValidateChatSession(ctx context.Context, ticket realtimechat.Ticket) error {
	if h == nil || ticket.UserID <= 0 || strings.TrimSpace(ticket.TokenFingerprint) == "" {
		return realtimechat.ErrSessionInvalid
	}
	if h.tokenRevocations != nil {
		revoked, err := h.tokenRevocations.IsRevokedFingerprint(ctx, ticket.TokenFingerprint)
		if err != nil {
			return fmt.Errorf("%w: token revocation check failed", realtimechat.ErrSessionValidationUnavailable)
		}
		if revoked {
			return realtimechat.ErrSessionInvalid
		}
	}
	// A ticket minted before session tracking carries no session id, so only
	// a populated id is checked; otherwise those sockets would be dropped.
	if h.tokenRevocations != nil && strings.TrimSpace(ticket.SessionID) != "" {
		revoked, err := h.tokenRevocations.IsSessionRevoked(ctx, ticket.SessionID)
		if err != nil {
			return fmt.Errorf("%w: session revocation check failed", realtimechat.ErrSessionValidationUnavailable)
		}
		if revoked {
			return realtimechat.ErrSessionInvalid
		}
	}
	if err := h.validateCredentialVersionValue(ctx, ticket.UserID, ticket.CredentialVersion, ticket.CredentialVersionClaim); err != nil {
		if errors.Is(err, errCredentialVersionUnavailable) {
			return fmt.Errorf("%w: credential version check failed", realtimechat.ErrSessionValidationUnavailable)
		}
		return fmt.Errorf("%w: credential version rejected", realtimechat.ErrSessionInvalid)
	}
	return nil
}
