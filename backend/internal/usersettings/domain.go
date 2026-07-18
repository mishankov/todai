package usersettings

import (
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
)

// Domain groups user-settings persistence and application operations.
type Domain struct {
	Repository *Repository
	Service    *Service
}

// New constructs the user-settings domain.
func New(db *sqlx.DB, events activityAppender, defaultAgentModel string, availableAgentModels []string) *Domain {
	repository := NewRepository(db, events)
	return &Domain{
		Repository: repository,
		Service:    NewService(repository, defaultAgentModel, availableAgentModels),
	}
}

// GetRepository exposes the repository for Platforma migration registration.
func (d *Domain) GetRepository() any {
	return d.Repository
}

var _ activityAppender = (*activity.Repository)(nil)
