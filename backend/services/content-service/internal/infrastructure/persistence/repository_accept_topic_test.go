package persistence

import (
	"strings"
	"testing"

	topicDomain "content-service/internal/domain/topic"
)

func TestAcceptTopicCommentWhereRequiresPublishedTopic(t *testing.T) {
	if !strings.Contains(acceptTopicCommentWhere, "status = ?") {
		t.Fatalf("accept update WHERE missing published status guard: %s", acceptTopicCommentWhere)
	}
	if !strings.Contains(acceptTopicCommentWhere, "accepted_comment_id = 0 OR accepted_comment_id = ?") {
		t.Fatalf("accept update WHERE missing idempotent comment guard: %s", acceptTopicCommentWhere)
	}
	args := acceptTopicCommentWhereArgs(101, 9001)
	if len(args) != 4 {
		t.Fatalf("accept update WHERE args = %#v, want 4 args", args)
	}
	if args[2] != int32(topicDomain.StatusPublished) {
		t.Fatalf("accept update status arg = %#v, want published", args[2])
	}
}
