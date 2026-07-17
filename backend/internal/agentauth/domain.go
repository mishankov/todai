// Package agentauth issues and authenticates short-lived tokens for built-in agent tools.
package agentauth

import "github.com/jmoiron/sqlx"

// Domain groups agent token persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the agent authentication domain.
func New(db *sqlx.DB) *Domain {
	repository := NewRepository(db)
	return &Domain{Repository: repository, Service: NewService(repository)}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}
