package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type articlePO struct {
	ID          int64     `gorm:"primaryKey"`
	Slug        string    `gorm:"uniqueIndex;size:128;not null"`
	Title       string    `gorm:"size:180;not null"`
	Summary     string    `gorm:"type:text"`
	Body        string    `gorm:"type:text;not null"`
	CoverURL    string    `gorm:"size:1024"`
	Tags        string    `gorm:"type:jsonb;not null;default:'[]'"`
	AuthorID    int64     `gorm:"index;not null"`
	Status      int32     `gorm:"not null;default:1;index"`
	CreatedAt   time.Time `gorm:"index"`
	UpdatedAt   time.Time
	PublishedAt *time.Time `gorm:"index"`
	ViewCount   int64      `gorm:"not null;default:0"`
}

func (articlePO) TableName() string {
	return "articles"
}

type topicPO struct {
	ID          int64     `gorm:"primaryKey"`
	Slug        string    `gorm:"uniqueIndex;size:128;not null"`
	Type        string    `gorm:"size:16;not null;default:'topic';index"`
	Title       string    `gorm:"size:180;not null;default:''"`
	Body        string    `gorm:"type:text;not null"`
	Tags        string    `gorm:"type:jsonb;not null;default:'[]'"`
	AuthorID    int64     `gorm:"index;not null"`
	CategoryID  int64     `gorm:"index;not null;default:1"`
	Status      int32     `gorm:"not null;default:1;index"`
	CreatedAt   time.Time `gorm:"index"`
	UpdatedAt   time.Time
	PublishedAt *time.Time `gorm:"index"`
	ViewCount   int64      `gorm:"not null;default:0"`
}

func (topicPO) TableName() string {
	return "topics"
}

type categoryPO struct {
	ID          int64  `gorm:"primaryKey"`
	Slug        string `gorm:"uniqueIndex;size:128;not null"`
	Name        string `gorm:"size:80;not null"`
	Description string `gorm:"type:text;not null;default:''"`
	Sort        int32  `gorm:"not null;default:0"`
	Status      int32  `gorm:"not null;default:2;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	TopicCount  int64 `gorm:"-"`
}

func (categoryPO) TableName() string {
	return "categories"
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

type TopicRepo struct {
	db *gorm.DB
}

func NewTopicRepo(db *gorm.DB) *TopicRepo {
	return &TopicRepo{db: db}
}

type CategoryRepo struct {
	db *gorm.DB
}

func NewCategoryRepo(db *gorm.DB) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func toPO(a *articleDomain.Article) articlePO {
	tags, _ := json.Marshal(a.Tags)
	return articlePO{
		ID:          a.ID,
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		Body:        a.Body,
		CoverURL:    a.CoverURL,
		Tags:        string(tags),
		AuthorID:    a.AuthorID,
		Status:      int32(a.Status),
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		PublishedAt: a.PublishedAt,
		ViewCount:   a.ViewCount,
	}
}

func toEntity(p *articlePO) *articleDomain.Article {
	var tags []string
	_ = json.Unmarshal([]byte(p.Tags), &tags)
	return &articleDomain.Article{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Summary:     p.Summary,
		Body:        p.Body,
		CoverURL:    p.CoverURL,
		Tags:        tags,
		AuthorID:    p.AuthorID,
		Status:      articleDomain.Status(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		PublishedAt: p.PublishedAt,
		ViewCount:   p.ViewCount,
	}
}

func toEntities(rows []articlePO) []*articleDomain.Article {
	out := make([]*articleDomain.Article, 0, len(rows))
	for i := range rows {
		out = append(out, toEntity(&rows[i]))
	}
	return out
}

func topicToPO(t *topicDomain.Topic) topicPO {
	tags, _ := json.Marshal(t.Tags)
	return topicPO{
		ID:          t.ID,
		Slug:        t.Slug,
		Type:        string(t.Type),
		Title:       t.Title,
		Body:        t.Body,
		Tags:        string(tags),
		AuthorID:    t.AuthorID,
		CategoryID:  t.CategoryID,
		Status:      int32(t.Status),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		PublishedAt: t.PublishedAt,
		ViewCount:   t.ViewCount,
	}
}

func topicToEntity(p *topicPO) *topicDomain.Topic {
	var tags []string
	_ = json.Unmarshal([]byte(p.Tags), &tags)
	return &topicDomain.Topic{
		ID:          p.ID,
		Slug:        p.Slug,
		Type:        topicDomain.NormalizeType(p.Type),
		Title:       p.Title,
		Body:        p.Body,
		Tags:        tags,
		AuthorID:    p.AuthorID,
		CategoryID:  p.CategoryID,
		Status:      topicDomain.Status(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		PublishedAt: p.PublishedAt,
		ViewCount:   p.ViewCount,
	}
}

func categoryToEntity(p *categoryPO) *categoryDomain.Category {
	return &categoryDomain.Category{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Sort:        p.Sort,
		Status:      categoryDomain.Status(p.Status),
		TopicCount:  p.TopicCount,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func categoryToPO(c *categoryDomain.Category) categoryPO {
	return categoryPO{
		ID:          c.ID,
		Slug:        c.Slug,
		Name:        c.Name,
		Description: c.Description,
		Sort:        c.Sort,
		Status:      int32(c.Status),
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func categoriesToEntities(rows []categoryPO) []*categoryDomain.Category {
	out := make([]*categoryDomain.Category, 0, len(rows))
	for i := range rows {
		out = append(out, categoryToEntity(&rows[i]))
	}
	return out
}

func topicToEntities(rows []topicPO) []*topicDomain.Topic {
	out := make([]*topicDomain.Topic, 0, len(rows))
	for i := range rows {
		out = append(out, topicToEntity(&rows[i]))
	}
	return out
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeTagLimit(limit int) int {
	if limit <= 0 {
		return 12
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func articleListOrder(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "active", "updated":
		return "updated_at DESC, published_at DESC NULLS LAST, id DESC"
	case "hot":
		return "view_count DESC, published_at DESC NULLS LAST, id DESC"
	default:
		return "published_at DESC NULLS LAST, id DESC"
	}
}

func topicListOrder(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "active", "updated", "recent-replies":
		return "updated_at DESC, published_at DESC NULLS LAST, id DESC"
	case "hot":
		return "view_count DESC, updated_at DESC, published_at DESC NULLS LAST, id DESC"
	default:
		return "published_at DESC NULLS LAST, id DESC"
	}
}

func (r *Repo) Create(ctx context.Context, a *articleDomain.Article) error {
	po := toPO(a)
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return articleDomain.ErrSlugExists
	}
	return nil
}

func (r *Repo) Update(ctx context.Context, a *articleDomain.Article) error {
	po := toPO(a)
	res := r.db.WithContext(ctx).Model(&articlePO{}).Where("id = ?", a.ID).Updates(map[string]any{
		"title":      po.Title,
		"summary":    po.Summary,
		"body":       po.Body,
		"cover_url":  po.CoverURL,
		"tags":       po.Tags,
		"updated_at": po.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return articleDomain.ErrNotFound
	}
	return nil
}

func (r *Repo) FindBySlug(ctx context.Context, slug string) (*articleDomain.Article, error) {
	var p articlePO
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, articleDomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toEntity(&p), nil
}

func (r *Repo) FindByID(ctx context.Context, id int64) (*articleDomain.Article, error) {
	var p articlePO
	err := r.db.WithContext(ctx).First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, articleDomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toEntity(&p), nil
}

func (r *Repo) List(ctx context.Context, status articleDomain.Status, tag string, authorID int64, sort string, limit, offset int) ([]*articleDomain.Article, error) {
	q := r.db.WithContext(ctx).Model(&articlePO{})
	if status > 0 {
		q = q.Where("status = ?", int32(status))
	}
	if authorID > 0 {
		q = q.Where("author_id = ?", authorID)
	}
	if tag != "" {
		q = q.Where("tags::text LIKE ?", "%\""+tag+"\"%")
	}
	var rows []articlePO
	if err := q.Order(articleListOrder(sort)).Limit(normalizeLimit(limit)).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return toEntities(rows), nil
}

func (r *Repo) ListTags(ctx context.Context, status articleDomain.Status, keyword string, limit int) ([]articleDomain.TagStats, error) {
	if status <= 0 {
		status = articleDomain.StatusPublished
	}
	args := []any{int32(status)}
	where := "WHERE articles.status = ?"
	if keyword != "" {
		where += " AND tag_value ILIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	args = append(args, normalizeTagLimit(limit))

	var rows []articleDomain.TagStats
	err := r.db.WithContext(ctx).Raw(`
SELECT tag_value AS name, COUNT(*) AS count
FROM articles
CROSS JOIN LATERAL jsonb_array_elements_text(tags) AS tag_value
`+where+`
  AND tag_value <> ''
GROUP BY tag_value
ORDER BY COUNT(*) DESC, tag_value ASC
LIMIT ?
`, args...).Scan(&rows).Error
	return rows, err
}

func (r *Repo) UpdateStatus(ctx context.Context, id int64, status articleDomain.Status, publishedAt *time.Time) error {
	updates := map[string]any{
		"status":     int32(status),
		"updated_at": time.Now(),
	}
	if publishedAt != nil {
		updates["published_at"] = publishedAt
	}
	res := r.db.WithContext(ctx).Model(&articlePO{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return articleDomain.ErrNotFound
	}
	return nil
}

func (r *Repo) FeedByTime(ctx context.Context, limit, offset int) ([]*articleDomain.Article, error) {
	var rows []articlePO
	err := r.db.WithContext(ctx).
		Where("status = ?", int32(articleDomain.StatusPublished)).
		Order("published_at DESC NULLS LAST, id DESC").
		Limit(normalizeLimit(limit)).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return toEntities(rows), nil
}

func (r *Repo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*articleDomain.Article, error) {
	out := make(map[int64]*articleDomain.Article, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []articlePO
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].ID] = toEntity(&rows[i])
	}
	return out, nil
}

func (r *Repo) IncrementViewCount(ctx context.Context, id int64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&articlePO{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, articleDomain.ErrNotFound
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&articlePO{}).Select("view_count").Where("id = ?", id).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *TopicRepo) CreateTopic(ctx context.Context, t *topicDomain.Topic) error {
	po := topicToPO(t)
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return topicDomain.ErrSlugExists
	}
	return nil
}

func (r *TopicRepo) UpdateTopic(ctx context.Context, t *topicDomain.Topic) error {
	po := topicToPO(t)
	res := r.db.WithContext(ctx).Model(&topicPO{}).Where("id = ?", t.ID).Updates(map[string]any{
		"title":       po.Title,
		"body":        po.Body,
		"tags":        po.Tags,
		"category_id": po.CategoryID,
		"updated_at":  po.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return topicDomain.ErrNotFound
	}
	return nil
}

func (r *TopicRepo) FindTopicBySlug(ctx context.Context, slug string) (*topicDomain.Topic, error) {
	var p topicPO
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, topicDomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return topicToEntity(&p), nil
}

func (r *TopicRepo) FindTopicByID(ctx context.Context, id int64) (*topicDomain.Topic, error) {
	var p topicPO
	err := r.db.WithContext(ctx).First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, topicDomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return topicToEntity(&p), nil
}

func (r *TopicRepo) ListTopics(ctx context.Context, status topicDomain.Status, typ topicDomain.Type, tag string, authorID int64, categoryID int64, sort string, limit, offset int) ([]*topicDomain.Topic, error) {
	q := r.db.WithContext(ctx).Model(&topicPO{})
	if status > 0 {
		q = q.Where("status = ?", int32(status))
	}
	if typ != "" {
		q = q.Where("type = ?", string(typ))
	}
	if authorID > 0 {
		q = q.Where("author_id = ?", authorID)
	}
	if categoryID > 0 {
		q = q.Where("category_id = ?", categoryID)
	}
	if tag != "" {
		q = q.Where("tags::text LIKE ?", "%\""+tag+"\"%")
	}
	var rows []topicPO
	if err := q.Order(topicListOrder(sort)).Limit(normalizeLimit(limit)).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return topicToEntities(rows), nil
}

func (r *TopicRepo) UpdateTopicStatus(ctx context.Context, id int64, status topicDomain.Status, publishedAt *time.Time) error {
	updates := map[string]any{
		"status":     int32(status),
		"updated_at": time.Now(),
	}
	if publishedAt != nil {
		updates["published_at"] = publishedAt
	}
	res := r.db.WithContext(ctx).Model(&topicPO{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return topicDomain.ErrNotFound
	}
	return nil
}

func (r *TopicRepo) IncrementTopicViewCount(ctx context.Context, id int64) (int64, error) {
	res := r.db.WithContext(ctx).Model(&topicPO{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, topicDomain.ErrNotFound
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&topicPO{}).Select("view_count").Where("id = ?", id).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *CategoryRepo) FindCategoryByID(ctx context.Context, id int64) (*categoryDomain.Category, error) {
	var row categoryPO
	err := r.db.WithContext(ctx).Raw(`
SELECT categories.*,
       COUNT(topics.id) FILTER (WHERE topics.status = ?) AS topic_count
FROM categories
LEFT JOIN topics ON topics.category_id = categories.id
WHERE categories.id = ?
GROUP BY categories.id
`, int32(topicDomain.StatusPublished), id).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, categoryDomain.ErrNotFound
	}
	return categoryToEntity(&row), nil
}

func (r *CategoryRepo) ListCategories(ctx context.Context, status categoryDomain.Status, limit, offset int) ([]*categoryDomain.Category, error) {
	q := r.db.WithContext(ctx).Model(&categoryPO{}).
		Select("categories.*, COUNT(topics.id) FILTER (WHERE topics.status = ?) AS topic_count", int32(topicDomain.StatusPublished)).
		Joins("LEFT JOIN topics ON topics.category_id = categories.id").
		Group("categories.id").
		Order("categories.sort ASC, categories.id ASC").
		Limit(normalizeLimit(limit)).
		Offset(offset)
	if status > 0 {
		q = q.Where("categories.status = ?", int32(status))
	}
	var rows []categoryPO
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return categoriesToEntities(rows), nil
}

func (r *CategoryRepo) CreateCategory(ctx context.Context, category *categoryDomain.Category) error {
	po := categoryToPO(category)
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return categoryDomain.ErrSlugExists
	}
	return nil
}

func (r *CategoryRepo) UpdateCategory(ctx context.Context, category *categoryDomain.Category) error {
	po := categoryToPO(category)
	res := r.db.WithContext(ctx).Model(&categoryPO{}).Where("id = ?", category.ID).Updates(map[string]any{
		"slug":        po.Slug,
		"name":        po.Name,
		"description": po.Description,
		"sort":        po.Sort,
		"status":      po.Status,
		"updated_at":  po.UpdatedAt,
	})
	if duplicateKey(res.Error) {
		return categoryDomain.ErrSlugExists
	}
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return categoryDomain.ErrNotFound
	}
	return nil
}

func (r *CategoryRepo) DeleteCategory(ctx context.Context, id int64) error {
	var topicCount int64
	if err := r.db.WithContext(ctx).Model(&topicPO{}).Where("category_id = ?", id).Count(&topicCount).Error; err != nil {
		return err
	}
	if topicCount > 0 {
		return categoryDomain.ErrInUse
	}
	res := r.db.WithContext(ctx).Delete(&categoryPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return categoryDomain.ErrNotFound
	}
	return nil
}

func duplicateKey(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
