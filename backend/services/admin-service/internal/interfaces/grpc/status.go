package grpc

import (
	"errors"

	domain "admin/internal/domain/admin"

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
	code := codes.Internal
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrInvalidToken):
		code = codes.Unauthenticated
	case errors.Is(err, domain.ErrTooManyLoginAttempts):
		code = codes.ResourceExhausted
	case errors.Is(err, domain.ErrSystemNotificationUnavailable):
		code = codes.Unavailable
	case errors.Is(err, domain.ErrSystemNotificationRecipientValidationUnavailable):
		code = codes.Unavailable
	case errors.Is(err, domain.ErrSearchRebuildUnavailable):
		code = codes.Unavailable
	case errors.Is(err, domain.ErrAdminDisabled):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrPermissionDenied),
		errors.Is(err, domain.ErrProtectedSystemUser),
		errors.Is(err, domain.ErrProtectedSystemRole):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrAdminUserExists),
		errors.Is(err, domain.ErrSearchRebuildInProgress),
		errors.Is(err, domain.ErrSystemRoleExists),
		errors.Is(err, domain.ErrSystemMenuExists),
		errors.Is(err, domain.ErrSystemDeptExists):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrSystemMenuHasChildren),
		errors.Is(err, domain.ErrSystemDeptHasChildren),
		errors.Is(err, domain.ErrSystemDeptHasUsers),
		errors.Is(err, domain.ErrSystemRoleHasUsers),
		errors.Is(err, domain.ErrTaskDefinitionsManaged):
		code = codes.FailedPrecondition
	case errors.Is(err, domain.ErrInvalidActor),
		errors.Is(err, domain.ErrSystemNotificationRecipientsNotFound),
		errors.Is(err, domain.ErrInvalidArticleID),
		errors.Is(err, domain.ErrInvalidAdminUserID),
		errors.Is(err, domain.ErrInvalidBadge),
		errors.Is(err, domain.ErrInvalidBadgeID),
		errors.Is(err, domain.ErrInvalidCategory),
		errors.Is(err, domain.ErrInvalidCategoryID),
		errors.Is(err, domain.ErrInvalidChannelID),
		errors.Is(err, domain.ErrInvalidCommentID),
		errors.Is(err, domain.ErrInvalidForbiddenWord),
		errors.Is(err, domain.ErrInvalidForbiddenWordID),
		errors.Is(err, domain.ErrInvalidLevel),
		errors.Is(err, domain.ErrInvalidLevelID),
		errors.Is(err, domain.ErrInvalidLink),
		errors.Is(err, domain.ErrInvalidLinkID),
		errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrInvalidRoleKeys),
		errors.Is(err, domain.ErrInvalidReportID),
		errors.Is(err, domain.ErrInvalidReportAction),
		errors.Is(err, domain.ErrInvalidSetting),
		errors.Is(err, domain.ErrInvalidSettingID),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidSystemDept),
		errors.Is(err, domain.ErrInvalidSystemMenu),
		errors.Is(err, domain.ErrInvalidSystemRole),
		errors.Is(err, domain.ErrInvalidSystemUser),
		errors.Is(err, domain.ErrSystemDeptInvalidParent),
		errors.Is(err, domain.ErrSystemDeptParentNotFound),
		errors.Is(err, domain.ErrSystemMenuInvalidParent),
		errors.Is(err, domain.ErrSystemMenuParentNotFound),
		errors.Is(err, domain.ErrInvalidAdminProfile),
		errors.Is(err, domain.ErrInvalidTask),
		errors.Is(err, domain.ErrInvalidTaskID),
		errors.Is(err, domain.ErrInvalidTopicID),
		errors.Is(err, domain.ErrInvalidUserID):
		code = codes.InvalidArgument
	}
	return status.Error(code, err.Error())
}
