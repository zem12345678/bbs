package persistence

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListCompatibilityUsesStableCursorAndTypeFilters(t *testing.T) {
	dsn := os.Getenv("BBS_NOTIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NOTIFICATION_TEST_DSN to run postgres-backed notification compatibility tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	defer pool.Close()
	repo := NewPostgresRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	userID := int64(9_900_000_000) + time.Now().UnixNano()%100_000_000
	prefix := fmt.Sprintf("compat-notification-%d-", userID)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = $1 OR source_event_id LIKE $2`, userID, prefix+"%")
	})
	insert := func(notificationType string) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `
INSERT INTO notifications(user_id, type, title, source_event_id)
VALUES($1, $2, 'compatibility test', $3)
RETURNING id`, userID, notificationType, prefix+notificationType).Scan(&id); err != nil {
			t.Fatalf("insert %s notification: %v", notificationType, err)
		}
		return id
	}
	followID := insert(domain.NotificationTypeFollow)
	commentID := insert(domain.NotificationTypeComment)
	likeID := insert(domain.NotificationTypeLike)

	included, err := repo.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{
		UserID: userID, Limit: 10, IncludeTypesSet: true, IncludeTypes: []string{domain.NotificationTypeFollow, domain.NotificationTypeLike},
	})
	if err != nil {
		t.Fatalf("list included notifications: %v", err)
	}
	assertCompatibilityIDs(t, included, likeID, followID)

	forward, err := repo.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{UserID: userID, Limit: 10, SinceID: followID})
	if err != nil {
		t.Fatalf("list forward notifications: %v", err)
	}
	assertCompatibilityIDs(t, forward, commentID, likeID)

	before, err := repo.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{UserID: userID, Limit: 10, UntilID: likeID})
	if err != nil {
		t.Fatalf("list backward notifications: %v", err)
	}
	assertCompatibilityIDs(t, before, commentID, followID)

	excluded, err := repo.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{
		UserID: userID, Limit: 10, ExcludeTypesSet: true, ExcludeTypes: []string{domain.NotificationTypeComment, domain.NotificationTypeLike},
	})
	if err != nil {
		t.Fatalf("list excluded notifications: %v", err)
	}
	assertCompatibilityIDs(t, excluded, followID)

	empty, err := repo.ListCompatibility(ctx, domain.NotificationCompatibilityQuery{UserID: userID, Limit: 10, IncludeTypesSet: true})
	if err != nil {
		t.Fatalf("list empty include: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty include returned %+v", empty)
	}
}

func assertCompatibilityIDs(t *testing.T, items []domain.Notification, want ...int64) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("notification count = %d, want %d (%+v)", len(items), len(want), items)
	}
	for index, id := range want {
		if items[index].ID != id {
			t.Fatalf("notification ID[%d] = %d, want %d", index, items[index].ID, id)
		}
	}
}
