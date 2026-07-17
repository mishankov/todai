// Package app assembles the Todai backend from Platforma domains and services.
package app

import (
	"fmt"
	"time"

	"github.com/platforma-dev/platforma/application"
	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/database"
	"github.com/platforma-dev/platforma/httpserver"
	"github.com/platforma-dev/platforma/log"
	"github.com/platforma-dev/platforma/session"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/config"
	"github.com/mishankov/todai/backend/internal/httpapi"
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
	"github.com/mishankov/todai/backend/internal/tasktools"
)

const databaseName = "main"

// Resources exposes the small set of dependencies needed by CLI commands.
type Resources struct {
	Database    *database.Database
	Auth        *auth.Domain
	AgentTokens *agentauth.Service
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
	activityDomain := activity.New(db.Connection())
	productApp.RegisterDomain("activity", databaseName, activityDomain)
	agentAuthDomain := agentauth.New(db.Connection())
	productApp.RegisterDomain("agent_auth", databaseName, agentAuthDomain)
	taskDomain := task.New(db.Connection(), activityDomain.Repository)
	productApp.RegisterDomain("task", databaseName, taskDomain)
	projectDomain := project.New(db.Connection(), activityDomain.Repository)
	productApp.RegisterDomain("project", databaseName, projectDomain)

	server := httpserver.New(cfg.HTTPPort, 5*time.Second)
	server.Use(log.NewTraceIDMiddleware(nil, ""))
	server.Handle("GET /health", application.NewHealthCheckHandler(productApp))
	server.Mount(
		"/api",
		httpapi.New(
			authDomain,
			task.NewHTTPModule(authDomain, taskDomain.Service),
			project.NewHTTPModule(authDomain, projectDomain.Service),
			activity.NewHTTPModule(authDomain, activityDomain.Service),
		),
	)
	internalTools := httpserver.NewHandlerGroup()
	tasktools.NewHTTPModule(
		agentAuthDomain.Service, taskDomain.Service, projectDomain.Service,
	).Mount(internalTools)
	server.Mount("/internal/tools", internalTools)
	productApp.RegisterService("http", server)

	return productApp, &Resources{
		Database: db, Auth: authDomain, AgentTokens: agentAuthDomain.Service,
	}, nil
}
