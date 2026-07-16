package app_test

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
)

func TestApplicationAuthenticationAndInboxFlow(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

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
	processEnvironment := append(
		os.Environ(),
		"GOCOVERDIR="+coverageDir,
		"TODAI_DATABASE_URL="+databaseURL,
		"TODAI_HTTP_PORT="+port,
	)
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
	assertStatus(t, client, http.MethodGet, baseURL+"/api/views/inbox", http.StatusUnauthorized)
	assertStatus(t, client, http.MethodPost, baseURL+"/api/auth/register", http.StatusNotFound)
	assertStatus(t, client, http.MethodGet, baseURL+"/protected/ping", http.StatusNotFound)
	assertLoginStatus(t, client, baseURL, "owner", "wrong password", http.StatusUnauthorized)
	sessionCookie := login(t, client, baseURL, "owner", "correct horse battery staple")
	assertCurrentUser(t, client, baseURL, sessionCookie, "owner")
	assertCreateTaskStatus(t, client, baseURL, sessionCookie, "   ", http.StatusBadRequest)
	created := createTask(t, client, baseURL, sessionCookie, "Buy milk")
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
	dueToday := localDayStart.Add(12 * time.Hour)
	dueTomorrow := localDayStart.AddDate(0, 0, 1).Add(12 * time.Hour)
	updated := updateTask(t, client, baseURL, sessionCookie, created.ID, map[string]any{
		"version":     created.Version,
		"title":       "Buy oat milk",
		"description": "For breakfast",
		"priority":    3,
		"dueAt":       dueToday.Format(time.RFC3339),
		"dueTimezone": "Europe/Moscow",
	})
	if updated.Title != "Buy oat milk" || updated.Description == nil ||
		*updated.Description != "For breakfast" || updated.Priority != 3 ||
		updated.DueAt == nil || updated.DueTimezone == nil || updated.Version != 2 {
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
	assertInbox(t, client, baseURL, sessionCookie, false, []task.Task{updated})
	future := createTask(t, client, baseURL, sessionCookie, "Plan tomorrow")
	future = updateTask(t, client, baseURL, sessionCookie, future.ID, map[string]any{
		"version":     future.Version,
		"dueAt":       dueTomorrow.Format(time.RFC3339),
		"dueTimezone": "Europe/Moscow",
	})
	assertToday(t, client, baseURL, sessionCookie, "Europe/Moscow", false, []task.Task{updated})

	completed := changeTaskStatus(t, client, baseURL, sessionCookie, created.ID, "complete")
	if completed.Status != task.StatusCompleted || completed.CompletedAt == nil || completed.Version != 3 {
		t.Errorf("completed task = %#v", completed)
	}
	assertInbox(t, client, baseURL, sessionCookie, false, []task.Task{future})
	assertInbox(t, client, baseURL, sessionCookie, true, []task.Task{future, completed})
	assertToday(t, client, baseURL, sessionCookie, "Europe/Moscow", false, []task.Task{})
	assertToday(t, client, baseURL, sessionCookie, "Europe/Moscow", true, []task.Task{completed})

	reopened := changeTaskStatus(t, client, baseURL, sessionCookie, created.ID, "reopen")
	if reopened.Status != task.StatusActive || reopened.CompletedAt != nil || reopened.Version != 4 {
		t.Errorf("reopened task = %#v", reopened)
	}
	assertInbox(t, client, baseURL, sessionCookie, false, []task.Task{reopened, future})
	assertToday(t, client, baseURL, sessionCookie, "Europe/Moscow", true, []task.Task{reopened})

	createdProject := createProject(t, client, baseURL, sessionCookie, " Personal ")
	if createdProject.Name != "Personal" || createdProject.Version != 1 {
		t.Errorf("created project = %#v", createdProject)
	}
	assertProjects(t, client, baseURL, sessionCookie, false, []project.Project{createdProject})
	renamedProject := updateProject(t, client, baseURL, sessionCookie, createdProject.ID, map[string]any{
		"version": createdProject.Version,
		"name":    "Home",
	})
	if renamedProject.Name != "Home" || renamedProject.Version != 2 {
		t.Errorf("renamed project = %#v", renamedProject)
	}
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
	assertInbox(t, client, baseURL, sessionCookie, false, []task.Task{future})
	assertProjectTasks(
		t, client, baseURL, sessionCookie, renamedProject.ID, true, []task.Task{moved, projectTask},
	)
	moved = updateTask(t, client, baseURL, sessionCookie, moved.ID, map[string]any{
		"version":   moved.Version,
		"projectId": nil,
	})
	assertInbox(t, client, baseURL, sessionCookie, false, []task.Task{moved, future})
	archivedProject := updateProject(
		t, client, baseURL, sessionCookie, renamedProject.ID,
		map[string]any{"version": renamedProject.Version, "archived": true},
	)
	if archivedProject.ArchivedAt == nil || archivedProject.Version != 3 {
		t.Errorf("archived project = %#v", archivedProject)
	}
	assertProjects(t, client, baseURL, sessionCookie, false, []project.Project{})
	assertProjects(t, client, baseURL, sessionCookie, true, []project.Project{archivedProject})

	deleteTask(t, client, baseURL, sessionCookie, created.ID)
	deleteTask(t, client, baseURL, sessionCookie, future.ID)
	deleteTask(t, client, baseURL, sessionCookie, projectTask.ID)
	assertStatusWithCookie(
		t,
		client,
		http.MethodGet,
		baseURL+"/api/tasks/"+created.ID,
		sessionCookie,
		http.StatusNotFound,
	)
	assertInbox(t, client, baseURL, sessionCookie, true, []task.Task{})

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
	want int,
) {
	t.Helper()

	response := sendTaskCreate(t, client, baseURL, cookie, title)
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
) task.Task {
	t.Helper()

	response := sendTaskCreate(t, client, baseURL, cookie, title)
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
) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(map[string]string{"title": title}); err != nil {
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

	found := decodeTask(t, response)
	if found.ID != taskID || found.Status != wantStatus {
		t.Errorf("task = %#v, want ID %q and status %q", found, taskID, wantStatus)
	}
}

func assertInbox(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	url := baseURL + "/api/views/inbox?include_completed=" + strconv.FormatBool(includeCompleted)
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

func assertToday(
	t *testing.T,
	client *http.Client,
	baseURL string,
	cookie *http.Cookie,
	timezone string,
	includeCompleted bool,
	want []task.Task,
) {
	t.Helper()

	endpoint := baseURL + "/api/views/today?timezone=" + url.QueryEscape(timezone) +
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
) task.Task {
	t.Helper()

	url := baseURL + "/api/tasks/" + taskID + "/" + operation
	request, err := http.NewRequest(http.MethodPost, url, http.NoBody)
	if err != nil {
		t.Fatalf("create %s task request: %v", operation, err)
	}
	request.AddCookie(cookie)
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
) {
	t.Helper()

	request, err := http.NewRequest(http.MethodDelete, baseURL+"/api/tasks/"+taskID, http.NoBody)
	if err != nil {
		t.Fatalf("create delete task request: %v", err)
	}
	request.AddCookie(cookie)
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
