package agent_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	platformalog "github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/execution"
)

func TestAgentRoutesRequireAuthenticatedSession(t *testing.T) {
	handler, _ := testAgentAPI(nil)
	request := httptest.NewRequest(http.MethodPost, "/agent/sessions", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAgentRoutesPassUserScopeAndReturnContracts(t *testing.T) {
	handler, service := testAgentAPI(&auth.User{ID: "user-id", Username: "owner"})

	created := serveAgentRequest(t, handler, http.MethodPost, "/agent/sessions", nil, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.Code, http.StatusCreated)
	}
	if service.scope.UserID != "user-id" || service.scope.ActorType != execution.ActorUser ||
		service.scope.CorrelationID == "" {
		t.Errorf("create scope = %#v", service.scope)
	}

	posted := serveAgentRequest(
		t, handler, http.MethodPost, "/agent/sessions/session-id/messages",
		map[string]string{"message": "Triage inbox"}, nil,
	)
	if posted.Code != http.StatusAccepted {
		t.Fatalf("post status = %d, want %d; body = %q", posted.Code, http.StatusAccepted, posted.Body.String())
	}
	if service.sessionID != "session-id" || service.message != "Triage inbox" {
		t.Errorf("post arguments = (%q, %q)", service.sessionID, service.message)
	}

	found := serveAgentRequest(t, handler, http.MethodGet, "/agent/sessions/session-id", nil, nil)
	if found.Code != http.StatusOK || service.userID != "user-id" {
		t.Errorf("get status/user = %d/%q", found.Code, service.userID)
	}
	var conversation agent.Conversation
	if err := json.NewDecoder(found.Body).Decode(&conversation); err != nil {
		t.Fatalf("decode conversation: %v", err)
	}
	if conversation.LastStreamOffset != 7 {
		t.Errorf("last stream offset = %d, want 7", conversation.LastStreamOffset)
	}

	aborted := serveAgentRequest(t, handler, http.MethodPost, "/agent/runs/run-id/abort", nil, nil)
	if aborted.Code != http.StatusOK || service.runID != "run-id" {
		t.Errorf("abort status/run = %d/%q", aborted.Code, service.runID)
	}
}

func TestAgentEventStreamReplaysAfterLastEventID(t *testing.T) {
	handler, service := testAgentAPI(&auth.User{ID: "user-id", Username: "owner"})
	service.events = []agent.RunEvent{{
		StreamOffset: 8, RunID: "run-id", SessionID: "session-id", Sequence: 2,
		Type: agent.EventMessageDelta, Payload: json.RawMessage(`{"delta":"Hello"}`),
	}}

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/agent/sessions/session-id/events", nil).WithContext(ctx)
	request.AddCookie(agentCookie())
	request.Header.Set("Last-Event-ID", "7")
	response := newCancelOnFlushRecorder(cancel)
	handler.ServeHTTP(response, request)

	if response.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.status, http.StatusOK)
	}
	if service.after != 7 || service.limit != 100 || service.userID != "user-id" {
		t.Errorf("ListEvents() = user %q after %d limit %d", service.userID, service.after, service.limit)
	}
	body := response.body.String()
	if !strings.Contains(body, "id: 8\n") ||
		!strings.Contains(body, "event: "+agent.EventMessageDelta+"\n") ||
		!strings.Contains(body, `"delta":"Hello"`) {
		t.Errorf("SSE body = %q", body)
	}
}

func TestAgentEventStreamRejectsInvalidLastEventID(t *testing.T) {
	handler, _ := testAgentAPI(&auth.User{ID: "user-id", Username: "owner"})
	response := serveAgentRequest(
		t, handler, http.MethodGet, "/agent/sessions/session-id/events", nil,
		map[string]string{"Last-Event-ID": "invalid"},
	)
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func testAgentAPI(user *auth.User) (http.Handler, *fakeHTTPAgentService) {
	repository := fakeAgentAuthRepository{user: user}
	storage := &fakeAgentSessionStorage{sessions: make(map[string]string)}
	if user != nil {
		storage.sessions["session-id"] = user.ID
	}
	authService := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	authDomain := &auth.Domain{
		Service: authService, Middleware: auth.NewAuthenticationMiddleware(authService),
	}
	service := &fakeHTTPAgentService{}
	api := httpserver.NewHandlerGroup()
	api.Use(platformalog.NewTraceIDMiddleware(nil, ""))
	agent.NewHTTPModule(authDomain, service).Mount(api)
	return api, service
}

func serveAgentRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &encoded)
	request.AddCookie(agentCookie())
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func agentCookie() *http.Cookie {
	return &http.Cookie{Name: "todai_session", Value: "session-id"}
}

type fakeHTTPAgentService struct {
	scope     execution.Scope
	userID    string
	sessionID string
	runID     string
	message   string
	after     int64
	limit     int
	events    []agent.RunEvent
}

func (s *fakeHTTPAgentService) CreateSession(_ context.Context, scope execution.Scope) (agent.Session, error) {
	s.scope = scope
	return agent.Session{ID: "session-id"}, nil
}

func (s *fakeHTTPAgentService) GetSession(
	_ context.Context,
	userID string,
	sessionID string,
) (agent.Conversation, error) {
	s.userID = userID
	s.sessionID = sessionID
	return agent.Conversation{Session: agent.Session{ID: sessionID}, LastStreamOffset: 7}, nil
}

func (s *fakeHTTPAgentService) PostMessage(
	_ context.Context,
	scope execution.Scope,
	sessionID string,
	message string,
) (agent.PostedMessage, error) {
	s.scope = scope
	s.sessionID = sessionID
	s.message = message
	return agent.PostedMessage{
		Message: agent.Message{ID: "message-id"},
		Run:     agent.Run{ID: "run-id", Status: agent.RunStatusQueued},
	}, nil
}

func (s *fakeHTTPAgentService) ListEvents(
	_ context.Context,
	userID string,
	sessionID string,
	after int64,
	limit int,
) ([]agent.RunEvent, error) {
	s.userID = userID
	s.sessionID = sessionID
	s.after = after
	s.limit = limit
	return s.events, nil
}

func (s *fakeHTTPAgentService) Abort(
	_ context.Context,
	scope execution.Scope,
	runID string,
) (agent.Run, error) {
	s.scope = scope
	s.runID = runID
	return agent.Run{ID: runID, Status: agent.RunStatusAborted}, nil
}

type fakeAgentAuthRepository struct {
	user *auth.User
}

func (r fakeAgentAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (r fakeAgentAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (fakeAgentAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeAgentAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeAgentAuthRepository) Delete(context.Context, string) error { return nil }

type fakeAgentSessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeAgentSessionStorage) GetUserIdFromSessionId(_ context.Context, sessionID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID], nil
}

func (s *fakeAgentSessionStorage) CreateSessionForUser(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions["session-id"] = userID
	return "session-id", nil
}

func (s *fakeAgentSessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeAgentSessionStorage) DeleteSessionsByUserId(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}
	return nil
}

type cancelOnFlushRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
	cancel context.CancelFunc
}

func newCancelOnFlushRecorder(cancel context.CancelFunc) *cancelOnFlushRecorder {
	return &cancelOnFlushRecorder{header: make(http.Header), cancel: cancel}
}

func (r *cancelOnFlushRecorder) Header() http.Header {
	return r.header
}

func (r *cancelOnFlushRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *cancelOnFlushRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *cancelOnFlushRecorder) Flush() {
	r.cancel()
}
