package project

import "github.com/jmoiron/sqlx"

// Domain groups project persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the project domain.
func New(db *sqlx.DB) *Domain {
	repository := NewRepository(db)
	return &Domain{Repository: repository, Service: NewService(repository)}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}
