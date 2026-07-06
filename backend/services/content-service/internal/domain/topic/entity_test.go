package topic

import "testing"

func TestTopicPublishStateMachine(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "hello-topic", Type: "topic", Title: "Hello", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if topic.Status != StatusDraft {
		t.Fatalf("status = %v, want draft", topic.Status)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	if topic.Status != StatusPublished || topic.PublishedAt == nil {
		t.Fatalf("publish failed: status=%v publishedAt=%v", topic.Status, topic.PublishedAt)
	}
	if err := topic.Hide(); err != nil {
		t.Fatal(err)
	}
	if topic.Status != StatusHidden {
		t.Fatalf("status = %v, want hidden", topic.Status)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := topic.Archive(); err != nil {
		t.Fatal(err)
	}
	if err := topic.Publish(); err != ErrArchived {
		t.Fatalf("publish archived err = %v, want ErrArchived", err)
	}
}

func TestTweetDoesNotRequireTitle(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "quick-note", Type: "tweet", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if topic.Type != TypeTweet {
		t.Fatalf("type = %v, want tweet", topic.Type)
	}
}
