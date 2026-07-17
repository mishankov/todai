package activity

import (
	"context"
	"fmt"
)

type repository interface {
	List(context.Context, string, int) ([]Event, error)
}

// Service provides user-scoped activity operations.
type Service struct {
	repository repository
}

// NewService constructs an activity service.
func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

// List returns the user's newest events first.
func (s *Service) List(ctx context.Context, userID string, limit int) ([]Event, error) {
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}
	events, err := s.repository.List(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list activity events: %w", err)
	}
	return events, nil
}
