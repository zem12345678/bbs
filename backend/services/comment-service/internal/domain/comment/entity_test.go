package comment

import "testing"

func TestNewRootComment(t *testing.T) {
	c, err := NewRoot(1, CreateCmd{
		EntityType: string(EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    " hello ",
	})
	if err != nil {
		t.Fatalf("NewRoot() error = %v", err)
	}
	if c.Content != "hello" {
		t.Fatalf("content = %q", c.Content)
	}
	if !c.IsRoot() {
		t.Fatalf("root comment reported as reply")
	}
	if len(c.Events()) != 1 {
		t.Fatalf("expected created event")
	}
}

func TestNewReplyRequiresParent(t *testing.T) {
	_, err := NewReply(2, CreateCmd{
		EntityType: string(EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    "reply",
	}, 0, 0)
	if err != ErrInvalidParent {
		t.Fatalf("error = %v, want %v", err, ErrInvalidParent)
	}
}

func TestHideRequiresAuthorOrModerator(t *testing.T) {
	c, err := NewRoot(1, CreateCmd{
		EntityType: string(EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Hide(30, false); err != ErrPermissionDenied {
		t.Fatalf("Hide() error = %v, want %v", err, ErrPermissionDenied)
	}
	if err := c.Hide(30, true); err != nil {
		t.Fatalf("moderator Hide() error = %v", err)
	}
}
