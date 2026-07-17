package project

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

type projectHandlers struct {
	service HTTPService
}

// HTTPService describes the project operations exposed over HTTP.
type HTTPService interface {
	Create(context.Context, string, string) (Project, error)
	Get(context.Context, string, string) (Project, error)
	List(context.Context, string, bool) ([]Project, error)
	Update(context.Context, string, string, Update) (Project, error)
	CreateSection(context.Context, string, string, string) (Section, error)
	ListSections(context.Context, string, string) ([]Section, error)
	UpdateSection(context.Context, string, string, string, SectionUpdate) (Section, error)
	DeleteSection(context.Context, string, string, string, int64) error
	ReorderSection(context.Context, string, string, string, int64, *string) ([]Section, error)
}

// HTTPModule owns the project domain's routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type createProjectRequest struct {
	Name string `json:"name"`
}

type updateProjectRequest struct {
	Version  *int64           `json:"version"`
	Name     optional[string] `json:"name"`
	Archived optional[bool]   `json:"archived"`
	Layout   optional[Layout] `json:"layout"`
}

type createSectionRequest struct {
	Name string `json:"name"`
}

type updateSectionRequest struct {
	Version *int64           `json:"version"`
	Name    optional[string] `json:"name"`
}

type sectionVersionRequest struct {
	Version *int64 `json:"version"`
}

type reorderSectionRequest struct {
	Version         *int64  `json:"version"`
	BeforeSectionID *string `json:"beforeSectionId"`
}

type optional[T any] struct {
	Set   bool
	Value T
}

func (o *optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

type projectListResponse struct {
	Projects []Project `json:"projects"`
}

type sectionListResponse struct {
	Sections []Section `json:"sections"`
}

// NewHTTPModule constructs the project HTTP module.
func NewHTTPModule(authDomain *auth.Domain, service HTTPService) *HTTPModule {
	return &HTTPModule{authDomain: authDomain, service: service}
}

// Mount registers all project-owned routes on the product API.
func (m *HTTPModule) Mount(api *httpserver.HandlerGroup) {
	handlers := projectHandlers{service: m.service}

	projectsAPI := httpserver.NewHandlerGroup()
	projectsAPI.Use(m.authDomain.Middleware)
	projectsAPI.HandleFunc("GET /", handlers.list)
	projectsAPI.HandleFunc("POST /", handlers.create)
	projectsAPI.HandleFunc("GET /{id}", handlers.get)
	projectsAPI.HandleFunc("PATCH /{id}", handlers.update)
	projectsAPI.HandleFunc("GET /{id}/sections", handlers.listSections)
	projectsAPI.HandleFunc("POST /{id}/sections", handlers.createSection)
	projectsAPI.HandleFunc("PATCH /{id}/sections/{sectionId}", handlers.updateSection)
	projectsAPI.HandleFunc("DELETE /{id}/sections/{sectionId}", handlers.deleteSection)
	projectsAPI.HandleFunc("POST /{id}/sections/{sectionId}/reorder", handlers.reorderSection)
	api.Mount("/projects", projectsAPI)
}

func (h projectHandlers) list(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	includeArchived, err := parseIncludeArchived(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	projects, err := h.service.List(r.Context(), user.ID, includeArchived)
	if err != nil {
		writeProjectError(w, r, "list", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, projectListResponse{Projects: projects})
}

func (h projectHandlers) create(w http.ResponseWriter, r *http.Request) {
	var request createProjectRequest
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

	created, err := h.service.Create(r.Context(), user.ID, request.Name)
	if err != nil {
		writeProjectError(w, r, "create", err)
		return
	}

	writeProjectJSON(w, r, http.StatusCreated, created)
}

func (h projectHandlers) get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	found, err := h.service.Get(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeProjectError(w, r, "get", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, found)
}

func (h projectHandlers) update(w http.ResponseWriter, r *http.Request) {
	var request updateProjectRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "project version is required", http.StatusBadRequest)
		return
	}

	update := Update{Version: *request.Version}
	if request.Name.Set {
		update.Name = &request.Name.Value
	}
	if request.Archived.Set {
		update.Archived = &request.Archived.Value
	}
	if request.Layout.Set {
		update.Layout = &request.Layout.Value
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	updated, err := h.service.Update(r.Context(), user.ID, r.PathValue("id"), update)
	if err != nil {
		writeProjectError(w, r, "update", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, updated)
}

func (h projectHandlers) listSections(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	sections, err := h.service.ListSections(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeProjectError(w, r, "list_sections", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, sectionListResponse{Sections: sections})
}

func (h projectHandlers) createSection(w http.ResponseWriter, r *http.Request) {
	var request createSectionRequest
	if err := decodeProjectRequest(r, &request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	created, err := h.service.CreateSection(r.Context(), user.ID, r.PathValue("id"), request.Name)
	if err != nil {
		writeProjectError(w, r, "create_section", err)
		return
	}

	writeProjectJSON(w, r, http.StatusCreated, created)
}

func (h projectHandlers) updateSection(w http.ResponseWriter, r *http.Request) {
	var request updateSectionRequest
	if err := decodeProjectRequest(r, &request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "project section version is required", http.StatusBadRequest)
		return
	}
	update := SectionUpdate{Version: *request.Version}
	if request.Name.Set {
		update.Name = &request.Name.Value
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	updated, err := h.service.UpdateSection(
		r.Context(), user.ID, r.PathValue("id"), r.PathValue("sectionId"), update,
	)
	if err != nil {
		writeProjectError(w, r, "update_section", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, updated)
}

func (h projectHandlers) deleteSection(w http.ResponseWriter, r *http.Request) {
	var request sectionVersionRequest
	if err := decodeProjectRequest(r, &request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "project section version is required", http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := h.service.DeleteSection(
		r.Context(), user.ID, r.PathValue("id"), r.PathValue("sectionId"), *request.Version,
	); err != nil {
		writeProjectError(w, r, "delete_section", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h projectHandlers) reorderSection(w http.ResponseWriter, r *http.Request) {
	var request reorderSectionRequest
	if err := decodeProjectRequest(r, &request); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	if request.Version == nil {
		http.Error(w, "project section version is required", http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	sections, err := h.service.ReorderSection(
		r.Context(), user.ID, r.PathValue("id"), r.PathValue("sectionId"),
		*request.Version, request.BeforeSectionID,
	)
	if err != nil {
		writeProjectError(w, r, "reorder_section", err)
		return
	}

	writeProjectJSON(w, r, http.StatusOK, sectionListResponse{Sections: sections})
}

func writeProjectError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrNameRequired),
		errors.Is(err, ErrNameTooLong),
		errors.Is(err, ErrSectionNameRequired),
		errors.Is(err, ErrSectionNameTooLong),
		errors.Is(err, ErrSectionNoChanges),
		errors.Is(err, ErrInvalidLayout),
		errors.Is(err, ErrInvalidVersion),
		errors.Is(err, ErrNoChanges):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrProjectNotFound):
		http.Error(w, ErrProjectNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrSectionNotFound):
		http.Error(w, ErrSectionNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, ErrVersionConflict.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "project request failed", "operation", operation, "error", err)
		http.Error(w, "project request failed", http.StatusInternalServerError)
	}
}

func decodeProjectRequest(r *http.Request, value any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func parseIncludeArchived(r *http.Request) (bool, error) {
	value := r.URL.Query().Get("include_archived")
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New("invalid include_archived value")
	}

	return parsed, nil
}

func writeProjectJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if err := httpserver.WriteJSON(w, status, value); err != nil {
		log.ErrorContext(r.Context(), "failed to write project response", "error", err)
	}
}
