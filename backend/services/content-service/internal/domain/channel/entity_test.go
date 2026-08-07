package channel

import (
	"errors"
	"strings"
	"testing"
)

func TestNewNormalizesChannel(t *testing.T) {
	channel, err := New(10, CreateCmd{OwnerID: 20, Name: "  Engineering  ", Description: "  Discussions  "})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if channel.Name != "Engineering" || channel.Description != "Discussions" {
		t.Fatalf("channel text was not normalized: %#v", channel)
	}
	if channel.Color != DefaultColor {
		t.Fatalf("Color = %q, want %q", channel.Color, DefaultColor)
	}
	if channel.CategoryID != 0 {
		t.Fatalf("CategoryID = %d, want 0", channel.CategoryID)
	}
}

func TestChannelValidation(t *testing.T) {
	tests := []struct {
		name string
		cmd  CreateCmd
		want error
	}{
		{name: "owner", cmd: CreateCmd{Name: "name"}, want: ErrOwnerRequired},
		{name: "name", cmd: CreateCmd{OwnerID: 1}, want: ErrNameRequired},
		{name: "name too long", cmd: CreateCmd{OwnerID: 1, Name: strings.Repeat("a", 129)}, want: ErrNameTooLong},
		{name: "description too long", cmd: CreateCmd{OwnerID: 1, Name: "name", Description: strings.Repeat("a", 2049)}, want: ErrDescriptionTooLong},
		{name: "color", cmd: CreateCmd{OwnerID: 1, Name: "name", Color: "blue"}, want: ErrColorInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(10, test.cmd)
			if !errors.Is(err, test.want) {
				t.Fatalf("New error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestArchivedChannelRejectsUpdate(t *testing.T) {
	channel, err := New(10, CreateCmd{OwnerID: 20, Name: "name"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := channel.Archive(); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if err := channel.Update(UpdateCmd{Name: "renamed"}); !errors.Is(err, ErrArchived) {
		t.Fatalf("Update error = %v, want %v", err, ErrArchived)
	}
}
