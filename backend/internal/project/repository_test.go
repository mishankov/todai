package project_test

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
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
)

func TestRepositoryMutationsAreAtomicAndEmitAttributedEvents(t *testing.T) {
	db, events := projectRepositoryDatabase(t)
	ctx := context.Background()
	appendError := errors.New("append activity")
	failingRepository := project.NewRepository(db, failingProjectActivityAppender{err: appendError})

	_, err := failingRepository.Create(ctx, execution.UserScope("user-id", "failed-correlation"), "Rollback")
	if !errors.Is(err, appendError) {
		t.Fatalf("Create() error = %v, want %v", err, appendError)
	}
	if count := projectCount(t, ctx, db); count != 0 {
		t.Fatalf("project count after activity failure = %d, want 0", count)
	}

	agentID := "agent-id"
	runID := "run-id"
	scope := execution.Scope{
		UserID:        "user-id",
		ActorType:     execution.ActorBuiltInAgent,
		ActorID:       &agentID,
		Source:        execution.SourceInternalAPI,
		CorrelationID: "correlation-id",
		AgentRunID:    &runID,
	}
	repository := project.NewRepository(db, events)
	created, err := repository.Create(ctx, scope, "Work")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	name := "Deep work"
	updated, err := repository.Update(ctx, scope, created.ID, project.Update{
		Version: created.Version,
		Name:    &name,
	})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	first, err := repository.CreateSection(ctx, scope, created.ID, "First")
	if err != nil {
		t.Fatalf("create first section: %v", err)
	}
	second, err := repository.CreateSection(ctx, scope, created.ID, "Second")
	if err != nil {
		t.Fatalf("create second section: %v", err)
	}
	sectionName := "Ready"
	first, err = repository.UpdateSection(ctx, scope, created.ID, first.ID, project.SectionUpdate{
		Version: first.Version,
		Name:    &sectionName,
	})
	if err != nil {
		t.Fatalf("update section: %v", err)
	}

	unchanged, err := repository.ReorderSection(
		ctx, scope, created.ID, second.ID, second.Version, &second.ID,
	)
	if err != nil {
		t.Fatalf("self-before reorder section: %v", err)
	}
	if len(unchanged) != 2 || unchanged[0].ID != first.ID || unchanged[1].ID != second.ID {
		t.Fatalf("sections after self-before reorder = %#v", unchanged)
	}
	unchanged, err = repository.ReorderSection(ctx, scope, created.ID, second.ID, second.Version, nil)
	if err != nil {
		t.Fatalf("no-op reorder section: %v", err)
	}
	if len(unchanged) != 2 || unchanged[0].ID != first.ID || unchanged[1].ID != second.ID {
		t.Fatalf("sections after no-op reorder = %#v", unchanged)
	}
	reordered, err := repository.ReorderSection(
		ctx, scope, created.ID, second.ID, second.Version, &first.ID,
	)
	if err != nil {
		t.Fatalf("reorder section: %v", err)
	}
	if len(reordered) != 2 || reordered[0].ID != second.ID || reordered[1].ID != first.ID {
		t.Fatalf("reordered sections = %#v", reordered)
	}
	first = reordered[1]
	if err := repository.DeleteSection(ctx, scope, created.ID, first.ID, first.Version); err != nil {
		t.Fatalf("delete section: %v", err)
	}
	archived := true
	archivedProject, err := repository.Update(ctx, scope, created.ID, project.Update{
		Version:  updated.Version,
		Archived: &archived,
	})
	if err != nil {
		t.Fatalf("archive project: %v", err)
	}
	if archivedProject.ArchivedAt == nil || archivedProject.LastModifiedBy != agentID {
		t.Errorf("archived project = %#v", archivedProject)
	}

	activityEvents, err := events.List(ctx, "user-id", 50)
	if err != nil {
		t.Fatalf("list activity events: %v", err)
	}
	wantTypes := map[string]int{
		"project.created":   1,
		"project.updated":   1,
		"project.archived":  1,
		"section.created":   2,
		"section.updated":   1,
		"section.reordered": 1,
		"section.deleted":   1,
	}
	if len(activityEvents) != 8 {
		t.Fatalf("activity event count = %d, want 8: %#v", len(activityEvents), activityEvents)
	}
	for _, event := range activityEvents {
		wantTypes[event.Type]--
		if event.ActorType != execution.ActorBuiltInAgent || event.ActorID == nil ||
			*event.ActorID != agentID || event.Source != execution.SourceInternalAPI ||
			event.CorrelationID != "correlation-id" || event.AgentRunID == nil || *event.AgentRunID != runID {
			t.Errorf("event attribution = %#v", event)
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode %s payload: %v", event.Type, err)
		}
		if payload["schemaVersion"] != float64(1) {
			t.Errorf("%s payload = %#v", event.Type, payload)
		}
		if event.Type == "section.deleted" && payload["affectedTaskCount"] != float64(0) {
			t.Errorf("section.deleted payload = %#v", payload)
		}
	}
	for eventType, remaining := range wantTypes {
		if remaining != 0 {
			t.Errorf("event %s remaining count = %d", eventType, remaining)
		}
	}
}

func projectRepositoryDatabase(t *testing.T) (*sqlx.DB, *activity.Repository) {
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

	db := platformaDatabase.Connection()
	events := activity.NewRepository(db)
	projects := project.NewRepository(db, events)
	platformaDatabase.RegisterRepository("activity_repository", events)
	platformaDatabase.RegisterRepository("project_repository", projects)
	platformaDatabase.RegisterRepository("task_repository", task.NewRepository(db, events))
	if err := platformaDatabase.Migrate(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return db, events
}

func projectCount(t *testing.T, ctx context.Context, db *sqlx.DB) int {
	t.Helper()
	var count int
	if err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM projects"); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	return count
}

type failingProjectActivityAppender struct {
	err error
}

func (a failingProjectActivityAppender) Append(
	context.Context,
	sqlx.ExtContext,
	execution.Scope,
	activity.NewEvent,
) (activity.Event, error) {
	return activity.Event{}, a.err
}
