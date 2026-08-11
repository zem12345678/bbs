package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func (r *Repository) ListEmojis(ctx context.Context, query string, limit int32, offset int32) (domain.EmojiList, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	db := r.db.WithContext(ctx).Model(&po.Emoji{})
	if query = strings.ToLower(strings.TrimSpace(query)); query != "" {
		like := "%" + query + "%"
		db = db.Where(`LOWER(name) LIKE ? OR LOWER(COALESCE(category, '')) LIKE ? OR EXISTS (
			SELECT 1
			FROM jsonb_array_elements_text(COALESCE(admin_emoji.aliases, '[]'::jsonb)) AS emoji_alias(value)
			WHERE LOWER(emoji_alias.value) LIKE ?
		)`, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.EmojiList{}, err
	}
	var rows []po.Emoji
	if err := db.Order("LOWER(COALESCE(category, '')) ASC, LOWER(name) ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.EmojiList{}, err
	}
	items := make([]domain.Emoji, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainEmoji(row))
	}
	return domain.EmojiList{Items: items, Total: total}, nil
}

func (r *Repository) GetEmojiByName(ctx context.Context, name string) (domain.Emoji, error) {
	var row po.Emoji
	err := r.db.WithContext(ctx).Where("LOWER(name) = ?", strings.ToLower(strings.TrimSpace(name))).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Emoji{}, domain.ErrEmojiNotFound
	}
	if err != nil {
		return domain.Emoji{}, err
	}
	return toDomainEmoji(row), nil
}

func (r *Repository) CreateEmoji(ctx context.Context, command domain.CreateEmojiCommand) (domain.Emoji, error) {
	now := time.Now()
	originalURL := command.OriginalURL
	if originalURL == "" {
		originalURL = command.URL
	}
	row := po.Emoji{
		Name: command.Name, URL: command.URL, OriginalURL: originalURL, ContentType: command.ContentType,
		Category: command.Category, Aliases: command.Aliases, License: command.License,
		IsSensitive: command.IsSensitive, LocalOnly: command.LocalOnly,
		RoleIDsThatCanBeUsedThisEmojiAsReaction: command.RoleIDsThatCanBeUsedThisEmojiAsReaction,
		CreatedAt:                               now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if emojiUniqueViolation(err) {
			return domain.Emoji{}, domain.ErrEmojiNameExists
		}
		return domain.Emoji{}, err
	}
	return toDomainEmoji(row), nil
}

func (r *Repository) UpdateEmoji(ctx context.Context, command domain.UpdateEmojiCommand) (domain.Emoji, error) {
	updates := po.Emoji{UpdatedAt: time.Now()}
	fields := []string{"updated_at"}
	if command.Name != nil {
		updates.Name = *command.Name
		fields = append(fields, "name")
	}
	if command.URL != nil {
		updates.URL = *command.URL
		fields = append(fields, "url")
	}
	if command.OriginalURL != nil {
		updates.OriginalURL = *command.OriginalURL
		fields = append(fields, "original_url")
	}
	if command.ContentType != nil {
		updates.ContentType = *command.ContentType
		fields = append(fields, "content_type")
	}
	if command.Category != nil {
		updates.Category = *command.Category
		fields = append(fields, "category")
	}
	if command.Aliases != nil {
		updates.Aliases = append([]string{}, (*command.Aliases)...)
		fields = append(fields, "aliases")
	}
	if command.License != nil {
		updates.License = *command.License
		fields = append(fields, "license")
	}
	if command.IsSensitive != nil {
		updates.IsSensitive = *command.IsSensitive
		fields = append(fields, "is_sensitive")
	}
	if command.LocalOnly != nil {
		updates.LocalOnly = *command.LocalOnly
		fields = append(fields, "local_only")
	}
	if command.RoleIDsThatCanBeUsedThisEmojiAsReaction != nil {
		updates.RoleIDsThatCanBeUsedThisEmojiAsReaction = append([]string{}, (*command.RoleIDsThatCanBeUsedThisEmojiAsReaction)...)
		fields = append(fields, "reaction_role_ids")
	}
	result := r.db.WithContext(ctx).Model(&po.Emoji{}).Where("id = ?", command.ID).Select(fields).Updates(&updates)
	if result.Error != nil {
		if emojiUniqueViolation(result.Error) {
			return domain.Emoji{}, domain.ErrEmojiNameExists
		}
		return domain.Emoji{}, result.Error
	}
	if result.RowsAffected == 0 {
		return domain.Emoji{}, domain.ErrEmojiNotFound
	}
	var row po.Emoji
	if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Emoji{}, domain.ErrEmojiNotFound
		}
		return domain.Emoji{}, err
	}
	return toDomainEmoji(row), nil
}

func (r *Repository) DeleteEmoji(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.Emoji{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrEmojiNotFound
	}
	return nil
}

func emojiUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func toDomainEmoji(row po.Emoji) domain.Emoji {
	return domain.Emoji{
		ID: row.ID, Name: row.Name, URL: row.URL, OriginalURL: row.OriginalURL, ContentType: row.ContentType,
		Category: row.Category, Aliases: append([]string(nil), row.Aliases...), License: row.License,
		IsSensitive: row.IsSensitive, LocalOnly: row.LocalOnly,
		RoleIDsThatCanBeUsedThisEmojiAsReaction: append([]string(nil), row.RoleIDsThatCanBeUsedThisEmojiAsReaction...),
		CreatedAt:                               timeMillis(row.CreatedAt), UpdatedAt: timeMillis(row.UpdatedAt),
	}
}
