package persistence

import (
	"context"

	domain "mall-service/internal/domain/mall"
)

const resolveMallUserLockIDSQL = `
SELECT COALESCE(
  (SELECT user_id
   FROM mall_erased_users
   WHERE anonymized_user_id = $1::BIGINT),
  $1::BIGINT
)`

const mallUserErasedSQL = `
SELECT EXISTS (
  SELECT 1
  FROM mall_erased_users
  WHERE user_id = $1::BIGINT
     OR anonymized_user_id = $1::BIGINT
)`

func lockActiveMallUser(ctx context.Context, db queryer, userID int64) error {
	lockUserID := userID
	if err := db.QueryRow(ctx, resolveMallUserLockIDSQL, userID).Scan(&lockUserID); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, mallUserAdvisoryLockSQL, lockUserID); err != nil {
		return err
	}

	var erased bool
	if err := db.QueryRow(ctx, mallUserErasedSQL, userID).Scan(&erased); err != nil {
		return err
	}
	if erased {
		return domain.ErrAccountErased
	}
	return nil
}
