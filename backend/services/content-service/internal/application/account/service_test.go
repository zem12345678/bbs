package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domain "content-service/internal/domain/account"
)

func TestArchiveAccountContentValidatesRequest(t *testing.T) {
	service := NewService(&fakeErasureRepository{}, nil)
	for _, input := range []struct {
		userID        int64
		jobID         int64
		policyVersion int32
	}{{0, 1, 1}, {1, 0, 1}, {1, 1, 0}} {
		if _, err := service.ArchiveAccountContent(context.Background(), input.userID, input.jobID, input.policyVersion); !errors.Is(err, domain.ErrInvalidErasure) {
			t.Fatalf("ArchiveAccountContent(%+v) error = %v, want invalid erasure", input, err)
		}
	}
}

func TestArchiveAccountContentInvalidatesArchivedArticleCache(t *testing.T) {
	repo := &fakeErasureRepository{result: domain.ErasureResult{
		ArchivedArticles: 2, ArchivedTopics: 1, DeletedPollBallots: 3,
		ArticleSlugs: []string{"first", "second"},
	}}
	cache := &recordingArticleCache{}
	service := NewService(repo, cache)

	result, err := service.ArchiveAccountContent(context.Background(), 42, 1001, 3)
	if err != nil {
		t.Fatalf("ArchiveAccountContent() error = %v", err)
	}
	if result.ArchivedArticles != 2 || result.ArchivedTopics != 1 || result.DeletedPollBallots != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !reflect.DeepEqual(cache.slugs, []string{"first", "second"}) {
		t.Fatalf("invalidated slugs = %v", cache.slugs)
	}
}

type fakeErasureRepository struct {
	result domain.ErasureResult
	err    error
}

func (f *fakeErasureRepository) ArchiveAccountContent(context.Context, int64, int64, int32) (domain.ErasureResult, error) {
	return f.result, f.err
}

type recordingArticleCache struct {
	slugs []string
}

func (c *recordingArticleCache) Del(_ context.Context, slug string) {
	c.slugs = append(c.slugs, slug)
}
