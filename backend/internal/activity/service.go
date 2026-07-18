package activity

import (
	"context"
	"fmt"
)

type repository interface {
	List(context.Context, string, int) ([]Event, error)
	LatestOffset(context.Context, string) (int64, error)
	ListAfter(context.Context, string, int64, int) ([]Event, error)
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

// LatestOffset returns the newest durable event offset visible to a user.
func (s *Service) LatestOffset(ctx context.Context, userID string) (int64, error) {
	offset, err := s.repository.LatestOffset(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get latest activity offset: %w", err)
	}
	return offset, nil
}

// ListAfter returns the user's events after a durable stream cursor.
func (s *Service) ListAfter(
	ctx context.Context,
	userID string,
	after int64,
	limit int,
) ([]Event, error) {
	if after < 0 {
		return nil, ErrInvalidStreamCursor
	}
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}
	events, err := s.repository.ListAfter(ctx, userID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list activity stream events: %w", err)
	}
	return events, nil
}
