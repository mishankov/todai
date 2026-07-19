package usersettings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/usersettings"
)

func TestServiceReturnsDefaultsAndResolvesSavedAgentPreferences(t *testing.T) {
	t.Parallel()

	repository := &fakeSettingsRepository{}
	service := usersettings.NewService(repository, "gpt-default", []string{"gpt-default", "gpt-fast"})

	view, err := service.Get(context.Background(), "user-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if view.Settings.Version != 0 || view.Settings.Timezone != nil ||
		view.Settings.AgentModel != "gpt-default" ||
		view.Settings.AgentThinkingEffort != usersettings.DefaultAgentThinkingEffort ||
		len(view.AvailableAgentModels) != 2 || len(view.AvailableAgentThinkingEfforts) != 7 {
		t.Errorf("default view = %#v", view)
	}

	updated, err := service.Update(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		usersettings.Update{
			Timezone: "Europe/Moscow", AgentModel: "gpt-fast", AgentThinkingEffort: "high", Version: 0,
		},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Settings.Version != 1 || updated.Settings.Timezone == nil ||
		*updated.Settings.Timezone != "Europe/Moscow" || updated.Settings.AgentModel != "gpt-fast" ||
		updated.Settings.AgentThinkingEffort != "high" {
		t.Errorf("updated view = %#v", updated)
	}

	timezone, model, thinkingEffort, err := service.ResolveAgent(context.Background(), "user-id")
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}
	if timezone != "Europe/Moscow" || model != "gpt-fast" || thinkingEffort != "high" {
		t.Errorf("agent preferences = (%q, %q, %q)", timezone, model, thinkingEffort)
	}
}

func TestServiceRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	service := usersettings.NewService(&fakeSettingsRepository{}, "gpt-default", []string{"gpt-default"})
	tests := []struct {
		name   string
		update usersettings.Update
		want   error
	}{
		{name: "timezone required", update: usersettings.Update{AgentModel: "gpt-default"}, want: usersettings.ErrInvalidTimezone},
		{name: "timezone unknown", update: usersettings.Update{Timezone: "Mars/Olympus", AgentModel: "gpt-default"}, want: usersettings.ErrInvalidTimezone},
		{name: "model unavailable", update: usersettings.Update{Timezone: "UTC", AgentModel: "gpt-other"}, want: usersettings.ErrInvalidAgentModel},
		{name: "thinking effort unavailable", update: usersettings.Update{Timezone: "UTC", AgentModel: "gpt-default", AgentThinkingEffort: "extreme"}, want: usersettings.ErrInvalidAgentThinkingEffort},
		{name: "negative version", update: usersettings.Update{Timezone: "UTC", AgentModel: "gpt-default", Version: -1}, want: usersettings.ErrInvalidVersion},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := service.Update(
				context.Background(), execution.UserScope("user-id", "correlation-id"), test.update,
			)
			if !errors.Is(err, test.want) {
				t.Errorf("Update() error = %v, want %v", err, test.want)
			}
		})
	}
}

type fakeSettingsRepository struct {
	settings usersettings.Settings
	found    bool
}

func (r *fakeSettingsRepository) Get(context.Context, string) (usersettings.Settings, bool, error) {
	return r.settings, r.found, nil
}

func (r *fakeSettingsRepository) Update(
	_ context.Context,
	scope execution.Scope,
	update usersettings.Update,
) (usersettings.Settings, error) {
	timezone := update.Timezone
	r.settings = usersettings.Settings{
		UserID: scope.UserID, Timezone: &timezone, AgentModel: update.AgentModel,
		AgentThinkingEffort: update.AgentThinkingEffort,
		Version:             update.Version + 1, LastModifiedBy: scope.ModifiedBy(),
	}
	r.found = true
	return r.settings, nil
}
