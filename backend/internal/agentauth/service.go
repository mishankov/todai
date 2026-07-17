package agentauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxTokenTTL   = 15 * time.Minute
	rawTokenBytes = 32
)

var (
	// ErrUserIDRequired indicates that the token has no user authorization scope.
	ErrUserIDRequired = errors.New("agent token user ID is required")
	// ErrAgentSessionIDRequired indicates that the token is not scoped to an agent session.
	ErrAgentSessionIDRequired = errors.New("agent token session ID is required")
	// ErrAgentRunIDRequired indicates that the token is not scoped to an agent run.
	ErrAgentRunIDRequired = errors.New("agent token run ID is required")
	// ErrAllowedToolsRequired indicates that the token has no permissions.
	ErrAllowedToolsRequired = errors.New("agent token allowed tools are required")
	// ErrInvalidTool indicates an unsupported tool name.
	ErrInvalidTool = errors.New("agent token tool is invalid")
	// ErrInvalidTTL indicates a non-positive or overly long token lifetime.
	ErrInvalidTTL = errors.New("agent token TTL must be between 1ns and 15m")
	// ErrTokenRequired indicates that no raw token was supplied.
	ErrTokenRequired = errors.New("agent token is required")
	// ErrTokenUnknown indicates that no stored grant matches the token.
	ErrTokenUnknown = errors.New("agent token is unknown")
	// ErrTokenExpired indicates that the token grant is no longer active.
	ErrTokenExpired = errors.New("agent token is expired")
	// ErrToolNotAllowed indicates that the token does not grant the requested tool.
	ErrToolNotAllowed = errors.New("agent token does not allow the required tool")
)

type tokenRepository interface {
	Create(context.Context, []byte, Claims) error
	Get(context.Context, []byte) (Claims, error)
}

// Service issues and authenticates short-lived scoped agent tokens.
type Service struct {
	repository tokenRepository
}

// NewService constructs an agent token service.
func NewService(repository tokenRepository) *Service {
	return &Service{repository: repository}
}

// Issue creates a random opaque token and persists only its SHA-256 hash.
func (s *Service) Issue(ctx context.Context, request IssueRequest) (IssuedToken, error) {
	claims, err := validateIssueRequest(request)
	if err != nil {
		return IssuedToken{}, err
	}

	raw := make([]byte, rawTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return IssuedToken{}, fmt.Errorf("generate agent token: %w", err)
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(rawToken))
	claims.ExpiresAt = time.Now().UTC().Add(request.TTL)

	if err := s.repository.Create(ctx, hash[:], claims); err != nil {
		return IssuedToken{}, fmt.Errorf("store agent token: %w", err)
	}

	return IssuedToken{Token: rawToken, ExpiresAt: claims.ExpiresAt}, nil
}

// Authenticate verifies a raw token and its permission for one required tool.
func (s *Service) Authenticate(ctx context.Context, rawToken string, required Tool) (Claims, error) {
	if strings.TrimSpace(rawToken) == "" {
		return Claims{}, ErrTokenRequired
	}
	if !validTool(required) {
		return Claims{}, ErrInvalidTool
	}

	hash := sha256.Sum256([]byte(rawToken))
	claims, err := s.repository.Get(ctx, hash[:])
	if err != nil {
		return Claims{}, fmt.Errorf("get agent token: %w", err)
	}
	if !time.Now().UTC().Before(claims.ExpiresAt) {
		return Claims{}, ErrTokenExpired
	}
	if !containsTool(claims.AllowedTools, required) {
		return Claims{}, ErrToolNotAllowed
	}

	claims.AllowedTools = append([]Tool(nil), claims.AllowedTools...)
	return claims, nil
}

func validateIssueRequest(request IssueRequest) (Claims, error) {
	userID := strings.TrimSpace(request.UserID)
	if userID == "" {
		return Claims{}, ErrUserIDRequired
	}
	sessionID := strings.TrimSpace(request.AgentSessionID)
	if sessionID == "" {
		return Claims{}, ErrAgentSessionIDRequired
	}
	runID := strings.TrimSpace(request.AgentRunID)
	if runID == "" {
		return Claims{}, ErrAgentRunIDRequired
	}
	if request.TTL <= 0 || request.TTL > maxTokenTTL {
		return Claims{}, ErrInvalidTTL
	}
	tools, err := normalizeTools(request.AllowedTools)
	if err != nil {
		return Claims{}, err
	}

	return Claims{
		UserID: userID, AgentSessionID: sessionID, AgentRunID: runID, AllowedTools: tools,
	}, nil
}

func normalizeTools(tools []Tool) ([]Tool, error) {
	if len(tools) == 0 {
		return nil, ErrAllowedToolsRequired
	}

	normalized := make([]Tool, 0, len(tools))
	seen := make(map[Tool]struct{}, len(tools))
	for _, tool := range tools {
		if !validTool(tool) {
			return nil, ErrInvalidTool
		}
		if _, exists := seen[tool]; exists {
			continue
		}
		seen[tool] = struct{}{}
		normalized = append(normalized, tool)
	}

	return normalized, nil
}

func validTool(tool Tool) bool {
	switch tool {
	case ToolTaskGet, ToolTaskSearch, ToolProjectList, ToolViewQuery,
		ToolTaskCreate, ToolTaskUpdate, ToolTaskComplete, ToolTaskReopen,
		ToolTaskMove, ToolTaskReorder:
		return true
	default:
		return false
	}
}

func containsTool(tools []Tool, required Tool) bool {
	for _, tool := range tools {
		if tool == required {
			return true
		}
	}

	return false
}
