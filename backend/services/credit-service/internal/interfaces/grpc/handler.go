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

func (h *Handler) ListLeaderboard(ctx context.Context, req *pb.ListLeaderboardRequest) (*pb.ListLeaderboardResponse, error) {
	items, err := h.service.ListLeaderboard(ctx, req.GetLimit())
	if err != nil {
		return nil, err
	}
	resp := &pb.ListLeaderboardResponse{Items: make([]*pb.LeaderboardEntry, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, leaderboardEntryToPB(item))
	}
	return resp, nil
}

func (h *Handler) GetCheckInStatus(ctx context.Context, req *pb.GetCheckInStatusRequest) (*pb.CheckInStatusResponse, error) {
	checkIn, checkedIn, err := h.service.GetCheckInStatus(ctx, req.GetUserId(), time.Now())
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.CheckInStatusResponse{
		CheckIn:       checkInToPB(checkIn),
		CheckedIn:     checkedIn,
		RewardCredits: app.DailyCheckInDelta,
	}, nil
}

func (h *Handler) CheckIn(ctx context.Context, req *pb.CheckInRequest) (*pb.CheckInResponse, error) {
	checkIn, ledger, balance, duplicate, err := h.service.DailyCheckIn(ctx, req.GetUserId(), time.Now())
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.CheckInResponse{
		CheckIn:       checkInToPB(checkIn),
		Balance:       balanceToPB(balance),
		Ledger:        ledgerToPB(ledger),
		Duplicate:     duplicate,
		RewardCredits: app.DailyCheckInDelta,
	}, nil
}

func (h *Handler) GetTaskClaimStatus(ctx context.Context, req *pb.GetTaskClaimStatusRequest) (*pb.TaskClaimStatusResponse, error) {
	claimStatus, err := h.service.GetTaskClaimStatus(ctx, req.GetUserId(), req.GetTaskId(), req.GetTaskKey(), time.Now())
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.TaskClaimStatusResponse{Status: taskClaimStatusToPB(claimStatus)}, nil
}

func (h *Handler) ListTaskClaimStatuses(ctx context.Context, req *pb.ListTaskClaimStatusesRequest) (*pb.ListTaskClaimStatusesResponse, error) {
	inputs := make([]app.TaskClaimStatusInput, 0, len(req.GetTasks()))
	for _, task := range req.GetTasks() {
		inputs = append(inputs, app.TaskClaimStatusInput{
			TaskID:  task.GetTaskId(),
			TaskKey: task.GetTaskKey(),
		})
	}
	claimStatuses, err := h.service.ListTaskClaimStatuses(ctx, req.GetUserId(), inputs, time.Now())
	if err != nil {
		return nil, creditError(err)
	}
	resp := &pb.ListTaskClaimStatusesResponse{Items: make([]*pb.TaskClaimStatus, 0, len(claimStatuses))}
	for _, claimStatus := range claimStatuses {
		resp.Items = append(resp.Items, taskClaimStatusToPB(claimStatus))
	}
	return resp, nil
}

func (h *Handler) ClaimTask(ctx context.Context, req *pb.ClaimTaskRequest) (*pb.ClaimTaskResponse, error) {
	claimStatus, ledger, balance, duplicate, err := h.service.ClaimTask(
		ctx,
		req.GetUserId(),
		req.GetTaskId(),
		req.GetTaskKey(),
		req.GetRewardCredits(),
		req.GetTaskTitle(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.ClaimTaskResponse{
		Status:    taskClaimStatusToPB(claimStatus),
		Balance:   balanceToPB(balance),
		Ledger:    ledgerToPB(ledger),
		Duplicate: duplicate,
	}, nil
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

func (h *Handler) TransferCredits(ctx context.Context, req *pb.TransferCreditsRequest) (*pb.TransferCreditsResponse, error) {
	err := h.service.TransferCredits(ctx, app.TransferCreditsCommand{
		PayerUserID:       req.GetPayerUserId(),
		PayeeUserID:       req.GetPayeeUserId(),
		Amount:            req.GetAmount(),
		DebitReason:       req.GetDebitReason(),
		DebitDescription:  req.GetDebitDescription(),
		CreditReason:      req.GetCreditReason(),
		CreditDescription: req.GetCreditDescription(),
		SourceEventID:     req.GetSourceEventId(),
		SourceType:        req.GetSourceType(),
		SourceID:          req.GetSourceId(),
	})
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.TransferCreditsResponse{}, nil
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

func (h *Handler) ReverseQAAcceptance(ctx context.Context, req *pb.ReverseQAAcceptanceRequest) (*pb.ReverseQAAcceptanceResponse, error) {
	duplicate, err := h.service.ReverseQAAcceptance(
		ctx,
		req.GetQuestionAuthorId(),
		req.GetTopicId(),
		req.GetAcceptedCommentId(),
		req.GetAcceptedCommentAuthorId(),
		req.GetRewardCredits(),
		req.GetAcceptanceCycle(),
		req.GetTitle(),
		time.Now(),
	)
	if err != nil {
		return nil, creditError(err)
	}
	return &pb.ReverseQAAcceptanceResponse{Duplicate: duplicate}, nil
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

func leaderboardEntryToPB(item domain.LeaderboardEntry) *pb.LeaderboardEntry {
	return &pb.LeaderboardEntry{
		UserId: item.UserID,
		Total:  item.Total,
		Rank:   item.Rank,
	}
}

func checkInToPB(checkIn domain.CheckIn) *pb.DailyCheckIn {
	return &pb.DailyCheckIn{
		Id:              checkIn.ID,
		UserId:          checkIn.UserID,
		LatestDay:       checkIn.LatestDay,
		ConsecutiveDays: checkIn.ConsecutiveDays,
		CreatedAt:       millis(checkIn.CreatedAt),
		UpdatedAt:       millis(checkIn.UpdatedAt),
	}
}

func taskClaimStatusToPB(claimStatus domain.TaskClaimStatus) *pb.TaskClaimStatus {
	return &pb.TaskClaimStatus{
		TaskId:    claimStatus.TaskID,
		TaskKey:   claimStatus.TaskKey,
		Cycle:     claimStatus.Cycle,
		Completed: claimStatus.Completed,
		Claimed:   claimStatus.Claimed,
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
	case errors.Is(err, domain.ErrInvalidCreditTransfer), errors.Is(err, domain.ErrUnbalancedCreditTransfer):
		return status.Error(codes.InvalidArgument, "积分转账参数无效")
	case errors.Is(err, domain.ErrInconsistentCreditTransfer):
		return status.Error(codes.FailedPrecondition, "积分转账账本不一致")
	case errors.Is(err, domain.ErrCreditReservationNotFound):
		return status.Error(codes.NotFound, "积分冻结记录不存在")
	case errors.Is(err, domain.ErrCreditReservationMismatch):
		return status.Error(codes.FailedPrecondition, "积分冻结记录不匹配")
	case errors.Is(err, domain.ErrQAAcceptanceSettlementPending):
		return status.Error(codes.Aborted, "采纳悬赏尚未结算")
	case errors.Is(err, domain.ErrCheckInStateMismatch):
		return status.Error(codes.FailedPrecondition, "签到状态与积分账本不匹配")
	case errors.Is(err, domain.ErrCheckInDayRegression):
		return status.Error(codes.FailedPrecondition, "签到日期早于最近记录")
	case errors.Is(err, domain.ErrInvalidTaskClaim), errors.Is(err, domain.ErrUnsupportedTask):
		return status.Error(codes.InvalidArgument, "任务领取参数无效")
	case errors.Is(err, domain.ErrTaskNotCompleted):
		return status.Error(codes.FailedPrecondition, "尚未满足任务完成条件")
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
