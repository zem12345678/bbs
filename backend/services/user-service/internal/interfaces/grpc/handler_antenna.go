package grpc

import (
	"context"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	domain "user-service/internal/domain/user"
)

func (h *Handler) CreateAntenna(ctx context.Context, req *pb.CreateAntennaRequest) (*pb.AntennaInfoResponse, error) {
	antenna, err := h.cmd.CreateAntenna(ctx, req.GetOwnerId(), antennaInputFromCreate(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AntennaInfoResponse{Antenna: toPbAntenna(antenna)}, nil
}

func (h *Handler) UpdateAntenna(ctx context.Context, req *pb.UpdateAntennaRequest) (*pb.AntennaInfoResponse, error) {
	antenna, err := h.cmd.UpdateAntenna(ctx, req.GetOwnerId(), req.GetAntennaId(), antennaInputFromUpdate(req))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AntennaInfoResponse{Antenna: toPbAntenna(antenna)}, nil
}

func (h *Handler) DeleteAntenna(ctx context.Context, req *pb.DeleteAntennaRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeleteAntenna(ctx, req.GetOwnerId(), req.GetAntennaId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) GetAntenna(ctx context.Context, req *pb.GetAntennaRequest) (*pb.AntennaInfoResponse, error) {
	antenna, err := h.qry.GetAntenna(ctx, req.GetOwnerId(), req.GetAntennaId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.AntennaInfoResponse{Antenna: toPbAntenna(antenna)}, nil
}

func (h *Handler) ListAntennas(ctx context.Context, req *pb.ListAntennasRequest) (*pb.AntennaListResponse, error) {
	items, err := h.qry.ListAntennas(ctx, req.GetOwnerId())
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.AntennaInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPbAntenna(item))
	}
	return &pb.AntennaListResponse{Items: out, Total: int64(len(out))}, nil
}

func antennaInputFromCreate(req *pb.CreateAntennaRequest) command.AntennaInput {
	return command.AntennaInput{
		Name: req.GetName(), Source: req.GetSource(), UserListID: req.GetUserListId(),
		Keywords: fromPbKeywordGroups(req.GetKeywords()), ExcludeKeywords: fromPbKeywordGroups(req.GetExcludeKeywords()), Users: req.GetUsers(),
		CaseSensitive: req.GetCaseSensitive(), LocalOnly: req.GetLocalOnly(), ExcludeBots: req.GetExcludeBots(),
		WithReplies: req.GetWithReplies(), WithFile: req.GetWithFile(), ExcludeSensitiveChannel: req.GetExcludeNotesInSensitiveChannel(),
	}
}

func antennaInputFromUpdate(req *pb.UpdateAntennaRequest) command.AntennaInput {
	return command.AntennaInput{
		Name: req.GetName(), Source: req.GetSource(), UserListID: req.GetUserListId(),
		Keywords: fromPbKeywordGroups(req.GetKeywords()), ExcludeKeywords: fromPbKeywordGroups(req.GetExcludeKeywords()), Users: req.GetUsers(),
		CaseSensitive: req.GetCaseSensitive(), LocalOnly: req.GetLocalOnly(), ExcludeBots: req.GetExcludeBots(),
		WithReplies: req.GetWithReplies(), WithFile: req.GetWithFile(), ExcludeSensitiveChannel: req.GetExcludeNotesInSensitiveChannel(),
	}
}

func toPbAntenna(antenna *domain.Antenna) *pb.AntennaInfo {
	if antenna == nil {
		return nil
	}
	return &pb.AntennaInfo{
		Id: antenna.ID, OwnerId: antenna.OwnerID, Name: antenna.Name, Source: antenna.Source, UserListId: antenna.UserListID,
		Keywords: toPbKeywordGroups(antenna.Keywords), ExcludeKeywords: toPbKeywordGroups(antenna.ExcludeKeywords), Users: antenna.Users,
		CaseSensitive: antenna.CaseSensitive, LocalOnly: antenna.LocalOnly, ExcludeBots: antenna.ExcludeBots,
		WithReplies: antenna.WithReplies, WithFile: antenna.WithFile, ExcludeNotesInSensitiveChannel: antenna.ExcludeNotesInSensitiveChannel,
		IsActive: antenna.IsActive, CreatedAt: antenna.CreatedAt.UnixMilli(), UpdatedAt: antenna.UpdatedAt.UnixMilli(), LastUsedAt: antenna.LastUsedAt.UnixMilli(),
	}
}

func fromPbKeywordGroups(groups []*pb.AntennaKeywordGroup) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			out = append(out, append([]string(nil), group.GetTerms()...))
		}
	}
	return out
}

func toPbKeywordGroups(groups [][]string) []*pb.AntennaKeywordGroup {
	out := make([]*pb.AntennaKeywordGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, &pb.AntennaKeywordGroup{Terms: append([]string(nil), group...)})
	}
	return out
}
