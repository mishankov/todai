package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	"golang.org/x/crypto/bcrypt"

	"github.com/mishankov/todai/backend/internal/httpapi"
)

func TestAPIDoesNotExposeRegistration(t *testing.T) {
	t.Parallel()

	handler := testAPI()
	request := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAPIMountsModules(t *testing.T) {
	t.Parallel()

	handler := httpapi.New(testAuthDomain(), fakeModule{})
	request := httptest.NewRequest(http.MethodGet, "/module/ping", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestLoginExposesCurrentUserAndLogoutClearsCookie(t *testing.T) {
	t.Parallel()

	handler := testAPIWithUser(t, "owner", "correct horse battery staple")
	loginResponse := serveJSON(
		t,
		handler,
		http.MethodPost,
		"/auth/login",
		map[string]string{"login": "owner", "password": "correct horse battery staple"},
		nil,
	)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResponse.Code, http.StatusOK)
	}

	sessionCookie := responseCookie(t, loginResponse, "todai_session")
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if !sessionCookie.Secure {
		t.Error("session cookie must be Secure")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %d, want %d", sessionCookie.SameSite, http.SameSiteLaxMode)
	}

	meResponse := serveJSON(t, handler, http.MethodGet, "/auth/me", nil, sessionCookie)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("current user status = %d, want %d", meResponse.Code, http.StatusOK)
	}
	var currentUser struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(meResponse.Body).Decode(&currentUser); err != nil {
		t.Fatalf("decode current user: %v", err)
	}
	if currentUser.Username != "owner" {
		t.Errorf("current user = %q, want %q", currentUser.Username, "owner")
	}

	logoutResponse := serveJSON(t, handler, http.MethodPost, "/auth/logout", nil, sessionCookie)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", logoutResponse.Code, http.StatusOK)
	}
	clearedCookie := responseCookie(t, logoutResponse, "todai_session")
	if clearedCookie.Value != "" || !clearedCookie.Expires.Before(time.Now()) {
		t.Errorf("logout cookie was not cleared: %#v", clearedCookie)
	}

}

func testAPI() http.Handler {
	return httpapi.New(testAuthDomain())
}

func testAuthDomain() *auth.Domain {
	service := auth.NewService(
		fakeAuthRepository{},
		&fakeSessionStorage{sessions: make(map[string]string)},
		"todai_session",
		nil,
		nil,
		nil,
	)
	domain := &auth.Domain{
		Service:    service,
		Middleware: auth.NewAuthenticationMiddleware(service),
	}

	return domain
}

func testAPIWithUser(t *testing.T, username, password string) http.Handler {
	t.Helper()

	salt := "test-salt"
	hash, err := bcrypt.GenerateFromPassword([]byte(password+":"+salt), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repository := fakeAuthRepository{user: &auth.User{
		ID:       "user-id",
		Username: username,
		Password: string(hash),
		Salt:     salt,
	}}
	storage := &fakeSessionStorage{sessions: make(map[string]string)}
	service := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	domain := &auth.Domain{
		Service:    service,
		Middleware: auth.NewAuthenticationMiddleware(service),
	}

	return httpapi.New(domain)
}

func serveJSON(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	request := httptest.NewRequest(method, path, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func responseCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("response did not set cookie %q", name)
	return nil
}

type fakeAuthRepository struct {
	user *auth.User
}

type fakeModule struct{}

func (fakeModule) Mount(api *httpserver.HandlerGroup) {
	api.HandleFunc("GET /module/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func (r fakeAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}

	return r.user, nil
}

func (r fakeAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}

	return r.user, nil
}

func (fakeAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeAuthRepository) Delete(context.Context, string) error { return nil }

type fakeSessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeSessionStorage) GetUserIdFromSessionId(_ context.Context, sessionID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sessions[sessionID], nil
}

func (s *fakeSessionStorage) CreateSessionForUser(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const sessionID = "session-id"
	s.sessions[sessionID] = userID
	return sessionID, nil
}

func (s *fakeSessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeSessionStorage) DeleteSessionsByUserId(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}

	return nil
}
