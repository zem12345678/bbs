package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	accountDomain "content-service/internal/domain/account"
	channelDomain "content-service/internal/domain/channel"
	topicDomain "content-service/internal/domain/topic"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type channelPO struct {
	ID          int64  `gorm:"primaryKey"`
	OwnerID     int64  `gorm:"not null;index"`
	CategoryID  *int64 `gorm:"index"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text;not null;default:''"`
	Color       string `gorm:"size:16;not null;default:'#3b82f6'"`
	IsArchived  bool   `gorm:"not null;default:false;index"`
	IsFeatured  bool   `gorm:"not null;default:false;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (channelPO) TableName() string { return "channels" }

type channelViewPO struct {
	ID              int64
	OwnerID         int64
	CategoryID      *int64
	Name            string
	Description     string
	Color           string
	IsArchived      bool
	IsFeatured      bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	FollowersCount  int64      `gorm:"column:followers_count"`
	TopicsCount     int64      `gorm:"column:topics_count"`
	LastPostedAt    *time.Time `gorm:"column:last_posted_at"`
	ViewerFollowing bool       `gorm:"column:viewer_following"`
	ViewerFavorited bool       `gorm:"column:viewer_favorited"`
}

type channelRelationPO struct {
	ChannelID int64 `gorm:"primaryKey"`
	UserID    int64 `gorm:"primaryKey"`
	CreatedAt time.Time
}

type ChannelRepo struct {
	db *gorm.DB
}

func NewChannelRepo(db *gorm.DB) *ChannelRepo {
	return &ChannelRepo{db: db}
}

func (r *ChannelRepo) CreateChannel(ctx context.Context, channel *channelDomain.Channel) error {
	po := channelToPO(channel)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockContentUser(tx, channel.OwnerID); err != nil {
			return err
		}
		erased, err := contentUserErased(tx, channel.OwnerID)
		if err != nil {
			return err
		}
		if erased {
			return accountDomain.ErrUserErased
		}
		return tx.Create(&po).Error
	})
}

func (r *ChannelRepo) UpdateChannel(ctx context.Context, channel *channelDomain.Channel) error {
	po := channelToPO(channel)
	res := r.db.WithContext(ctx).Model(&channelPO{}).
		Where("id = ? AND is_archived = FALSE", channel.ID).
		Updates(map[string]any{
			"category_id": po.CategoryID,
			"name":        po.Name,
			"description": po.Description,
			"color":       po.Color,
			"updated_at":  po.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return r.channelStateError(ctx, channel.ID)
	}
	return nil
}

func (r *ChannelRepo) ArchiveChannel(ctx context.Context, channel *channelDomain.Channel) error {
	res := r.db.WithContext(ctx).Model(&channelPO{}).
		Where("id = ? AND is_archived = FALSE", channel.ID).
		Updates(map[string]any{"is_archived": true, "is_featured": false, "updated_at": channel.UpdatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return r.channelStateError(ctx, channel.ID)
	}
	return nil
}

func (r *ChannelRepo) SetChannelFeatured(ctx context.Context, channel *channelDomain.Channel) error {
	q := r.db.WithContext(ctx).Model(&channelPO{}).Where("id = ?", channel.ID)
	if channel.IsFeatured {
		q = q.Where("is_archived = FALSE")
	}
	res := q.Updates(map[string]any{"is_featured": channel.IsFeatured, "updated_at": channel.UpdatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return r.channelStateError(ctx, channel.ID)
	}
	return nil
}

func (r *ChannelRepo) SetChannelArchived(ctx context.Context, channel *channelDomain.Channel) error {
	res := r.db.WithContext(ctx).Model(&channelPO{}).
		Where("id = ?", channel.ID).
		Updates(map[string]any{
			"is_archived": channel.IsArchived,
			"is_featured": channel.IsFeatured,
			"updated_at":  channel.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return r.channelStateError(ctx, channel.ID)
	}
	return nil
}

func (r *ChannelRepo) FindChannelByID(ctx context.Context, id, viewerID int64, includeArchived bool) (*channelDomain.Channel, error) {
	var row channelViewPO
	err := r.channelQuery(ctx, viewerID).
		Where("channels.id = ?", id).
		Where("? OR channels.is_archived = FALSE", includeArchived).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, channelDomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return channelToEntity(&row), nil
}

func (r *ChannelRepo) ListChannels(ctx context.Context, filter channelDomain.ListFilter) ([]*channelDomain.Channel, int64, error) {
	q := r.channelQuery(ctx, filter.ViewerID).
		Joins("LEFT JOIN categories AS channel_category ON channel_category.id = channels.category_id")
	switch filter.ArchivedStatus {
	case channelDomain.ArchivedStatusActive:
		q = q.Where("channels.is_archived = FALSE")
	case channelDomain.ArchivedStatusArchived:
		q = q.Where("channels.is_archived = TRUE")
	default:
		if !filter.IncludeArchived {
			q = q.Where("channels.is_archived = FALSE")
		}
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		pattern := "%" + query + "%"
		q = q.Where("channels.name ILIKE ? OR channels.description ILIKE ?", pattern, pattern)
	}
	if category := strings.TrimSpace(filter.Category); category != "" {
		q = q.Where("LOWER(channel_category.slug) = LOWER(?)", category)
	}
	if filter.CategoryID > 0 {
		q = q.Where("channels.category_id = ?", filter.CategoryID)
	}
	if filter.Uncategorized {
		q = q.Where("channels.category_id IS NULL")
	}
	if filter.OwnerID > 0 {
		q = q.Where("channels.owner_id = ?", filter.OwnerID)
	}
	if filter.FollowerUserID > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM channel_followers AS selected_follow WHERE selected_follow.channel_id = channels.id AND selected_follow.user_id = ?)", filter.FollowerUserID)
	}
	if filter.FavoritedUserID > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM channel_favorites AS selected_favorite WHERE selected_favorite.channel_id = channels.id AND selected_favorite.user_id = ?)", filter.FavoritedUserID)
	}

	order := "last_posted_at DESC NULLS LAST, channels.updated_at DESC, channels.id DESC"
	if filter.Featured {
		q = q.Where("channels.is_featured = TRUE")
		order = "followers_count DESC, topics_count DESC, last_posted_at DESC NULLS LAST, channels.id DESC"
	}
	var rows []channelViewPO
	if err := q.Order(order).Limit(normalizeLimit(filter.Limit)).Offset(normalizeOffset(filter.Offset)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*channelDomain.Channel, 0, len(rows))
	for i := range rows {
		out = append(out, channelToEntity(&rows[i]))
	}
	return out, total, nil
}

func (r *ChannelRepo) FollowChannel(ctx context.Context, channelID, userID int64) error {
	return r.addChannelRelation(ctx, "channel_followers", channelID, userID)
}

func (r *ChannelRepo) UnfollowChannel(ctx context.Context, channelID, userID int64) error {
	return r.removeChannelRelation(ctx, "channel_followers", channelID, userID)
}

func (r *ChannelRepo) FavoriteChannel(ctx context.Context, channelID, userID int64) error {
	return r.addChannelRelation(ctx, "channel_favorites", channelID, userID)
}

func (r *ChannelRepo) UnfavoriteChannel(ctx context.Context, channelID, userID int64) error {
	return r.removeChannelRelation(ctx, "channel_favorites", channelID, userID)
}

func (r *ChannelRepo) ListChannelCategoryAggregates(ctx context.Context, includeArchived bool) ([]channelDomain.CategoryAggregate, error) {
	where := "WHERE channels.is_archived = FALSE"
	if includeArchived {
		where = ""
	}
	var rows []channelDomain.CategoryAggregate
	err := r.db.WithContext(ctx).Raw(`
SELECT COALESCE(channels.category_id, 0) AS category_id,
       COALESCE(categories.slug, '') AS slug,
       COALESCE(categories.name, '') AS name,
       COUNT(channels.id) AS channel_count,
       COALESCE(SUM((SELECT COUNT(*) FROM channel_followers WHERE channel_followers.channel_id = channels.id)), 0) AS followers_count,
       COALESCE(SUM((SELECT COUNT(*) FROM topics WHERE topics.channel_id = channels.id AND topics.status = ?)), 0) AS topics_count,
       MAX((SELECT MAX(topics.published_at) FROM topics WHERE topics.channel_id = channels.id AND topics.status = ?)) AS last_posted_at
FROM channels
LEFT JOIN categories ON categories.id = channels.category_id
`+where+`
GROUP BY channels.category_id, categories.slug, categories.name
ORDER BY channel_count DESC, name ASC, category_id ASC
`, int32(topicDomain.StatusPublished), int32(topicDomain.StatusPublished)).Scan(&rows).Error
	return rows, err
}

func (r *ChannelRepo) channelQuery(ctx context.Context, viewerID int64) *gorm.DB {
	return r.db.WithContext(ctx).Table("channels").Select(`
channels.*,
(SELECT COUNT(*) FROM channel_followers WHERE channel_followers.channel_id = channels.id) AS followers_count,
(SELECT COUNT(*) FROM topics WHERE topics.channel_id = channels.id AND topics.status = ?) AS topics_count,
(SELECT MAX(topics.published_at) FROM topics WHERE topics.channel_id = channels.id AND topics.status = ?) AS last_posted_at,
CASE WHEN CAST(? AS BIGINT) > 0 THEN EXISTS (SELECT 1 FROM channel_followers WHERE channel_followers.channel_id = channels.id AND channel_followers.user_id = ?) ELSE FALSE END AS viewer_following,
CASE WHEN CAST(? AS BIGINT) > 0 THEN EXISTS (SELECT 1 FROM channel_favorites WHERE channel_favorites.channel_id = channels.id AND channel_favorites.user_id = ?) ELSE FALSE END AS viewer_favorited
`, int32(topicDomain.StatusPublished), int32(topicDomain.StatusPublished), viewerID, viewerID, viewerID, viewerID)
}

func (r *ChannelRepo) addChannelRelation(ctx context.Context, table string, channelID, userID int64) error {
	if channelID <= 0 {
		return channelDomain.ErrNotFound
	}
	if userID <= 0 {
		return channelDomain.ErrForbidden
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockContentUser(tx, userID); err != nil {
			return err
		}
		erased, err := contentUserErased(tx, userID)
		if err != nil {
			return err
		}
		if erased {
			return accountDomain.ErrUserErased
		}
		var channel channelPO
		err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Select("id", "is_archived").First(&channel, channelID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return channelDomain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if channel.IsArchived {
			return channelDomain.ErrArchived
		}
		return tx.Table(table).Clauses(clause.OnConflict{DoNothing: true}).Create(&channelRelationPO{
			ChannelID: channelID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}).Error
	})
}

func (r *ChannelRepo) removeChannelRelation(ctx context.Context, table string, channelID, userID int64) error {
	if channelID <= 0 || userID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Table(table).Where("channel_id = ? AND user_id = ?", channelID, userID).Delete(&channelRelationPO{}).Error
}

func (r *ChannelRepo) channelStateError(ctx context.Context, id int64) error {
	var channel channelPO
	err := r.db.WithContext(ctx).Select("id", "is_archived").First(&channel, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return channelDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if channel.IsArchived {
		return channelDomain.ErrArchived
	}
	return channelDomain.ErrNotFound
}

func channelToPO(channel *channelDomain.Channel) channelPO {
	var categoryID *int64
	if channel.CategoryID > 0 {
		value := channel.CategoryID
		categoryID = &value
	}
	return channelPO{
		ID:          channel.ID,
		OwnerID:     channel.OwnerID,
		CategoryID:  categoryID,
		Name:        channel.Name,
		Description: channel.Description,
		Color:       channel.Color,
		IsArchived:  channel.IsArchived,
		IsFeatured:  channel.IsFeatured,
		CreatedAt:   channel.CreatedAt,
		UpdatedAt:   channel.UpdatedAt,
	}
}

func channelToEntity(row *channelViewPO) *channelDomain.Channel {
	var categoryID int64
	if row.CategoryID != nil {
		categoryID = *row.CategoryID
	}
	return &channelDomain.Channel{
		ID:              row.ID,
		OwnerID:         row.OwnerID,
		CategoryID:      categoryID,
		Name:            row.Name,
		Description:     row.Description,
		Color:           row.Color,
		IsArchived:      row.IsArchived,
		IsFeatured:      row.IsFeatured,
		FollowersCount:  row.FollowersCount,
		TopicsCount:     row.TopicsCount,
		LastPostedAt:    row.LastPostedAt,
		ViewerFollowing: row.ViewerFollowing,
		ViewerFavorited: row.ViewerFavorited,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
