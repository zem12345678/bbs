package grpc

import (
	"context"
	"testing"
	"time"

	pb "notification-service/api/proto/notificationpb"
	app "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEraseUserDataDelegatesAuthenticatedIdentity(t *testing.T) {
	repo := &erasureHandlerRepository{}
	handler := NewHandler(app.NewService(repo))

	response, err := handler.EraseUserData(t.Context(), &pb.EraseUserDataRequest{UserId: 42, DeletionJobId: 9001, PolicyVersion: 3})
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	if !response.GetSuccess() || repo.userID != 42 || repo.jobID != 9001 || repo.policyVersion != 3 {
		t.Fatalf("response = %#v, repository request = %d/%d/%d", response, repo.userID, repo.jobID, repo.policyVersion)
	}
}

func TestEraseUserDataRejectsInvalidRequest(t *testing.T) {
	handler := NewHandler(app.NewService(&erasureHandlerRepository{}))
	_, err := handler.EraseUserData(t.Context(), &pb.EraseUserDataRequest{UserId: 42, DeletionJobId: 9001})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("EraseUserData() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestNotificationPreferencesRoundTrip(t *testing.T) {
	repo := &preferencesHandlerRepository{}
	handler := NewHandler(app.NewService(repo))

	response, err := handler.GetPreferences(t.Context(), &pb.GetPreferencesRequest{UserId: 42})
	if err != nil {
		t.Fatalf("GetPreferences() error = %v", err)
	}
	if len(response.GetItems()) == 0 {
		t.Fatal("GetPreferences() returned no defaults")
	}

	response, err = handler.UpdatePreferences(t.Context(), &pb.UpdatePreferencesRequest{
		UserId: 42,
		Items:  []*pb.NotificationPreference{{Type: domain.NotificationTypeComment, Enabled: false}},
	})
	if err != nil {
		t.Fatalf("UpdatePreferences() error = %v", err)
	}
	for _, item := range response.GetItems() {
		if item.GetType() == domain.NotificationTypeComment && item.GetEnabled() {
			t.Fatal("comment preference remained enabled")
		}
	}
}

func TestCreateExportCompletedNotificationDelegates(t *testing.T) {
	repo := &exportCompletedHandlerRepository{}
	handler := NewHandler(app.NewService(repo))

	response, err := handler.CreateExportCompletedNotification(t.Context(), &pb.CreateExportCompletedNotificationRequest{
		RecipientId: 42, FileId: 9001, ExportedEntity: "clip", IdempotencyKey: "export-42-9001",
	})
	if err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if !response.GetSuccess() {
		t.Fatalf("response = %#v", response)
	}
	if repo.item.UserID != 42 || repo.item.Type != domain.NotificationTypeExportCompleted || repo.item.EntityType != "file" || repo.item.EntityID != 9001 || repo.eventID != "export_completed:export-42-9001" {
		t.Fatalf("repository notification = %#v, event ID = %q", repo.item, repo.eventID)
	}
}

func TestCreateExportCompletedNotificationRejectsInvalidRequest(t *testing.T) {
	handler := NewHandler(app.NewService(&exportCompletedHandlerRepository{}))
	_, err := handler.CreateExportCompletedNotification(t.Context(), &pb.CreateExportCompletedNotificationRequest{
		RecipientId: 42, FileId: 9001, ExportedEntity: "data", IdempotencyKey: "export-42-9001",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateExportCompletedNotification() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

type erasureHandlerRepository struct {
	domain.Repository
	userID        int64
	jobID         int64
	policyVersion int32
}

type preferencesHandlerRepository struct {
	domain.Repository
	items []domain.NotificationPreference
}

type exportCompletedHandlerRepository struct {
	domain.Repository
	item    domain.Notification
	eventID string
}

func (r *exportCompletedHandlerRepository) Create(_ context.Context, item domain.Notification, eventID string, _ time.Time) error {
	r.item = item
	r.eventID = eventID
	return nil
}

func (r *preferencesHandlerRepository) ListPreferences(context.Context, int64) ([]domain.NotificationPreference, error) {
	return r.items, nil
}

func (r *preferencesHandlerRepository) ReplacePreferences(_ context.Context, _ int64, items []domain.NotificationPreference) error {
	r.items = items
	return nil
}

func (r *erasureHandlerRepository) EraseUserData(_ context.Context, userID, jobID int64, policyVersion int32) error {
	r.userID = userID
	r.jobID = jobID
	r.policyVersion = policyVersion
	return nil
}
