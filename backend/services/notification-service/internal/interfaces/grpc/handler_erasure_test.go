package grpc

import (
	"context"
	"testing"

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
