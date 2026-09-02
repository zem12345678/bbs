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
	store       domain.Store
	reports     domain.ReportRepository
	likes       domain.LikeRepository
	favorites   domain.FavoriteRepository
	reactions   domain.ReactionRepository
	pins        domain.PinRepository
	collections domain.CollectionRepository
}

func NewService(store domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository, pins domain.PinRepository, collections domain.CollectionRepository, reactionRepositories ...domain.ReactionRepository) *Service {
	var reactions domain.ReactionRepository
	if len(reactionRepositories) > 0 {
		reactions = reactionRepositories[0]
	}
	return &Service{store: store, reports: reports, likes: likes, favorites: favorites, reactions: reactions, pins: pins, collections: collections}
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

func (s *Service) ListReactions(ctx context.Context, userID int64, entityType domain.EntityType, limit, offset int, sinceID, untilID, sinceDate, untilDate int64) ([]*domain.Reaction, int64, error) {
	if s.reactions == nil {
		return nil, 0, domain.ErrReactionRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	if sinceID < 0 || untilID < 0 || sinceDate < 0 || untilDate < 0 {
		return nil, 0, domain.ErrInvalidReactionCursor
	}
	return s.reactions.ListReactions(ctx, userID, entityType, limit, offset, sinceID, untilID, sinceDate, untilDate)
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

func (s *Service) ListFavoritesAfterID(ctx context.Context, userID int64, entityType domain.EntityType, afterID int64, limit int) ([]*domain.Favorite, int64, error) {
	favorites, ok := s.favorites.(domain.FavoriteKeysetRepository)
	if !ok {
		return nil, 0, domain.ErrReportNotFound
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if entityType != "" && !entityType.Valid() {
		return nil, 0, domain.ErrInvalidEntityType
	}
	if afterID < 0 {
		return nil, 0, domain.ErrInvalidFavoriteCursor
	}
	return favorites.ListFavoritesAfterID(ctx, userID, entityType, afterID, limit)
}

func (s *Service) ListPins(ctx context.Context, userID int64, limit, offset int) ([]*domain.Pin, int64, error) {
	if s.pins == nil {
		return nil, 0, domain.ErrPinRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	return s.pins.ListPins(ctx, userID, limit, offset)
}

func (s *Service) ListCollections(ctx context.Context, userID int64, limit, offset int) ([]*domain.Collection, int64, error) {
	if s.collections == nil {
		return nil, 0, domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	return s.collections.ListCollections(ctx, userID, limit, offset)
}

func (s *Service) ListCollectionsAfterID(ctx context.Context, userID, afterID int64, limit int) ([]*domain.Collection, int64, error) {
	collections, ok := s.collections.(domain.CollectionKeysetRepository)
	if !ok {
		return nil, 0, domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if afterID < 0 {
		return nil, 0, domain.ErrInvalidCollectionCursor
	}
	return collections.ListCollectionsAfterID(ctx, userID, afterID, limit)
}

func (s *Service) ListCollectionItems(ctx context.Context, userID, collectionID int64, entityType domain.EntityType, limit, offset int) ([]*domain.CollectionItem, int64, error) {
	if s.collections == nil {
		return nil, 0, domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return nil, 0, domain.ErrInvalidCollectionID
	}
	if entityType != "" && !domain.ValidCollectionEntityType(entityType) {
		return nil, 0, domain.ErrInvalidCollectionEntityType
	}
	return s.collections.ListCollectionItems(ctx, userID, collectionID, entityType, limit, offset)
}

func (s *Service) ListCollectionItemsAfterID(ctx context.Context, userID, collectionID int64, entityType domain.EntityType, afterID int64, limit int) ([]*domain.CollectionItem, int64, error) {
	collections, ok := s.collections.(domain.CollectionKeysetRepository)
	if !ok {
		return nil, 0, domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if collectionID <= 0 {
		return nil, 0, domain.ErrInvalidCollectionID
	}
	if entityType != "" && !domain.ValidCollectionEntityType(entityType) {
		return nil, 0, domain.ErrInvalidCollectionEntityType
	}
	if afterID < 0 {
		return nil, 0, domain.ErrInvalidCollectionCursor
	}
	return collections.ListCollectionItemsAfterID(ctx, userID, collectionID, entityType, afterID, limit)
}

func (s *Service) GetCollection(ctx context.Context, collectionID, viewerUserID int64) (*domain.Collection, error) {
	publicCollections, ok := s.collections.(domain.PublicCollectionRepository)
	if !ok {
		return nil, domain.ErrCollectionRepositoryUnavailable
	}
	if collectionID <= 0 {
		return nil, domain.ErrInvalidCollectionID
	}
	if viewerUserID < 0 {
		return nil, domain.ErrInvalidUserID
	}
	return publicCollections.GetCollection(ctx, collectionID, viewerUserID)
}

func (s *Service) ListPublicCollectionItems(ctx context.Context, collectionID, viewerUserID int64, limit, offset int, sinceID, untilID int64) ([]*domain.CollectionItem, int64, error) {
	publicCollections, ok := s.collections.(domain.PublicCollectionRepository)
	if !ok {
		return nil, 0, domain.ErrCollectionRepositoryUnavailable
	}
	if collectionID <= 0 {
		return nil, 0, domain.ErrInvalidCollectionID
	}
	if viewerUserID < 0 {
		return nil, 0, domain.ErrInvalidUserID
	}
	if sinceID < 0 || untilID < 0 {
		return nil, 0, domain.ErrInvalidCollectionCursor
	}
	return publicCollections.ListPublicCollectionItems(ctx, collectionID, viewerUserID, limit, offset, sinceID, untilID)
}

func (s *Service) ListPublicCollections(ctx context.Context, userID, viewerUserID int64, limit int, sinceID, untilID int64) ([]*domain.Collection, error) {
	publicCollections, ok := s.collections.(domain.PublicCollectionRepository)
	if !ok {
		return nil, domain.ErrCollectionRepositoryUnavailable
	}
	if userID <= 0 || viewerUserID < 0 || sinceID < 0 || untilID < 0 {
		return nil, domain.ErrInvalidUserID
	}
	return publicCollections.ListPublicCollections(ctx, userID, viewerUserID, limit, sinceID, untilID)
}

func (s *Service) ListPublicCollectionsForEntity(ctx context.Context, entity domain.EntityRef, viewerUserID int64, limit int) ([]*domain.Collection, error) {
	publicCollections, ok := s.collections.(domain.PublicCollectionRepository)
	if !ok {
		return nil, domain.ErrCollectionRepositoryUnavailable
	}
	if viewerUserID < 0 {
		return nil, domain.ErrInvalidUserID
	}
	if err := entity.ValidateForCollection(); err != nil {
		return nil, err
	}
	return publicCollections.ListPublicCollectionsForEntity(ctx, entity, viewerUserID, limit)
}
