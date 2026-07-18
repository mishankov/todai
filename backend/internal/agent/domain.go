package agent

import (
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/agentauth"
)

// Domain groups agent persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the built-in agent domain.
func New(
	db *sqlx.DB,
	events ActivityAppender,
	runtime Runtime,
	tokens *agentauth.Service,
	config ServiceConfig,
) *Domain {
	repository := NewRepository(db, events)
	return &Domain{Repository: repository, Service: NewService(repository, runtime, tokens, config)}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}

var _ ActivityAppender = (*activity.Repository)(nil)
