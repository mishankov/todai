package task_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
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

func TestProjectRequirementMigrationBackfillsLegacyInbox(t *testing.T) {
	db := taskRepositoryDatabase(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `ALTER TABLE tasks ALTER COLUMN project_id DROP NOT NULL`); err != nil {
		t.Fatalf("allow legacy Inbox row: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tasks (
			id, user_id, project_id, title, status, priority, position, version,
			created_at, updated_at, last_modified_by
		) VALUES (
			'legacy-task', 'legacy-user', NULL, 'Legacy Inbox task', 'active', 0, 1024, 1,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'legacy-user'
		)
	`); err != nil {
		t.Fatalf("insert legacy Inbox task: %v", err)
	}

	repository := task.NewRepository(db, activity.NewRepository(db))
	migration, err := fs.ReadFile(repository.Migrations(), "1784647301_require_task_project.sql")
	if err != nil {
		t.Fatalf("read project requirement migration: %v", err)
	}
	upSQL := strings.Split(string(migration), "-- +migrate Down")[0]
	if _, err := db.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("reapply project requirement migration: %v", err)
	}

	var projectID string
	if err := db.GetContext(ctx, &projectID, `
		SELECT project_id FROM tasks WHERE id = 'legacy-task'
	`); err != nil {
		t.Fatalf("read migrated task: %v", err)
	}
	var migratedProject project.Project
	if err := db.GetContext(ctx, &migratedProject, `
		SELECT id, user_id, name, layout, color_theme, agent_model, agent_thinking_effort,
			position, version, archived_at, created_at, updated_at, last_modified_by
		FROM projects WHERE id = $1
	`, projectID); err != nil {
		t.Fatalf("read generated personal project: %v", err)
	}
	if migratedProject.UserID != "legacy-user" || migratedProject.Name != "Personal" ||
		migratedProject.ColorTheme != project.ColorThemeSage ||
		migratedProject.AgentThinkingEffort != "medium" {
		t.Errorf("generated personal project = %#v", migratedProject)
	}
	var projectRequired bool
	if err := db.GetContext(ctx, &projectRequired, `
		SELECT attnotnull
		FROM pg_attribute
		WHERE attrelid = 'tasks'::REGCLASS AND attname = 'project_id'
	`); err != nil {
		t.Fatalf("read project_id nullability: %v", err)
	}
	if !projectRequired {
		t.Error("tasks.project_id remains nullable")
	}
}

func TestCreateRollsBackTaskWhenActivityAppendFails(t *testing.T) {
	db := taskRepositoryDatabase(t)
	appendError := errors.New("append activity")
	scope := execution.UserScope("user-id", "correlation-id")
	createdProject, err := project.NewService(
		project.NewRepository(db, activity.NewRepository(db)),
	).Create(context.Background(), scope, "Work")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	repository := task.NewRepository(db, failingActivityAppender{err: appendError})

	_, err = repository.Create(
		context.Background(),
		scope,
		"Task that must roll back",
		&createdProject.ID,
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

func TestRepositoryPersistsSubtasksCommentsAndHierarchyInvariants(t *testing.T) {
	db := taskRepositoryDatabase(t)
	ctx := context.Background()
	scope := execution.UserScope("user-id", "correlation-id")
	events := activity.NewRepository(db)
	repository := task.NewRepository(db, events)
	projects := project.NewService(project.NewRepository(db, events))

	firstProject, err := projects.Create(ctx, scope, "First")
	if err != nil {
		t.Fatalf("create first project: %v", err)
	}
	firstSection, err := projects.CreateSection(ctx, scope, firstProject.ID, "First section")
	if err != nil {
		t.Fatalf("create first section: %v", err)
	}
	secondProject, err := projects.Create(ctx, scope, "Second")
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}
	secondSection, err := projects.CreateSection(ctx, scope, secondProject.ID, "Second section")
	if err != nil {
		t.Fatalf("create second section: %v", err)
	}

	parent, err := repository.Create(
		ctx, scope, "Parent", &firstProject.ID, &firstSection.ID, nil,
	)
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	firstChild, err := repository.Create(ctx, scope, "First child", nil, nil, &parent.ID)
	if err != nil {
		t.Fatalf("create first child: %v", err)
	}
	secondChild, err := repository.Create(ctx, scope, "Second child", nil, nil, &parent.ID)
	if err != nil {
		t.Fatalf("create second child: %v", err)
	}
	grandchild, err := repository.Create(ctx, scope, "Grandchild", nil, nil, &firstChild.ID)
	if err != nil {
		t.Fatalf("create grandchild: %v", err)
	}
	if !sameString(firstChild.ProjectID, &firstProject.ID) ||
		!sameString(firstChild.SectionID, &firstSection.ID) || firstChild.ParentID == nil ||
		*firstChild.ParentID != parent.ID {
		t.Errorf("first child placement = %#v", firstChild)
	}
	children, err := repository.ListSubtasks(ctx, "user-id", parent.ID)
	if err != nil {
		t.Fatalf("list subtasks: %v", err)
	}
	if len(children) != 2 || children[0].ID != firstChild.ID || children[1].ID != secondChild.ID ||
		children[0].Position >= children[1].Position {
		t.Errorf("subtasks = %#v", children)
	}

	moved, err := repository.Update(ctx, scope, parent.ID, task.Update{
		Version:   parent.Version,
		ProjectID: &task.Nullable[string]{Value: &secondProject.ID},
		SectionID: &task.Nullable[string]{Value: &secondSection.ID},
	})
	if err != nil {
		t.Fatalf("move parent: %v", err)
	}
	for _, taskID := range []string{firstChild.ID, secondChild.ID, grandchild.ID} {
		found, getErr := repository.Get(ctx, "user-id", taskID)
		if getErr != nil {
			t.Fatalf("get moved descendant: %v", getErr)
		}
		if !sameString(found.ProjectID, &secondProject.ID) ||
			!sameString(found.SectionID, &secondSection.ID) || found.Version != 2 {
			t.Errorf("moved descendant = %#v", found)
		}
	}
	if _, err := repository.Complete(ctx, scope, parent.ID, moved.Version); !errors.Is(err, task.ErrActiveSubtasks) {
		t.Fatalf("complete parent error = %v, want %v", err, task.ErrActiveSubtasks)
	}
	if _, err := repository.Complete(ctx, scope, firstChild.ID, 2); !errors.Is(err, task.ErrActiveSubtasks) {
		t.Fatalf("complete child error = %v, want %v", err, task.ErrActiveSubtasks)
	}
	if _, err := repository.Complete(ctx, scope, grandchild.ID, 2); err != nil {
		t.Fatalf("complete grandchild: %v", err)
	}
	completedChild, err := repository.Complete(ctx, scope, firstChild.ID, 2)
	if err != nil {
		t.Fatalf("complete first child: %v", err)
	}
	summaries, err := repository.ListProject(ctx, "user-id", secondProject.ID, true)
	if err != nil {
		t.Fatalf("list project task summaries: %v", err)
	}
	parentSummary := findTaskSummary(t, summaries, parent.ID)
	if parentSummary.SubtaskCount != 2 || parentSummary.CompletedSubtaskCount != 1 {
		t.Errorf("parent subtask progress = %d/%d, want 1/2",
			parentSummary.CompletedSubtaskCount, parentSummary.SubtaskCount)
	}
	if _, err := repository.Complete(ctx, scope, secondChild.ID, 2); err != nil {
		t.Fatalf("complete second child: %v", err)
	}
	completedParent, err := repository.Complete(ctx, scope, parent.ID, moved.Version)
	if err != nil {
		t.Fatalf("complete parent: %v", err)
	}
	if _, err := repository.Reopen(
		ctx, scope, firstChild.ID, completedChild.Version,
	); !errors.Is(err, task.ErrParentCompleted) {
		t.Errorf("reopen below completed parent error = %v", err)
	}
	if completedParent.Status != task.StatusCompleted {
		t.Errorf("completed parent = %#v", completedParent)
	}

	comment, err := repository.CreateComment(ctx, scope, parent.ID, "First note")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if comment.Body != "First note" || comment.AuthorID != "user-id" || comment.Version != 1 {
		t.Errorf("created comment = %#v", comment)
	}
	updatedComment, err := repository.UpdateComment(
		ctx, scope, parent.ID, comment.ID, "Updated note", comment.Version,
	)
	if err != nil {
		t.Fatalf("update comment: %v", err)
	}
	if updatedComment.Body != "Updated note" || updatedComment.Version != 2 {
		t.Errorf("updated comment = %#v", updatedComment)
	}
	if _, err := repository.UpdateComment(
		ctx, scope, parent.ID, comment.ID, "Stale", comment.Version,
	); !errors.Is(err, task.ErrCommentVersionConflict) {
		t.Errorf("stale update error = %v", err)
	}
	comments, err := repository.ListComments(ctx, "user-id", parent.ID)
	if err != nil || len(comments) != 1 || comments[0].ID != comment.ID {
		t.Errorf("comments = %#v, error = %v", comments, err)
	}
	if err := repository.DeleteComment(
		ctx, scope, parent.ID, comment.ID, updatedComment.Version,
	); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	if _, err := repository.ListComments(ctx, "other-user", parent.ID); !errors.Is(err, task.ErrTaskNotFound) {
		t.Errorf("other user list comments error = %v", err)
	}

	activityEvents, err := events.List(ctx, "user-id", firstProject.ID, 100)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	secondProjectEvents, err := events.List(ctx, "user-id", secondProject.ID, 100)
	if err != nil {
		t.Fatalf("list moved-project activity: %v", err)
	}
	activityEvents = append(activityEvents, secondProjectEvents...)
	wantTypes := map[string]bool{
		"task.subtask.created": false,
		"task.comment.created": false,
		"task.comment.updated": false,
		"task.comment.deleted": false,
	}
	for _, event := range activityEvents {
		if _, ok := wantTypes[event.Type]; ok {
			wantTypes[event.Type] = true
		}
		if event.Type == "task.comment.created" &&
			(!jsonContains(event.Payload, `"taskId": "`+parent.ID+`"`) ||
				!jsonContains(event.Payload, `"title": "Parent"`)) {
			t.Errorf("comment activity payload = %s", event.Payload)
		}
	}
	for eventType, found := range wantTypes {
		if !found {
			t.Errorf("activity event %q not found", eventType)
		}
	}
}

func findTaskSummary(t *testing.T, summaries []task.TaskSummary, taskID string) task.TaskSummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.ID == taskID {
			return summary
		}
	}
	t.Fatalf("task summary %q not found in %#v", taskID, summaries)
	return task.TaskSummary{}
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

	activityRepository := activity.NewRepository(platformaDatabase.Connection())
	repository := task.NewRepository(platformaDatabase.Connection(), failingActivityAppender{})
	platformaDatabase.RegisterRepository("activity_repository", activityRepository)
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

func sameString(left, right *string) bool {
	return left != nil && right != nil && *left == *right
}

func jsonContains(value []byte, fragment string) bool {
	return strings.Contains(string(value), fragment)
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
