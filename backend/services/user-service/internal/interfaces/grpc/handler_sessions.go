package grpc

import (
	"context"
	"strconv"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	domain "user-service/internal/domain/user"
)

// withSessionClient attaches the caller-reported client metadata so the command
// layer can attribute an issued session to a device.
func withSessionClient(ctx context.Context, client *pb.SessionClientInfo) context.Context {
	if client == nil {
		return ctx
	}
	return command.WithSessionClient(ctx, domain.SessionClientInfo{
		IPAddress: client.GetIpAddress(),
		UserAgent: client.GetUserAgent(),
	})
}

func (h *Handler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.SessionListResponse, error) {
	sessions, err := h.cmd.ListSessions(ctx, req.GetUserId(), int(req.GetLimit()))
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, toPBSession(session))
	}
	return &pb.SessionListResponse{Items: items, Total: int64(len(items))}, nil
}

func (h *Handler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.SessionResponse, error) {
	session, err := h.cmd.GetSession(ctx, req.GetUserId(), req.GetSessionId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SessionResponse{Session: toPBSession(session)}, nil
}

func (h *Handler) RevokeSession(ctx context.Context, req *pb.RevokeSessionRequest) (*pb.SessionResponse, error) {
	session, err := h.cmd.RevokeSession(ctx, req.GetUserId(), req.GetSessionId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SessionResponse{Session: toPBSession(session)}, nil
}

func (h *Handler) ListLoginEvents(ctx context.Context, req *pb.ListLoginEventsRequest) (*pb.LoginEventListResponse, error) {
	events, err := h.cmd.ListLoginEvents(ctx, req.GetUserId(), int(req.GetLimit()))
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.LoginEventInfo, 0, len(events))
	for _, event := range events {
		items = append(items, toPBLoginEvent(event))
	}
	return &pb.LoginEventListResponse{Items: items, Total: int64(len(items))}, nil
}

func (h *Handler) CreateAPIToken(ctx context.Context, req *pb.CreateAPITokenRequest) (*pb.CreateAPITokenResponse, error) {
	session, token, err := h.cmd.CreateAPIToken(
		withSessionClient(ctx, req.GetClient()),
		req.GetUserId(), req.GetName(), req.GetScopes(), int(req.GetExpiresInDays()),
	)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CreateAPITokenResponse{Token: token.Value, ApiToken: toPBAPIToken(session)}, nil
}

func (h *Handler) ListAPITokens(ctx context.Context, req *pb.ListAPITokensRequest) (*pb.APITokenListResponse, error) {
	tokens, total, err := h.cmd.ListAPITokens(ctx, req.GetUserId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.APITokenInfo, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, toPBAPIToken(token))
	}
	return &pb.APITokenListResponse{Items: items, Total: total}, nil
}

func (h *Handler) RevokeAPIToken(ctx context.Context, req *pb.RevokeAPITokenRequest) (*pb.APITokenResponse, error) {
	token, err := h.cmd.RevokeAPIToken(ctx, req.GetUserId(), req.GetTokenId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.APITokenResponse{ApiToken: toPBAPIToken(token)}, nil
}

func toPBAPIToken(token domain.UserSession) *pb.APITokenInfo {
	info := &pb.APITokenInfo{
		Id: token.SessionID, UserId: token.UserID, Name: token.APITokenName,
		Scopes: token.APITokenScopes, CredentialValid: token.APITokenCredentialValid,
	}
	if !token.CreatedAt.IsZero() {
		info.CreatedAt = token.CreatedAt.Unix()
	}
	if !token.ExpiresAt.IsZero() {
		info.ExpiresAt = token.ExpiresAt.Unix()
	}
	if token.RevokedAt != nil && !token.RevokedAt.IsZero() {
		info.RevokedAt = token.RevokedAt.Unix()
	}
	return info
}

func toPBSession(session domain.UserSession) *pb.SessionInfo {
	info := &pb.SessionInfo{
		SessionId:   session.SessionID,
		UserId:      session.UserID,
		IpAddress:   session.IPAddress,
		UserAgent:   session.UserAgent,
		LoginMethod: session.LoginMethod,
	}
	if !session.CreatedAt.IsZero() {
		info.CreatedAt = session.CreatedAt.Unix()
	}
	if !session.ExpiresAt.IsZero() {
		info.ExpiresAt = session.ExpiresAt.Unix()
	}
	if session.RevokedAt != nil && !session.RevokedAt.IsZero() {
		info.RevokedAt = session.RevokedAt.Unix()
	}
	return info
}

func toPBLoginEvent(event domain.LoginEvent) *pb.LoginEventInfo {
	info := &pb.LoginEventInfo{
		Id:            strconv.FormatInt(event.ID, 10),
		UserId:        event.UserID,
		SessionId:     event.SessionID,
		IpAddress:     event.IPAddress,
		UserAgent:     event.UserAgent,
		Success:       event.Success,
		FailureReason: event.FailureReason,
	}
	if !event.CreatedAt.IsZero() {
		info.CreatedAt = event.CreatedAt.Unix()
	}
	return info
}
