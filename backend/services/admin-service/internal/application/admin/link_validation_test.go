package admin

import (
	"errors"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestNormalizeLinkCommandTrimsSafeHTTPSLink(t *testing.T) {
	command, err := normalizeLinkCommand(domain.UpsertLinkCommand{
		Key:         "  docs  ",
		Title:       "  开发文档  ",
		URL:         "  https://docs.example.com/guides?lang=zh  ",
		Description: "  团队文档入口  ",
	})
	if err != nil {
		t.Fatalf("normalizeLinkCommand() error = %v", err)
	}
	if command.Key != "docs" || command.Title != "开发文档" || command.URL != "https://docs.example.com/guides?lang=zh" || command.Description != "团队文档入口" {
		t.Fatalf("normalized command = %#v", command)
	}
}

func TestNormalizeLinkCommandRejectsUnsafeURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "javascript scheme", url: "javascript:alert(1)"},
		{name: "data scheme", url: "data:text/html,unsafe"},
		{name: "ftp scheme", url: "ftp://files.example.com/manual.pdf"},
		{name: "relative path", url: "/docs/getting-started"},
		{name: "missing host", url: "https:///docs"},
		{name: "credentials", url: "https://admin:secret@example.com/private"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeLinkCommand(domain.UpsertLinkCommand{Title: "资源", URL: tt.url})
			if !errors.Is(err, domain.ErrInvalidLink) {
				t.Fatalf("normalizeLinkCommand() error = %v, want ErrInvalidLink", err)
			}
		})
	}
}
