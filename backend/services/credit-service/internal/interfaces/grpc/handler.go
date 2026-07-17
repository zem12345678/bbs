package grpc

import (
	"context"
	"errors"
	"time"

	pb "credit-service/api/proto/creditpb"
	app "credit-service/internal/application/credit"
	domain "credit-service/internal/domain/credit"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (h *Handler) DebitCredits(ctx context.Context, req *pb.DebitCreditsRequest) (*pb.DebitCreditsResponse, error) {
	ledger, balance, duplicate, err := h.service.DebitCredits(
		ctx,
		req.GetUserId(),
		req.GetAmount(),
		req.GetReason(),
		req.GetDescription(),
		req.GetSourceEventId(),
		req.GetSourceType(),
		req.GetSourceId(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.DebitCreditsResponse{
		Balance:   balanceToPB(balance),
		Ledger:    ledgerToPB(ledger),
		Duplicate: duplicate,
	}, nil
}

func (h *Handler) AdjustCredits(ctx context.Context, req *pb.AdjustCreditsRequest) (*pb.AdjustCreditsResponse, error) {
	ledger, balance, duplicate, err := h.service.AdjustCredits(
		ctx,
		req.GetUserId(),
		req.GetDelta(),
		req.GetReason(),
		req.GetDescription(),
		req.GetSourceEventId(),
		req.GetSourceType(),
		req.GetSourceId(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.AdjustCreditsResponse{
		Balance:   balanceToPB(balance),
		Ledger:    ledgerToPB(ledger),
		Duplicate: duplicate,
	}, nil
}

func (h *Handler) ReserveCredits(ctx context.Context, req *pb.ReserveCreditsRequest) (*pb.ReserveCreditsResponse, error) {
	reservation, balance, duplicate, err := h.service.ReserveCredits(
		ctx,
		req.GetUserId(),
		req.GetAmount(),
		req.GetReason(),
		req.GetDescription(),
		req.GetSourceEventId(),
		req.GetSourceType(),
		req.GetSourceId(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.ReserveCreditsResponse{
		Balance:     balanceToPB(balance),
		Reservation: reservationToPB(reservation),
		Duplicate:   duplicate,
	}, nil
}

func (h *Handler) ReleaseCredits(ctx context.Context, req *pb.ReleaseCreditsRequest) (*pb.ReleaseCreditsResponse, error) {
	reservation, balance, duplicate, err := h.service.ReleaseCredits(
		ctx,
		req.GetUserId(),
		req.GetAmount(),
		req.GetReservationReason(),
		req.GetReleaseReason(),
		req.GetDescription(),
		req.GetSourceEventId(),
		req.GetSourceType(),
		req.GetSourceId(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.ReleaseCreditsResponse{
		Balance:     balanceToPB(balance),
		Reservation: reservationToPB(reservation),
		Duplicate:   duplicate,
	}, nil
}

func reservationToPB(item domain.CreditReservation) *pb.CreditReservation {
	return &pb.CreditReservation{
		Id:            item.ID,
		UserId:        item.UserID,
		Amount:        item.Amount,
		Status:        item.Status,
		Reason:        item.Reason,
		Description:   item.Description,
		SourceEventId: item.SourceEventID,
		SourceType:    item.SourceType,
		SourceId:      item.SourceID,
		CreatedAt:     millis(item.CreatedAt),
		UpdatedAt:     millis(item.UpdatedAt),
		SettledAt:     millis(item.SettledAt),
	}
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

func creditError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInsufficientCredit):
		return status.Error(codes.FailedPrecondition, "积分余额不足")
	case errors.Is(err, domain.ErrCreditLedgerMismatch):
		return status.Error(codes.FailedPrecondition, "积分账本记录不匹配")
	case errors.Is(err, domain.ErrCreditReservationNotFound):
		return status.Error(codes.NotFound, "积分冻结记录不存在")
	case errors.Is(err, domain.ErrCreditReservationMismatch):
		return status.Error(codes.FailedPrecondition, "积分冻结记录不匹配")
	default:
		return err
	}
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
