package agentauth

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Repository persists opaque agent token grants in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

// NewRepository constructs an agent token repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Migrations exposes agent authentication migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Create stores a token hash and its scoped claims.
func (r *Repository) Create(ctx context.Context, tokenHash []byte, claims Claims) error {
	tools := make([]string, len(claims.AllowedTools))
	for index, tool := range claims.AllowedTools {
		tools[index] = string(tool)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_tokens (
			token_hash, user_id, agent_session_id, agent_run_id, allowed_tools, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, tokenHash, claims.UserID, claims.AgentSessionID, claims.AgentRunID, pq.Array(tools), claims.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert agent token: %w", err)
	}

	return nil
}

// Get returns claims for a token hash.
func (r *Repository) Get(ctx context.Context, tokenHash []byte) (Claims, error) {
	var stored struct {
		UserID         string         `db:"user_id"`
		AgentSessionID string         `db:"agent_session_id"`
		AgentRunID     string         `db:"agent_run_id"`
		AllowedTools   pq.StringArray `db:"allowed_tools"`
		ExpiresAt      time.Time      `db:"expires_at"`
	}
	if err := r.db.GetContext(ctx, &stored, `
		SELECT user_id, agent_session_id, agent_run_id, allowed_tools, expires_at
		FROM agent_tokens
		WHERE token_hash = $1
	`, tokenHash); errors.Is(err, sql.ErrNoRows) {
		return Claims{}, ErrTokenUnknown
	} else if err != nil {
		return Claims{}, fmt.Errorf("select agent token: %w", err)
	}

	tools := make([]Tool, len(stored.AllowedTools))
	for index, tool := range stored.AllowedTools {
		tools[index] = Tool(tool)
	}

	return Claims{
		UserID:         stored.UserID,
		AgentSessionID: stored.AgentSessionID,
		AgentRunID:     stored.AgentRunID,
		AllowedTools:   tools,
		ExpiresAt:      stored.ExpiresAt,
	}, nil
}

// RevokeRun deletes all token grants for one user-owned run.
func (r *Repository) RevokeRun(ctx context.Context, userID, runID string) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM agent_tokens
		WHERE user_id = $1 AND agent_run_id = $2
	`, userID, runID); err != nil {
		return fmt.Errorf("delete agent run tokens: %w", err)
	}
	return nil
}
