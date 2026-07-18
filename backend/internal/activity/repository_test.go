package activity_test

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
)

func TestRepositoryAppendsAndListsUserEventsNewestFirst(t *testing.T) {
	db, repository := testRepository(t)
	ctx := context.Background()

	taskType := "task"
	first := appendEvent(t, ctx, repository, db, execution.UserScope("user-id", "correlation-id"), activity.NewEvent{
		Type:          "task.created",
		AggregateType: &taskType,
		AggregateID:   stringPointer("first-task"),
		Payload:       map[string]any{"schemaVersion": 1, "title": "First"},
	})
	secondScope := execution.Scope{
		UserID:        "user-id",
		ActorType:     execution.ActorBuiltInAgent,
		Source:        execution.SourceInternalAPI,
		CorrelationID: "correlation-id",
		AgentRunID:    stringPointer("run-id"),
	}
	second := appendEvent(t, ctx, repository, db, secondScope, activity.NewEvent{
		Type:          "task.updated",
		AggregateType: &taskType,
		AggregateID:   stringPointer("first-task"),
		Payload: struct {
			SchemaVersion int `json:"schemaVersion"`
		}{SchemaVersion: 1},
	})
	system := appendEvent(t, ctx, repository, db, execution.Scope{
		UserID: "user-id", ActorType: execution.ActorSystem,
		Source: execution.SourceSystem, CorrelationID: "system-correlation",
	}, activity.NewEvent{
		Type:    "task.maintenance.completed",
		Payload: map[string]any{"schemaVersion": 1},
	})
	appendEvent(t, ctx, repository, db, execution.UserScope("other-user", "other-correlation"), activity.NewEvent{
		Type:    "task.created",
		Payload: map[string]any{"schemaVersion": 1},
	})

	events, err := repository.List(ctx, "user-id", 50)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3", len(events))
	}
	if events[0].ID != system.ID || events[1].ID != second.ID || events[2].ID != first.ID {
		t.Errorf("event order = [%s, %s, %s], want [%s, %s, %s]",
			events[0].ID, events[1].ID, events[2].ID, system.ID, second.ID, first.ID)
	}
	if events[0].ActorType != execution.ActorSystem || events[0].ActorID != nil {
		t.Errorf("system attribution = %#v", events[0])
	}
	if events[1].ActorType != execution.ActorBuiltInAgent || events[1].AgentRunID == nil ||
		*events[1].AgentRunID != "run-id" {
		t.Errorf("agent attribution = %#v", events[1])
	}
	var payload map[string]any
	if err := json.Unmarshal(events[2].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["title"] != "First" {
		t.Errorf("payload = %#v", payload)
	}
}

func TestRepositoryAppendParticipatesInCallerTransaction(t *testing.T) {
	db, repository := testRepository(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	appendEvent(t, ctx, repository, tx, execution.UserScope("user-id", "correlation-id"), activity.NewEvent{
		Type:    "task.created",
		Payload: map[string]any{"schemaVersion": 1},
	})
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}

	events, err := repository.List(ctx, "user-id", 50)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("event count after rollback = %d, want 0", len(events))
	}
}

func TestRepositoryListsEventsAfterDurableOffset(t *testing.T) {
	db, repository := testRepository(t)
	ctx := context.Background()
	first := appendEvent(t, ctx, repository, db, execution.UserScope("user-id", "first"), activity.NewEvent{
		Type: "task.created", Payload: map[string]any{"schemaVersion": 1},
	})
	second := appendEvent(t, ctx, repository, db, execution.UserScope("user-id", "second"), activity.NewEvent{
		Type: "task.updated", Payload: map[string]any{"schemaVersion": 1},
	})
	appendEvent(t, ctx, repository, db, execution.UserScope("other-user", "other"), activity.NewEvent{
		Type: "task.created", Payload: map[string]any{"schemaVersion": 1},
	})

	latest, err := repository.LatestOffset(ctx, "user-id")
	if err != nil {
		t.Fatalf("latest offset: %v", err)
	}
	if latest != second.StreamOffset || second.StreamOffset <= first.StreamOffset {
		t.Errorf("offsets = first %d, second %d, latest %d", first.StreamOffset, second.StreamOffset, latest)
	}
	events, err := repository.ListAfter(ctx, "user-id", first.StreamOffset, 100)
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(events) != 1 || events[0].ID != second.ID {
		t.Errorf("events after first = %#v", events)
	}
}

func TestRepositoryRejectsInvalidEventsBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	repository := activity.NewRepository(nil)
	validScope := execution.UserScope("user-id", "correlation-id")
	taskType := "task"
	tests := []struct {
		name  string
		scope execution.Scope
		event activity.NewEvent
		want  error
	}{
		{
			name:  "execution",
			scope: execution.Scope{},
			event: activity.NewEvent{Type: "task.created", Payload: map[string]any{}},
			want:  execution.ErrUserIDRequired,
		},
		{
			name:  "type",
			scope: validScope,
			event: activity.NewEvent{Payload: map[string]any{}},
			want:  activity.ErrTypeRequired,
		},
		{
			name:  "aggregate",
			scope: validScope,
			event: activity.NewEvent{Type: "task.created", AggregateType: &taskType, Payload: map[string]any{}},
			want:  activity.ErrAggregateIncomplete,
		},
		{
			name:  "payload",
			scope: validScope,
			event: activity.NewEvent{Type: "task.created", Payload: []string{"not", "an", "object"}},
			want:  activity.ErrInvalidPayload,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := repository.Append(context.Background(), nil, test.scope, test.event)
			if !errors.Is(err, test.want) {
				t.Errorf("Append() error = %v, want %v", err, test.want)
			}
		})
	}
}

func testRepository(t *testing.T) (*sqlx.DB, *activity.Repository) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	container, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("todai"),
		postgres.WithUsername("todai"),
		postgres.WithPassword("todai"),
		postgres.BasicWaitStrategies(),
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
	repository := activity.NewRepository(platformaDatabase.Connection())
	platformaDatabase.RegisterRepository("activity_repository", repository)
	if err := platformaDatabase.Migrate(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return platformaDatabase.Connection(), repository
}

func appendEvent(
	t *testing.T,
	ctx context.Context,
	repository *activity.Repository,
	executor sqlx.ExtContext,
	scope execution.Scope,
	newEvent activity.NewEvent,
) activity.Event {
	t.Helper()
	appended, err := repository.Append(ctx, executor, scope, newEvent)
	if err != nil {
		t.Fatalf("append activity event: %v", err)
	}
	return appended
}

func stringPointer(value string) *string {
	return &value
}
