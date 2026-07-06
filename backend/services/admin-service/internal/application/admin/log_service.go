package admin

import (
	"context"

	domain "admin/internal/domain/admin"
)

func (s *Service) ListLoginLogs(ctx context.Context, actor domain.Actor, status int32, query string, limit int32, offset int32) (domain.LoginLogList, error) {
	if err := actor.Validate(); err != nil {
		return domain.LoginLogList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListLoginLogs); err != nil {
		return domain.LoginLogList{}, err
	}
	return s.ops.ListLoginLogs(ctx, status, query, limit, offset)
}

func (s *Service) ListOperationLogs(ctx context.Context, actor domain.Actor, status int32, query string, limit int32, offset int32) (domain.OperationLogList, error) {
	if err := actor.Validate(); err != nil {
		return domain.OperationLogList{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionListOperationLogs); err != nil {
		return domain.OperationLogList{}, err
	}
	return s.ops.ListOperationLogs(ctx, status, query, limit, offset)
}

func (s *Service) RecordOperationLog(ctx context.Context, actor domain.Actor, command domain.RecordOperationLogCommand) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if command.OperatorName == "" {
		command.OperatorName = actor.Username
	}
	if command.Status != 0 && command.Status != 1 {
		command.Status = 0
	}
	return s.ops.RecordOperationLog(ctx, command)
}
