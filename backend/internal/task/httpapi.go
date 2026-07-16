package task

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"
)

type taskHandlers struct {
	service HTTPService
}

// HTTPService describes the task operations exposed over HTTP.
type HTTPService interface {
	Create(context.Context, string, string) (Task, error)
	Get(context.Context, string, string) (Task, error)
	ListInbox(context.Context, string, bool) ([]Task, error)
	ListToday(context.Context, string, string, bool) ([]Task, error)
	Complete(context.Context, string, string) (Task, error)
	Reopen(context.Context, string, string) (Task, error)
	Update(context.Context, string, string, Update) (Task, error)
	Delete(context.Context, string, string) error
}

// HTTPModule owns the task domain's routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type createTaskRequest struct {
	Title string `json:"title"`
}

type updateTaskRequest struct {
	Version     *int64           `json:"version"`
	Title       optional[string] `json:"title"`
	Description nullable[string] `json:"description"`
	Priority    optional[int]    `json:"priority"`
	DueAt       nullable[string] `json:"dueAt"`
	DueTimezone nullable[string] `json:"dueTimezone"`
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
	api.Mount("/tasks", tasksAPI)

	viewsAPI := httpserver.NewHandlerGroup()
	viewsAPI.Use(m.authDomain.Middleware)
	viewsAPI.HandleFunc("GET /inbox", handlers.listInbox)
	viewsAPI.HandleFunc("GET /today", handlers.listToday)
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

	created, err := h.service.Create(r.Context(), user.ID, request.Title)
	if err != nil {
		writeTaskError(w, r, "create", err)
		return
	}

	writeJSON(w, r, http.StatusCreated, created)
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
		errors.Is(err, ErrInvalidTimezone),
		errors.Is(err, ErrInvalidVersion),
		errors.Is(err, ErrNoChanges):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrTaskNotFound):
		http.Error(w, ErrTaskNotFound.Error(), http.StatusNotFound)
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
	if r.Priority.Set {
		update.Priority = &r.Priority.Value
	}
	if r.DueAt.Set {
		update.DueAt = &Nullable[time.Time]{}
		if r.DueAt.Value != nil {
			dueAt, err := time.Parse(time.RFC3339, *r.DueAt.Value)
			if err != nil {
				return Update{}, errors.New("task due date must use RFC3339")
			}
			update.DueAt.Value = &dueAt
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
