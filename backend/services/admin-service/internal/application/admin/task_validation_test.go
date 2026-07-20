package admin

import (
	"context"
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

func TestTaskDefinitionsCannotBeCreatedOrDeleted(t *testing.T) {
	authorizer := &taskMutationAuthorizer{}
	service := &Service{auth: authorizer}
	actor := domain.Actor{ID: 1, Username: "admin"}

	_, err := service.CreateTask(t.Context(), actor, domain.UpsertTaskCommand{
		Key:          "daily_check_in",
		Title:        "Daily check-in",
		RewardPoints: 5,
		Status:       2,
	})
	if !errors.Is(err, domain.ErrTaskDefinitionsManaged) {
		t.Fatalf("CreateTask() error = %v, want ErrTaskDefinitionsManaged", err)
	}
	if err := service.DeleteTask(t.Context(), actor, 1); !errors.Is(err, domain.ErrTaskDefinitionsManaged) {
		t.Fatalf("DeleteTask() error = %v, want ErrTaskDefinitionsManaged", err)
	}
	if len(authorizer.actions) != 2 || authorizer.actions[0] != domain.ActionCreateTask || authorizer.actions[1] != domain.ActionDeleteTask {
		t.Fatalf("authorized actions = %v, want create and delete task", authorizer.actions)
	}
}

type taskMutationAuthorizer struct {
	actions []domain.Action
}

func (a *taskMutationAuthorizer) Authorize(_ context.Context, _ domain.Actor, action domain.Action) error {
	a.actions = append(a.actions, action)
	return nil
}

func (*taskMutationAuthorizer) Reload(context.Context) error {
	return nil
}
