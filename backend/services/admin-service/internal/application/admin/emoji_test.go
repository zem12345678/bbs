package admin

import (
	"errors"
	"strings"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestNormalizeCreateEmoji(t *testing.T) {
	category := "  reactions  "
	license := "  CC0  "
	command, err := normalizeCreateEmoji(domain.CreateEmojiCommand{
		Name: "  party  ", URL: "  https://cdn.example.com/party.webp  ", OriginalURL: " https://cdn.example.com/original.webp ", ContentType: " IMAGE/WEBP ",
		Category: &category, License: &license, Aliases: []string{" celebrate ", "CELEBRATE"},
	})
	if err != nil {
		t.Fatalf("normalizeCreateEmoji() error = %v", err)
	}
	if command.Name != "party" || command.URL != "https://cdn.example.com/party.webp" || command.OriginalURL != "https://cdn.example.com/original.webp" || command.ContentType != "image/webp" {
		t.Fatalf("normalized command = %#v", command)
	}
	if command.Category == nil || *command.Category != "reactions" || command.License == nil || *command.License != "CC0" {
		t.Fatalf("normalized nullable fields = category %#v, license %#v", command.Category, command.License)
	}
	if len(command.Aliases) != 1 || command.Aliases[0] != "celebrate" {
		t.Fatalf("normalized aliases = %#v", command.Aliases)
	}
}

func TestNormalizeUpdateEmojiPreservesClearAndFalsePresence(t *testing.T) {
	var clearCategory *string
	aliases := []string{}
	localOnly := false
	command, err := normalizeUpdateEmoji(domain.UpdateEmojiCommand{
		ID: 42, Category: &clearCategory, Aliases: &aliases, LocalOnly: &localOnly,
	})
	if err != nil {
		t.Fatalf("normalizeUpdateEmoji() error = %v", err)
	}
	if command.Category == nil || *command.Category != nil {
		t.Fatalf("category clear presence lost: %#v", command.Category)
	}
	if command.Aliases == nil || len(*command.Aliases) != 0 {
		t.Fatalf("empty aliases presence lost: %#v", command.Aliases)
	}
	if command.LocalOnly == nil || *command.LocalOnly {
		t.Fatalf("false localOnly presence lost: %#v", command.LocalOnly)
	}
}

func TestNormalizeEmojiRejectsUnsafeURLsAndInvalidNames(t *testing.T) {
	tests := []domain.CreateEmojiCommand{
		{Name: "party", URL: "javascript:alert(1)"},
		{Name: "party", URL: "https://user:secret@example.com/party.webp"},
		{Name: "party time", URL: "https://example.com/party.webp"},
	}
	for _, command := range tests {
		if _, err := normalizeCreateEmoji(command); !errors.Is(err, domain.ErrInvalidEmoji) {
			t.Fatalf("normalizeCreateEmoji(%#v) error = %v, want ErrInvalidEmoji", command, err)
		}
	}
}

func TestNormalizeEmojiValidatesContentType(t *testing.T) {
	valid := "IMAGE/" + strings.Repeat("a", maxEmojiContentType-len("image/"))
	command, err := normalizeCreateEmoji(domain.CreateEmojiCommand{
		Name: "party", URL: "https://example.com/party.webp", ContentType: valid,
	})
	if err != nil {
		t.Fatalf("normalizeCreateEmoji() error = %v", err)
	}
	if command.ContentType != strings.ToLower(valid) || len(command.ContentType) != maxEmojiContentType {
		t.Fatalf("normalized content type = %q", command.ContentType)
	}

	for _, contentType := range []string{
		"not-a-media-type",
		"text/html",
		"image/" + strings.Repeat("a", maxEmojiContentType-len("image/")+1),
	} {
		if _, err := normalizeCreateEmoji(domain.CreateEmojiCommand{
			Name: "party", URL: "https://example.com/party.webp", ContentType: contentType,
		}); !errors.Is(err, domain.ErrInvalidEmoji) {
			t.Errorf("normalizeCreateEmoji(contentType %q) error = %v, want ErrInvalidEmoji", contentType, err)
		}
		if _, err := normalizeUpdateEmoji(domain.UpdateEmojiCommand{ID: 1, ContentType: &contentType}); !errors.Is(err, domain.ErrInvalidEmoji) {
			t.Errorf("normalizeUpdateEmoji(contentType %q) error = %v, want ErrInvalidEmoji", contentType, err)
		}
	}
}
