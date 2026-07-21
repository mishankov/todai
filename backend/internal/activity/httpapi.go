package activity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"
)

const (
	defaultListLimit    = 50
	maximumListLimit    = 200
	changePageSize      = 100
	changePollInterval  = 250 * time.Millisecond
	changeRequestMaxAge = 15 * time.Second
)

// HTTPService describes the activity operations exposed over HTTP.
type HTTPService interface {
	List(context.Context, string, string, int) ([]Event, error)
	LatestOffset(context.Context, string, string) (int64, error)
	ListAfter(context.Context, string, string, int64, int) ([]Event, error)
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

type changeListResponse struct {
	Cursor int64   `json:"cursor"`
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
	activityAPI.HandleFunc("GET /changes", handlers.changes)
	api.Mount("/activity", activityAPI)
}

func (h activityHandlers) changes(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	rawCursor := strings.TrimSpace(r.URL.Query().Get("after"))
	after, err := parseChangeCursor(rawCursor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if rawCursor == "" {
		after, err = h.service.LatestOffset(r.Context(), user.ID, projectID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			writeActivityError(w, r, err)
			return
		}
		writeActivityJSON(w, r, changeListResponse{Cursor: after, Events: []Event{}})
		return
	}

	events, err := h.waitForChanges(r.Context(), user.ID, projectID, after)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		writeActivityError(w, r, err)
		return
	}
	if len(events) > 0 {
		after = events[len(events)-1].StreamOffset
	}
	writeActivityJSON(w, r, changeListResponse{Cursor: after, Events: events})
}

func (h activityHandlers) waitForChanges(
	ctx context.Context,
	userID, projectID string,
	after int64,
) ([]Event, error) {
	poll := time.NewTicker(changePollInterval)
	timeout := time.NewTimer(changeRequestMaxAge)
	defer poll.Stop()
	defer timeout.Stop()
	for {
		events, err := h.service.ListAfter(ctx, userID, projectID, after, changePageSize)
		if err != nil || len(events) > 0 {
			return events, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return []Event{}, nil
		case <-poll.C:
		}
	}
}

func parseChangeCursor(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	after, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || after < 0 {
		return 0, ErrInvalidStreamCursor
	}
	return after, nil
}

func writeActivityJSON(w http.ResponseWriter, r *http.Request, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.ErrorContext(r.Context(), "encode activity response", "error", err)
	}
}

func (h activityHandlers) list(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}

	limit, err := parseLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	events, err := h.service.List(r.Context(), user.ID, projectID, limit)
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
