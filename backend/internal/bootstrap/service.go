// Package bootstrap creates the single user for a personal installation.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ErrAlreadyInitialized indicates that the installation already has a user.
var ErrAlreadyInitialized = errors.New("installation already has a user")

type userCounter interface {
	CountUsers(context.Context) (int, error)
}

type userCreator interface {
	CreateWithLoginAndPassword(context.Context, string, string) error
}

// Service enforces the single-user bootstrap policy.
type Service struct {
	counter userCounter
	creator userCreator
}

// New constructs a bootstrap service backed by PostgreSQL and Platforma auth.
func New(db *sqlx.DB, creator userCreator) *Service {
	return &Service{
		counter: &sqlUserCounter{db: db},
		creator: creator,
	}
}

func newService(counter userCounter, creator userCreator) *Service {
	return &Service{counter: counter, creator: creator}
}

// CreateUser creates the only user allowed by the personal MVP bootstrap flow.
func (s *Service) CreateUser(ctx context.Context, username, password string) error {
	count, err := s.counter.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count != 0 {
		return ErrAlreadyInitialized
	}

	if err := s.creator.CreateWithLoginAndPassword(ctx, username, password); err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}

	return nil
}

type sqlUserCounter struct {
	db *sqlx.DB
}

func (c *sqlUserCounter) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := c.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users"); err != nil {
		return 0, fmt.Errorf("query users table: %w", err)
	}

	return count, nil
}
