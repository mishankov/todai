package usersettings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/execution"
)

// HTTPService describes user-settings operations exposed over HTTP.
type HTTPService interface {
	Get(context.Context, string) (View, error)
	Update(context.Context, execution.Scope, Update) (View, error)
}

// HTTPModule owns user-settings routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type settingsHandlers struct {
	service HTTPService
}

type updateSettingsRequest struct {
	Timezone            string `json:"timezone"`
	AgentModel          string `json:"agentModel"`
	AgentThinkingEffort string `json:"agentThinkingEffort"`
	Version             *int64 `json:"version"`
}

// NewHTTPModule constructs the user-settings HTTP module.
func NewHTTPModule(authDomain *auth.Domain, service HTTPService) *HTTPModule {
	return &HTTPModule{authDomain: authDomain, service: service}
}

// Mount registers all user-settings routes.
func (m *HTTPModule) Mount(api *httpserver.HandlerGroup) {
	handlers := settingsHandlers{service: m.service}
	settingsAPI := httpserver.NewHandlerGroup()
	settingsAPI.Use(m.authDomain.Middleware)
	settingsAPI.HandleFunc("GET /", handlers.get)
	settingsAPI.HandleFunc("PATCH /", handlers.update)
	api.Mount("/settings", settingsAPI)
}

func (h settingsHandlers) get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	view, err := h.service.Get(r.Context(), user.ID)
	if err != nil {
		writeSettingsError(w, r, "get", err)
		return
	}
	writeSettingsJSON(w, r, http.StatusOK, view)
}

func (h settingsHandlers) update(w http.ResponseWriter, r *http.Request) {
	var request updateSettingsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Version == nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	correlationID, _ := r.Context().Value(log.TraceIDKey).(string)
	scope := execution.UserScope(user.ID, correlationID)
	if err := scope.Validate(); err != nil {
		log.ErrorContext(r.Context(), "invalid user settings web execution scope", "error", err)
		http.Error(w, "request execution context unavailable", http.StatusInternalServerError)
		return
	}
	view, err := h.service.Update(r.Context(), scope, Update{
		Timezone: request.Timezone, AgentModel: request.AgentModel,
		AgentThinkingEffort: request.AgentThinkingEffort, Version: *request.Version,
	})
	if err != nil {
		writeSettingsError(w, r, "update", err)
		return
	}
	writeSettingsJSON(w, r, http.StatusOK, view)
}

func writeSettingsError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrInvalidTimezone), errors.Is(err, ErrInvalidAgentModel),
		errors.Is(err, ErrInvalidAgentThinkingEffort), errors.Is(err, ErrInvalidVersion):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrVersionConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "user settings request failed", "operation", operation, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeSettingsJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.ErrorContext(r.Context(), "encode user settings response", "error", err)
	}
}
