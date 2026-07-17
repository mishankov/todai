package activity

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

const (
	defaultListLimit = 50
	maximumListLimit = 200
)

// HTTPService describes the activity operations exposed over HTTP.
type HTTPService interface {
	List(context.Context, string, int) ([]Event, error)
}

// HTTPModule owns the activity domain's routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type activityHandlers struct {
	service HTTPService
}

type eventListResponse struct {
	Events []Event `json:"events"`
}

// NewHTTPModule constructs the activity HTTP module.
func NewHTTPModule(authDomain *auth.Domain, service HTTPService) *HTTPModule {
	return &HTTPModule{authDomain: authDomain, service: service}
}

// Mount registers all activity-owned routes on the product API.
func (m *HTTPModule) Mount(api *httpserver.HandlerGroup) {
	handlers := activityHandlers{service: m.service}
	activityAPI := httpserver.NewHandlerGroup()
	activityAPI.Use(m.authDomain.Middleware)
	activityAPI.HandleFunc("GET /", handlers.list)
	api.Mount("/activity", activityAPI)
}

func (h activityHandlers) list(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	limit, err := parseLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	events, err := h.service.List(r.Context(), user.ID, limit)
	if err != nil {
		writeActivityError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(eventListResponse{Events: events}); err != nil {
		log.ErrorContext(r.Context(), "encode activity response", "error", err)
	}
}

func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultListLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maximumListLimit {
		return 0, ErrInvalidLimit
	}
	return limit, nil
}

func writeActivityError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrInvalidLimit) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.ErrorContext(r.Context(), "activity request failed", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
