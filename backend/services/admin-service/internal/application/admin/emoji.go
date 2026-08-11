package admin

import (
	"context"
	"mime"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	domain "admin/internal/domain/admin"
)

const (
	maxEmojiNameLength    = 128
	maxEmojiAliases       = 64
	maxEmojiTextField     = 128
	maxEmojiLicenseLength = 1024
	maxEmojiURLLength     = 1024
	maxEmojiContentType   = 64
)

var emojiNamePattern = regexp.MustCompile(`^[\p{L}\p{N}\p{M}_+\-]+$`)

func (s *Service) ListEmojis(ctx context.Context, actor domain.Actor, query string, limit int32, offset int32) (domain.EmojiList, error) {
	if actor.ID > 0 || actor.Username != "" {
		if err := actor.Validate(); err != nil {
			return domain.EmojiList{}, err
		}
		if err := s.auth.Authorize(ctx, actor, domain.ActionListEmojis); err != nil {
			return domain.EmojiList{}, err
		}
	}
	store, ok := s.ops.(EmojiStore)
	if !ok {
		return domain.EmojiList{}, domain.ErrEmojiStoreUnavailable
	}
	return store.ListEmojis(ctx, strings.TrimSpace(query), limit, offset)
}

func (s *Service) GetEmoji(ctx context.Context, name string) (domain.Emoji, error) {
	name = strings.TrimSpace(name)
	if !validEmojiName(name) {
		return domain.Emoji{}, domain.ErrInvalidEmoji
	}
	store, ok := s.ops.(EmojiStore)
	if !ok {
		return domain.Emoji{}, domain.ErrEmojiStoreUnavailable
	}
	return store.GetEmojiByName(ctx, name)
}

func (s *Service) CreateEmoji(ctx context.Context, actor domain.Actor, command domain.CreateEmojiCommand) (domain.Emoji, error) {
	if err := actor.Validate(); err != nil {
		return domain.Emoji{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionCreateEmoji); err != nil {
		return domain.Emoji{}, err
	}
	normalized, err := normalizeCreateEmoji(command)
	if err != nil {
		return domain.Emoji{}, err
	}
	store, ok := s.ops.(EmojiStore)
	if !ok {
		return domain.Emoji{}, domain.ErrEmojiStoreUnavailable
	}
	return store.CreateEmoji(ctx, normalized)
}

func (s *Service) UpdateEmoji(ctx context.Context, actor domain.Actor, command domain.UpdateEmojiCommand) (domain.Emoji, error) {
	if err := actor.Validate(); err != nil {
		return domain.Emoji{}, err
	}
	if command.ID <= 0 {
		return domain.Emoji{}, domain.ErrInvalidEmojiID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionUpdateEmoji); err != nil {
		return domain.Emoji{}, err
	}
	normalized, err := normalizeUpdateEmoji(command)
	if err != nil {
		return domain.Emoji{}, err
	}
	store, ok := s.ops.(EmojiStore)
	if !ok {
		return domain.Emoji{}, domain.ErrEmojiStoreUnavailable
	}
	return store.UpdateEmoji(ctx, normalized)
}

func (s *Service) DeleteEmoji(ctx context.Context, actor domain.Actor, id int64) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidEmojiID
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionDeleteEmoji); err != nil {
		return err
	}
	store, ok := s.ops.(EmojiStore)
	if !ok {
		return domain.ErrEmojiStoreUnavailable
	}
	return store.DeleteEmoji(ctx, id)
}

func normalizeCreateEmoji(command domain.CreateEmojiCommand) (domain.CreateEmojiCommand, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.URL = strings.TrimSpace(command.URL)
	command.OriginalURL = strings.TrimSpace(command.OriginalURL)
	command.ContentType = strings.ToLower(strings.TrimSpace(command.ContentType))
	if !validEmojiName(command.Name) || !validEmojiURL(command.URL) || (command.OriginalURL != "" && !validEmojiURL(command.OriginalURL)) || !validEmojiContentType(command.ContentType) {
		return domain.CreateEmojiCommand{}, domain.ErrInvalidEmoji
	}
	var ok bool
	if command.Category, ok = normalizedNullableText(command.Category, maxEmojiTextField); !ok {
		return domain.CreateEmojiCommand{}, domain.ErrInvalidEmoji
	}
	if command.License, ok = normalizedNullableText(command.License, maxEmojiLicenseLength); !ok {
		return domain.CreateEmojiCommand{}, domain.ErrInvalidEmoji
	}
	if command.Aliases, ok = normalizedEmojiList(command.Aliases, maxEmojiAliases); !ok {
		return domain.CreateEmojiCommand{}, domain.ErrInvalidEmoji
	}
	if command.RoleIDsThatCanBeUsedThisEmojiAsReaction, ok = normalizedEmojiList(command.RoleIDsThatCanBeUsedThisEmojiAsReaction, maxEmojiAliases); !ok {
		return domain.CreateEmojiCommand{}, domain.ErrInvalidEmoji
	}
	return command, nil
}

func normalizeUpdateEmoji(command domain.UpdateEmojiCommand) (domain.UpdateEmojiCommand, error) {
	if command.Name != nil {
		value := strings.TrimSpace(*command.Name)
		if !validEmojiName(value) {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.Name = &value
	}
	if command.URL != nil {
		value := strings.TrimSpace(*command.URL)
		if !validEmojiURL(value) {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.URL = &value
	}
	if command.OriginalURL != nil {
		value := strings.TrimSpace(*command.OriginalURL)
		if !validEmojiURL(value) {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.OriginalURL = &value
	}
	if command.ContentType != nil {
		value := strings.ToLower(strings.TrimSpace(*command.ContentType))
		if !validEmojiContentType(value) {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.ContentType = &value
	}
	var ok bool
	if command.Category != nil {
		var value *string
		if value, ok = normalizedNullableText(*command.Category, maxEmojiTextField); !ok {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.Category = &value
	}
	if command.License != nil {
		var value *string
		if value, ok = normalizedNullableText(*command.License, maxEmojiLicenseLength); !ok {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.License = &value
	}
	if command.Aliases != nil {
		var values []string
		if values, ok = normalizedEmojiList(*command.Aliases, maxEmojiAliases); !ok {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.Aliases = &values
	}
	if command.RoleIDsThatCanBeUsedThisEmojiAsReaction != nil {
		var values []string
		if values, ok = normalizedEmojiList(*command.RoleIDsThatCanBeUsedThisEmojiAsReaction, maxEmojiAliases); !ok {
			return domain.UpdateEmojiCommand{}, domain.ErrInvalidEmoji
		}
		command.RoleIDsThatCanBeUsedThisEmojiAsReaction = &values
	}
	return command, nil
}

func validEmojiName(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxEmojiNameLength && emojiNamePattern.MatchString(value)
}

func validEmojiURL(value string) bool {
	if value == "" || len(value) > maxEmojiURLLength {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validEmojiContentType(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maxEmojiContentType {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.HasPrefix(mediaType, "image/")
}

func normalizedNullableText(value *string, maxLength int) (*string, bool) {
	if value == nil {
		return nil, true
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, true
	}
	if utf8.RuneCountInString(normalized) > maxLength {
		return nil, false
	}
	return &normalized, true
}

func normalizedEmojiList(values []string, maxItems int) ([]string, bool) {
	if len(values) > maxItems {
		return nil, false
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || utf8.RuneCountInString(value) > maxEmojiTextField {
			return nil, false
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out, true
}
