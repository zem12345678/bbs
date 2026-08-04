package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domain "reaction-service/internal/domain/account"
)

func TestEraseAccountReactionsValidatesInput(t *testing.T) {
	service := NewService(&erasureRepositoryStub{}, &erasureCacheStub{})
	for _, testCase := range []struct {
		userID        int64
		jobID         int64
		policyVersion int32
	}{
		{userID: 0, jobID: 1, policyVersion: 1},
		{userID: 1, jobID: 0, policyVersion: 1},
		{userID: 1, jobID: 1, policyVersion: 0},
	} {
		if _, err := service.EraseAccountReactions(t.Context(), testCase.userID, testCase.jobID, testCase.policyVersion); !errors.Is(err, domain.ErrInvalidErasure) {
			t.Fatalf("EraseAccountReactions(%d, %d, %d) error = %v, want ErrInvalidErasure", testCase.userID, testCase.jobID, testCase.policyVersion, err)
		}
	}
}

func TestEraseAccountReactionsTombstonesBeforeDatabaseAndPurgesAfter(t *testing.T) {
	var calls []string
	want := domain.ErasureResult{DeletedLikes: 2, DeletedFavorites: 3, DeletedCollections: 1, AnonymizedReports: 4, AnonymizedHandledReports: 5}
	repo := &erasureRepositoryStub{result: want, calls: &calls}
	cache := &erasureCacheStub{calls: &calls}
	service := NewService(repo, cache)

	got, err := service.EraseAccountReactions(t.Context(), 42, 9001, 3)
	if err != nil {
		t.Fatalf("EraseAccountReactions() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EraseAccountReactions() = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(calls, []string{"tombstone", "repository", "purge"}) {
		t.Fatalf("calls = %v, want tombstone, repository, purge", calls)
	}
}

func TestEraseAccountReactionsStopsWhenTombstoneFails(t *testing.T) {
	cacheErr := errors.New("redis unavailable")
	var calls []string
	service := NewService(&erasureRepositoryStub{calls: &calls}, &erasureCacheStub{calls: &calls, tombstoneErr: cacheErr})

	_, err := service.EraseAccountReactions(t.Context(), 42, 9001, 3)
	if !errors.Is(err, cacheErr) {
		t.Fatalf("EraseAccountReactions() error = %v, want %v", err, cacheErr)
	}
	if !reflect.DeepEqual(calls, []string{"tombstone"}) {
		t.Fatalf("calls = %v, want only tombstone", calls)
	}
}

type erasureRepositoryStub struct {
	result domain.ErasureResult
	err    error
	calls  *[]string
}

func (s *erasureRepositoryStub) EraseAccountReactions(context.Context, int64, int64, int32) (domain.ErasureResult, error) {
	if s.calls != nil {
		*s.calls = append(*s.calls, "repository")
	}
	return s.result, s.err
}

type erasureCacheStub struct {
	calls        *[]string
	tombstoneErr error
	purgeErr     error
}

func (s *erasureCacheStub) TombstoneAccount(context.Context, int64, int64, int32) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, "tombstone")
	}
	return s.tombstoneErr
}

func (s *erasureCacheStub) PurgeAccount(context.Context, int64) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, "purge")
	}
	return s.purgeErr
}
