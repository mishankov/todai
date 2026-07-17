package task_test

import (
	"context"
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

func TestCreateRollsBackTaskWhenActivityAppendFails(t *testing.T) {
	db := taskRepositoryDatabase(t)
	appendError := errors.New("append activity")
	repository := task.NewRepository(db, failingActivityAppender{err: appendError})

	_, err := repository.Create(
		context.Background(),
		execution.UserScope("user-id", "correlation-id"),
		"Task that must roll back",
		nil,
		nil,
	)
	if !errors.Is(err, appendError) {
		t.Fatalf("Create() error = %v, want %v", err, appendError)
	}

	var count int
	if err := db.GetContext(context.Background(), &count, "SELECT COUNT(*) FROM tasks"); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("task count = %d, want 0", count)
	}
}

func taskRepositoryDatabase(t *testing.T) *sqlx.DB {
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

	repository := task.NewRepository(platformaDatabase.Connection(), failingActivityAppender{})
	platformaDatabase.RegisterRepository("task_repository", repository)
	platformaDatabase.RegisterRepository(
		"project_repository",
		project.NewRepository(platformaDatabase.Connection(), failingActivityAppender{}),
	)
	if err := platformaDatabase.Migrate(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return platformaDatabase.Connection()
}

type failingActivityAppender struct {
	err error
}

func (a failingActivityAppender) Append(
	context.Context,
	sqlx.ExtContext,
	execution.Scope,
	activity.NewEvent,
) (activity.Event, error) {
	return activity.Event{}, a.err
}
