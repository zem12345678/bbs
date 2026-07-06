package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	domain "comment-service/internal/domain/comment"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func TestRepositoryMongoSmoke(t *testing.T) {
	if os.Getenv("BBS_MONGO_SMOKE") != "1" {
		t.Skip("set BBS_MONGO_SMOKE=1 to run MongoDB smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	uri := getenv("BBS_MONGO_URI", "mongodb://127.0.0.1:27017")
	dbName := getenv("BBS_MONGO_DATABASE", "bbs_comment")
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		t.Fatalf("ping mongo: %v", err)
	}

	repo := NewRepository(client.Database(dbName))
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	entityID := time.Now().UnixNano()
	defer func() {
		_, _ = repo.comments().DeleteMany(context.Background(), bson.M{"entityId": entityID})
	}()

	root, err := domain.NewRoot(entityID, domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   entityID,
		AuthorID:   101,
		Content:    "mongo smoke root",
	})
	if err != nil {
		t.Fatalf("new root: %v", err)
	}
	root.Events()
	if err := repo.Save(ctx, root); err != nil {
		t.Fatalf("save root: %v", err)
	}

	reply, err := domain.NewReply(entityID+1, domain.CreateCmd{
		EntityType: string(domain.EntityArticle),
		EntityID:   entityID,
		AuthorID:   102,
		Content:    "mongo smoke reply",
	}, root.ID, root.ID)
	if err != nil {
		t.Fatalf("new reply: %v", err)
	}
	reply.Events()
	if err := repo.Save(ctx, reply); err != nil {
		t.Fatalf("save reply: %v", err)
	}
	if err := repo.IncrementReplyCount(ctx, root.ID, 1); err != nil {
		t.Fatalf("increment reply count: %v", err)
	}

	comments, total, err := repo.ListByEntity(ctx, domain.ListQuery{
		EntityType: string(domain.EntityArticle),
		EntityID:   entityID,
		Page:       1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if total != 1 || len(comments) != 1 || comments[0].ID != root.ID {
		t.Fatalf("root list total=%d len=%d", total, len(comments))
	}

	replies, total, err := repo.ListReplies(ctx, domain.ReplyListQuery{RootID: root.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list replies: %v", err)
	}
	if total != 1 || len(replies) != 1 || replies[0].ID != reply.ID {
		t.Fatalf("reply list total=%d len=%d", total, len(replies))
	}

	if err := root.Hide(101, false); err != nil {
		t.Fatalf("hide root: %v", err)
	}
	if err := repo.Hide(ctx, root); err != nil {
		t.Fatalf("persist hide: %v", err)
	}
	hidden, err := repo.FindByID(ctx, root.ID)
	if err != nil {
		t.Fatalf("find hidden: %v", err)
	}
	if hidden.Status != domain.StatusHidden {
		t.Fatalf("status=%d, want hidden", hidden.Status)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
