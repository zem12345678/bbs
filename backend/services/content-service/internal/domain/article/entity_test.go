package article

import "testing"

func TestArticlePublishStateMachine(t *testing.T) {
	a, err := New(1, CreateCmd{Slug: "hello", Title: "Hello", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusDraft {
		t.Fatalf("status = %v, want draft", a.Status)
	}
	if err := a.Publish(); err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusPublished || a.PublishedAt == nil {
		t.Fatalf("publish failed: status=%v publishedAt=%v", a.Status, a.PublishedAt)
	}
	if err := a.Hide(); err != nil {
		t.Fatal(err)
	}
	if a.Status != StatusHidden {
		t.Fatalf("status = %v, want hidden", a.Status)
	}
	if err := a.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := a.Archive(); err != nil {
		t.Fatal(err)
	}
	if err := a.Publish(); err != ErrArchived {
		t.Fatalf("publish archived err = %v, want ErrArchived", err)
	}
}
