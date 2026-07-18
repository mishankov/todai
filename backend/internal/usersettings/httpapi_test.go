package usersettings_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	platformalog "github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/usersettings"
)

func TestSettingsHTTPUpdatesAuthenticatedUserPreferences(t *testing.T) {
	t.Parallel()

	service := &recordingSettingsHTTPService{scopes: make(chan execution.Scope, 1)}
	handler := settingsTestAPI(&auth.User{ID: "user-id", Username: "owner"}, service)
	request := settingsJSONRequest(t, http.MethodPatch, "/settings", map[string]any{
		"timezone": "Europe/Moscow", "agentModel": "gpt-fast", "version": 0,
	})
	request.AddCookie(&http.Cookie{Name: "todai_session", Value: "session-id"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	scope := <-service.scopes
	if err := scope.Validate(); err != nil || scope.UserID != "user-id" ||
		scope.ActorType != execution.ActorUser || scope.Source != execution.SourceWeb {
		t.Errorf("execution scope = %#v, error = %v", scope, err)
	}
	if service.update.Timezone != "Europe/Moscow" || service.update.AgentModel != "gpt-fast" {
		t.Errorf("update = %#v", service.update)
	}
}

func TestSettingsHTTPRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := settingsTestAPI(nil, &recordingSettingsHTTPService{})
	request := httptest.NewRequest(http.MethodGet, "/settings", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func settingsTestAPI(user *auth.User, service usersettings.HTTPService) http.Handler {
	repository := fakeSettingsAuthRepository{user: user}
	storage := &fakeSettingsSessionStorage{sessions: make(map[string]string)}
	if user != nil {
		storage.sessions["session-id"] = user.ID
	}
	authService := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	authDomain := &auth.Domain{
		Service: authService, Middleware: auth.NewAuthenticationMiddleware(authService),
	}
	api := httpserver.NewHandlerGroup()
	api.Use(platformalog.NewTraceIDMiddleware(nil, ""))
	usersettings.NewHTTPModule(authDomain, service).Mount(api)
	return api
}

func settingsJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	request := httptest.NewRequest(method, path, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	return request
}

type recordingSettingsHTTPService struct {
	scopes chan execution.Scope
	update usersettings.Update
}

func (s *recordingSettingsHTTPService) Get(context.Context, string) (usersettings.View, error) {
	return usersettings.View{}, nil
}

func (s *recordingSettingsHTTPService) Update(
	_ context.Context,
	scope execution.Scope,
	update usersettings.Update,
) (usersettings.View, error) {
	s.update = update
	if s.scopes != nil {
		s.scopes <- scope
	}
	timezone := update.Timezone
	return usersettings.View{Settings: usersettings.Settings{
		Timezone: &timezone, AgentModel: update.AgentModel, Version: update.Version + 1,
	}}, nil
}

type fakeSettingsAuthRepository struct {
	user *auth.User
}

func (r fakeSettingsAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (r fakeSettingsAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (fakeSettingsAuthRepository) Create(context.Context, *auth.User) error { return nil }
func (fakeSettingsAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}
func (fakeSettingsAuthRepository) Delete(context.Context, string) error { return nil }

type fakeSettingsSessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeSettingsSessionStorage) GetUserIdFromSessionId(
	_ context.Context,
	sessionID string,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID], nil
}

func (s *fakeSettingsSessionStorage) CreateSessionForUser(
	_ context.Context,
	userID string,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions["session-id"] = userID
	return "session-id", nil
}

func (s *fakeSettingsSessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeSettingsSessionStorage) DeleteSessionsByUserId(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}
	return nil
}
