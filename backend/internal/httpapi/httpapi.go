// Package httpapi constructs the backend HTTP API.
package httpapi

import (
	"net/http"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
)

// Module owns a cohesive set of product API routes.
type Module interface {
	Mount(*httpserver.HandlerGroup)
}

// New constructs the HTTP API handler.
func New(authDomain *auth.Domain, modules ...Module) http.Handler {
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

	for _, module := range modules {
		module.Mount(api)
	}

	return api
}
