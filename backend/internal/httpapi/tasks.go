package httpapi

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

	"github.com/mishankov/todai/backend/internal/task"
)

type taskHandlers struct {
	service taskService
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

type inboxResponse struct {
	Tasks []task.Task `json:"tasks"`
}

func mountTaskAPI(api *httpserver.HandlerGroup, authDomain *auth.Domain, service taskService) {
	handlers := taskHandlers{service: service}

	tasksAPI := httpserver.NewHandlerGroup()
	tasksAPI.Use(authDomain.Middleware)
	tasksAPI.HandleFunc("POST /", handlers.create)
	tasksAPI.HandleFunc("GET /{id}", handlers.get)
	tasksAPI.HandleFunc("PATCH /{id}", handlers.update)
	tasksAPI.HandleFunc("DELETE /{id}", handlers.delete)
	tasksAPI.HandleFunc("POST /{id}/complete", handlers.complete)
	tasksAPI.HandleFunc("POST /{id}/reopen", handlers.reopen)
	api.Mount("/tasks", tasksAPI)

	viewsAPI := httpserver.NewHandlerGroup()
	viewsAPI.Use(authDomain.Middleware)
	viewsAPI.HandleFunc("GET /inbox", handlers.listInbox)
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

	includeCompleted := false
	if value := r.URL.Query().Get("include_completed"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			http.Error(w, "invalid include_completed value", http.StatusBadRequest)
			return
		}
		includeCompleted = parsed
	}

	tasks, err := h.service.ListInbox(r.Context(), user.ID, includeCompleted)
	if err != nil {
		writeTaskError(w, r, "list_inbox", err)
		return
	}

	writeJSON(w, r, http.StatusOK, inboxResponse{Tasks: tasks})
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
	change func(context.Context, string, string) (task.Task, error),
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
	case errors.Is(err, task.ErrTitleRequired),
		errors.Is(err, task.ErrTitleTooLong),
		errors.Is(err, task.ErrDescriptionTooLong),
		errors.Is(err, task.ErrInvalidPriority),
		errors.Is(err, task.ErrInvalidTimezone),
		errors.Is(err, task.ErrInvalidVersion),
		errors.Is(err, task.ErrNoChanges):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, task.ErrTaskNotFound):
		http.Error(w, task.ErrTaskNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, task.ErrVersionConflict):
		http.Error(w, task.ErrVersionConflict.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "task request failed", "operation", operation, "error", err)
		http.Error(w, "task request failed", http.StatusInternalServerError)
	}
}

func (r updateTaskRequest) taskUpdate() (task.Update, error) {
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
	if r.DueAt.Set {
		update.DueAt = &task.Nullable[time.Time]{}
		if r.DueAt.Value != nil {
			dueAt, err := time.Parse(time.RFC3339, *r.DueAt.Value)
			if err != nil {
				return task.Update{}, errors.New("task due date must use RFC3339")
			}
			update.DueAt.Value = &dueAt
		}
	}
	if r.DueTimezone.Set {
		update.DueTimezone = &task.Nullable[string]{Value: r.DueTimezone.Value}
	}

	return update, nil
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if err := httpserver.WriteJSON(w, status, value); err != nil {
		log.ErrorContext(r.Context(), "failed to write task response", "error", err)
	}
}
