package activity_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"

	"github.com/mishankov/todai/backend/internal/activity"
)

func TestActivityEndpointRequiresSession(t *testing.T) {
	t.Parallel()

	handler, _ := testActivityAPI(nil)
	request := httptest.NewRequest(http.MethodGet, "/activity", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestActivityEndpointListsAuthenticatedUsersEvents(t *testing.T) {
	t.Parallel()

	handler, service := testActivityAPI(&auth.User{ID: "user-id", Username: "owner"})
	service.events = []activity.Event{{ID: "event-id", Type: "task.created"}}
	request := httptest.NewRequest(http.MethodGet, "/activity?limit=12", nil)
	request.AddCookie(activityCookie())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.userID != "user-id" || service.limit != 12 {
		t.Errorf("List() arguments = (%q, %d), want (user-id, 12)", service.userID, service.limit)
	}
	var body struct {
		Events []activity.Event `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != "event-id" {
		t.Errorf("events = %#v", body.Events)
	}
}

func TestActivityEndpointUsesDefaultLimit(t *testing.T) {
	t.Parallel()

	handler, service := testActivityAPI(&auth.User{ID: "user-id", Username: "owner"})
	request := httptest.NewRequest(http.MethodGet, "/activity", nil)
	request.AddCookie(activityCookie())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.limit != 50 {
		t.Errorf("limit = %d, want 50", service.limit)
	}
}

func TestActivityEndpointRejectsInvalidLimits(t *testing.T) {
	t.Parallel()

	for _, limit := range []string{"0", "201", "invalid"} {
		t.Run(limit, func(t *testing.T) {
			t.Parallel()
			handler, _ := testActivityAPI(&auth.User{ID: "user-id", Username: "owner"})
			request := httptest.NewRequest(http.MethodGet, "/activity?limit="+limit, nil)
			request.AddCookie(activityCookie())
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Errorf("limit %q status = %d, want %d", limit, response.Code, http.StatusBadRequest)
			}
		})
	}
}

func testActivityAPI(user *auth.User) (http.Handler, *fakeActivityService) {
	repository := fakeActivityAuthRepository{user: user}
	storage := &fakeActivitySessionStorage{sessions: make(map[string]string)}
	if user != nil {
		storage.sessions["session-id"] = user.ID
	}
	authService := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	authDomain := &auth.Domain{
		Service:    authService,
		Middleware: auth.NewAuthenticationMiddleware(authService),
	}
	service := &fakeActivityService{}
	api := httpserver.NewHandlerGroup()
	activity.NewHTTPModule(authDomain, service).Mount(api)
	return api, service
}

func activityCookie() *http.Cookie {
	return &http.Cookie{Name: "todai_session", Value: "session-id"}
}

type fakeActivityService struct {
	userID string
	limit  int
	events []activity.Event
}

func (s *fakeActivityService) List(_ context.Context, userID string, limit int) ([]activity.Event, error) {
	s.userID = userID
	s.limit = limit
	return s.events, nil
}

type fakeActivityAuthRepository struct {
	user *auth.User
}

func (r fakeActivityAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (r fakeActivityAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (fakeActivityAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeActivityAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeActivityAuthRepository) Delete(context.Context, string) error { return nil }

type fakeActivitySessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeActivitySessionStorage) GetUserIdFromSessionId(_ context.Context, sessionID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID], nil
}

func (s *fakeActivitySessionStorage) CreateSessionForUser(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions["session-id"] = userID
	return "session-id", nil
}

func (s *fakeActivitySessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeActivitySessionStorage) DeleteSessionsByUserId(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}
	return nil
}
