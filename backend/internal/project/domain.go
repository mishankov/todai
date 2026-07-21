package project

import (
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
)

// Domain groups project persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the project domain.
func New(
	db *sqlx.DB,
	events *activity.Repository,
	defaultAgentModel string,
	availableAgentModels []string,
) *Domain {
	repository := NewRepository(db, events)
	return &Domain{Repository: repository, Service: NewService(repository, ServiceConfig{
		DefaultAgentModel:          defaultAgentModel,
		AvailableAgentModels:       availableAgentModels,
		DefaultAgentThinkingEffort: "medium",
	})}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}
