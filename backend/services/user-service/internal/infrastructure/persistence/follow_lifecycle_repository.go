package persistence

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func createFollowLifecycle(tx *gorm.DB, followerID, followeeID int64, followedAt time.Time) error {
	return tx.Create(&followLifecyclePO{
		FollowerID: followerID,
		FolloweeID: followeeID,
		FollowedAt: followedAt,
	}).Error
}

func closeFollowLifecycle(tx *gorm.DB, followerID, followeeID int64, unfollowedAt time.Time) error {
	result := tx.Model(&followLifecyclePO{}).
		Where("follower_id = ? AND followee_id = ? AND unfollowed_at IS NULL", followerID, followeeID).
		Update("unfollowed_at", unfollowedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("active follow lifecycle missing for %d -> %d", followerID, followeeID)
	}
	return nil
}

func closeAllFollowLifecyclesForUser(tx *gorm.DB, userID int64, unfollowedAt time.Time) error {
	var activeCount int64
	if err := tx.Model(&followPO{}).
		Where("follower_id = ? OR followee_id = ?", userID, userID).
		Count(&activeCount).Error; err != nil {
		return err
	}
	result := tx.Model(&followLifecyclePO{}).
		Where("(follower_id = ? OR followee_id = ?) AND unfollowed_at IS NULL", userID, userID).
		Update("unfollowed_at", unfollowedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != activeCount {
		return fmt.Errorf("active follow lifecycle count mismatch for user %d: got %d, want %d", userID, result.RowsAffected, activeCount)
	}
	return nil
}
