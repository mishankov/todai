package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/execution"
)

const (
	eventPageSize       = 100
	eventPollInterval   = 250 * time.Millisecond
	eventHeartbeatEvery = 15 * time.Second
)

// HTTPService describes agent operations exposed over product HTTP.
type HTTPService interface {
	CreateSession(context.Context, execution.Scope) (Session, error)
	GetSession(context.Context, string, string) (Conversation, error)
	PostMessage(context.Context, execution.Scope, string, string) (PostedMessage, error)
	ListEvents(context.Context, string, string, int64, int) ([]RunEvent, error)
	Abort(context.Context, execution.Scope, string) (Run, error)
}

// HTTPModule owns the built-in agent's public routes and handlers.
type HTTPModule struct {
	authDomain *auth.Domain
	service    HTTPService
}

type agentHandlers struct {
	service HTTPService
}

type postMessageRequest struct {
	Message string `json:"message"`
}

// NewHTTPModule constructs the agent HTTP module.
func NewHTTPModule(authDomain *auth.Domain, service HTTPService) *HTTPModule {
	return &HTTPModule{authDomain: authDomain, service: service}
}

// Mount registers all agent-owned product routes.
func (m *HTTPModule) Mount(api *httpserver.HandlerGroup) {
	handlers := agentHandlers{service: m.service}
	agentAPI := httpserver.NewHandlerGroup()
	agentAPI.Use(m.authDomain.Middleware)
	agentAPI.HandleFunc("POST /sessions", handlers.createSession)
	agentAPI.HandleFunc("GET /sessions/{id}", handlers.getSession)
	agentAPI.HandleFunc("POST /sessions/{id}/messages", handlers.postMessage)
	agentAPI.HandleFunc("GET /sessions/{id}/events", handlers.events)
	agentAPI.HandleFunc("POST /runs/{id}/abort", handlers.abort)
	api.Mount("/agent", agentAPI)
}

func (h agentHandlers) createSession(w http.ResponseWriter, r *http.Request) {
	scope, ok := webScope(w, r)
	if !ok {
		return
	}
	created, err := h.service.CreateSession(r.Context(), scope)
	if err != nil {
		writeAgentError(w, r, "create_session", err)
		return
	}
	writeAgentJSON(w, r, http.StatusCreated, created)
}

func (h agentHandlers) getSession(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	found, err := h.service.GetSession(r.Context(), user.ID, r.PathValue("id"))
	if err != nil {
		writeAgentError(w, r, "get_session", err)
		return
	}
	writeAgentJSON(w, r, http.StatusOK, found)
}

func (h agentHandlers) postMessage(w http.ResponseWriter, r *http.Request) {
	var request postMessageRequest
	if !decodeAgentRequest(w, r, &request) {
		return
	}
	scope, ok := webScope(w, r)
	if !ok {
		return
	}
	posted, err := h.service.PostMessage(r.Context(), scope, r.PathValue("id"), request.Message)
	if err != nil {
		writeAgentError(w, r, "post_message", err)
		return
	}
	writeAgentJSON(w, r, http.StatusAccepted, posted)
}

func (h agentHandlers) abort(w http.ResponseWriter, r *http.Request) {
	scope, ok := webScope(w, r)
	if !ok {
		return
	}
	finished, err := h.service.Abort(r.Context(), scope, r.PathValue("id"))
	if err != nil {
		writeAgentError(w, r, "abort_run", err)
		return
	}
	writeAgentJSON(w, r, http.StatusOK, finished)
}

func (h agentHandlers) events(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	after, err := parseLastEventID(r.Header.Get("Last-Event-ID"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	events, err := h.service.ListEvents(r.Context(), user.ID, r.PathValue("id"), after, eventPageSize)
	if err != nil {
		writeAgentError(w, r, "list_events", err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "retry: 1000\n\n")
	after, err = writeEventBatch(w, events, after)
	if err != nil {
		log.ErrorContext(r.Context(), "write agent event stream", "error", err)
		return
	}
	flusher.Flush()

	poll := time.NewTicker(eventPollInterval)
	heartbeat := time.NewTicker(eventHeartbeatEvery)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			events, err := h.service.ListEvents(
				r.Context(), user.ID, r.PathValue("id"), after, eventPageSize,
			)
			if err != nil {
				log.ErrorContext(r.Context(), "poll agent event stream", "error", err)
				return
			}
			if len(events) == 0 {
				continue
			}
			after, err = writeEventBatch(w, events, after)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEventBatch(w io.Writer, events []RunEvent, after int64) (int64, error) {
	for _, event := range events {
		if event.StreamOffset <= after {
			continue
		}
		data, err := json.Marshal(event)
		if err != nil {
			return after, fmt.Errorf("encode agent stream event: %w", err)
		}
		if _, err := fmt.Fprintf(
			w, "id: %d\nevent: %s\ndata: %s\n\n", event.StreamOffset, event.Type, data,
		); err != nil {
			return after, fmt.Errorf("write agent stream event: %w", err)
		}
		after = event.StreamOffset
	}
	return after, nil
}

func parseLastEventID(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	after, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || after < 0 {
		return 0, ErrInvalidEventCursor
	}
	return after, nil
}

func webScope(w http.ResponseWriter, r *http.Request) (execution.Scope, bool) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return execution.Scope{}, false
	}
	correlationID, _ := r.Context().Value(log.TraceIDKey).(string)
	if correlationID == "" {
		log.ErrorContext(r.Context(), "agent request is missing Platforma trace ID")
		http.Error(w, "agent request is missing trace ID", http.StatusInternalServerError)
		return execution.Scope{}, false
	}
	return execution.UserScope(user.ID, correlationID), true
}

func decodeAgentRequest(w http.ResponseWriter, r *http.Request, target any) bool {
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

func writeAgentJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.ErrorContext(r.Context(), "encode agent response", "error", err)
	}
}

func writeAgentError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrSessionNotFound), errors.Is(err, ErrRunNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrMessageRequired), errors.Is(err, ErrInvalidEventCursor),
		errors.Is(err, ErrInvalidEventLimit), errors.Is(err, ErrInvalidEvent):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrRunFinished), errors.Is(err, ErrServiceStopped):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.ErrorContext(r.Context(), "agent request failed", "operation", operation, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
