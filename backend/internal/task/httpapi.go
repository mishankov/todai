package task

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/execution"
)

type taskHandlers struct {
	service HTTPService
}

// HTTPService describes the task operations exposed over HTTP.
type HTTPService interface {
	Create(context.Context, execution.Scope, string, *string, *string) (Task, error)
	CreateWithProperties(context.Context, execution.Scope, CreateInput) (Task, error)
	CreateSubtask(context.Context, execution.Scope, string, string) (Task, error)
	Get(context.Context, string, string) (Task, error)
	ListSubtasks(context.Context, string, string) ([]Task, error)
	ListInbox(context.Context, string, string, bool) ([]TaskSummary, error)
	ListAll(context.Context, string, string, bool) ([]TaskSummary, error)
	ListProject(context.Context, string, string, bool) ([]TaskSummary, error)
	ListToday(context.Context, string, string, string, bool) ([]TaskSummary, error)
	Search(context.Context, string, SearchQuery) ([]Task, error)
	Complete(context.Context, execution.Scope, string, int64) (Task, error)
	Reopen(context.Context, execution.Scope, string, int64) (Task, error)
	Update(context.Context, execution.Scope, string, Update) (Task, error)
	Reorder(context.Context, execution.Scope, string, Reorder) ([]TaskSummary, error)
	Delete(context.Context, execution.Scope, string, int64) error
	ListComments(context.Context, string, string) ([]Comment, error)
	CreateComment(context.Context, execution.Scope, string, string) (Comment, error)
	UpdateComment(context.Context, execution.Scope, string, string, string, int64) (Comment, error)
	DeleteComment(context.Context, execution.Scope, string, string, int64) error
}

// HTTPModule owns the task domain's routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type createTaskRequest struct {
	Title       string           `json:"title"`
	Description nullable[string] `json:"description"`
	ProjectID   *string          `json:"projectId"`
	SectionID   *string          `json:"sectionId"`
	ParentID    *string          `json:"parentId"`
	Priority    optional[int]    `json:"priority"`
	DueDate     nullable[string] `json:"dueDate"`
	DueTime     nullable[string] `json:"dueTime"`
	DueTimezone nullable[string] `json:"dueTimezone"`
}

type updateTaskRequest struct {
	Version     *int64           `json:"version"`
	Title       optional[string] `json:"title"`
	Description nullable[string] `json:"description"`
	ProjectID   nullable[string] `json:"projectId"`
	SectionID   nullable[string] `json:"sectionId"`
	Priority    optional[int]    `json:"priority"`
	DueDate     nullable[string] `json:"dueDate"`
	DueTime     nullable[string] `json:"dueTime"`
	DueTimezone nullable[string] `json:"dueTimezone"`
}

type reorderTaskRequest struct {
	Version      *int64           `json:"version"`
	SectionID    nullable[string] `json:"sectionId"`
	BeforeTaskID *string          `json:"beforeTaskId"`
}

type taskVersionRequest struct {
	Version *int64 `json:"version"`
}

type commentRequest struct {
	Body    string `json:"body"`
	Version *int64 `json:"version"`
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
	Tasks []TaskSummary `json:"tasks"`
}

type subtaskListResponse struct {
	Tasks []Task `json:"tasks"`
}

type commentListResponse struct {
	Comments []Comment `json:"comments"`
}

// NewHTTPModule constructs the task HTTP module.
func NewHTTPModule(authDomain *auth.Domain, service HTTPService) *HTTPModule {
	return &HTTPModule{authDomain: authDomain, service: service}
}

// Mount registers all task-owned routes on the product API.
func (m *HTTPModule) Mount(api *httpserver.HandlerGroup) {
	handlers := taskHandlers{service: m.service}

	tasksAPI := httpserver.NewHandlerGroup()
	tasksAPI.Use(m.authDomain.Middleware)
	tasksAPI.HandleFunc("POST /", handlers.create)
	tasksAPI.HandleFunc("GET /search", handlers.search)
	tasksAPI.HandleFunc("GET /{id}", handlers.get)
	tasksAPI.HandleFunc("GET /{id}/subtasks", handlers.listSubtasks)
	tasksAPI.HandleFunc("GET /{id}/comments", handlers.listComments)
	tasksAPI.HandleFunc("POST /{id}/comments", handlers.createComment)
	tasksAPI.HandleFunc("PATCH /{id}/comments/{commentId}", handlers.updateComment)
	tasksAPI.HandleFunc("DELETE /{id}/comments/{commentId}", handlers.deleteComment)
	tasksAPI.HandleFunc("PATCH /{id}", handlers.update)
	tasksAPI.HandleFunc("DELETE /{id}", handlers.delete)
	tasksAPI.HandleFunc("POST /{id}/complete", handlers.complete)
	tasksAPI.HandleFunc("POST /{id}/reopen", handlers.reopen)
	tasksAPI.HandleFunc("POST /{id}/reorder", handlers.reorder)
	api.Mount("/tasks", tasksAPI)

	viewsAPI := httpserver.NewHandlerGroup()
	viewsAPI.Use(m.authDomain.Middleware)
	viewsAPI.HandleFunc("GET /projects/{id}", handlers.listProject)
	viewsAPI.HandleFunc("GET /projects/{id}/all", handlers.listAll)
	viewsAPI.HandleFunc("GET /projects/{id}/inbox", handlers.listInbox)
	viewsAPI.HandleFunc("GET /projects/{id}/today", handlers.listToday)
	api.Mount("/views", viewsAPI)
}

func (h taskHandlers) search(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	query := r.URL.Query()
	text := query.Get("query")
	projectID := strings.TrimSpace(query.Get("project_id"))
	if len(query["query"]) != 1 || len(query["project_id"]) != 1 || projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(text) == "" {
		http.Error(w, ErrSearchQueryRequired.Error(), http.StatusBadRequest)
		return
	}

	limit := 20
	if len(query["limit"]) > 1 || len(query["status"]) > 1 {
		http.Error(w, "search parameters must not be repeated", http.StatusBadRequest)
		return
	}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			http.Error(w, ErrInvalidSearchLimit.Error(), http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	var status *Status
	if value := query.Get("status"); value != "" {
		parsed := Status(value)
		status = &parsed
	}

	tasks, err := h.service.Search(r.Context(), user.ID, SearchQuery{
		Query: text, ProjectID: &projectID, Status: status, Limit: limit,
	})
	if err != nil {
		writeTaskError(w, r, "search", err)
		return
	}
	writeJSON(w, r, http.StatusOK, struct {
		Tasks []Task `json:"tasks"`
	}{Tasks: tasks})
}

func (h taskHandlers) create(w http.ResponseWriter, r *http.Request) {
	var request createTaskRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}

	if request.ParentID != nil {
		if request.ProjectID != nil || request.SectionID != nil {
			writeTaskError(w, r, "create", ErrSubtaskPlacement)
			return
		}
	} else {
		if request.ProjectID == nil {
			writeTaskError(w, r, "create", ErrProjectRequired)
			return
		}
	}

	input := CreateInput{
		Title:       request.Title,
		Description: request.Description.Value,
		ProjectID:   request.ProjectID,
		SectionID:   request.SectionID,
		ParentID:    request.ParentID,
		DueTimezone: request.DueTimezone.Value,
	}
	if request.Priority.Set {
		input.Priority = request.Priority.Value
	}
	if request.DueDate.Value != nil {
		value := Date(*request.DueDate.Value)
		input.DueDate = &value
	}
	if request.DueTime.Value != nil {
		value := TimeOfDay(*request.DueTime.Value)
		input.DueTime = &value
	}
	created, err := h.service.CreateWithProperties(r.Context(), scope, input)
	if err != nil {
		writeTaskError(w, r, "create", err)
		return
	}

	writeJSON(w, r, http.StatusCreated, created)
}

func (h taskHandlers) listSubtasks(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	tasks, err := h.service.ListSubtasks(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeTaskError(w, r, "list_subtasks", err)
		return
	}
	writeJSON(w, r, http.StatusOK, subtaskListResponse{Tasks: tasks})
}

func (h taskHandlers) listComments(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	comments, err := h.service.ListComments(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeTaskError(w, r, "list_comments", err)
		return
	}
	writeJSON(w, r, http.StatusOK, commentListResponse{Comments: comments})
}

func (h taskHandlers) createComment(w http.ResponseWriter, r *http.Request) {
	var request commentRequest
	if !decodeTaskJSON(w, r, &request) {
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}
	created, err := h.service.CreateComment(r.Context(), scope, r.PathValue("id"), request.Body)
	if err != nil {
		writeTaskError(w, r, "create_comment", err)
		return
	}
	writeJSON(w, r, http.StatusCreated, created)
}

func (h taskHandlers) updateComment(w http.ResponseWriter, r *http.Request) {
	var request commentRequest
	if !decodeTaskJSON(w, r, &request) {
		return
	}
	if request.Version == nil {
		http.Error(w, "task comment version is required", http.StatusBadRequest)
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}
	updated, err := h.service.UpdateComment(
		r.Context(), scope, r.PathValue("id"), r.PathValue("commentId"),
		request.Body, *request.Version,
	)
	if err != nil {
		writeTaskError(w, r, "update_comment", err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (h taskHandlers) deleteComment(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeTaskVersion(w, r)
	if !ok {
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteComment(
		r.Context(), scope, r.PathValue("id"), r.PathValue("commentId"), *request.Version,
	); err != nil {
		writeTaskError(w, r, "delete_comment", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeTaskJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return false
	}
	return true
}

func (h taskHandlers) listProject(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	includeCompleted, err := parseIncludeCompleted(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tasks, err := h.service.ListProject(
		r.Context(), user.ID, r.PathValue("id"), includeCompleted,
	)
	if err != nil {
		writeTaskError(w, r, "list_project", err)
		return
	}

	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (h taskHandlers) get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	found, err := h.service.Get(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeTaskError(w, r, "get", err)
		return
	}

	writeJSON(w, r, http.StatusOK, found)
}

func (h taskHandlers) update(w http.ResponseWriter, r *http.Request) {
	var request updateTaskRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "task version is required", http.StatusBadRequest)
		return
	}

	update, err := request.taskUpdate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}

	updated, err := h.service.Update(r.Context(), scope, r.PathValue("id"), update)
	if err != nil {
		writeTaskError(w, r, "update", err)
		return
	}

	writeJSON(w, r, http.StatusOK, updated)
}

func (h taskHandlers) listInbox(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	includeCompleted, err := parseIncludeCompleted(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tasks, err := h.service.ListInbox(r.Context(), user.ID, r.PathValue("id"), includeCompleted)
	if err != nil {
		writeTaskError(w, r, "list_inbox", err)
		return
	}

	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (h taskHandlers) listAll(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	includeCompleted, err := parseIncludeCompleted(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tasks, err := h.service.ListAll(r.Context(), user.ID, r.PathValue("id"), includeCompleted)
	if err != nil {
		writeTaskError(w, r, "list_all", err)
		return
	}

	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (h taskHandlers) listToday(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	timezone := r.URL.Query().Get("timezone")
	if timezone == "" {
		http.Error(w, "timezone is required", http.StatusBadRequest)
		return
	}
	includeCompleted, err := parseIncludeCompleted(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tasks, err := h.service.ListToday(
		r.Context(), user.ID, r.PathValue("id"), timezone, includeCompleted,
	)
	if err != nil {
		writeTaskError(w, r, "list_today", err)
		return
	}

	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (h taskHandlers) complete(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, "complete", h.service.Complete)
}

func (h taskHandlers) reopen(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, "reopen", h.service.Reopen)
}

func (h taskHandlers) reorder(w http.ResponseWriter, r *http.Request) {
	var request reorderTaskRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "task version is required", http.StatusBadRequest)
		return
	}
	if !request.SectionID.Set {
		http.Error(w, "task section is required", http.StatusBadRequest)
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}

	tasks, err := h.service.Reorder(r.Context(), scope, r.PathValue("id"), Reorder{
		Version:      *request.Version,
		SectionID:    request.SectionID.Value,
		BeforeTaskID: request.BeforeTaskID,
	})
	if err != nil {
		writeTaskError(w, r, "reorder", err)
		return
	}

	writeJSON(w, r, http.StatusOK, taskListResponse{Tasks: tasks})
}

func (h taskHandlers) delete(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeTaskVersion(w, r)
	if !ok {
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), scope, r.PathValue("id"), *request.Version); err != nil {
		writeTaskError(w, r, "delete", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h taskHandlers) changeStatus(
	w http.ResponseWriter,
	r *http.Request,
	operation string,
	change func(context.Context, execution.Scope, string, int64) (Task, error),
) {
	request, ok := decodeTaskVersion(w, r)
	if !ok {
		return
	}
	scope, ok := webUserScope(w, r)
	if !ok {
		return
	}

	updated, err := change(r.Context(), scope, r.PathValue("id"), *request.Version)
	if err != nil {
		writeTaskError(w, r, operation, err)
		return
	}

	writeJSON(w, r, http.StatusOK, updated)
}

func decodeTaskVersion(w http.ResponseWriter, r *http.Request) (taskVersionRequest, bool) {
	var request taskVersionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return taskVersionRequest{}, false
	}
	if request.Version == nil {
		http.Error(w, "task version is required", http.StatusBadRequest)
		return taskVersionRequest{}, false
	}

	return request, true
}

func webUserScope(w http.ResponseWriter, r *http.Request) (execution.Scope, bool) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return execution.Scope{}, false
	}
	correlationID, _ := r.Context().Value(log.TraceIDKey).(string)
	scope := execution.UserScope(user.ID, correlationID)
	if err := scope.Validate(); err != nil {
		log.ErrorContext(r.Context(), "invalid web execution scope", "error", err)
		http.Error(w, "request execution context unavailable", http.StatusInternalServerError)
		return execution.Scope{}, false
	}

	return scope, true
}

func writeTaskError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrTitleRequired),
		errors.Is(err, ErrTitleTooLong),
		errors.Is(err, ErrProjectRequired),
		errors.Is(err, ErrDescriptionTooLong),
		errors.Is(err, ErrInvalidPriority),
		errors.Is(err, ErrInvalidDueDate),
		errors.Is(err, ErrInvalidDueTime),
		errors.Is(err, ErrDueDateRequired),
		errors.Is(err, ErrTaskNotReorderable),
		errors.Is(err, ErrInvalidTimezone),
		errors.Is(err, ErrInvalidVersion),
		errors.Is(err, ErrNoChanges),
		errors.Is(err, ErrSubtaskPlacement),
		errors.Is(err, ErrCommentRequired),
		errors.Is(err, ErrCommentTooLong),
		errors.Is(err, ErrSearchQueryRequired),
		errors.Is(err, ErrInvalidSearchLimit),
		errors.Is(err, ErrInvalidSearchStatus):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrTaskNotFound):
		http.Error(w, ErrTaskNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrCommentNotFound):
		http.Error(w, ErrCommentNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrProjectNotFound):
		http.Error(w, ErrProjectNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrSectionNotFound):
		http.Error(w, ErrSectionNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, ErrVersionConflict.Error(), http.StatusConflict)
	case errors.Is(err, ErrCommentVersionConflict):
		http.Error(w, ErrCommentVersionConflict.Error(), http.StatusConflict)
	case errors.Is(err, ErrActiveSubtasks), errors.Is(err, ErrParentCompleted):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "task request failed", "operation", operation, "error", err)
		http.Error(w, "task request failed", http.StatusInternalServerError)
	}
}

func parseIncludeCompleted(r *http.Request) (bool, error) {
	value := r.URL.Query().Get("include_completed")
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New("invalid include_completed value")
	}

	return parsed, nil
}

func (r updateTaskRequest) taskUpdate() (Update, error) {
	update := Update{Version: *r.Version}
	if r.Title.Set {
		update.Title = &r.Title.Value
	}
	if r.Description.Set {
		update.Description = &Nullable[string]{Value: r.Description.Value}
	}
	if r.ProjectID.Set {
		update.ProjectID = &Nullable[string]{Value: r.ProjectID.Value}
	}
	if r.SectionID.Set {
		update.SectionID = &Nullable[string]{Value: r.SectionID.Value}
	}
	if r.Priority.Set {
		update.Priority = &r.Priority.Value
	}
	if r.DueDate.Set {
		update.DueDate = &Nullable[Date]{}
		if r.DueDate.Value != nil {
			dueDate, err := ParseDate(*r.DueDate.Value)
			if err != nil {
				return Update{}, errors.New("task due date must use YYYY-MM-DD")
			}
			update.DueDate.Value = &dueDate
		}
	}
	if r.DueTime.Set {
		update.DueTime = &Nullable[TimeOfDay]{}
		if r.DueTime.Value != nil {
			dueTime, err := ParseTimeOfDay(*r.DueTime.Value)
			if err != nil {
				return Update{}, errors.New("task due time must use HH:MM")
			}
			update.DueTime.Value = &dueTime
		}
	}
	if r.DueTimezone.Set {
		update.DueTimezone = &Nullable[string]{Value: r.DueTimezone.Value}
	}

	return update, nil
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if err := httpserver.WriteJSON(w, status, value); err != nil {
		log.ErrorContext(r.Context(), "failed to write task response", "error", err)
	}
}
