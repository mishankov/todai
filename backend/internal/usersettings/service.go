package usersettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishankov/todai/backend/internal/execution"
)

var (
	// ErrInvalidTimezone indicates that the preference is not an IANA timezone.
	ErrInvalidTimezone = errors.New("settings timezone is invalid")
	// ErrInvalidAgentModel indicates that the model is not enabled by the server.
	ErrInvalidAgentModel = errors.New("settings agent model is not available")
	// ErrInvalidVersion indicates that the caller did not provide an observed version.
	ErrInvalidVersion = errors.New("settings version must not be negative")
	// ErrVersionConflict indicates that settings changed after the caller loaded them.
	ErrVersionConflict = errors.New("settings version conflict")
)

type repository interface {
	Get(context.Context, string) (Settings, bool, error)
	Update(context.Context, execution.Scope, Update) (Settings, error)
}

// Service validates preferences and supplies agent defaults.
type Service struct {
	repository           repository
	defaultAgentModel    string
	availableAgentModels []string
}

// NewService constructs a user-settings service.
func NewService(repository repository, defaultAgentModel string, availableAgentModels []string) *Service {
	return &Service{
		repository: repository, defaultAgentModel: strings.TrimSpace(defaultAgentModel),
		availableAgentModels: append([]string(nil), availableAgentModels...),
	}
}

// Get returns persisted settings or unsaved defaults.
func (s *Service) Get(ctx context.Context, userID string) (View, error) {
	settings, found, err := s.repository.Get(ctx, userID)
	if err != nil {
		return View{}, fmt.Errorf("get user settings: %w", err)
	}
	if !found {
		settings = Settings{UserID: userID, AgentModel: s.defaultAgentModel}
	}
	return View{Settings: settings, AvailableAgentModels: append([]string(nil), s.availableAgentModels...)}, nil
}

// Update validates and persists all editable preferences.
func (s *Service) Update(ctx context.Context, scope execution.Scope, update Update) (View, error) {
	if err := scope.Validate(); err != nil {
		return View{}, err
	}
	if update.Version < 0 {
		return View{}, ErrInvalidVersion
	}
	update.Timezone = strings.TrimSpace(update.Timezone)
	if update.Timezone == "" {
		return View{}, ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(update.Timezone); err != nil {
		return View{}, ErrInvalidTimezone
	}
	update.AgentModel = strings.TrimSpace(update.AgentModel)
	if !containsModel(s.availableAgentModels, update.AgentModel) {
		return View{}, ErrInvalidAgentModel
	}

	updated, err := s.repository.Update(ctx, scope, update)
	if err != nil {
		return View{}, fmt.Errorf("update user settings: %w", err)
	}
	return View{Settings: updated, AvailableAgentModels: append([]string(nil), s.availableAgentModels...)}, nil
}

// ResolveAgent returns the timezone and model effective for a new agent run.
func (s *Service) ResolveAgent(ctx context.Context, userID string) (string, string, error) {
	view, err := s.Get(ctx, userID)
	if err != nil {
		return "", "", err
	}
	timezone := ""
	if view.Settings.Timezone != nil {
		timezone = *view.Settings.Timezone
	}
	return timezone, view.Settings.AgentModel, nil
}

func containsModel(models []string, wanted string) bool {
	for _, model := range models {
		if model == wanted {
			return true
		}
	}
	return false
}
