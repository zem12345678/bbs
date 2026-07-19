package credit

import (
	"context"
	"fmt"
	"strings"

	"content-service/api/proto/creditpb"
	topiccommand "content-service/internal/application/topic/command"
	topicDomain "content-service/internal/domain/topic"
	iocgrpc "content-service/internal/ioc/grpc"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	client creditpb.CreditServiceClient
}

func NewClient(grpcClient *iocgrpc.Client, v *viper.Viper) (*Client, error) {
	service := serviceName(v.GetString("upstreams.credit"))
	if service == "" {
		service = "bbs-credit-service"
	}
	conn, err := grpcClient.Dial(service, false)
	if err != nil {
		return nil, err
	}
	return &Client{client: creditpb.NewCreditServiceClient(conn)}, nil
}

func serviceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "credit-service" {
		return "bbs-credit-service"
	}
	return value
}

func (c *Client) ReserveQABounty(ctx context.Context, userID, topicID, amount int64, title string) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	_, err := c.client.ReserveCredits(ctx, &creditpb.ReserveCreditsRequest{
		UserId:        userID,
		Amount:        amount,
		Reason:        "qa_bounty_reserved",
		Description:   qaBountyReservationDescription(topicID, title),
		SourceEventId: fmt.Sprintf("content.qa.bounty:%d", topicID),
		SourceType:    "topic",
		SourceId:      topicID,
	})
	if status.Code(err) == codes.FailedPrecondition {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) ReleaseQABounty(ctx context.Context, userID, topicID, amount int64, title string) (bool, error) {
	if amount <= 0 {
		return true, nil
	}
	_, err := c.client.ReleaseCredits(ctx, &creditpb.ReleaseCreditsRequest{
		UserId:            userID,
		Amount:            amount,
		ReservationReason: "qa_bounty_reserved",
		ReleaseReason:     "qa_bounty_released",
		Description:       qaBountyReleaseDescription(topicID, title),
		SourceEventId:     fmt.Sprintf("content.qa.bounty:%d", topicID),
		SourceType:        "topic",
		SourceId:          topicID,
	})
	if status.Code(err) == codes.NotFound {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *Client) ReverseQAAcceptance(ctx context.Context, questionAuthorID, topicID, acceptedCommentID, acceptedCommentAuthorID, amount, acceptanceCycle int64, title string) error {
	_, err := c.client.ReverseQAAcceptance(ctx, &creditpb.ReverseQAAcceptanceRequest{
		QuestionAuthorId:        questionAuthorID,
		TopicId:                 topicID,
		AcceptedCommentId:       acceptedCommentID,
		AcceptedCommentAuthorId: acceptedCommentAuthorID,
		RewardCredits:           amount,
		AcceptanceCycle:         acceptanceCycle,
		Title:                   title,
	})
	switch status.Code(err) {
	case codes.OK:
		return nil
	case codes.Aborted:
		return topicDomain.ErrQAAcceptanceSettlementPending
	case codes.FailedPrecondition:
		return topicDomain.ErrQAAcceptanceReversalInsufficientCredit
	default:
		return err
	}
}

var _ topiccommand.BountyCreditReader = (*Client)(nil)

func qaBountyReservationDescription(topicID int64, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("问答悬赏冻结：话题 #%d", topicID)
	}
	return fmt.Sprintf("问答悬赏冻结：话题《%s》", title)
}

func qaBountyReleaseDescription(topicID int64, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("问答悬赏返还：话题 #%d", topicID)
	}
	return fmt.Sprintf("问答悬赏返还：话题《%s》", title)
}
