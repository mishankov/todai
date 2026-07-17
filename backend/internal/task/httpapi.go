package task

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"
)

type taskHandlers struct {
	service HTTPService
}

// HTTPService describes the task operations exposed over HTTP.
type HTTPService interface {
	Create(context.Context, string, string, *string, *string) (Task, error)
	Get(context.Context, string, string) (Task, error)
	ListInbox(context.Context, string, bool) ([]Task, error)
	ListAll(context.Context, string, bool) ([]Task, error)
	ListProject(context.Context, string, string, bool) ([]Task, error)
	ListToday(context.Context, string, string, bool) ([]Task, error)
	Complete(context.Context, string, string) (Task, error)
	Reopen(context.Context, string, string) (Task, error)
	Update(context.Context, string, string, Update) (Task, error)
	Reorder(context.Context, string, string, Reorder) ([]Task, error)
	Delete(context.Context, string, string) error
}

// HTTPModule owns the task domain's routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type createTaskRequest struct {
	Title     string  `json:"title"`
	ProjectID *string `json:"projectId"`
	SectionID *string `json:"sectionId"`
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
	Tasks []Task `json:"tasks"`
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
	tasksAPI.HandleFunc("GET /{id}", handlers.get)
	tasksAPI.HandleFunc("PATCH /{id}", handlers.update)
	tasksAPI.HandleFunc("DELETE /{id}", handlers.delete)
	tasksAPI.HandleFunc("POST /{id}/complete", handlers.complete)
	tasksAPI.HandleFunc("POST /{id}/reopen", handlers.reopen)
	tasksAPI.HandleFunc("POST /{id}/reorder", handlers.reorder)
	api.Mount("/tasks", tasksAPI)

	viewsAPI := httpserver.NewHandlerGroup()
	viewsAPI.Use(m.authDomain.Middleware)
	viewsAPI.HandleFunc("GET /all", handlers.listAll)
	viewsAPI.HandleFunc("GET /inbox", handlers.listInbox)
	viewsAPI.HandleFunc("GET /today", handlers.listToday)
	viewsAPI.HandleFunc("GET /projects/{id}", handlers.listProject)
	api.Mount("/views", viewsAPI)
}

func (h taskHandlers) create(w http.ResponseWriter, r *http.Request) {
	var request createTaskRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	created, err := h.service.Create(
		r.Context(), user.ID, request.Title, request.ProjectID, request.SectionID,
	)
	if err != nil {
		writeTaskError(w, r, "create", err)
		return
	}

	writeJSON(w, r, http.StatusCreated, created)
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

	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	updated, err := h.service.Update(r.Context(), user.ID, r.PathValue("id"), update)
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

	tasks, err := h.service.ListInbox(r.Context(), user.ID, includeCompleted)
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

	tasks, err := h.service.ListAll(r.Context(), user.ID, includeCompleted)
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

	tasks, err := h.service.ListToday(r.Context(), user.ID, timezone, includeCompleted)
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
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tasks, err := h.service.Reorder(r.Context(), user.ID, r.PathValue("id"), Reorder{
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
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := h.service.Delete(r.Context(), user.ID, r.PathValue("id")); err != nil {
		writeTaskError(w, r, "delete", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h taskHandlers) changeStatus(
	w http.ResponseWriter,
	r *http.Request,
	operation string,
	change func(context.Context, string, string) (Task, error),
) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	updated, err := change(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeTaskError(w, r, operation, err)
		return
	}

	writeJSON(w, r, http.StatusOK, updated)
}

func writeTaskError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrTitleRequired),
		errors.Is(err, ErrTitleTooLong),
		errors.Is(err, ErrDescriptionTooLong),
		errors.Is(err, ErrInvalidPriority),
		errors.Is(err, ErrInvalidDueDate),
		errors.Is(err, ErrInvalidDueTime),
		errors.Is(err, ErrDueDateRequired),
		errors.Is(err, ErrTaskNotReorderable),
		errors.Is(err, ErrInvalidTimezone),
		errors.Is(err, ErrInvalidVersion),
		errors.Is(err, ErrNoChanges):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrTaskNotFound):
		http.Error(w, ErrTaskNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrProjectNotFound):
		http.Error(w, ErrProjectNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrSectionNotFound):
		http.Error(w, ErrSectionNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, ErrVersionConflict.Error(), http.StatusConflict)
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
