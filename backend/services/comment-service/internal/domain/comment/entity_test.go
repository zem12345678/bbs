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

func TestRestoreRequiresModerator(t *testing.T) {
	c, err := NewRoot(1, CreateCmd{
		EntityType: string(EntityArticle),
		EntityID:   10,
		AuthorID:   20,
		Content:    "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Events()
	if err := c.Hide(30, true); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	if c.DeletedAt == nil {
		t.Fatalf("DeletedAt should be set after hide")
	}
	if err := c.Restore(20, false); err != ErrPermissionDenied {
		t.Fatalf("Restore() error = %v, want %v", err, ErrPermissionDenied)
	}
	if err := c.Restore(30, true); err != nil {
		t.Fatalf("moderator Restore() error = %v", err)
	}
	if c.Status != StatusVisible {
		t.Fatalf("status = %d, want visible", c.Status)
	}
	if c.DeletedAt != nil {
		t.Fatalf("DeletedAt should be cleared after restore")
	}
	events := c.Events()
	if len(events) != 2 {
		t.Fatalf("events len = %d, want hidden and restored events", len(events))
	}
	if events[1].EventName() != "comment.restored" {
		t.Fatalf("event = %s, want comment.restored", events[1].EventName())
	}
}
