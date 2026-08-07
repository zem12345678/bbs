package grpc

import (
	"testing"
	"time"

	categoryDomain "content-service/internal/domain/category"
	channelDomain "content-service/internal/domain/channel"
	topicDomain "content-service/internal/domain/topic"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestToStatusMapsMembershipEntitlementRequired(t *testing.T) {
	err := toStatus(topicDomain.ErrMembershipEntitlementRequired)
	if grpcstatus.Code(err) != codes.PermissionDenied {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.PermissionDenied)
	}
}

func TestToStatusMapsChannelCategoryErrors(t *testing.T) {
	if err := toStatus(categoryDomain.ErrNotFound); grpcstatus.Code(err) != codes.NotFound {
		t.Fatalf("missing category status code = %v, want %v", grpcstatus.Code(err), codes.NotFound)
	}
	if err := toStatus(channelDomain.ErrCategoryDisabled); grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("disabled category status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}

func TestToPbTopicPollMarksViewerSelectionsAndExpiry(t *testing.T) {
	expiresAt := time.Now().Add(-time.Minute)
	poll := toPbTopicPoll(&topicDomain.Poll{
		Multiple:    true,
		ExpiresAt:   &expiresAt,
		TotalVoters: 3,
		MyChoices:   []int32{1},
		Choices: []topicDomain.PollChoice{
			{Index: 0, Text: "first", Votes: 1},
			{Index: 1, Text: "second", Votes: 2},
		},
	})

	if !poll.GetHasVoted() || !poll.GetExpired() {
		t.Fatalf("poll state = has_voted %v expired %v, want true/true", poll.GetHasVoted(), poll.GetExpired())
	}
	if poll.GetTotalVoters() != 3 || poll.GetChoices()[0].GetSelected() || !poll.GetChoices()[1].GetSelected() {
		t.Fatalf("unexpected poll projection: %+v", poll)
	}
}

func TestToStatusMapsPollErrors(t *testing.T) {
	if err := toStatus(topicDomain.ErrPollAlreadyVoted); grpcstatus.Code(err) != codes.AlreadyExists {
		t.Fatalf("already-voted status code = %v, want %v", grpcstatus.Code(err), codes.AlreadyExists)
	}
	if err := toStatus(topicDomain.ErrPollExpired); grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expired status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
	if err := toStatus(topicDomain.ErrPollSelectionInvalid); grpcstatus.Code(err) != codes.InvalidArgument {
		t.Fatalf("selection status code = %v, want %v", grpcstatus.Code(err), codes.InvalidArgument)
	}
}

func TestToStatusMapsCannotAcceptOwnComment(t *testing.T) {
	err := toStatus(topicDomain.ErrCannotAcceptOwnComment)
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusMapsBountyCreditInsufficient(t *testing.T) {
	err := toStatus(topicDomain.ErrBountyCreditInsufficient)
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}

func TestToStatusMapsQAAcceptanceReversalErrors(t *testing.T) {
	if err := toStatus(topicDomain.ErrQAAcceptanceSettlementPending); grpcstatus.Code(err) != codes.Aborted {
		t.Fatalf("pending status code = %v, want %v", grpcstatus.Code(err), codes.Aborted)
	}
	if err := toStatus(topicDomain.ErrQAAcceptanceReversalInsufficientCredit); grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("insufficient status code = %v, want %v", grpcstatus.Code(err), codes.FailedPrecondition)
	}
}
