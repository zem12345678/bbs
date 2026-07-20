package admin

import (
	"errors"
	"testing"

	domain "admin/internal/domain/admin"
)

func TestNormalizeTaskCommandTrimsSupportedTask(t *testing.T) {
	command, err := normalizeTaskCommand(domain.UpsertTaskCommand{
		Key:          "  FIRST_TOPIC  ",
		Title:        "  First topic  ",
		Description:  "  Publish one topic  ",
		RewardPoints: 20,
	}, false)
	if err != nil {
		t.Fatalf("normalizeTaskCommand() error = %v", err)
	}
	if command.Key != "first_topic" || command.Title != "First topic" || command.Description != "Publish one topic" || command.Status != 2 {
		t.Fatalf("normalized command = %#v", command)
	}
}

func TestNormalizeTaskCommandRejectsUnclaimableActiveTasks(t *testing.T) {
	tests := []struct {
		name    string
		command domain.UpsertTaskCommand
	}{
		{name: "missing key", command: domain.UpsertTaskCommand{Title: "Task", RewardPoints: 10, Status: 2}},
		{name: "unsupported key", command: domain.UpsertTaskCommand{Key: "complete_profile", Title: "Task", RewardPoints: 10, Status: 2}},
		{name: "unsupported disabled key", command: domain.UpsertTaskCommand{Key: "complete_profile", Title: "Task", RewardPoints: 10, Status: 1}},
		{name: "zero reward", command: domain.UpsertTaskCommand{Key: "first_topic", Title: "Task", Status: 2}},
		{name: "invalid status", command: domain.UpsertTaskCommand{Key: "first_topic", Title: "Task", RewardPoints: 10, Status: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeTaskCommand(tt.command, false)
			if !errors.Is(err, domain.ErrInvalidTask) {
				t.Fatalf("normalizeTaskCommand() error = %v, want ErrInvalidTask", err)
			}
		})
	}
}

func TestNormalizeTaskCommandAllowsDisabledLegacyTaskMaintenance(t *testing.T) {
	command, err := normalizeTaskCommand(domain.UpsertTaskCommand{
		Key:          "complete_profile",
		Title:        "Legacy task",
		RewardPoints: 10,
		Status:       1,
	}, true)
	if err != nil {
		t.Fatalf("normalizeTaskCommand() error = %v", err)
	}
	if command.Key != "complete_profile" || command.Status != 1 {
		t.Fatalf("normalized command = %#v", command)
	}
}
