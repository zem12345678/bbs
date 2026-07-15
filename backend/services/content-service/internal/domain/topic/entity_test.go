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
	if topic.BountyScore != 0 || topic.QAStatus != "" || topic.AcceptedCommentID != 0 || topic.AcceptedCommentAuthorID != 0 {
		t.Fatalf("non-qa fields = bounty:%d status:%q accepted:%d acceptedAuthor:%d, want cleared", topic.BountyScore, topic.QAStatus, topic.AcceptedCommentID, topic.AcceptedCommentAuthorID)
	}
}

func TestQATopicAcceptCommentResolvesQuestion(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	changed, err := topic.AcceptComment(9001, 22)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if topic.QAStatus != QAStatusResolved || topic.AcceptedCommentID != 9001 || topic.AcceptedCommentAuthorID != 22 {
		t.Fatalf("accept failed: status=%q accepted=%d acceptedAuthor=%d", topic.QAStatus, topic.AcceptedCommentID, topic.AcceptedCommentAuthorID)
	}
}

func TestQATopicRejectsAcceptCommentBeforePublish(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9001, 22); err != ErrNotPublished {
		t.Fatalf("err = %v, want ErrNotPublished", err)
	}
}

func TestQATopicRejectsOwnCommentAcceptance(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9001, 10); err != ErrCannotAcceptOwnComment {
		t.Fatalf("err = %v, want ErrCannotAcceptOwnComment", err)
	}
	if topic.AcceptedCommentID != 0 || topic.QAStatus != QAStatusOpen {
		t.Fatalf("topic acceptance = status:%q comment:%d, want unchanged open topic", topic.QAStatus, topic.AcceptedCommentID)
	}
}

func TestQATopicAcceptSameCommentIsIdempotent(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9001, 22); err != nil {
		t.Fatal(err)
	}
	changed, err := topic.AcceptComment(9001, 22)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("changed = true, want false")
	}
}

func TestQATopicRejectsDifferentAcceptedComment(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "need-help", Type: "qa", Title: "How to debug?", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := topic.Publish(); err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9001, 22); err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9002, 33); err != ErrAlreadyAccepted {
		t.Fatalf("err = %v, want ErrAlreadyAccepted", err)
	}
}

func TestNonQATopicRejectsAcceptComment(t *testing.T) {
	topic, err := New(1, CreateCmd{Slug: "normal-topic", Type: "topic", Title: "Hello", Body: "body", AuthorID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := topic.AcceptComment(9001, 22); err != ErrNotQuestion {
		t.Fatalf("err = %v, want ErrNotQuestion", err)
	}
}
