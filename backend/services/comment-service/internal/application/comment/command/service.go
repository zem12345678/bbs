package command

import (
	"context"

	domain "comment-service/internal/domain/comment"
	"comment-service/internal/infrastructure/messaging"
	"comment-service/pkg/logger"
)

type IDGenerator interface {
	Generate() int64
}

type Service struct {
	repo      domain.Repository
	idgen     IDGenerator
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{repo: repo, idgen: idgen, publisher: publisher, log: log}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Comment, error) {
	var (
		c   *domain.Comment
		err error
	)
	if cmd.ParentID > 0 {
		c, err = s.createReply(ctx, cmd)
	} else {
		c, err = domain.NewRoot(s.idgen.Generate(), cmd)
	}
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return nil, err
	}
	if !c.IsRoot() {
		if err := s.repo.IncrementReplyCount(ctx, c.RootID, 1); err != nil && s.log != nil {
			s.log.Warn("increment comment reply count failed", logger.Int64("root_id", c.RootID), logger.Error(err))
		}
	}
	s.publishEvents(ctx, c.Events()...)
	return c, nil
}

func (s *Service) createReply(ctx context.Context, cmd domain.CreateCmd) (*domain.Comment, error) {
	parent, err := s.repo.FindByID(ctx, cmd.ParentID)
	if err != nil {
		return nil, err
	}
	if parent.Status != domain.StatusVisible {
		return nil, domain.ErrInvalidParent
	}
	entityType, err := domain.ParseEntityType(cmd.EntityType)
	if err != nil {
		return nil, err
	}
	if cmd.EntityID <= 0 {
		return nil, domain.ErrInvalidEntityID
	}
	if parent.EntityType != string(entityType) || parent.EntityID != cmd.EntityID {
		return nil, domain.ErrInvalidParent
	}
	rootID := parent.RootID
	if parent.IsRoot() {
		rootID = parent.ID
	}
	return domain.NewReply(s.idgen.Generate(), cmd, rootID, parent.ID)
}

func (s *Service) Delete(ctx context.Context, id int64, actorID int64, moderator bool) (*domain.Comment, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.Hide(actorID, moderator); err != nil {
		return nil, err
	}
	if err := s.repo.Hide(ctx, c); err != nil {
		return nil, err
	}
	if !c.IsRoot() {
		if err := s.repo.IncrementReplyCount(ctx, c.RootID, -1); err != nil && s.log != nil {
			s.log.Warn("decrement comment reply count failed", logger.Int64("root_id", c.RootID), logger.Error(err))
		}
	}
	s.publishEvents(ctx, c.Events()...)
	return c, nil
}

func (s *Service) Restore(ctx context.Context, id int64, actorID int64, moderator bool) (*domain.Comment, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.Restore(actorID, moderator); err != nil {
		return nil, err
	}
	if err := s.repo.Restore(ctx, c); err != nil {
		return nil, err
	}
	if !c.IsRoot() {
		if err := s.repo.IncrementReplyCount(ctx, c.RootID, 1); err != nil && s.log != nil {
			s.log.Warn("increment comment reply count failed", logger.Int64("root_id", c.RootID), logger.Error(err))
		}
	}
	s.publishEvents(ctx, c.Events()...)
	return c, nil
}

func (s *Service) RedactAccountComments(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (int64, error) {
	if userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return 0, domain.ErrInvalidUserErasure
	}
	return s.repo.RedactAccountComments(ctx, userID, deletionJobID, policyVersion)
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	if err := s.publisher.PublishDomainEvents(ctx, events); err != nil && s.log != nil {
		s.log.Warn("publish comment events failed", logger.Error(err))
	}
}
