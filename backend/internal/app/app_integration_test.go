package app_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/platforma-dev/platforma/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/fakeagent"
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
)

func TestApplicationAuthenticationAndInboxFlow(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	t.Setenv("TODAI_AGENT_RUNTIME", "fake")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	coverageDir := coverageDirectory(t)
	binary := buildBinary(t, ctx)
	database := startPostgres(t, ctx)
	databaseURL, err := database.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	port := availablePort(t)
	runnerExecutable := filepath.Clean(filepath.Join(moduleRoot(t), "../pi-runner/dist/pi-runner"))
	runnerAvailable := true
	if _, err := os.Stat(runnerExecutable); errors.Is(err, os.ErrNotExist) {
		runnerAvailable = false
	} else if err != nil {
		t.Fatalf("stat compiled runner: %v", err)
	}
	processEnvironment := append(
		os.Environ(),
		"GOCOVERDIR="+coverageDir,
		"TODAI_DATABASE_URL="+databaseURL,
		"TODAI_HTTP_PORT="+port,
	)
	if runnerAvailable {
		processEnvironment = append(
			processEnvironment,
			"TODAI_RUNNER_EXECUTABLE="+runnerExecutable,
		)
	}
	runBinaryCommand(t, ctx, binary, processEnvironment, nil, "migrate")
	runBinaryCommand(
		t,
		ctx,
		binary,
		processEnvironment,
		strings.NewReader("correct horse battery staple\n"),
		"bootstrap-user",
		"--username",
		"owner",
		"--password-stdin",
	)

	var logs synchronizedBuffer
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("application logs:\n%s", logs.String())
		}
	})
	command := exec.Command(binary, "run")
	command.Env = processEnvironment
	command.Stdout = &logs
	command.Stderr = &logs

	if err := command.Start(); err != nil {
		t.Fatalf("start application: %v", err)
	}

	processDone := make(chan error, 1)
	go func() {
		processDone <- command.Wait()
	}()

	processStopped := false
	t.Cleanup(func() {
		if processStopped {
			return
		}

		_ = command.Process.Kill()
		<-processDone
	})

	client := &http.Client{Timeout: time.Second}
	baseURL := "http://127.0.0.1:" + port
	waitForStatus(t, client, http.MethodGet, baseURL+"/health", http.StatusOK, &logs)
	assertStatus(t, client, http.MethodGet, baseURL+"/api/views/projects/missing/inbox", http.StatusUnauthorized)
	assertStatus(t, client, http.MethodGet, baseURL+"/api/views/projects/missing/all", http.StatusUnauthorized)
	assertStatus(t, client, http.MethodPost, baseURL+"/api/auth/register", http.StatusNotFound)
	assertStatus(t, client, http.MethodGet, baseURL+"/protected/ping", http.StatusNotFound)
	assertLoginStatus(t, client, baseURL, "owner", "wrong password", http.StatusUnauthorized)
	sessionCookie := login(t, client, baseURL, "owner", "correct horse battery staple")
	assertCurrentUser(t, client, baseURL, sessionCookie, "owner")
	personalProject := createProject(t, client, baseURL, sessionCookie, "Personal")
	if personalProject.Name != "Personal" || personalProject.Version != 1 ||
		personalProject.ColorTheme != project.ColorThemeSage {
		t.Errorf("personal project = %#v", personalProject)
	}
	assertCreateTaskStatus(
		t, client, baseURL, sessionCookie, "   ", personalProject.ID, http.StatusBadRequest,
	)
	created := createTask(t, client, baseURL, sessionCookie, "Buy milk", personalProject.ID)
	if created.Title != "Buy milk" || created.Status != task.StatusActive || created.Version != 1 {
		t.Errorf("created task = %#v", created)
	}
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load Moscow timezone: %v", err)
	}
	localNow := time.Now().In(moscow)
	localDayStart := time.Date(
		localNow.Year(),
		localNow.Month(),
		localNow.Day(),
		0,
		0,
		0,
		0,
		moscow,
	)
	dueToday := localDayStart.Format("2006-01-02")
	dueTomorrow := localDayStart.AddDate(0, 0, 1).Format("2006-01-02")
	updated := updateTask(t, client, baseURL, sessionCookie, created.ID, map[string]any{
		"version":     created.Version,
		"title":       "Buy oat milk",
		"description": "For breakfast",
		"priority":    3,
		"dueDate":     dueToday,
		"dueTime":     "12:00",
		"dueTimezone": "Europe/Moscow",
	})
	if updated.Title != "Buy oat milk" || updated.Description == nil ||
		*updated.Description != "For breakfast" || updated.Priority != 3 ||
		updated.DueDate == nil || *updated.DueDate != task.Date(dueToday) ||
		updated.DueTime == nil || *updated.DueTime != task.TimeOfDay("12:00") ||
		updated.DueTimezone == nil || updated.Version != 2 {
		t.Errorf("updated task = %#v", updated)
	}
	assertUpdateTaskStatus(
		t,
		client,
		baseURL,
		sessionCookie,
		created.ID,
		map[string]any{"version": created.Version, "title": "Stale update"},
		http.StatusConflict,
	)
	assertTask(t, client, baseURL, sessionCookie, created.ID, task.StatusActive)
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, false, []task.Task{updated})
	future := createTask(t, client, baseURL, sessionCookie, "Plan tomorrow", personalProject.ID)
	future = updateTask(t, client, baseURL, sessionCookie, future.ID, map[string]any{
		"version": future.Version,
		"dueDate": dueTomorrow,
		"dueTime": nil,
	})
	assertToday(
		t, client, baseURL, sessionCookie, personalProject.ID,
		"Europe/Moscow", false, []task.Task{updated},
	)

	completed := changeTaskStatus(
		t, client, baseURL, sessionCookie, created.ID, "complete", updated.Version,
	)
	if completed.Status != task.StatusCompleted || completed.CompletedAt == nil || completed.Version != 3 {
		t.Errorf("completed task = %#v", completed)
	}
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, false, []task.Task{future})
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, true, []task.Task{future, completed})
	assertToday(t, client, baseURL, sessionCookie, personalProject.ID, "Europe/Moscow", false, []task.Task{})
	assertToday(t, client, baseURL, sessionCookie, personalProject.ID, "Europe/Moscow", true, []task.Task{completed})

	reopened := changeTaskStatus(
		t, client, baseURL, sessionCookie, created.ID, "reopen", completed.Version,
	)
	if reopened.Status != task.StatusActive || reopened.CompletedAt != nil || reopened.Version != 4 {
		t.Errorf("reopened task = %#v", reopened)
	}
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, false, []task.Task{reopened, future})
	assertToday(t, client, baseURL, sessionCookie, personalProject.ID, "Europe/Moscow", true, []task.Task{reopened})

	createdProject := createProject(t, client, baseURL, sessionCookie, " Work ")
	if createdProject.Name != "Work" || createdProject.Version != 1 {
		t.Errorf("created project = %#v", createdProject)
	}
	assertProjects(
		t, client, baseURL, sessionCookie, false,
		[]project.Project{personalProject, createdProject},
	)
	renamedProject := updateProject(t, client, baseURL, sessionCookie, createdProject.ID, map[string]any{
		"version": createdProject.Version,
		"name":    "Home",
	})
	if renamedProject.Name != "Home" || renamedProject.Version != 2 {
		t.Errorf("renamed project = %#v", renamedProject)
	}
	boardProject := updateProject(t, client, baseURL, sessionCookie, renamedProject.ID, map[string]any{
		"version": renamedProject.Version,
		"layout":  project.LayoutBoard,
	})
	if boardProject.Layout != project.LayoutBoard || boardProject.Version != 3 {
		t.Errorf("board project = %#v", boardProject)
	}
	planned := createSection(t, client, baseURL, sessionCookie, boardProject.ID, " Backlog ")
	doing := createSection(t, client, baseURL, sessionCookie, boardProject.ID, "Doing")
	planned = updateSection(
		t, client, baseURL, sessionCookie, boardProject.ID, planned.ID,
		map[string]any{"version": planned.Version, "name": "Planned"},
	)
	sections := reorderSection(
		t, client, baseURL, sessionCookie, boardProject.ID, doing.ID,
		map[string]any{"version": doing.Version, "beforeSectionId": planned.ID},
	)
	if len(sections) != 2 || sections[0].ID != doing.ID || sections[1].ID != planned.ID {
		t.Fatalf("reordered sections = %#v", sections)
	}
	doing = sections[0]
	planned = sections[1]
	assertReorderSectionStatus(
		t, client, baseURL, sessionCookie, boardProject.ID, doing.ID,
		map[string]any{"version": 1, "beforeSectionId": nil}, http.StatusConflict,
	)

	plannedFirst := createTaskInSection(
		t, client, baseURL, sessionCookie, "First planned task", boardProject.ID, planned.ID,
	)
	plannedSecond := createTaskInSection(
		t, client, baseURL, sessionCookie, "Second planned task", boardProject.ID, planned.ID,
	)
	doingTask := createTaskInSection(
		t, client, baseURL, sessionCookie, "Doing task", boardProject.ID, doing.ID,
	)
	reorderedTasks := reorderTask(
		t, client, baseURL, sessionCookie, plannedSecond.ID,
		map[string]any{
			"version": plannedSecond.Version, "sectionId": planned.ID,
			"beforeTaskId": plannedFirst.ID,
		},
	)
	plannedSecond = findTask(t, reorderedTasks, plannedSecond.ID)
	plannedFirst = findTask(t, reorderedTasks, plannedFirst.ID)
	if plannedSecond.Position >= plannedFirst.Position || plannedSecond.Version != 2 || plannedFirst.Version != 2 {
		t.Errorf("tasks after same-section reorder = %#v", reorderedTasks)
	}
	assertReorderTaskStatus(
		t, client, baseURL, sessionCookie, plannedSecond.ID,
		map[string]any{
			"version": 1, "sectionId": planned.ID, "beforeTaskId": plannedFirst.ID,
		},
		http.StatusConflict,
	)
	reorderedTasks = reorderTask(
		t, client, baseURL, sessionCookie, plannedSecond.ID,
		map[string]any{
			"version": plannedSecond.Version, "sectionId": doing.ID,
			"beforeTaskId": doingTask.ID,
		},
	)
	plannedSecond = findTask(t, reorderedTasks, plannedSecond.ID)
	doingTask = findTask(t, reorderedTasks, doingTask.ID)
	if plannedSecond.SectionID == nil || *plannedSecond.SectionID != doing.ID ||
		plannedSecond.Position >= doingTask.Position {
		t.Errorf("tasks after cross-section reorder = %#v", reorderedTasks)
	}
	assertDeleteSectionStatus(
		t, client, baseURL, sessionCookie, boardProject.ID, planned.ID,
		map[string]any{"version": planned.Version - 1}, http.StatusConflict,
	)
	deleteSection(t, client, baseURL, sessionCookie, boardProject.ID, planned.ID, planned.Version)
	plannedFirstAfterDelete := getTask(t, client, baseURL, sessionCookie, plannedFirst.ID)
	if plannedFirstAfterDelete.SectionID != nil || plannedFirstAfterDelete.Version != plannedFirst.Version+1 {
		t.Errorf("task after section deletion = %#v", plannedFirstAfterDelete)
	}
	assertSections(t, client, baseURL, sessionCookie, boardProject.ID, []project.Section{doing})
	deleteTask(t, client, baseURL, sessionCookie, plannedFirst.ID, plannedFirstAfterDelete.Version)
	deleteTask(t, client, baseURL, sessionCookie, plannedSecond.ID, plannedSecond.Version)
	deleteTask(t, client, baseURL, sessionCookie, doingTask.ID, doingTask.Version)

	assertUpdateTaskStatus(
		t,
		client,
		baseURL,
		sessionCookie,
		reopened.ID,
		map[string]any{"version": reopened.Version, "projectId": "missing-project"},
		http.StatusNotFound,
	)
	projectTask := createTaskInProject(
		t, client, baseURL, sessionCookie, "Clean kitchen", renamedProject.ID,
	)
	if projectTask.ProjectID == nil || *projectTask.ProjectID != renamedProject.ID {
		t.Errorf("project task = %#v", projectTask)
	}
	moved := updateTask(t, client, baseURL, sessionCookie, reopened.ID, map[string]any{
		"version":   reopened.Version,
		"projectId": renamedProject.ID,
	})
	assertAllTasks(
		t, client, baseURL, sessionCookie, renamedProject.ID, true, []task.Task{moved, projectTask},
	)
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, false, []task.Task{future})
	assertProjectTasks(
		t, client, baseURL, sessionCookie, renamedProject.ID, true, []task.Task{moved, projectTask},
	)
	moved = updateTask(t, client, baseURL, sessionCookie, moved.ID, map[string]any{
		"version":   moved.Version,
		"projectId": personalProject.ID,
	})
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, false, []task.Task{moved, future})
	archivedProject := updateProject(
		t, client, baseURL, sessionCookie, renamedProject.ID,
		map[string]any{"version": boardProject.Version, "archived": true},
	)
	if archivedProject.ArchivedAt == nil || archivedProject.Version != 4 {
		t.Errorf("archived project = %#v", archivedProject)
	}
	assertProjects(t, client, baseURL, sessionCookie, false, []project.Project{personalProject})
	assertProjects(
		t, client, baseURL, sessionCookie, true,
		[]project.Project{personalProject, archivedProject},
	)

	deleteTask(t, client, baseURL, sessionCookie, moved.ID, moved.Version)
	deleteTask(t, client, baseURL, sessionCookie, future.ID, future.Version)
	deleteTask(t, client, baseURL, sessionCookie, projectTask.ID, projectTask.Version)
	assertStatusWithCookie(
		t,
		client,
		http.MethodGet,
		baseURL+"/api/tasks/"+created.ID,
		sessionCookie,
		http.StatusNotFound,
	)
	assertInbox(t, client, baseURL, sessionCookie, personalProject.ID, true, []task.Task{})
	assertActivityTypes(t, client, baseURL, sessionCookie, renamedProject.ID, map[string]int{
		"task.created":      4,
		"task.moved":        1,
		"task.reordered":    2,
		"task.deleted":      4,
		"project.created":   1,
		"project.updated":   2,
		"project.archived":  1,
		"section.created":   2,
		"section.updated":   1,
		"section.reordered": 1,
		"section.deleted":   1,
	})
	runFakeAgentFlow(t, ctx, client, baseURL, databaseURL, "owner", sessionCookie)
	if runnerAvailable {
		runCompiledRunnerFlow(t, baseURL, sessionCookie, archivedProject.ID)
	} else {
		t.Log("compiled pi-runner is unavailable; skipping cross-component runner flow")
	}

	logout(t, client, baseURL, sessionCookie)
	assertStatus(t, client, http.MethodGet, baseURL+"/api/auth/me", http.StatusUnauthorized)

	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt application: %v\n%s", err, logs.String())
	}

	select {
	case err := <-processDone:
		processStopped = true
		if err != nil {
			t.Fatalf("application stopped with error: %v\n%s", err, logs.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("application did not stop gracefully\n%s", logs.String())
	}

	assertCoverageData(t, coverageDir)
}

func createProject(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	name string,
) project.Project {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/projects", cookie, map[string]any{"name": name},
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	var created project.Project
	decodeJSON(t, response, &created)
	return created
}

func updateProject(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	update map[string]any,
) project.Project {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPatch, baseURL+"/api/projects/"+projectID, cookie, update,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update project status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var updated project.Project
	decodeJSON(t, response, &updated)
	return updated
}

func createSection(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	name string,
) project.Section {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/projects/"+projectID+"/sections", cookie,
		map[string]any{"name": name},
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create section status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	var created project.Section
	decodeJSON(t, response, &created)
	return created
}

func updateSection(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	update map[string]any,
) project.Section {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPatch,
		baseURL+"/api/projects/"+projectID+"/sections/"+sectionID, cookie, update,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update section status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var updated project.Section
	decodeJSON(t, response, &updated)
	return updated
}

func reorderSection(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	reorder map[string]any,
) []project.Section {
	t.Helper()

	response := sendSectionReorder(
		t, client, baseURL, cookie, projectID, sectionID, reorder,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reorder section status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Sections []project.Section `json:"sections"`
	}
	decodeJSON(t, response, &body)
	return body.Sections
}

func assertReorderSectionStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	reorder map[string]any,
	want int,
) {
	t.Helper()

	response := sendSectionReorder(
		t, client, baseURL, cookie, projectID, sectionID, reorder,
	)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("reorder section status = %d, want %d", response.StatusCode, want)
	}
}

func sendSectionReorder(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	reorder map[string]any,
) *http.Response {
	t.Helper()

	return sendJSONRequest(
		t, client, http.MethodPost,
		baseURL+"/api/projects/"+projectID+"/sections/"+sectionID+"/reorder", cookie, reorder,
	)
}

func deleteSection(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	version int64,
) {
	t.Helper()

	assertDeleteSectionStatus(
		t, client, baseURL, cookie, projectID, sectionID,
		map[string]any{"version": version}, http.StatusNoContent,
	)
}

func assertDeleteSectionStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sectionID string,
	body map[string]any,
	want int,
) {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodDelete,
		baseURL+"/api/projects/"+projectID+"/sections/"+sectionID, cookie, body,
	)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("delete section status = %d, want %d", response.StatusCode, want)
	}
}

func assertSections(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	want []project.Section,
) {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodGet, baseURL+"/api/projects/"+projectID+"/sections", cookie, nil,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list sections status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Sections []project.Section `json:"sections"`
	}
	decodeJSON(t, response, &body)
	if len(body.Sections) != len(want) {
		t.Fatalf("sections = %#v, want %#v", body.Sections, want)
	}
	for index := range want {
		if body.Sections[index].ID != want[index].ID ||
			body.Sections[index].Version != want[index].Version {
			t.Errorf("section %d = %#v, want %#v", index, body.Sections[index], want[index])
		}
	}
}

func assertProjects(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	includeArchived bool,
	want []project.Project,
) {
	t.Helper()

	response := sendJSONRequest(
		t,
		client,
		http.MethodGet,
		baseURL+"/api/projects?include_archived="+strconv.FormatBool(includeArchived),
		cookie,
		nil,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list projects status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Projects []project.Project `json:"projects"`
	}
	decodeJSON(t, response, &body)
	if len(body.Projects) != len(want) {
		t.Fatalf("projects = %#v, want %#v", body.Projects, want)
	}
	for index := range want {
		if body.Projects[index].ID != want[index].ID {
			t.Errorf("project %d = %#v, want %#v", index, body.Projects[index], want[index])
		}
	}
}

func createTaskInProject(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	title string,
	projectID string,
) task.Task {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/tasks", cookie,
		map[string]any{"title": title, "projectId": projectID},
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create project task status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	return decodeTask(t, response)
}

func createTaskInSection(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	title string,
	projectID string,
	sectionID string,
) task.Task {
	t.Helper()

	response := sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/tasks", cookie,
		map[string]any{"title": title, "projectId": projectID, "sectionId": sectionID},
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create section task status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	return decodeTask(t, response)
}

func reorderTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	reorder map[string]any,
) []task.Task {
	t.Helper()

	response := sendTaskReorder(t, client, baseURL, cookie, taskID, reorder)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reorder task status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Tasks []task.Task `json:"tasks"`
	}
	decodeJSON(t, response, &body)
	return body.Tasks
}

func assertReorderTaskStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	reorder map[string]any,
	want int,
) {
	t.Helper()

	response := sendTaskReorder(t, client, baseURL, cookie, taskID, reorder)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("reorder task status = %d, want %d", response.StatusCode, want)
	}
}

func sendTaskReorder(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	reorder map[string]any,
) *http.Response {
	t.Helper()

	return sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/tasks/"+taskID+"/reorder", cookie, reorder,
	)
}

func findTask(t *testing.T, tasks []task.Task, taskID string) task.Task {
	t.Helper()

	for _, found := range tasks {
		if found.ID == taskID {
			return found
		}
	}

	t.Fatalf("task %q not found in %#v", taskID, tasks)
	return task.Task{}
}

func assertProjectTasks(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	requestURL := baseURL + "/api/views/projects/" + projectID +
		"?include_completed=" + strconv.FormatBool(includeCompleted)
	response := sendJSONRequest(t, client, http.MethodGet, requestURL, cookie, nil)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list project tasks status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Tasks []task.Task `json:"tasks"`
	}
	decodeJSON(t, response, &body)
	if len(body.Tasks) != len(want) {
		t.Fatalf("project tasks = %#v, want %#v", body.Tasks, want)
	}
	for index := range want {
		if body.Tasks[index].ID != want[index].ID {
			t.Errorf("project task %d = %#v, want %#v", index, body.Tasks[index], want[index])
		}
	}
}

func sendJSONRequest(
	t *testing.T,
	client *http.Client,
	method string,
	requestURL string,
	cookie *http.Cookie,
	body any,
) *http.Response {
	t.Helper()

	var requestBody io.Reader = http.NoBody
	if body != nil {
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatalf("encode JSON request: %v", err)
		}
		requestBody = &encoded
	}
	request, err := http.NewRequest(method, requestURL, requestBody)
	if err != nil {
		t.Fatalf("create JSON request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send JSON request: %v", err)
	}

	return response
}

func decodeJSON(t *testing.T, response *http.Response, target any) {
	t.Helper()

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
}

func assertUpdateTaskStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	update map[string]any,
	want int,
) {
	t.Helper()

	response := sendTaskUpdate(t, client, baseURL, cookie, taskID, update)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("update task status = %d, want %d", response.StatusCode, want)
	}
}

func updateTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	update map[string]any,
) task.Task {
	t.Helper()

	response := sendTaskUpdate(t, client, baseURL, cookie, taskID, update)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update task status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	return decodeTask(t, response)
}

func sendTaskUpdate(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	update map[string]any,
) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(update); err != nil {
		t.Fatalf("encode task update: %v", err)
	}
	request, err := http.NewRequest(http.MethodPatch, baseURL+"/api/tasks/"+taskID, &body)
	if err != nil {
		t.Fatalf("create update task request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send update task request: %v", err)
	}

	return response
}

func assertCreateTaskStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	title string,
	projectID string,
	want int,
) {
	t.Helper()

	response := sendTaskCreate(t, client, baseURL, cookie, title, projectID)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("create task status = %d, want %d", response.StatusCode, want)
	}
}

func createTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	title string,
	projectID string,
) task.Task {
	t.Helper()

	response := sendTaskCreate(t, client, baseURL, cookie, title, projectID)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	return decodeTask(t, response)
}

func sendTaskCreate(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	title string,
	projectID string,
) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]string{
		"title": title, "projectId": projectID,
	}); err != nil {
		t.Fatalf("encode task request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/tasks", &body)
	if err != nil {
		t.Fatalf("create task request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send task request: %v", err)
	}

	return response
}

func assertTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	wantStatus task.Status,
) {
	t.Helper()

	found := getTask(t, client, baseURL, cookie, taskID)
	if found.ID != taskID || found.Status != wantStatus {
		t.Errorf("task = %#v, want ID %q and status %q", found, taskID, wantStatus)
	}
}

func getTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
) task.Task {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/tasks/"+taskID, http.NoBody)
	if err != nil {
		t.Fatalf("create get task request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send get task request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get task status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	return decodeTask(t, response)
}

func assertInbox(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	url := baseURL + "/api/views/projects/" + projectID +
		"/inbox?include_completed=" + strconv.FormatBool(includeCompleted)
	request, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("create Inbox request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send Inbox request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Inbox status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Tasks []task.Task `json:"tasks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode Inbox: %v", err)
	}
	if len(body.Tasks) != len(want) {
		t.Fatalf("Inbox task count = %d, want %d", len(body.Tasks), len(want))
	}
	for index := range want {
		if body.Tasks[index].ID != want[index].ID || body.Tasks[index].Status != want[index].Status {
			t.Errorf("Inbox task %d = %#v, want %#v", index, body.Tasks[index], want[index])
		}
	}
}

func assertAllTasks(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	endpoint := baseURL + "/api/views/projects/" + projectID +
		"/all?include_completed=" + strconv.FormatBool(includeCompleted)
	request, err := http.NewRequest(http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("create all tasks request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send all tasks request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("all tasks status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Tasks []task.Task `json:"tasks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode all tasks: %v", err)
	}
	if len(body.Tasks) != len(want) {
		t.Fatalf("all tasks count = %d, want %d", len(body.Tasks), len(want))
	}
	for index := range want {
		if body.Tasks[index].ID != want[index].ID || body.Tasks[index].Status != want[index].Status {
			t.Errorf("all task %d = %#v, want %#v", index, body.Tasks[index], want[index])
		}
	}
}

func assertToday(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	timezone string,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	endpoint := baseURL + "/api/views/projects/" + projectID +
		"/today?timezone=" + url.QueryEscape(timezone) +
		"&include_completed=" + strconv.FormatBool(includeCompleted)
	request, err := http.NewRequest(http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("create Today request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send Today request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Today status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Tasks []task.Task `json:"tasks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode Today: %v", err)
	}
	if len(body.Tasks) != len(want) {
		t.Fatalf("Today task count = %d, want %d", len(body.Tasks), len(want))
	}
	for index := range want {
		if body.Tasks[index].ID != want[index].ID || body.Tasks[index].Status != want[index].Status {
			t.Errorf("Today task %d = %#v, want %#v", index, body.Tasks[index], want[index])
		}
	}
}

func changeTaskStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	operation string,
	version int64,
) task.Task {
	t.Helper()

	url := baseURL + "/api/tasks/" + taskID + "/" + operation
	body, err := json.Marshal(map[string]any{"version": version})
	if err != nil {
		t.Fatalf("encode %s task request: %v", operation, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create %s task request: %v", operation, err)
	}
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send %s task request: %v", operation, err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("%s task status = %d, want %d", operation, response.StatusCode, http.StatusOK)
	}

	return decodeTask(t, response)
}

func deleteTask(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	taskID string,
	version int64,
) {
	t.Helper()

	body, err := json.Marshal(map[string]any{"version": version})
	if err != nil {
		t.Fatalf("encode delete task request: %v", err)
	}
	request, err := http.NewRequest(
		http.MethodDelete,
		baseURL+"/api/tasks/"+taskID,
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("create delete task request: %v", err)
	}
	request.AddCookie(cookie)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send delete task request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete task status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func decodeTask(t *testing.T, response *http.Response) task.Task {
	t.Helper()

	var decoded task.Task
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode task response: %v", err)
	}

	return decoded
}

func assertActivityTypes(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	want map[string]int,
) {
	t.Helper()

	request, err := http.NewRequest(
		http.MethodGet,
		baseURL+"/api/activity/?project_id="+url.QueryEscape(projectID)+"&limit=200",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("create activity request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send activity request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("activity status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if response.Header.Get("Platforma-Trace-Id") == "" {
		t.Error("activity response has no Platforma-Trace-Id header")
	}

	var body struct {
		Events []activity.Event `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode activity response: %v", err)
	}
	got := make(map[string]int)
	for _, event := range body.Events {
		got[event.Type]++
		if event.ActorType != "user" || event.Source != "web" || event.ActorID == nil ||
			*event.ActorID == "" || event.CorrelationID == "" {
			t.Errorf("incomplete activity attribution: %#v", event)
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Errorf("decode %s payload: %v", event.Type, err)
			continue
		}
		if payload["schemaVersion"] != float64(1) {
			t.Errorf("%s schemaVersion = %#v, want 1", event.Type, payload["schemaVersion"])
		}
		if activityEntityName(payload) == "" {
			t.Errorf("%s payload has no entity name: %#v", event.Type, payload)
		}
	}
	if !mapsEqual(got, want) {
		t.Errorf("activity type counts = %#v, want %#v", got, want)
	}
}

func activityEntityName(payload map[string]any) string {
	for _, key := range []string{"task", "after", "before"} {
		nested, ok := payload[key].(map[string]any)
		if !ok {
			continue
		}
		if title, ok := nested["title"].(string); ok {
			return title
		}
	}
	if name, ok := payload["name"].(string); ok {
		return name
	}
	if title, ok := payload["title"].(string); ok {
		return title
	}

	return ""
}

func runFakeAgentFlow(
	t *testing.T,
	ctx context.Context,
	client *http.Client,
	baseURL string,
	databaseURL string,
	username string,
	cookie *http.Cookie,
) {
	t.Helper()

	tokenDatabase, err := database.New(databaseURL)
	if err != nil {
		t.Fatalf("open agent token database: %v", err)
	}
	t.Cleanup(func() { _ = tokenDatabase.Connection().Close() })
	var userID string
	if err := tokenDatabase.Connection().GetContext(
		ctx, &userID, "SELECT id FROM users WHERE username = $1", username,
	); err != nil {
		t.Fatalf("get fake agent user: %v", err)
	}
	var projectID string
	if err := tokenDatabase.Connection().GetContext(
		ctx, &projectID,
		"SELECT id FROM projects WHERE user_id = $1 ORDER BY created_at, id LIMIT 1", userID,
	); err != nil {
		t.Fatalf("get fake agent project: %v", err)
	}
	issued, err := agentauth.NewService(agentauth.NewRepository(tokenDatabase.Connection())).Issue(
		ctx,
		agentauth.IssueRequest{
			UserID:         userID,
			ProjectID:      projectID,
			AgentSessionID: "fake-session",
			AgentRunID:     "fake-run",
			AllowedTools: []agentauth.Tool{
				agentauth.ToolTaskCreate,
				agentauth.ToolTaskUpdate,
				agentauth.ToolTaskGet,
				agentauth.ToolTaskSearch,
				agentauth.ToolTaskComplete,
			},
			TTL: 5 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("issue fake agent token: %v", err)
	}

	agent := fakeagent.New(baseURL, issued.Token, client)
	var created task.Task
	if err := agent.Call(
		ctx,
		agentauth.ToolTaskCreate,
		map[string]any{"title": "Plan with fake agent"},
		&created,
	); err != nil {
		t.Fatalf("fake agent create: %v", err)
	}
	var updated task.Task
	if err := agent.Call(
		ctx,
		agentauth.ToolTaskUpdate,
		map[string]any{
			"taskId": created.ID, "version": created.Version,
			"title": "Plan with deterministic agent", "priority": 4,
		},
		&updated,
	); err != nil {
		t.Fatalf("fake agent update: %v", err)
	}
	var search struct {
		Tasks []task.Task `json:"tasks"`
	}
	var found task.Task
	if err := agent.Run(ctx, []fakeagent.Step{
		{
			Tool: agentauth.ToolTaskSearch,
			Input: map[string]any{
				"query": "deterministic agent", "status": task.StatusActive,
			},
			Output: &search,
		},
		{
			Tool:  agentauth.ToolTaskGet,
			Input: map[string]any{"taskId": created.ID}, Output: &found,
		},
	}); err != nil {
		t.Fatalf("fake agent read plan: %v", err)
	}
	if len(search.Tasks) != 1 || search.Tasks[0].ID != created.ID || found.ID != created.ID {
		t.Errorf("fake agent reads = (%#v, %#v)", search.Tasks, found)
	}
	var completed task.Task
	if err := agent.Call(
		ctx,
		agentauth.ToolTaskComplete,
		map[string]any{"taskId": created.ID, "version": updated.Version},
		&completed,
	); err != nil {
		t.Fatalf("fake agent complete: %v", err)
	}
	if completed.Status != task.StatusCompleted || completed.Version != updated.Version+1 {
		t.Errorf("fake agent completed task = %#v", completed)
	}

	assertAgentActivity(
		t, client, baseURL, cookie, projectID, "fake-session", "fake-run", created.ID,
	)
}

func assertAgentActivity(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
	sessionID string,
	runID string,
	taskID string,
) {
	t.Helper()

	request, err := http.NewRequest(
		http.MethodGet,
		baseURL+"/api/activity/?project_id="+url.QueryEscape(projectID)+"&limit=10",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("create agent activity request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send agent activity request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("agent activity status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body struct {
		Events []activity.Event `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode agent activity: %v", err)
	}
	wantTypes := map[string]int{
		"task.created": 1, "task.updated": 1, "task.completed": 1,
	}
	gotTypes := make(map[string]int)
	correlations := make(map[string]struct{})
	for _, event := range body.Events {
		if event.AggregateID == nil || *event.AggregateID != taskID {
			continue
		}
		gotTypes[event.Type]++
		if event.ActorType != "built_in_agent" || event.Source != "internal_api" ||
			event.ActorID == nil || *event.ActorID != sessionID || event.AgentRunID == nil ||
			*event.AgentRunID != runID || event.CorrelationID == "" {
			t.Errorf("incorrect fake agent attribution: %#v", event)
		}
		correlations[event.CorrelationID] = struct{}{}
	}
	if !mapsEqual(gotTypes, wantTypes) {
		t.Errorf("fake agent activity types = %#v, want %#v", gotTypes, wantTypes)
	}
	if len(correlations) != 3 {
		t.Errorf("fake agent correlation count = %d, want 3", len(correlations))
	}
}

func mapsEqual(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}

	return true
}

func runCompiledRunnerFlow(
	t *testing.T,
	baseURL string,
	cookie *http.Cookie,
	projectID string,
) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	response := sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/agent/sessions", cookie, nil,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create agent session status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	var session agent.Session
	decodeJSON(t, response, &session)

	response = sendJSONRequest(
		t, client, http.MethodPost, baseURL+"/api/agent/sessions/"+session.ID+"/messages",
		cookie, map[string]any{"projectId": projectID, "message": "Plan the day"},
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("post agent message status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	var posted agent.PostedMessage
	decodeJSON(t, response, &posted)
	if posted.Run.Status != agent.RunStatusQueued || posted.Message.Content != "Plan the day" {
		t.Errorf("posted agent message = %#v", posted)
	}

	events := readAgentSSE(t, baseURL, cookie, session.ID, 0, 4)
	wantTypes := []string{
		agent.EventRunStarted, agent.EventMessageDelta, agent.EventHistoryMessage, agent.EventRunCompleted,
	}
	for index, want := range wantTypes {
		if events[index].Type != want || events[index].Sequence != int64(index+1) {
			t.Errorf("agent event %d = %#v, want %s", index, events[index], want)
		}
	}
	replayed := readAgentSSE(t, baseURL, cookie, session.ID, events[2].StreamOffset, 1)
	if replayed[0].StreamOffset != events[3].StreamOffset ||
		replayed[0].Type != agent.EventRunCompleted {
		t.Errorf("replayed event = %#v, want %#v", replayed[0], events[3])
	}

	response = sendJSONRequest(
		t, client, http.MethodGet, baseURL+"/api/agent/sessions/"+session.ID, cookie, nil,
	)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get agent session status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var conversation agent.Conversation
	decodeJSON(t, response, &conversation)
	if len(conversation.Messages) != 2 || conversation.Messages[1].Role != agent.RoleAssistant ||
		conversation.Messages[1].Content != "Fake response to: Plan the day" {
		t.Errorf("agent conversation = %#v", conversation)
	}
}

func readAgentSSE(
	t *testing.T,
	baseURL string,
	cookie *http.Cookie,
	sessionID string,
	after int64,
	want int,
) []agent.RunEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, baseURL+"/api/agent/sessions/"+sessionID+"/events", http.NoBody,
	)
	if err != nil {
		t.Fatalf("create agent SSE request: %v", err)
	}
	request.AddCookie(cookie)
	if after > 0 {
		request.Header.Set("Last-Event-ID", strconv.FormatInt(after, 10))
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("open agent SSE: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK ||
		!strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("agent SSE response = (%d, %q)", response.StatusCode, response.Header.Get("Content-Type"))
	}

	events := make([]agent.RunEvent, 0, want)
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event agent.RunEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatalf("decode agent SSE event: %v", err)
		}
		events = append(events, event)
		if len(events) == want {
			cancel()
			return events
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read agent SSE: %v", err)
	}
	t.Fatalf("agent SSE event count = %d, want %d", len(events), want)
	return nil
}

func runBinaryCommand(
	t *testing.T,
	ctx context.Context,
	binary string,
	environment []string,
	stdin io.Reader,
	args ...string,
) {
	t.Helper()

	command := exec.CommandContext(ctx, binary, args...)
	command.Env = environment
	command.Stdin = stdin
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func buildBinary(t *testing.T, ctx context.Context) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "todai")
	command := exec.CommandContext(ctx, "go", "build", "-race", "-cover", "-o", binary, "./cmd/todai")
	command.Dir = moduleRoot(t)

	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build application: %v\n%s", err, output)
	}

	return binary
}

func coverageDirectory(t *testing.T) string {
	t.Helper()

	directory := os.Getenv("TODAI_INTEGRATION_COVERAGE_DIR")
	if directory == "" {
		directory = t.TempDir()
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create integration coverage directory: %v", err)
	}

	return directory
}

func assertCoverageData(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read integration coverage directory: %v", err)
	}

	var metadataFound bool
	var countersFound bool
	for _, entry := range entries {
		metadataFound = metadataFound || strings.HasPrefix(entry.Name(), "covmeta.")
		countersFound = countersFound || strings.HasPrefix(entry.Name(), "covcounters.")
	}

	if !metadataFound || !countersFound {
		t.Fatalf("integration coverage data is incomplete in %s", directory)
	}
}

func startPostgres(t *testing.T, ctx context.Context) *postgres.PostgresContainer {
	t.Helper()

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

	return container
}

func availablePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP port: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release HTTP port: %v", err)
	}

	return strconv.Itoa(port)
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}

func waitForStatus(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	want int,
	logs *synchronizedBuffer,
) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		status, err := requestStatus(client, method, url)
		if err == nil && status == want {
			return
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status = %d, want %d", status, want)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("wait for %s: %v\n%s", url, lastErr, logs.String())
}

func assertStatus(t *testing.T, client *http.Client, method, url string, want int) {
	t.Helper()

	status, err := requestStatus(client, method, url)
	if err != nil {
		t.Fatalf("request %s: %v", url, err)
	}
	if status != want {
		t.Fatalf("status for %s = %d, want %d", url, status, want)
	}
}

func assertLoginStatus(
	t *testing.T,
	client *http.Client,
	baseURL string,
	username string,
	password string,
	want int,
) {
	t.Helper()

	response := sendLogin(t, client, baseURL, username, password)
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("login status = %d, want %d", response.StatusCode, want)
	}
}

func login(t *testing.T, client *http.Client, baseURL, username, password string) *http.Cookie {
	t.Helper()

	response := sendLogin(t, client, baseURL, username, password)
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	for _, cookie := range response.Cookies() {
		if cookie.Name == "todai_session" {
			if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("session cookie has insecure attributes: %#v", cookie)
			}
			return cookie
		}
	}

	t.Fatal("login response did not set the session cookie")
	return nil
}

func sendLogin(t *testing.T, client *http.Client, baseURL, username, password string) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]string{
		"login":    username,
		"password": password,
	}); err != nil {
		t.Fatalf("encode login request: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/auth/login", &body)
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send login request: %v", err)
	}

	return response
}

func assertCurrentUser(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	wantUsername string,
) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/auth/me", http.NoBody)
	if err != nil {
		t.Fatalf("create current user request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send current user request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("current user status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var currentUser struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(response.Body).Decode(&currentUser); err != nil {
		t.Fatalf("decode current user response: %v", err)
	}
	if currentUser.Username != wantUsername {
		t.Errorf("current username = %q, want %q", currentUser.Username, wantUsername)
	}
}

func assertStatusWithCookie(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	cookie *http.Cookie,
	want int,
) {
	t.Helper()

	request, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != want {
		t.Fatalf("status for %s = %d, want %d", url, response.StatusCode, want)
	}
}

func logout(t *testing.T, client *http.Client, baseURL string, cookie *http.Cookie) {
	t.Helper()

	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/auth/logout", http.NoBody)
	if err != nil {
		t.Fatalf("create logout request: %v", err)
	}
	request.AddCookie(cookie)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send logout request: %v", err)
	}
	defer closeResponse(t, response)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	for _, responseCookie := range response.Cookies() {
		if responseCookie.Name == cookie.Name && responseCookie.Value == "" {
			return
		}
	}

	t.Fatal("logout response did not clear the session cookie")
}

func closeResponse(t *testing.T, response *http.Response) {
	t.Helper()

	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Errorf("read response body: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}

func requestStatus(client *http.Client, method, url string) (int, error) {
	request, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("send request: %w", err)
	}

	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return 0, fmt.Errorf("read response: %w", readErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("close response: %w", closeErr)
	}

	return response.StatusCode, nil
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.String()
}
