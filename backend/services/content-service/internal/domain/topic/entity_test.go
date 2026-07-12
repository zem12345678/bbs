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

func TestQATopicKeepsBountyAndDefaultsOpenStatus(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10, BountyScore: 30})
	if err != nil {
		t.Fatal(err)
	}
	if topic.Type != TypeQA {
		t.Fatalf("type = %v, want qa", topic.Type)
	}
	if topic.BountyScore != 30 {
		t.Fatalf("bounty score = %d, want 30", topic.BountyScore)
	}
	if topic.QAStatus != QAStatusOpen {
		t.Fatalf("qa status = %q, want open", topic.QAStatus)
	}
}

func TestQATopicRejectsNegativeBounty(t *testing.T) {
	_, err := New(1, CreateCmd{Slug: "bad-bounty", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10, BountyScore: -1})
	if err != ErrBountyInvalid {
		t.Fatalf("err = %v, want ErrBountyInvalid", err)
	}
}

func TestNonQATopicClearsQAFields(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "normal-topic", Type: "topic", Title: "Hello", Body: "body", AuthorID: 10, BountyScore: 30})
	if err != nil {
		t.Fatal(err)
	}
	if topic.BountyScore != 0 || topic.QAStatus != "" || topic.AcceptedCommentID != 0 {
		t.Fatalf("non-qa fields = bounty:%d status:%q accepted:%d, want cleared", topic.BountyScore, topic.QAStatus, topic.AcceptedCommentID)
	}
}
