package command

import (
	"context"
	"strings"
	"unicode/utf8"

	domain "reaction-service/internal/domain/reaction"
	"reaction-service/internal/infrastructure/messaging"
	"reaction-service/pkg/logger"
)

type Service struct {
	store       domain.Store
	reports     domain.ReportRepository
	likes       domain.LikeRepository
	favorites   domain.FavoriteRepository
	pins        domain.PinRepository
	collections domain.CollectionRepository
	publisher   messaging.EventPublisher
	log         logger.Logger
}

func NewService(store domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository, pins domain.PinRepository, collections domain.CollectionRepository, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{store: store, reports: reports, likes: likes, favorites: favorites, pins: pins, collections: collections, publisher: publisher, log: log}
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	if err := s.publisher.PublishDomainEvents(ctx, events); err != nil && s.log != nil {
		s.log.Warn("publish reaction events failed", logger.Error(err))
	}
}

type Result struct {
	Count   int64
	Changed bool
}

type ReportResult struct {
	Report  *domain.Report
	Created bool
}

func (s *Service) CreateCollection(ctx context.Context, userID int64, name, description string, isPublic bool) (*domain.Collection, error) {
	if s.collections == nil {
		return nil, domain.ErrCollectionRepositoryUnavailable
	}
	name, description, err := domain.ValidateCollectionFields(userID, name, description)
	if err != nil {
		return nil, err
	}
	collection := &domain.Collection{UserID: userID, Name: name, Description: description, IsPublic: isPublic}
	if err := s.collections.CreateCollection(ctx, collection); err != nil {
		return nil, err
	}
	return collection, nil
}

func (s *Service) UpdateCollection(ctx context.Context, userID, collectionID int64, name, description string, isPublic bool) (*domain.Collection, error) {
	if s.collections == nil {
		return nil, domain.ErrCollectionRepositoryUnavailable
	}
	name, description, err := domain.ValidateCollectionFields(userID, name, description)
	if err != nil {
		return nil, err
	}
	if collectionID <= 0 {
		return nil, domain.ErrInvalidCollectionID
	}
	return s.collections.UpdateCollection(ctx, userID, collectionID, name, description, isPublic)
}

func (s *Service) DeleteCollection(ctx context.Context, userID, collectionID int64) error {
	if s.collections == nil {
		return domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return domain.ErrInvalidCollectionID
	}
	return s.collections.DeleteCollection(ctx, userID, collectionID)
}

func (s *Service) AddCollectionItem(ctx context.Context, userID, collectionID int64, entity domain.EntityRef) (bool, error) {
	if s.collections == nil {
		return false, domain.ErrCollectionRepositoryUnavailable
	}
	if err := validateCollectionItem(userID, collectionID, entity); err != nil {
		return false, err
	}
	return s.collections.AddCollectionItem(ctx, userID, collectionID, entity)
}

func (s *Service) RemoveCollectionItem(ctx context.Context, userID, collectionID int64, entity domain.EntityRef) (bool, error) {
	if s.collections == nil {
		return false, domain.ErrCollectionRepositoryUnavailable
	}
	if err := validateCollectionItem(userID, collectionID, entity); err != nil {
		return false, err
	}
	return s.collections.RemoveCollectionItem(ctx, userID, collectionID, entity)
}

func (s *Service) Like(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if err := validate(ref, userID); err != nil {
		return Result{}, err
	}
	if s.likes != nil {
		count, changed, err := s.likes.Like(ctx, ref, userID)
		if err != nil {
			return Result{}, err
		}
		if _, _, err := s.store.Like(ctx, ref, userID); err != nil && s.log != nil {
			s.log.Warn("sync like cache failed", logger.Error(err))
		}
		s.publishEvents(ctx, domain.NewLikedEvent(ref, userID, count, changed))
		return Result{Count: count, Changed: changed}, nil
	}
	count, changed, err := s.store.Like(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	s.publishEvents(ctx, domain.NewLikedEvent(ref, userID, count, changed))
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) Unlike(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if err := validate(ref, userID); err != nil {
		return Result{}, err
	}
	if s.likes != nil {
		count, changed, err := s.likes.Unlike(ctx, ref, userID)
		if err != nil {
			return Result{}, err
		}
		if _, _, err := s.store.Unlike(ctx, ref, userID); err != nil && s.log != nil {
			s.log.Warn("sync like cache failed", logger.Error(err))
		}
		s.publishEvents(ctx, domain.NewUnlikedEvent(ref, userID, count, changed))
		return Result{Count: count, Changed: changed}, nil
	}
	count, changed, err := s.store.Unlike(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	s.publishEvents(ctx, domain.NewUnlikedEvent(ref, userID, count, changed))
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) Favorite(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if err := validate(ref, userID); err != nil {
		return Result{}, err
	}
	if s.favorites != nil {
		count, changed, err := s.favorites.Favorite(ctx, ref, userID)
		if err != nil {
			return Result{}, err
		}
		if _, _, err := s.store.Favorite(ctx, ref, userID); err != nil && s.log != nil {
			s.log.Warn("sync favorite cache failed", logger.Error(err))
		}
		s.publishEvents(ctx, domain.NewFavoritedEvent(ref, userID, count, changed))
		return Result{Count: count, Changed: changed}, nil
	}
	count, changed, err := s.store.Favorite(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	s.publishEvents(ctx, domain.NewFavoritedEvent(ref, userID, count, changed))
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) Unfavorite(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if err := validate(ref, userID); err != nil {
		return Result{}, err
	}
	if s.favorites != nil {
		count, changed, err := s.favorites.Unfavorite(ctx, ref, userID)
		if err != nil {
			return Result{}, err
		}
		if _, _, err := s.store.Unfavorite(ctx, ref, userID); err != nil && s.log != nil {
			s.log.Warn("sync favorite cache failed", logger.Error(err))
		}
		s.publishEvents(ctx, domain.NewUnfavoritedEvent(ref, userID, count, changed))
		return Result{Count: count, Changed: changed}, nil
	}
	count, changed, err := s.store.Unfavorite(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	s.publishEvents(ctx, domain.NewUnfavoritedEvent(ref, userID, count, changed))
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) Pin(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if s.pins == nil {
		return Result{}, domain.ErrPinRepositoryUnavailable
	}
	if err := validatePin(ref, userID); err != nil {
		return Result{}, err
	}
	count, changed, err := s.pins.Pin(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) Unpin(ctx context.Context, ref domain.EntityRef, userID int64) (Result, error) {
	if s.pins == nil {
		return Result{}, domain.ErrPinRepositoryUnavailable
	}
	if err := validatePin(ref, userID); err != nil {
		return Result{}, err
	}
	count, changed, err := s.pins.Unpin(ctx, ref, userID)
	if err != nil {
		return Result{}, err
	}
	return Result{Count: count, Changed: changed}, nil
}

func (s *Service) SubmitReport(ctx context.Context, cmd domain.SubmitReportCmd) (ReportResult, error) {
	if s.reports == nil {
		return ReportResult{}, domain.ErrReportNotFound
	}
	report, err := domain.NewReport(cmd)
	if err != nil {
		return ReportResult{}, err
	}
	created, err := s.reports.CreateReport(ctx, report)
	if err != nil {
		return ReportResult{}, err
	}
	if created {
		s.publishEvents(ctx, domain.NewReportSubmittedEvent(report))
	}
	return ReportResult{Report: report, Created: created}, nil
}

func (s *Service) AuditReport(ctx context.Context, id int64, nextStatus domain.ReportStatus, handlerID int64, auditNote string, targetAction string) (*domain.Report, error) {
	if s.reports == nil {
		return nil, domain.ErrReportNotFound
	}
	if id <= 0 {
		return nil, domain.ErrInvalidReportID
	}
	if !nextStatus.Valid() || nextStatus == domain.ReportStatusPending {
		return nil, domain.ErrInvalidReportStatus
	}
	if handlerID <= 0 {
		return nil, domain.ErrInvalidUserID
	}
	auditNote = strings.TrimSpace(auditNote)
	if utf8.RuneCountInString(auditNote) > domain.MaxReportAuditNoteRunes {
		return nil, domain.ErrInvalidReportNote
	}
	targetAction = strings.ToLower(strings.TrimSpace(targetAction))
	if targetAction != "" && targetAction != domain.ReportTargetActionHide {
		return nil, domain.ErrInvalidReportAction
	}
	return s.reports.AuditReport(ctx, id, nextStatus, handlerID, auditNote, targetAction)
}

func validate(ref domain.EntityRef, userID int64) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	return nil
}

func validateCollectionItem(userID, collectionID int64, entity domain.EntityRef) error {
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return domain.ErrInvalidCollectionID
	}
	return entity.ValidateForCollection()
}

func validatePin(entity domain.EntityRef, userID int64) error {
	if userID <= 0 {
		return domain.ErrInvalidUserID
	}
	return entity.ValidateForPin()
}
