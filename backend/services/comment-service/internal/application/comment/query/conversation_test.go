package query

import (
	"context"
	"errors"
	"testing"

	domain "comment-service/internal/domain/comment"
)

func TestConversationReturnsDirectParentThroughRoot(t *testing.T) {
	repo := conversationRepo(visibleComment(1, 0, 0), visibleComment(2, 1, 1), visibleComment(3, 1, 2), visibleComment(4, 1, 3))

	items, err := NewService(repo).Conversation(context.Background(), domain.ConversationQuery{CommentID: 4, Limit: 10})
	if err != nil {
		t.Fatalf("Conversation() error = %v", err)
	}
	assertCommentIDs(t, items, 3, 2, 1)
}

func TestConversationAppliesOffsetAndLimit(t *testing.T) {
	repo := conversationRepo(visibleComment(1, 0, 0), visibleComment(2, 1, 1), visibleComment(3, 1, 2), visibleComment(4, 1, 3))

	items, err := NewService(repo).Conversation(context.Background(), domain.ConversationQuery{CommentID: 4, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("Conversation() error = %v", err)
	}
	assertCommentIDs(t, items, 2)
}

func TestConversationStopsAtHiddenAncestor(t *testing.T) {
	root := visibleComment(1, 0, 0)
	hidden := visibleComment(2, 1, 1)
	hidden.Status = domain.StatusHidden
	repo := conversationRepo(root, hidden, visibleComment(3, 1, 2))

	items, err := NewService(repo).Conversation(context.Background(), domain.ConversationQuery{CommentID: 3, Limit: 10})
	if err != nil {
		t.Fatalf("Conversation() error = %v", err)
	}
	assertCommentIDs(t, items)
	if repo.lookups[1] != 0 {
		t.Fatalf("root lookup count = %d, want 0 after hidden ancestor", repo.lookups[1])
	}
}

func TestConversationDoesNotExposeAncestorsOfHiddenTarget(t *testing.T) {
	root := visibleComment(1, 0, 0)
	target := visibleComment(2, 1, 1)
	target.Status = domain.StatusHidden
	repo := conversationRepo(root, target)

	items, err := NewService(repo).Conversation(context.Background(), domain.ConversationQuery{CommentID: 2, Limit: 10})
	if err != nil {
		t.Fatalf("Conversation() error = %v", err)
	}
	assertCommentIDs(t, items)
	if repo.lookups[1] != 0 {
		t.Fatalf("root lookup count = %d, want 0 for hidden target", repo.lookups[1])
	}
}

func TestConversationRejectsCrossEntityAndCycles(t *testing.T) {
	tests := []struct {
		name string
		repo *conversationRepoStub
		id   int64
	}{
		{name: "cross entity", repo: conversationRepo(visibleComment(1, 0, 0), commentForEntity(2, 1, 1, 99), visibleComment(3, 1, 2)), id: 3},
		{name: "cycle", repo: conversationRepo(visibleComment(2, 1, 3), visibleComment(3, 1, 2), visibleComment(4, 1, 3)), id: 4},
		{name: "missing parent", repo: conversationRepo(visibleComment(3, 1, 2)), id: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repo).Conversation(context.Background(), domain.ConversationQuery{CommentID: test.id, Limit: 10})
			if !errors.Is(err, domain.ErrInvalidParent) {
				t.Fatalf("Conversation() error = %v, want %v", err, domain.ErrInvalidParent)
			}
		})
	}
}

func TestConversationValidatesRequestAndPreservesTargetNotFound(t *testing.T) {
	tests := []struct {
		query domain.ConversationQuery
		want  error
	}{
		{query: domain.ConversationQuery{CommentID: 0, Limit: 1}, want: domain.ErrInvalidID},
		{query: domain.ConversationQuery{CommentID: 1, Limit: 0}, want: domain.ErrConversationLimitInvalid},
		{query: domain.ConversationQuery{CommentID: 1, Limit: 101}, want: domain.ErrConversationLimitInvalid},
		{query: domain.ConversationQuery{CommentID: 1, Limit: 1, Offset: -1}, want: domain.ErrConversationOffsetInvalid},
		{query: domain.ConversationQuery{CommentID: 1, Limit: 1, Offset: 10001}, want: domain.ErrConversationOffsetInvalid},
		{query: domain.ConversationQuery{CommentID: 1, Limit: 1}, want: domain.ErrNotFound},
	}
	for _, test := range tests {
		_, err := NewService(conversationRepo()).Conversation(context.Background(), test.query)
		if !errors.Is(err, test.want) {
			t.Fatalf("Conversation(%+v) error = %v, want %v", test.query, err, test.want)
		}
	}
}

type conversationRepoStub struct {
	domain.Repository
	comments map[int64]*domain.Comment
	lookups  map[int64]int
}

func conversationRepo(comments ...*domain.Comment) *conversationRepoStub {
	repo := &conversationRepoStub{comments: make(map[int64]*domain.Comment), lookups: make(map[int64]int)}
	for _, comment := range comments {
		repo.comments[comment.ID] = comment
	}
	return repo
}

func (r *conversationRepoStub) FindByID(_ context.Context, id int64) (*domain.Comment, error) {
	r.lookups[id]++
	comment, ok := r.comments[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return comment, nil
}

func visibleComment(id, rootID, parentID int64) *domain.Comment {
	return commentForEntity(id, rootID, parentID, 10)
}

func commentForEntity(id, rootID, parentID, entityID int64) *domain.Comment {
	return &domain.Comment{ID: id, EntityType: string(domain.EntityArticle), EntityID: entityID, RootID: rootID, ParentID: parentID, AuthorID: 20, Content: "comment", Status: domain.StatusVisible}
}

func assertCommentIDs(t *testing.T, items []*domain.Comment, want ...int64) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("item count = %d, want %d", len(items), len(want))
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("items[%d].ID = %d, want %d", i, items[i].ID, id)
		}
	}
}
