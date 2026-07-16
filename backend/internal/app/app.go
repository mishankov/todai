// Package app assembles the Todai backend from Platforma domains and services.
package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/platforma-dev/platforma/application"
	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/database"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/session"

	"github.com/mishankov/todai/backend/internal/config"
)

const databaseName = "main"

// Resources exposes the small set of dependencies needed by CLI commands.
type Resources struct {
	Database *database.Database
	Auth     *auth.Domain
}

// New constructs the backend application without starting it.
func New(cfg config.Config) (*application.Application, *Resources, error) {
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	productApp := application.New()
	productApp.RegisterDatabase(databaseName, db)

	sessionDomain := session.New(db.Connection())
	productApp.RegisterDomain("session", databaseName, sessionDomain)

	authDomain := auth.New(
		db.Connection(),
		sessionDomain.Service,
		cfg.SessionCookieName,
		nil,
		nil,
		nil,
	)
	productApp.RegisterDomain("auth", databaseName, authDomain)

	server := httpserver.New(cfg.HTTPPort, 5*time.Second)
	server.Handle("GET /health", application.NewHealthCheckHandler(productApp))
	server.Mount("/api", newAPI(authDomain))
	productApp.RegisterService("http", server)

	return productApp, &Resources{Database: db, Auth: authDomain}, nil
}

func newAPI(authDomain *auth.Domain) http.Handler {
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
