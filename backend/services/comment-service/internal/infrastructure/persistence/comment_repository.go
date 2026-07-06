package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "comment-service/internal/domain/comment"

	"go.mongodb.org/mongo-driver/v2/bson"
	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const commentCollection = "comments"

type Repository struct {
	db *drivermongo.Database
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(db *drivermongo.Database) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	models := []drivermongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "entityType", Value: 1},
				{Key: "entityId", Value: 1},
				{Key: "parentId", Value: 1},
				{Key: "status", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("idx_comments_entity_parent_status_created"),
		},
		{
			Keys: bson.D{
				{Key: "rootId", Value: 1},
				{Key: "status", Value: 1},
				{Key: "createdAt", Value: 1},
			},
			Options: options.Index().SetName("idx_comments_root_status_created"),
		},
		{
			Keys: bson.D{
				{Key: "authorId", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("idx_comments_author_created"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("idx_comments_status_created"),
		},
	}
	_, err := r.comments().Indexes().CreateMany(ctx, models)
	if err != nil {
		return fmt.Errorf("create comment indexes: %w", err)
	}
	return nil
}

func (r *Repository) Save(ctx context.Context, c *domain.Comment) error {
	if c == nil {
		return nil
	}
	if err := c.Validate(); err != nil {
		return err
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	doc := toDocument(c)
	_, err := r.comments().InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*domain.Comment, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidID
	}
	var doc commentDocument
	err := r.comments().FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if errors.Is(err, drivermongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find comment: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *Repository) ListByEntity(ctx context.Context, q domain.ListQuery) ([]*domain.Comment, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := bson.M{
		"entityType": q.EntityType,
		"entityId":   q.EntityID,
		"parentId":   int64(0),
		"status":     int32(domain.StatusVisible),
	}
	total, err := r.comments().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}
	opts := options.Find().
		SetSkip(int64((q.Page - 1) * q.PageSize)).
		SetLimit(int64(q.PageSize)).
		SetSort(bson.D{{Key: "createdAt", Value: -1}})
	rows, err := r.find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) ListReplies(ctx context.Context, q domain.ReplyListQuery) ([]*domain.Comment, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := bson.M{
		"rootId": bson.M{"$eq": q.RootID},
		"status": int32(domain.StatusVisible),
	}
	total, err := r.comments().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count replies: %w", err)
	}
	opts := options.Find().
		SetSkip(int64((q.Page - 1) * q.PageSize)).
		SetLimit(int64(q.PageSize)).
		SetSort(bson.D{{Key: "createdAt", Value: 1}})
	rows, err := r.find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) ListForModeration(ctx context.Context, q domain.ModerationListQuery) ([]*domain.Comment, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := bson.M{}
	if q.EntityType != "" {
		filter["entityType"] = q.EntityType
	}
	if q.EntityID > 0 {
		filter["entityId"] = q.EntityID
	}
	if q.AuthorID > 0 {
		filter["authorId"] = q.AuthorID
	}
	if q.Status >= 0 {
		filter["status"] = q.Status
	}
	total, err := r.comments().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count moderation comments: %w", err)
	}
	opts := options.Find().
		SetSkip(int64((q.Page - 1) * q.PageSize)).
		SetLimit(int64(q.PageSize)).
		SetSort(bson.D{{Key: "createdAt", Value: -1}})
	rows, err := r.find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *Repository) Hide(ctx context.Context, c *domain.Comment) error {
	if c == nil || c.ID <= 0 {
		return domain.ErrInvalidID
	}
	update := bson.M{
		"$set": bson.M{
			"status":    int32(c.Status),
			"updatedAt": c.UpdatedAt,
			"deletedAt": c.DeletedAt,
		},
	}
	res, err := r.comments().UpdateOne(ctx, bson.M{"_id": c.ID}, update)
	if err != nil {
		return fmt.Errorf("hide comment: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) IncrementReplyCount(ctx context.Context, rootID int64, delta int64) error {
	if rootID <= 0 {
		return domain.ErrInvalidParent
	}
	res, err := r.comments().UpdateOne(ctx, bson.M{"_id": rootID}, bson.M{
		"$inc": bson.M{"replyCount": delta},
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("increment reply count: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) find(ctx context.Context, filter any, opts *options.FindOptionsBuilder) ([]*domain.Comment, error) {
	cur, err := r.comments().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find comments: %w", err)
	}
	defer cur.Close(ctx)
	var docs []commentDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode comments: %w", err)
	}
	out := make([]*domain.Comment, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

func (r *Repository) comments() *drivermongo.Collection {
	return r.db.Collection(commentCollection)
}

func normalizeList(page *int, pageSize *int) {
	if *page <= 0 {
		*page = 1
	}
	if *pageSize <= 0 {
		*pageSize = 20
	}
	if *pageSize > 100 {
		*pageSize = 100
	}
}

type commentDocument struct {
	ID         int64      `bson:"_id"`
	EntityType string     `bson:"entityType"`
	EntityID   int64      `bson:"entityId"`
	RootID     int64      `bson:"rootId"`
	ParentID   int64      `bson:"parentId"`
	AuthorID   int64      `bson:"authorId"`
	Content    string     `bson:"content"`
	Status     int32      `bson:"status"`
	ReplyCount int64      `bson:"replyCount"`
	LikeCount  int64      `bson:"likeCount"`
	CreatedAt  time.Time  `bson:"createdAt"`
	UpdatedAt  time.Time  `bson:"updatedAt"`
	DeletedAt  *time.Time `bson:"deletedAt,omitempty"`
}

func toDocument(c *domain.Comment) commentDocument {
	return commentDocument{
		ID:         c.ID,
		EntityType: c.EntityType,
		EntityID:   c.EntityID,
		RootID:     c.RootID,
		ParentID:   c.ParentID,
		AuthorID:   c.AuthorID,
		Content:    c.Content,
		Status:     int32(c.Status),
		ReplyCount: c.ReplyCount,
		LikeCount:  c.LikeCount,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		DeletedAt:  c.DeletedAt,
	}
}

func (d commentDocument) toDomain() *domain.Comment {
	return &domain.Comment{
		ID:         d.ID,
		EntityType: d.EntityType,
		EntityID:   d.EntityID,
		RootID:     d.RootID,
		ParentID:   d.ParentID,
		AuthorID:   d.AuthorID,
		Content:    d.Content,
		Status:     domain.Status(d.Status),
		ReplyCount: d.ReplyCount,
		LikeCount:  d.LikeCount,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
		DeletedAt:  d.DeletedAt,
	}
}
