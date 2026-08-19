package grpc

import (
	"context"
	"testing"

	pb "reaction-service/api/proto/reactionpb"
	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	domain "reaction-service/internal/domain/reaction"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPinHandlerForwardsContentReferenceAndListsPins(t *testing.T) {
	repo := &handlerPinRepository{rows: []*domain.Pin{{
		ID: 9, UserID: 42, Entity: domain.EntityRef{Type: domain.EntityTopic, ID: 88},
	}}}
	h := NewHandler(
		command.NewService(nil, nil, nil, nil, repo, nil, nil, nil),
		query.NewService(nil, nil, nil, nil, repo, nil),
		nil,
	)

	pinned, err := h.Pin(context.Background(), &pb.ReactRequest{UserId: 42, Entity: &pb.EntityRef{EntityType: "article", EntityId: 77}})
	require.NoError(t, err)
	require.True(t, pinned.GetChanged())
	require.Equal(t, int64(1), pinned.GetCount())
	require.Equal(t, domain.EntityArticle, repo.pinRef.Type)
	require.Equal(t, int64(77), repo.pinRef.ID)

	listed, err := h.ListPins(context.Background(), &pb.ListPinsRequest{UserId: 42, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), listed.GetTotal())
	require.Equal(t, "topic", listed.GetItems()[0].GetEntity().GetEntityType())
	require.Equal(t, int64(88), listed.GetItems()[0].GetEntity().GetEntityId())
}

func TestPinHandlerMapsPinLimit(t *testing.T) {
	repo := &handlerPinRepository{pinErr: domain.ErrPinLimitExceeded}
	h := NewHandler(command.NewService(nil, nil, nil, nil, repo, nil, nil, nil), nil, nil)

	_, err := h.Pin(context.Background(), &pb.ReactRequest{UserId: 42, Entity: &pb.EntityRef{EntityType: "article", EntityId: 77}})

	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

type handlerPinRepository struct {
	domain.PinRepository
	rows   []*domain.Pin
	pinRef domain.EntityRef
	pinErr error
}

func (r *handlerPinRepository) Pin(_ context.Context, ref domain.EntityRef, _ int64) (int64, bool, error) {
	r.pinRef = ref
	if r.pinErr != nil {
		return 0, false, r.pinErr
	}
	return 1, true, nil
}

func (r *handlerPinRepository) Unpin(context.Context, domain.EntityRef, int64) (int64, bool, error) {
	return 0, false, nil
}

func (r *handlerPinRepository) ListPins(context.Context, int64, int, int) ([]*domain.Pin, int64, error) {
	return r.rows, int64(len(r.rows)), nil
}
