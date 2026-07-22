package usersettings_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/platforma-dev/platforma/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/usersettings"
)

func TestRepositoryPersistsVersionedSettingsAndActivity(t *testing.T) {
	repository, events, db := settingsRepository(t)
	ctx := context.Background()
	scope := execution.UserScope("user-id", "correlation-id")

	created, err := repository.Update(ctx, scope, usersettings.Update{
		Timezone: "Europe/Moscow", AgentModel: "gpt-fast", AgentThinkingEffort: "high",
		Appearance: appearancePointer(usersettings.AppearanceDark), Version: 0,
	})
	if err != nil {
		t.Fatalf("create settings: %v", err)
	}
	if created.Version != 1 || created.LastModifiedBy != "user-id" ||
		created.Appearance != usersettings.AppearanceDark {
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
		updated.AgentThinkingEffort != "low" || updated.Appearance != usersettings.AppearanceDark {
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
	var payload struct {
		SchemaVersion int                     `json:"schemaVersion"`
		Appearance    usersettings.Appearance `json:"appearance"`
	}
	if err := json.Unmarshal(activityEvents[0].Payload, &payload); err != nil {
		t.Fatalf("decode activity payload: %v", err)
	}
	if payload.SchemaVersion != 3 || payload.Appearance != usersettings.AppearanceDark {
		t.Errorf("activity payload = %#v", payload)
	}
	if _, err := db.ExecContext(ctx, `UPDATE user_settings SET appearance = 'sepia' WHERE user_id = 'user-id'`); err == nil {
		t.Error("database accepted an invalid appearance")
	}
}

func settingsRepository(t *testing.T) (*usersettings.Repository, *activity.Repository, *sqlx.DB) {
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
	return repository, events, db
}
