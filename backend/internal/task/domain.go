package task

import "github.com/jmoiron/sqlx"

import "github.com/mishankov/todai/backend/internal/activity"

// Domain groups task persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the task domain.
func New(db *sqlx.DB, events *activity.Repository) *Domain {
	repository := NewRepository(db, events)
	return &Domain{
		Repository: repository,
		Service:    NewService(repository),
	}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}
