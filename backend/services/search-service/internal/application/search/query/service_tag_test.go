package query

import (
	"context"
	"errors"
	"testing"

	domain "search-service/internal/domain/search"
)

func TestSearchByTagNormalizesAndDefaultsCriteria(t *testing.T) {
	repo := &tagSearchRepository{}
	service := NewService(repo)

	_, err := service.SearchByTag(t.Context(), domain.SearchByTagCriteria{Tag: "  GoLang  "})
	if err != nil {
		t.Fatalf("SearchByTag() error = %v", err)
	}
	if repo.criteria.Tag != "golang" || repo.criteria.Limit != 10 || len(repo.criteria.Query) != 0 {
		t.Fatalf("criteria = %#v", repo.criteria)
	}

	_, err = service.SearchByTag(t.Context(), domain.SearchByTagCriteria{
		Tag:   "ignored",
		Query: []domain.TagQueryGroup{{Tags: []string{"other"}}},
	})
	if err != nil {
		t.Fatalf("SearchByTag(tag precedence) error = %v", err)
	}
	if repo.criteria.Tag != "ignored" || repo.criteria.Query != nil {
		t.Fatalf("tag precedence criteria = %#v", repo.criteria)
	}
}

func TestSearchByTagNormalizesOROfANDQuery(t *testing.T) {
	repo := &tagSearchRepository{}
	service := NewService(repo)

	_, err := service.SearchByTag(t.Context(), domain.SearchByTagCriteria{
		Limit: 25,
		Query: []domain.TagQueryGroup{
			{Tags: []string{" Go ", "Cloud"}},
			{Tags: []string{"BBS"}},
		},
	})
	if err != nil {
		t.Fatalf("SearchByTag() error = %v", err)
	}
	got := repo.criteria.Query
	if len(got) != 2 || len(got[0].Tags) != 2 || got[0].Tags[0] != "go" || got[0].Tags[1] != "cloud" || got[1].Tags[0] != "bbs" {
		t.Fatalf("normalized query = %#v", got)
	}
}

func TestSearchByTagRejectsUnsupportedProjectionFilters(t *testing.T) {
	value := false
	cases := []domain.SearchByTagCriteria{
		{Tag: "go", Reply: &value},
		{Tag: "go", Renote: &value},
		{Tag: "go", Poll: &value},
		{Tag: "go", WithFiles: true},
		{Tag: "go", Scope: "local"},
		{Tag: "go", Scope: "remote"},
	}
	service := NewService(&tagSearchRepository{})
	for _, criteria := range cases {
		_, err := service.SearchByTag(t.Context(), criteria)
		if !errors.Is(err, domain.ErrTagFilterUnsupported) {
			t.Fatalf("SearchByTag(%#v) error = %v", criteria, err)
		}
	}
}

func TestSearchByTagRejectsInvalidQueries(t *testing.T) {
	service := NewService(&tagSearchRepository{})
	cases := []struct {
		criteria domain.SearchByTagCriteria
		want     error
	}{
		{criteria: domain.SearchByTagCriteria{}, want: domain.ErrTagQueryRequired},
		{criteria: domain.SearchByTagCriteria{Tag: "go", Limit: 101}, want: domain.ErrTagQueryInvalid},
		{criteria: domain.SearchByTagCriteria{Tag: "go", SinceID: -1}, want: domain.ErrTagQueryInvalid},
		{criteria: domain.SearchByTagCriteria{Query: []domain.TagQueryGroup{{}}}, want: domain.ErrTagQueryInvalid},
		{criteria: domain.SearchByTagCriteria{Tag: string(make([]rune, 129))}, want: domain.ErrTagQueryInvalid},
	}
	for _, test := range cases {
		_, err := service.SearchByTag(t.Context(), test.criteria)
		if !errors.Is(err, test.want) {
			t.Fatalf("SearchByTag(%#v) error = %v, want %v", test.criteria, err, test.want)
		}
	}
}

type tagSearchRepository struct {
	domain.Repository
	criteria domain.SearchByTagCriteria
}

func (r *tagSearchRepository) SearchByTag(_ context.Context, criteria domain.SearchByTagCriteria) ([]domain.NoteLikeHit, error) {
	r.criteria = criteria
	return nil, nil
}
