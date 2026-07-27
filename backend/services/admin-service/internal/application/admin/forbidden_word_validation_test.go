package admin

import (
	"errors"
	"strings"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestNormalizeForbiddenWordCommandDefaultsAndTrims(t *testing.T) {
	command, err := normalizeForbiddenWordCommand(domain.UpsertForbiddenWordCommand{
		Word:        "  spam  ",
		Replacement: "  stale replacement  ",
		Description: "  blocks spam  ",
	})
	if err != nil {
		t.Fatalf("normalizeForbiddenWordCommand() error = %v", err)
	}
	if command.Word != "spam" || command.Scene != "content" || command.Action != "reject" || command.Replacement != "" || command.Description != "blocks spam" || command.Status != 2 {
		t.Fatalf("normalized command = %#v", command)
	}
}

func TestNormalizeForbiddenWordCommandAcceptsSupportedValues(t *testing.T) {
	for _, scene := range []string{"content", "comment", "profile", "account"} {
		for _, action := range []string{"reject", "review", "replace"} {
			t.Run(scene+"/"+action, func(t *testing.T) {
				command, err := normalizeForbiddenWordCommand(domain.UpsertForbiddenWordCommand{
					Word:        "广告",
					Scene:       "  " + strings.ToUpper(scene) + "  ",
					Action:      "  " + strings.ToUpper(action) + "  ",
					Replacement: "  ",
					Status:      1,
				})
				if err != nil {
					t.Fatalf("normalizeForbiddenWordCommand() error = %v", err)
				}
				if command.Scene != scene || command.Action != action || command.Status != 1 {
					t.Fatalf("normalized command = %#v", command)
				}
			})
		}
	}
}

func TestNormalizeForbiddenWordCommandRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		command domain.UpsertForbiddenWordCommand
	}{
		{name: "missing word", command: domain.UpsertForbiddenWordCommand{}},
		{name: "long word", command: domain.UpsertForbiddenWordCommand{Word: strings.Repeat("敏", 129)}},
		{name: "unsupported scene", command: domain.UpsertForbiddenWordCommand{Word: "spam", Scene: "message"}},
		{name: "unsupported action", command: domain.UpsertForbiddenWordCommand{Word: "spam", Action: "allow"}},
		{name: "invalid status", command: domain.UpsertForbiddenWordCommand{Word: "spam", Status: 3}},
		{name: "long replacement", command: domain.UpsertForbiddenWordCommand{Word: "spam", Action: "replace", Replacement: strings.Repeat("*", 129)}},
		{name: "long description", command: domain.UpsertForbiddenWordCommand{Word: "spam", Description: strings.Repeat("a", 513)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeForbiddenWordCommand(tt.command)
			if !errors.Is(err, domain.ErrInvalidForbiddenWord) {
				t.Fatalf("normalizeForbiddenWordCommand() error = %v, want ErrInvalidForbiddenWord", err)
			}
		})
	}
}
