package command

import (
	"context"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type IDGenerator interface {
	Generate() int64
}

type CommentRef struct {
	ID         int64
	EntityType string
	EntityID   int64
	AuthorID   int64
	Status     int32
}

const commentStatusVisible int32 = 1

type CommentReader interface {
	GetComment(ctx context.Context, id int64) (CommentRef, error)
}

type MembershipEntitlementReader interface {
	HasActiveMembership(ctx context.Context, userID int64) (bool, error)
}

type BountyCreditReader interface {
	ReserveQABounty(ctx context.Context, userID, topicID, amount int64, title string) (bool, error)
}

type Service struct {
	repo                   domain.Repository
	idgen                  IDGenerator
	publisher              messaging.EventPublisher
	commentReader          CommentReader
	membershipEntitlements MembershipEntitlementReader
	bountyCredits          BountyCreditReader
	log                    logger.Logger
}

func NewService(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, commentReader CommentReader, log logger.Logger, membershipEntitlements MembershipEntitlementReader, bountyCredits BountyCreditReader) *Service {
	return &Service{repo: repo, idgen: idgen, publisher: publisher, commentReader: commentReader, membershipEntitlements: membershipEntitlements, bountyCredits: bountyCredits, log: log}
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	out := make([]messaging.DomainEvent, 0, len(events))
	for _, event := range events {
		out = append(out, event)
	}
	if err := s.publisher.PublishDomainEvents(ctx, out); err != nil && s.log != nil {
		s.log.Warn("publish topic events failed", logger.Error(err))
	}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Topic, error) {
	t, err := domain.New(s.idgen.Generate(), cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateTopic(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Update(ctx context.Context, id int64, cmd domain.UpdateCmd) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	requiresMembership := topicBountyChangeRequiresMembership(t, cmd.BountyScore)
	if err := t.Update(cmd); err != nil {
		return nil, err
	}
	if requiresMembership {
		if err := s.ensureMembershipEntitlement(ctx, t); err != nil {
			return nil, err
		}
		if err := s.ensureBountyReserved(ctx, t); err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpdateTopic(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Publish(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Publish(); err != nil {
		return nil, err
	}
	if err := s.ensureMembershipEntitlement(ctx, t); err != nil {
		return nil, err
	}
	if err := s.ensureBountyReserved(ctx, t); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, t.PublishedAt); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicPublishedEvent(t))
	return t, nil
}

func (s *Service) Hide(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Hide(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, nil); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicHiddenEvent(t))
	return t, nil
}

func (s *Service) Archive(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Archive(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, nil); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicArchivedEvent(t))
	return t, nil
}

func (s *Service) ensureMembershipEntitlement(ctx context.Context, t *domain.Topic) error {
	if !topicRequiresMembership(t) {
		return nil
	}
	if s.membershipEntitlements == nil {
		return domain.ErrMembershipEntitlementRequired
	}
	ok, err := s.membershipEntitlements.HasActiveMembership(ctx, t.AuthorID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrMembershipEntitlementRequired
	}
	return nil
}

func topicRequiresMembership(t *domain.Topic) bool {
	return t != nil && t.Type == domain.TypeQA && t.BountyScore > 0
}

func topicBountyChangeRequiresMembership(t *domain.Topic, bountyScore int64) bool {
	if t == nil || t.Status != domain.StatusPublished {
		return false
	}
	return topicRequiresMembership(t) || (t.Type == domain.TypeQA && bountyScore > 0)
}

func (s *Service) ensureBountyReserved(ctx context.Context, t *domain.Topic) error {
	if t == nil || t.Type != domain.TypeQA || t.BountyScore <= 0 {
		return nil
	}
	if s.bountyCredits == nil {
		return domain.ErrBountyCreditInsufficient
	}
	ok, err := s.bountyCredits.ReserveQABounty(ctx, t.AuthorID, t.ID, t.BountyScore, t.Title)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrBountyCreditInsufficient
	}
	return nil
}

func (s *Service) AcceptComment(ctx context.Context, topicID, commentID, userID int64) (*domain.Topic, error) {
	if commentID <= 0 {
		return nil, domain.ErrInvalidComment
	}
	t, err := s.repo.FindTopicByID(ctx, topicID)
	if err != nil {
		return nil, err
	}
	if userID <= 0 || t.AuthorID != userID {
		return nil, domain.ErrTopicOwnerMismatch
	}
	if t.Type != domain.TypeQA {
		return nil, domain.ErrNotQuestion
	}
	if t.Status != domain.StatusPublished {
		return nil, domain.ErrNotPublished
	}
	if t.AcceptedCommentID > 0 && t.AcceptedCommentID != commentID {
		return nil, domain.ErrAlreadyAccepted
	}
	if t.AcceptedCommentID == commentID && t.AcceptedCommentAuthorID > 0 {
		return t, nil
	}
	comment, err := s.getAcceptableComment(ctx, topicID, commentID)
	if err != nil {
		return nil, err
	}
	if _, err := t.AcceptComment(comment.ID, comment.AuthorID); err != nil {
		return nil, err
	}
	if err := s.ensureMembershipEntitlement(ctx, t); err != nil {
		return nil, err
	}
	if err := s.ensureBountyReserved(ctx, t); err != nil {
		return nil, err
	}
	accepted, changed, err := s.repo.AcceptTopicComment(ctx, topicID, comment.ID, comment.AuthorID, t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if changed {
		s.publishEvents(ctx, domain.NewQAAcceptedEvent(accepted))
	}
	return accepted, nil
}

func (s *Service) getAcceptableComment(ctx context.Context, topicID, commentID int64) (CommentRef, error) {
	if s.commentReader == nil {
		return CommentRef{}, domain.ErrCommentNotFound
	}
	comment, err := s.commentReader.GetComment(ctx, commentID)
	if err != nil {
		return CommentRef{}, err
	}
	if comment.ID <= 0 || comment.AuthorID <= 0 {
		return CommentRef{}, domain.ErrCommentNotFound
	}
	if comment.Status != commentStatusVisible {
		return CommentRef{}, domain.ErrCommentNotFound
	}
	if comment.EntityType != "topic" || comment.EntityID != topicID {
		return CommentRef{}, domain.ErrCommentNotInTopic
	}
	return comment, nil
}
