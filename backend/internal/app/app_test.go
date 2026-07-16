package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platforma-dev/platforma/auth"
)

type fakeAuthRepository struct{}

func (fakeAuthRepository) Get(context.Context, string) (*auth.User, error) {
	return nil, sql.ErrNoRows
}

func (fakeAuthRepository) GetByUsername(context.Context, string) (*auth.User, error) {
	return nil, sql.ErrNoRows
}

func (fakeAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeAuthRepository) Delete(context.Context, string) error { return nil }

type fakeSessionStorage struct{}

func (fakeSessionStorage) GetUserIdFromSessionId(context.Context, string) (string, error) {
	return "", nil
}

func (fakeSessionStorage) CreateSessionForUser(context.Context, string) (string, error) {
	return "", nil
}

func (fakeSessionStorage) DeleteSession(context.Context, string) error { return nil }

func (fakeSessionStorage) DeleteSessionsByUserId(context.Context, string) error { return nil }

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

func TestProtectedEndpointRequiresSession(t *testing.T) {
	t.Parallel()

	handler := testAPI()
	request := httptest.NewRequest(http.MethodGet, "/protected/ping", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func testAPI() http.Handler {
	service := auth.NewService(fakeAuthRepository{}, fakeSessionStorage{}, "todai_session", nil, nil, nil)
	domain := &auth.Domain{
		Service:    service,
		Middleware: auth.NewAuthenticationMiddleware(service),
	}

	return newAPI(domain)
}
