// Package httpapi constructs the backend HTTP API.
package httpapi

import (
	"net/http"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
)

// New constructs the HTTP API handler.
func New(authDomain *auth.Domain) http.Handler {
	api := httpserver.NewHandlerGroup()

	// Mount individual handlers so the personal MVP never exposes public registration.
	authAPI := httpserver.NewHandlerGroup()
	authAPI.Handle("POST /login", auth.NewLoginHandler(authDomain.Service))
	authAPI.Handle("POST /logout", auth.NewLogoutHandler(authDomain.Service))
	authAPI.Handle("GET /me", auth.NewGetHandler(authDomain.Service))
	authAPI.Handle(
		"POST /change-password",
		authDomain.Middleware.Wrap(auth.NewChangePasswordHandler(authDomain.Service)),
	)
	api.Mount("/auth", authAPI)

	protected := httpserver.NewHandlerGroup()
	protected.Use(authDomain.Middleware)
	protected.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		if err := httpserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	})
	api.Mount("/protected", protected)

	return api
}
