package usersettings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/platforma-dev/platforma/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/usersettings"
)

func TestRepositoryPersistsVersionedSettingsAndActivity(t *testing.T) {
	repository, events := settingsRepository(t)
	ctx := context.Background()
	scope := execution.UserScope("user-id", "correlation-id")

	created, err := repository.Update(ctx, scope, usersettings.Update{
		Timezone: "Europe/Moscow", AgentModel: "gpt-fast", AgentThinkingEffort: "high", Version: 0,
	})
	if err != nil {
		t.Fatalf("create settings: %v", err)
	}
	if created.Version != 1 || created.LastModifiedBy != "user-id" {
		t.Errorf("created settings = %#v", created)
	}
	if _, err := repository.Update(ctx, scope, usersettings.Update{
		Timezone: "UTC", AgentModel: "gpt-default", AgentThinkingEffort: "low", Version: 0,
	}); !errors.Is(err, usersettings.ErrVersionConflict) {
		t.Fatalf("stale update error = %v, want version conflict", err)
	}
	updated, err := repository.Update(ctx, scope, usersettings.Update{
		Timezone: "UTC", AgentModel: "gpt-default", AgentThinkingEffort: "low", Version: created.Version,
	})
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if updated.Version != 2 || updated.Timezone == nil || *updated.Timezone != "UTC" ||
		updated.AgentThinkingEffort != "low" {
		t.Errorf("updated settings = %#v", updated)
	}

	found, exists, err := repository.Get(ctx, "user-id")
	if err != nil || !exists || found.Version != 2 {
		t.Errorf("Get() = (%#v, %v, %v)", found, exists, err)
	}
	activityEvents, err := events.List(ctx, "user-id", "", 10)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(activityEvents) != 2 || activityEvents[0].Type != "user_settings.updated" ||
		activityEvents[0].CorrelationID != "correlation-id" {
		t.Errorf("activity events = %#v", activityEvents)
	}
}

func settingsRepository(t *testing.T) (*usersettings.Repository, *activity.Repository) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	container, err := postgres.Run(
		ctx, "postgres:17-alpine", postgres.WithDatabase("todai"),
		postgres.WithUsername("todai"), postgres.WithPassword("todai"), postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	testcontainers.CleanupContainer(t, container)
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}
	platformaDatabase, err := database.New(databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = platformaDatabase.Connection().Close() })
	db := platformaDatabase.Connection()
	if _, err := db.ExecContext(ctx, `CREATE TABLE users (id VARCHAR PRIMARY KEY)`); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	events := activity.NewRepository(db)
	repository := usersettings.NewRepository(db, events)
	platformaDatabase.RegisterRepository("activity_repository", events)
	platformaDatabase.RegisterRepository("user_settings_repository", repository)
	if err := platformaDatabase.Migrate(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id) VALUES ('user-id')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return repository, events
}
