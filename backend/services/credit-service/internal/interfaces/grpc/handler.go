package grpc

import (
	"context"
	"time"

	pb "credit-service/api/proto/creditpb"
	app "credit-service/internal/application/credit"
	domain "credit-service/internal/domain/credit"
)

type Handler struct {
	pb.UnimplementedCreditServiceServer
	service *app.Service
}

func NewHandler(service *app.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	balance, err := h.service.GetBalance(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	return &pb.BalanceResponse{Balance: balanceToPB(balance)}, nil
}

func (h *Handler) ListLedger(ctx context.Context, req *pb.ListLedgerRequest) (*pb.ListLedgerResponse, error) {
	items, total, balance, err := h.service.ListLedger(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, err
	}
	resp := &pb.ListLedgerResponse{Items: make([]*pb.LedgerEntry, 0, len(items)), Total: total, Balance: balanceToPB(balance)}
	for _, item := range items {
		resp.Items = append(resp.Items, ledgerToPB(item))
	}
	return resp, nil
}

func balanceToPB(balance domain.Balance) *pb.Balance {
	return &pb.Balance{
		UserId:    balance.UserID,
		Total:     balance.Total,
		UpdatedAt: millis(balance.UpdatedAt),
	}
}

func ledgerToPB(item domain.LedgerEntry) *pb.LedgerEntry {
	return &pb.LedgerEntry{
		Id:            item.ID,
		UserId:        item.UserID,
		Delta:         item.Delta,
		BalanceAfter:  item.BalanceAfter,
		Reason:        item.Reason,
		Description:   item.Description,
		SourceEventId: item.SourceEventID,
		SourceType:    item.SourceType,
		SourceId:      item.SourceID,
		CreatedAt:     millis(item.CreatedAt),
	}
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
