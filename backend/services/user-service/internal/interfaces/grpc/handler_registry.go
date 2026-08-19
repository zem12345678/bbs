package grpc

import (
	"context"
	"strconv"

	pb "user-service/api/proto/userpb"
	domain "user-service/internal/domain/user"
)

func (h *Handler) SetRegistryItem(ctx context.Context, req *pb.SetRegistryItemRequest) (*pb.RegistryItemResponse, error) {
	item, err := h.cmd.SetRegistryItem(ctx, req.GetUserId(), fromPBRegistryDomain(req.GetDomain()), req.GetScope(), req.GetKey(), req.GetValueJson())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.RegistryItemResponse{Item: toPBRegistryItem(item)}, nil
}

func (h *Handler) GetRegistryItem(ctx context.Context, req *pb.GetRegistryItemRequest) (*pb.RegistryItemResponse, error) {
	item, err := h.qry.GetRegistryItem(ctx, req.GetUserId(), fromPBRegistryDomain(req.GetDomain()), req.GetScope(), req.GetKey())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.RegistryItemResponse{Item: toPBRegistryItem(item)}, nil
}

func (h *Handler) ListRegistryItems(ctx context.Context, req *pb.ListRegistryItemsRequest) (*pb.RegistryItemListResponse, error) {
	items, err := h.qry.ListRegistryItems(ctx, req.GetUserId(), fromPBRegistryDomain(req.GetDomain()), req.GetScope())
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.RegistryItemInfo, 0, len(items))
	for _, item := range items {
		out = append(out, toPBRegistryItem(item))
	}
	return &pb.RegistryItemListResponse{Items: out}, nil
}

func (h *Handler) RemoveRegistryItem(ctx context.Context, req *pb.GetRegistryItemRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.RemoveRegistryItem(ctx, req.GetUserId(), fromPBRegistryDomain(req.GetDomain()), req.GetScope(), req.GetKey()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ListRegistryScopeDomains(ctx context.Context, req *pb.UserIDRequest) (*pb.RegistryScopeDomainListResponse, error) {
	groups, err := h.qry.ListRegistryScopeDomains(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.RegistryScopeDomainInfo, 0, len(groups))
	for _, group := range groups {
		scopes := make([]*pb.RegistryScope, 0, len(group.Scopes))
		for _, scope := range group.Scopes {
			scopes = append(scopes, &pb.RegistryScope{Segments: append([]string(nil), scope...)})
		}
		out = append(out, &pb.RegistryScopeDomainInfo{Domain: toPBRegistryDomain(group.Domain), Scopes: scopes})
	}
	return &pb.RegistryScopeDomainListResponse{Items: out}, nil
}

func fromPBRegistryDomain(value *pb.RegistryDomain) *string {
	if value == nil {
		return nil
	}
	domainValue := value.GetValue()
	return &domainValue
}

func toPBRegistryDomain(value *string) *pb.RegistryDomain {
	if value == nil {
		return nil
	}
	return &pb.RegistryDomain{Value: *value}
}

func toPBRegistryItem(item *domain.RegistryItem) *pb.RegistryItemInfo {
	if item == nil {
		return nil
	}
	return &pb.RegistryItemInfo{
		Id:        strconv.FormatInt(item.ID, 10),
		UserId:    item.UserID,
		Domain:    toPBRegistryDomain(item.Domain),
		Scope:     append([]string(nil), item.Scope...),
		Key:       item.Key,
		ValueJson: append([]byte(nil), item.Value...),
		CreatedAt: item.CreatedAt.UnixMilli(),
		UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}
