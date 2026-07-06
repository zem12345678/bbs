package query

import (
	"context"

	domain "reaction-service/internal/domain/reaction"
)

type CountView struct {
	LikeCount     int64
	FavoriteCount int64
}

type Service struct {
	store     domain.Store
	reports   domain.ReportRepository
	likes     domain.LikeRepository
	favorites domain.FavoriteRepository
}

func NewService(store domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository) *Service {
	return &Service{store: store, reports: reports, likes: likes, favorites: favorites}
}

func (s *Service) Count(ctx context.Context, ref domain.EntityRef) (CountView, error) {
	if err := ref.Validate(); err != nil {
		return CountView{}, err
	}
	likes, err := s.store.LikeCount(ctx, ref)
	if s.likes != nil {
		likes, err = s.likes.Count(ctx, ref)
	}
	if err != nil {
		return CountView{}, err
	}
	favorites, err := s.store.FavoriteCount(ctx, ref)
	if s.favorites != nil {
		favorites, err = s.favorites.Count(ctx, ref)
	}
	if err != nil {
		return CountView{}, err
	}
	return CountView{LikeCount: likes, FavoriteCount: favorites}, nil
}

func (s *Service) HotIDs(ctx context.Context, entityType domain.EntityType, limit int) ([]int64, error) {
	if !entityType.Valid() {
		return nil, domain.ErrInvalidEntityType
	}
	if s.likes != nil {
		return s.likes.HotIDs(ctx, entityType, limit)
	}
	return s.store.HotIDs(ctx, entityType, limit)
}

func (s *Service) ListReports(ctx context.Context, status domain.ReportStatus, entityType domain.EntityType, limit, offset int) ([]*domain.Report, int64, error) {
	if s.reports == nil {
		return nil, 0, domain.ErrReportNotFound
	}
	if status != 0 && !status.Valid() {
		return nil, 0, domain.ErrInvalidReportStatus
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	return s.reports.ListReports(ctx, status, entityType, limit, offset)
}

func (s *Service) GetReport(ctx context.Context, id int64) (*domain.Report, error) {
	if s.reports == nil {
		return nil, domain.ErrReportNotFound
	}
	if id <= 0 {
		return nil, domain.ErrInvalidReportID
	}
	return s.reports.GetReport(ctx, id)
}

func (s *Service) ListLikes(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int) ([]*domain.Like, int64, error) {
	if s.likes == nil {
		return nil, 0, domain.ErrReportNotFound
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	return s.likes.ListLikes(ctx, userID, entityType, limit, offset)
}

func (s *Service) ListFavorites(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int) ([]*domain.Favorite, int64, error) {
	if s.favorites == nil {
		return nil, 0, domain.ErrReportNotFound
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	return s.favorites.ListFavorites(ctx, userID, entityType, limit, offset)
}
