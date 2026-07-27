package grpc

import (
	"errors"

	domain "notification-service/internal/domain/notification"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	if errors.Is(err, domain.ErrInvalidSystemNotification) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return err
}
