// Package tasktools exposes the closed HTTP surface used by the built-in agent.
package tasktools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
)

// Authorizer authenticates one internal bearer token and enforces one tool capability.
type Authorizer interface {
	Authenticate(context.Context, string, agentauth.Tool) (agentauth.Claims, error)
}

// TaskService describes task operations shared by product HTTP and internal tools.
type TaskService interface {
	Get(context.Context, string, string) (task.Task, error)
	ListSubtasks(context.Context, string, string) ([]task.Task, error)
	ListComments(context.Context, string, string) ([]task.Comment, error)
	ListInbox(context.Context, string, bool) ([]task.TaskSummary, error)
	ListAll(context.Context, string, bool) ([]task.TaskSummary, error)
	ListProject(context.Context, string, string, bool) ([]task.TaskSummary, error)
	ListToday(context.Context, string, string, bool) ([]task.TaskSummary, error)
	Search(context.Context, string, task.SearchQuery) ([]task.Task, error)
	Create(context.Context, execution.Scope, string, *string, *string) (task.Task, error)
	CreateSubtask(context.Context, execution.Scope, string, string) (task.Task, error)
	Update(context.Context, execution.Scope, string, task.Update) (task.Task, error)
	Complete(context.Context, execution.Scope, string, int64) (task.Task, error)
	Reopen(context.Context, execution.Scope, string, int64) (task.Task, error)
	Reorder(context.Context, execution.Scope, string, task.Reorder) ([]task.TaskSummary, error)
}

// ProjectService describes project reads exposed to internal tools.
type ProjectService interface {
	Get(context.Context, string, string) (project.Project, error)
	List(context.Context, string, bool) ([]project.Project, error)
	ListSections(context.Context, string, string) ([]project.Section, error)
}

// HTTPModule owns the internal task-tool routes.
type HTTPModule struct {
	authorizer Authorizer
	tasks      TaskService
	projects   ProjectService
}

type authorizedHandler func(http.ResponseWriter, *http.Request, agentauth.Claims)

type taskIDRequest struct {
	TaskID string `json:"taskId"`
}

type versionedTaskRequest struct {
	TaskID  string `json:"taskId"`
	Version *int64 `json:"version"`
}

type viewQueryRequest struct {
	View             string `json:"view"`
	ProjectID        string `json:"projectId"`
	Timezone         string `json:"timezone"`
	IncludeCompleted bool   `json:"includeCompleted"`
}

type projectListRequest struct {
	IncludeArchived bool `json:"includeArchived"`
}

type projectGetRequest struct {
	ProjectID string `json:"projectId"`
}

type taskSearchRequest struct {
	Query     string       `json:"query"`
	ProjectID *string      `json:"projectId"`
	Status    *task.Status `json:"status"`
	Limit     int          `json:"limit"`
}

type createTaskRequest struct {
	Title     string  `json:"title"`
	ProjectID *string `json:"projectId"`
	SectionID *string `json:"sectionId"`
	ParentID  *string `json:"parentId"`
}

type updateTaskRequest struct {
	TaskID      string           `json:"taskId"`
	Version     *int64           `json:"version"`
	Title       optional[string] `json:"title"`
	Description nullable[string] `json:"description"`
	Priority    optional[int]    `json:"priority"`
	DueDate     nullable[string] `json:"dueDate"`
	DueTime     nullable[string] `json:"dueTime"`
	DueTimezone nullable[string] `json:"dueTimezone"`
}

type moveTaskRequest struct {
	TaskID    string           `json:"taskId"`
	Version   *int64           `json:"version"`
	ProjectID nullable[string] `json:"projectId"`
	SectionID nullable[string] `json:"sectionId"`
}

type reorderTaskRequest struct {
	TaskID       string           `json:"taskId"`
	Version      *int64           `json:"version"`
	SectionID    nullable[string] `json:"sectionId"`
	BeforeTaskID *string          `json:"beforeTaskId"`
}

type optional[T any] struct {
	Set   bool
	Value T
}

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

type nullable[T any] struct {
	Set   bool
	Value *T
}

func (n *nullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	n.Value = &value
	return nil
}

type taskListResponse struct {
	Tasks []task.TaskSummary `json:"tasks"`
}

type taskSearchResponse struct {
	Tasks []task.Task `json:"tasks"`
}

type taskGetResponse struct {
	task.Task
	Subtasks []task.Task      `json:"subtasks"`
	Comments []task.Comment   `json:"comments"`
	Project  *project.Project `json:"project"`
	Section  *project.Section `json:"section"`
}

type projectListResponse struct {
	Projects []project.Project `json:"projects"`
}

type projectGetResponse struct {
	Project  project.Project   `json:"project"`
	Sections []project.Section `json:"sections"`
}

// NewHTTPModule constructs the closed task-tool HTTP module.
func NewHTTPModule(authorizer Authorizer, tasks TaskService, projects ProjectService) *HTTPModule {
	return &HTTPModule{authorizer: authorizer, tasks: tasks, projects: projects}
}

// Mount registers explicit internal tool routes on the supplied group.
func (m *HTTPModule) Mount(group *httpserver.HandlerGroup) {
	group.HandleFunc("POST /task_get", m.authorize(agentauth.ToolTaskGet, m.taskGet))
	group.HandleFunc("POST /view_query", m.authorize(agentauth.ToolViewQuery, m.viewQuery))
	group.HandleFunc("POST /project_get", m.authorize(agentauth.ToolProjectGet, m.projectGet))
	group.HandleFunc("POST /project_list", m.authorize(agentauth.ToolProjectList, m.projectList))
	group.HandleFunc("POST /task_search", m.authorize(agentauth.ToolTaskSearch, m.taskSearch))
	group.HandleFunc("POST /task_create", m.authorize(agentauth.ToolTaskCreate, m.taskCreate))
	group.HandleFunc("POST /task_update", m.authorize(agentauth.ToolTaskUpdate, m.taskUpdate))
	group.HandleFunc("POST /task_complete", m.authorize(agentauth.ToolTaskComplete, m.taskComplete))
	group.HandleFunc("POST /task_reopen", m.authorize(agentauth.ToolTaskReopen, m.taskReopen))
	group.HandleFunc("POST /task_move", m.authorize(agentauth.ToolTaskMove, m.taskMove))
	group.HandleFunc("POST /task_reorder", m.authorize(agentauth.ToolTaskReorder, m.taskReorder))
}

func (m *HTTPModule) authorize(tool agentauth.Tool, next authorizedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			http.Error(w, agentauth.ErrTokenRequired.Error(), http.StatusUnauthorized)
			return
		}

		claims, err := m.authorizer.Authenticate(r.Context(), raw, tool)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		if traceID, _ := r.Context().Value(log.TraceIDKey).(string); traceID == "" {
			log.ErrorContext(r.Context(), "internal tool request is missing Platforma trace ID")
			http.Error(w, "internal tool request is missing trace ID", http.StatusInternalServerError)
			return
		}

		next(w, r, claims)
	}
}

func (m *HTTPModule) taskGet(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request taskIDRequest
	if !decodeRequest(w, r, &request) || !requireTaskID(w, request.TaskID) {
		return
	}

	found, err := m.tasks.Get(r.Context(), claims.UserID, request.TaskID)
	if err != nil {
		writeToolError(w, r, "task_get", err)
		return
	}
	subtasks, err := m.tasks.ListSubtasks(r.Context(), claims.UserID, request.TaskID)
	if err != nil {
		writeToolError(w, r, "task_get", err)
		return
	}
	comments, err := m.tasks.ListComments(r.Context(), claims.UserID, request.TaskID)
	if err != nil {
		writeToolError(w, r, "task_get", err)
		return
	}
	response := taskGetResponse{Task: found, Subtasks: subtasks, Comments: comments}
	if found.ProjectID != nil {
		foundProject, err := m.projects.Get(r.Context(), claims.UserID, *found.ProjectID)
		if err != nil {
			writeToolError(w, r, "task_get", err)
			return
		}
		response.Project = &foundProject
		if found.SectionID != nil {
			sections, err := m.projects.ListSections(r.Context(), claims.UserID, *found.ProjectID)
			if err != nil {
				writeToolError(w, r, "task_get", err)
				return
			}
			for index := range sections {
				if sections[index].ID == *found.SectionID {
					response.Section = &sections[index]
					break
				}
			}
		}
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (m *HTTPModule) viewQuery(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request viewQueryRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	var (
		tasks []task.TaskSummary
		err   error
	)
	switch request.View {
	case "inbox":
		tasks, err = m.tasks.ListInbox(r.Context(), claims.UserID, request.IncludeCompleted)
	case "all":
		tasks, err = m.tasks.ListAll(r.Context(), claims.UserID, request.IncludeCompleted)
	case "today":
		if strings.TrimSpace(request.Timezone) == "" {
			http.Error(w, "timezone is required for Today", http.StatusBadRequest)
			return
		}
		tasks, err = m.tasks.ListToday(
			r.Context(), claims.UserID, request.Timezone, request.IncludeCompleted,
		)
	case "project":
		if strings.TrimSpace(request.ProjectID) == "" {
			http.Error(w, "projectId is required for a project view", http.StatusBadRequest)
			return
		}
		tasks, err = m.tasks.ListProject(
			r.Context(), claims.UserID, request.ProjectID, request.IncludeCompleted,
		)
	default:
		http.Error(w, "view must be inbox, all, today, or project", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeToolError(w, r, "view_query", err)
		return
	}
	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (m *HTTPModule) projectList(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request projectListRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	projects, err := m.projects.List(r.Context(), claims.UserID, request.IncludeArchived)
	if err != nil {
		writeToolError(w, r, "project_list", err)
		return
	}
	writeJSON(w, r, http.StatusOK, projectListResponse{Projects: projects})
}

func (m *HTTPModule) projectGet(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request projectGetRequest
	if !decodeRequest(w, r, &request) || !requireProjectID(w, request.ProjectID) {
		return
	}

	found, err := m.projects.Get(r.Context(), claims.UserID, request.ProjectID)
	if err != nil {
		writeToolError(w, r, "project_get", err)
		return
	}
	sections, err := m.projects.ListSections(r.Context(), claims.UserID, request.ProjectID)
	if err != nil {
		writeToolError(w, r, "project_get", err)
		return
	}
	writeJSON(w, r, http.StatusOK, projectGetResponse{Project: found, Sections: sections})
}

func (m *HTTPModule) taskSearch(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request taskSearchRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	results, err := m.tasks.Search(r.Context(), claims.UserID, task.SearchQuery{
		Query: request.Query, ProjectID: request.ProjectID, Status: request.Status, Limit: request.Limit,
	})
	if err != nil {
		writeToolError(w, r, "task_search", err)
		return
	}
	writeJSON(w, r, http.StatusOK, taskSearchResponse{Tasks: results})
}

func (m *HTTPModule) taskCreate(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request createTaskRequest
	if !decodeRequest(w, r, &request) {
		return
	}

	var created task.Task
	var err error
	if request.ParentID != nil {
		if request.ProjectID != nil || request.SectionID != nil {
			http.Error(w, task.ErrSubtaskPlacement.Error(), http.StatusBadRequest)
			return
		}
		created, err = m.tasks.CreateSubtask(
			r.Context(), scopeFor(r, claims), request.Title, *request.ParentID,
		)
	} else {
		created, err = m.tasks.Create(
			r.Context(), scopeFor(r, claims), request.Title, request.ProjectID, request.SectionID,
		)
	}
	if err != nil {
		writeToolError(w, r, "task_create", err)
		return
	}
	writeJSON(w, r, http.StatusCreated, created)
}

func (m *HTTPModule) taskUpdate(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request updateTaskRequest
	if !decodeRequest(w, r, &request) || !requireVersionedTask(w, request.TaskID, request.Version) {
		return
	}

	update, err := request.update()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	updated, err := m.tasks.Update(r.Context(), scopeFor(r, claims), request.TaskID, update)
	if err != nil {
		writeToolError(w, r, "task_update", err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (m *HTTPModule) taskComplete(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	m.changeStatus(w, r, claims, "task_complete", m.tasks.Complete)
}

func (m *HTTPModule) taskReopen(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	m.changeStatus(w, r, claims, "task_reopen", m.tasks.Reopen)
}

func (m *HTTPModule) changeStatus(
	w http.ResponseWriter,
	r *http.Request,
	claims agentauth.Claims,
	operation string,
	change func(context.Context, execution.Scope, string, int64) (task.Task, error),
) {
	var request versionedTaskRequest
	if !decodeRequest(w, r, &request) || !requireVersionedTask(w, request.TaskID, request.Version) {
		return
	}

	updated, err := change(r.Context(), scopeFor(r, claims), request.TaskID, *request.Version)
	if err != nil {
		writeToolError(w, r, operation, err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (m *HTTPModule) taskMove(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request moveTaskRequest
	if !decodeRequest(w, r, &request) || !requireVersionedTask(w, request.TaskID, request.Version) {
		return
	}
	if !request.ProjectID.Set || !request.SectionID.Set {
		http.Error(w, "projectId and sectionId are required", http.StatusBadRequest)
		return
	}

	updated, err := m.tasks.Update(r.Context(), scopeFor(r, claims), request.TaskID, task.Update{
		Version:   *request.Version,
		ProjectID: &task.Nullable[string]{Value: request.ProjectID.Value},
		SectionID: &task.Nullable[string]{Value: request.SectionID.Value},
	})
	if err != nil {
		writeToolError(w, r, "task_move", err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (m *HTTPModule) taskReorder(w http.ResponseWriter, r *http.Request, claims agentauth.Claims) {
	var request reorderTaskRequest
	if !decodeRequest(w, r, &request) || !requireVersionedTask(w, request.TaskID, request.Version) {
		return
	}
	if !request.SectionID.Set {
		http.Error(w, "sectionId is required", http.StatusBadRequest)
		return
	}

	tasks, err := m.tasks.Reorder(r.Context(), scopeFor(r, claims), request.TaskID, task.Reorder{
		Version: *request.Version, SectionID: request.SectionID.Value, BeforeTaskID: request.BeforeTaskID,
	})
	if err != nil {
		writeToolError(w, r, "task_reorder", err)
		return
	}
	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (r updateTaskRequest) update() (task.Update, error) {
	update := task.Update{Version: *r.Version}
	if r.Title.Set {
		update.Title = &r.Title.Value
	}
	if r.Description.Set {
		update.Description = &task.Nullable[string]{Value: r.Description.Value}
	}
	if r.Priority.Set {
		update.Priority = &r.Priority.Value
	}
	if r.DueDate.Set {
		update.DueDate = &task.Nullable[task.Date]{}
		if r.DueDate.Value != nil {
			value, err := task.ParseDate(*r.DueDate.Value)
			if err != nil {
				return task.Update{}, errors.New("dueDate must use YYYY-MM-DD")
			}
			update.DueDate.Value = &value
		}
	}
	if r.DueTime.Set {
		update.DueTime = &task.Nullable[task.TimeOfDay]{}
		if r.DueTime.Value != nil {
			value, err := task.ParseTimeOfDay(*r.DueTime.Value)
			if err != nil {
				return task.Update{}, errors.New("dueTime must use HH:MM")
			}
			update.DueTime.Value = &value
		}
	}
	if r.DueTimezone.Set {
		update.DueTimezone = &task.Nullable[string]{Value: r.DueTimezone.Value}
	}

	return update, nil
}

func bearerToken(header string) (string, bool) {
	scheme, raw, ok := strings.Cut(strings.TrimSpace(header), " ")
	raw = strings.TrimSpace(raw)
	return raw, ok && strings.EqualFold(scheme, "Bearer") && raw != "" && !strings.Contains(raw, " ")
}

func scopeFor(r *http.Request, claims agentauth.Claims) execution.Scope {
	correlationID, _ := r.Context().Value(log.TraceIDKey).(string)
	return claims.ExecutionScope(correlationID)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return false
	}
	return true
}

func requireTaskID(w http.ResponseWriter, taskID string) bool {
	if strings.TrimSpace(taskID) != "" {
		return true
	}
	http.Error(w, "taskId is required", http.StatusBadRequest)
	return false
}

func requireProjectID(w http.ResponseWriter, projectID string) bool {
	if strings.TrimSpace(projectID) != "" {
		return true
	}
	http.Error(w, "projectId is required", http.StatusBadRequest)
	return false
}

func requireVersionedTask(w http.ResponseWriter, taskID string, version *int64) bool {
	if !requireTaskID(w, taskID) {
		return false
	}
	if version != nil {
		return true
	}
	http.Error(w, "version is required", http.StatusBadRequest)
	return false
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, agentauth.ErrTokenRequired),
		errors.Is(err, agentauth.ErrTokenUnknown),
		errors.Is(err, agentauth.ErrTokenExpired):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, agentauth.ErrToolNotAllowed):
		http.Error(w, agentauth.ErrToolNotAllowed.Error(), http.StatusForbidden)
	default:
		log.ErrorContext(r.Context(), "internal tool authorization failed", "error", err)
		http.Error(w, "internal tool authorization failed", http.StatusInternalServerError)
	}
}

func writeToolError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, task.ErrTitleRequired),
		errors.Is(err, task.ErrTitleTooLong),
		errors.Is(err, task.ErrDescriptionTooLong),
		errors.Is(err, task.ErrInvalidPriority),
		errors.Is(err, task.ErrInvalidDueDate),
		errors.Is(err, task.ErrInvalidDueTime),
		errors.Is(err, task.ErrDueDateRequired),
		errors.Is(err, task.ErrTaskNotReorderable),
		errors.Is(err, task.ErrInvalidTimezone),
		errors.Is(err, task.ErrInvalidVersion),
		errors.Is(err, task.ErrInvalidSearchLimit),
		errors.Is(err, task.ErrInvalidSearchStatus),
		errors.Is(err, task.ErrNoChanges),
		errors.Is(err, task.ErrSubtaskPlacement):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, task.ErrTaskNotFound),
		errors.Is(err, task.ErrProjectNotFound),
		errors.Is(err, task.ErrSectionNotFound),
		errors.Is(err, project.ErrProjectNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, task.ErrVersionConflict), errors.Is(err, project.ErrVersionConflict),
		errors.Is(err, task.ErrActiveSubtasks), errors.Is(err, task.ErrParentCompleted):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "internal task tool failed", "operation", operation, "error", err)
		http.Error(w, "internal task tool failed", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if err := httpserver.WriteJSON(w, status, value); err != nil {
		log.ErrorContext(r.Context(), "failed to write internal task tool response", "error", err)
	}
}
