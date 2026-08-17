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
	if errors.Is(err, domain.ErrInvalidSystemNotification) || errors.Is(err, domain.ErrInvalidExportCompletedNotification) || errors.Is(err, domain.ErrInvalidUserErasure) || errors.Is(err, domain.ErrInvalidNotificationPreferences) || errors.Is(err, domain.ErrInvalidWebPushSubscription) || errors.Is(err, domain.ErrInvalidWebhook) || errors.Is(err, domain.ErrWebhookUnsafeURL) || errors.Is(err, domain.ErrWebhookPayloadTooBig) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, domain.ErrWebPushDisabled) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	if errors.Is(err, domain.ErrWebPushSubscriptionLimit) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, domain.ErrWebhookLimitReached) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, domain.ErrWebhookNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return err
}
