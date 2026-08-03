package command

import (
	"context"
	"errors"
	"testing"

	domain "comment-service/internal/domain/comment"
)

func TestCreateReplyRequiresSameEntityAsParent(t *testing.T) {
	parent, err := domain.NewRoot(100, domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    "parent",
	})
	if err != nil {
		t.Fatal(err)
	}
	parent.Events()

	repo := &fakeCommandRepo{parent: parent}
	svc := NewService(repo, fixedIDGenerator(101), nil, nil)

	_, err = svc.Create(context.Background(), domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   11,
		ParentID:   parent.ID,
		AuthorID:   21,
		Content:    "reply",
	})
	if err != domain.ErrInvalidParent {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrInvalidParent)
	}
	if repo.saved != nil {
		t.Fatalf("reply should not be saved for mismatched entity")
	}
	if repo.incrementCalls != 0 {
		t.Fatalf("reply count increments = %d, want 0", repo.incrementCalls)
	}
}

func TestCreateReplyWithSameEntitySavesReply(t *testing.T) {
	parent, err := domain.NewRoot(100, domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    "parent",
	})
	if err != nil {
		t.Fatal(err)
	}
	parent.Events()

	repo := &fakeCommandRepo{parent: parent}
	svc := NewService(repo, fixedIDGenerator(101), nil, nil)

	reply, err := svc.Create(context.Background(), domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   10,
		ParentID:   parent.ID,
		AuthorID:   21,
		Content:    "reply",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.saved == nil || repo.saved.ID != reply.ID {
		t.Fatalf("reply was not saved")
	}
	if reply.RootID != parent.ID || reply.ParentID != parent.ID {
		t.Fatalf("reply root=%d parent=%d, want %d", reply.RootID, reply.ParentID, parent.ID)
	}
	if repo.incrementCalls != 1 || repo.incrementRootID != parent.ID || repo.incrementDelta != 1 {
		t.Fatalf("reply count increment root=%d delta=%d calls=%d", repo.incrementRootID, repo.incrementDelta, repo.incrementCalls)
	}
}

func TestRedactAccountCommentsValidatesAndDelegates(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	repo := &fakeCommandRepo{redactCount: 3, redactErr: wantErr}
	svc := NewService(repo, fixedIDGenerator(101), nil, nil)

	for _, args := range [][3]int64{{0, 2, 3}, {1, 0, 3}, {1, 2, 0}} {
		if _, err := svc.RedactAccountComments(context.Background(), args[0], args[1], int32(args[2])); !errors.Is(err, domain.ErrInvalidUserErasure) {
			t.Fatalf("RedactAccountComments(%v) error = %v, want invalid erasure", args, err)
		}
	}

	count, err := svc.RedactAccountComments(context.Background(), 41, 91, 4)
	if count != 3 || !errors.Is(err, wantErr) {
		t.Fatalf("RedactAccountComments() count=%d error=%v", count, err)
	}
	if repo.redactUserID != 41 || repo.redactJobID != 91 || repo.redactPolicy != 4 || repo.redactCalls != 1 {
		t.Fatalf("redact request user=%d job=%d policy=%d calls=%d", repo.redactUserID, repo.redactJobID, repo.redactPolicy, repo.redactCalls)
	}
}

type fixedIDGenerator int64

func (g fixedIDGenerator) Generate() int64 {
	return int64(g)
}

type fakeCommandRepo struct {
	parent          *domain.Comment
	saved           *domain.Comment
	incrementRootID int64
	incrementDelta  int64
	incrementCalls  int
	redactUserID    int64
	redactJobID     int64
	redactPolicy    int32
	redactCount     int64
	redactErr       error
	redactCalls     int
}

func (r *fakeCommandRepo) Save(_ context.Context, c *domain.Comment) error {
	r.saved = c
	return nil
}

func (r *fakeCommandRepo) FindByID(_ context.Context, _ int64) (*domain.Comment, error) {
	if r.parent == nil {
		return nil, domain.ErrNotFound
	}
	return r.parent, nil
}

func (r *fakeCommandRepo) ListByEntity(context.Context, domain.ListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}

func (r *fakeCommandRepo) ListReplies(context.Context, domain.ReplyListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}

func (r *fakeCommandRepo) ListForModeration(context.Context, domain.ModerationListQuery) ([]*domain.Comment, int64, error) {
	return nil, 0, nil
}

func (r *fakeCommandRepo) Hide(context.Context, *domain.Comment) error {
	return nil
}

func (r *fakeCommandRepo) Restore(context.Context, *domain.Comment) error {
	return nil
}

func (r *fakeCommandRepo) IncrementReplyCount(_ context.Context, rootID int64, delta int64) error {
	r.incrementRootID = rootID
	r.incrementDelta = delta
	r.incrementCalls++
	return nil
}

func (r *fakeCommandRepo) RedactAccountComments(_ context.Context, userID, deletionJobID int64, policyVersion int32) (int64, error) {
	r.redactUserID = userID
	r.redactJobID = deletionJobID
	r.redactPolicy = policyVersion
	r.redactCalls++
	return r.redactCount, r.redactErr
}
