package agentauth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/execution"
)

func TestIssueStoresOnlyHashAndReturnsRawTokenOnce(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := agentauth.NewService(repository)
	request := validIssueRequest()
	before := time.Now().UTC()

	issued, err := service.Issue(context.Background(), request)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if issued.Token == "" {
		t.Fatal("issued token is empty")
	}
	after := time.Now().UTC()
	if issued.ExpiresAt.Before(before.Add(request.TTL)) || issued.ExpiresAt.After(after.Add(request.TTL)) {
		t.Errorf("expires at = %v, want between %v and %v", issued.ExpiresAt, before.Add(request.TTL), after.Add(request.TTL))
	}
	wantHash := sha256.Sum256([]byte(issued.Token))
	if !bytes.Equal(repository.createdHash, wantHash[:]) {
		t.Errorf("stored hash = %x, want %x", repository.createdHash, wantHash)
	}
	if len(repository.createdHash) != sha256.Size {
		t.Errorf("stored hash length = %d, want %d", len(repository.createdHash), sha256.Size)
	}
	if repository.createdClaims.UserID != request.UserID ||
		repository.createdClaims.AgentSessionID != request.AgentSessionID ||
		repository.createdClaims.AgentRunID != request.AgentRunID {
		t.Errorf("stored claims = %#v", repository.createdClaims)
	}
	if len(repository.createdClaims.AllowedTools) != 2 {
		t.Errorf("stored tools = %#v, want duplicate removed", repository.createdClaims.AllowedTools)
	}

	second, err := service.Issue(context.Background(), request)
	if err != nil {
		t.Fatalf("issue second token: %v", err)
	}
	if second.Token == issued.Token {
		t.Error("two issued tokens are equal")
	}
}

func TestIssueRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request agentauth.IssueRequest
		want    error
	}{
		{name: "user", request: issueRequestWithoutUser(), want: agentauth.ErrUserIDRequired},
		{name: "session", request: issueRequestWithoutSession(), want: agentauth.ErrAgentSessionIDRequired},
		{name: "run", request: issueRequestWithoutRun(), want: agentauth.ErrAgentRunIDRequired},
		{name: "tools", request: issueRequestWithoutTools(), want: agentauth.ErrAllowedToolsRequired},
		{name: "unknown tool", request: issueRequestWithTools(agentauth.Tool("shell")), want: agentauth.ErrInvalidTool},
		{name: "zero TTL", request: issueRequestWithTTL(0), want: agentauth.ErrInvalidTTL},
		{name: "negative TTL", request: issueRequestWithTTL(-time.Second), want: agentauth.ErrInvalidTTL},
		{name: "long TTL", request: issueRequestWithTTL(15*time.Minute + time.Nanosecond), want: agentauth.ErrInvalidTTL},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := newFakeRepository()
			_, err := agentauth.NewService(repository).Issue(context.Background(), test.request)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if repository.createCalls != 0 {
				t.Errorf("repository create calls = %d, want 0", repository.createCalls)
			}
		})
	}
}

func TestAuthenticateReturnsClaimsForAllowedTool(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := agentauth.NewService(repository)
	issued, err := service.Issue(context.Background(), validIssueRequest())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := service.Authenticate(context.Background(), issued.Token, agentauth.ToolTaskGet)
	if err != nil {
		t.Fatalf("authenticate token: %v", err)
	}
	if claims.UserID != "user-id" || claims.AgentSessionID != "session-id" || claims.AgentRunID != "run-id" {
		t.Errorf("claims = %#v", claims)
	}
	claims.AllowedTools[0] = agentauth.ToolTaskMove
	second, err := service.Authenticate(context.Background(), issued.Token, agentauth.ToolTaskGet)
	if err != nil {
		t.Fatalf("authenticate token again: %v", err)
	}
	if second.AllowedTools[0] != agentauth.ToolTaskGet {
		t.Errorf("stored permissions were mutated: %#v", second.AllowedTools)
	}
}

func TestAuthenticateRejectsUnknownExpiredAndInsufficientTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawToken string
		required agentauth.Tool
		claims   *agentauth.Claims
		want     error
	}{
		{name: "empty", required: agentauth.ToolTaskGet, want: agentauth.ErrTokenRequired},
		{name: "unknown", rawToken: "unknown", required: agentauth.ToolTaskGet, want: agentauth.ErrTokenUnknown},
		{
			name: "expired", rawToken: "expired", required: agentauth.ToolTaskGet,
			claims: claimsWithExpiry(time.Now().Add(-time.Second)), want: agentauth.ErrTokenExpired,
		},
		{
			name: "tool mismatch", rawToken: "limited", required: agentauth.ToolTaskUpdate,
			claims: claimsWithExpiry(time.Now().Add(time.Minute)), want: agentauth.ErrToolNotAllowed,
		},
		{
			name: "invalid required tool", rawToken: "limited", required: agentauth.Tool("shell"),
			claims: claimsWithExpiry(time.Now().Add(time.Minute)), want: agentauth.ErrInvalidTool,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := newFakeRepository()
			if test.claims != nil {
				repository.put(test.rawToken, *test.claims)
			}
			_, err := agentauth.NewService(repository).Authenticate(
				context.Background(), test.rawToken, test.required,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestClaimsBuildBuiltInAgentExecutionScope(t *testing.T) {
	t.Parallel()

	claims := *claimsWithExpiry(time.Now().Add(time.Minute))
	scope := claims.ExecutionScope("correlation-id")
	if err := scope.Validate(); err != nil {
		t.Fatalf("validate execution scope: %v", err)
	}
	if scope.UserID != claims.UserID || scope.ActorType != execution.ActorBuiltInAgent ||
		scope.ActorID == nil || *scope.ActorID != claims.AgentSessionID ||
		scope.Source != execution.SourceInternalAPI ||
		scope.AgentRunID == nil || *scope.AgentRunID != claims.AgentRunID {
		t.Errorf("execution scope = %#v", scope)
	}
}

func TestRepositoryExposesAgentTokenMigration(t *testing.T) {
	t.Parallel()

	migrations := agentauth.NewRepository(nil).Migrations()
	contents, err := fs.ReadFile(migrations, "1784291000_create_agent_tokens.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if !bytes.Contains(contents, []byte("CREATE TABLE agent_tokens")) {
		t.Error("migration does not create agent_tokens")
	}
}

type fakeRepository struct {
	createdHash   []byte
	createdClaims agentauth.Claims
	createCalls   int
	byHash        map[[sha256.Size]byte]agentauth.Claims
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{byHash: make(map[[sha256.Size]byte]agentauth.Claims)}
}

func (r *fakeRepository) Create(_ context.Context, tokenHash []byte, claims agentauth.Claims) error {
	r.createCalls++
	r.createdHash = append([]byte(nil), tokenHash...)
	r.createdClaims = cloneClaims(claims)
	var key [sha256.Size]byte
	copy(key[:], tokenHash)
	r.byHash[key] = cloneClaims(claims)
	return nil
}

func (r *fakeRepository) Get(_ context.Context, tokenHash []byte) (agentauth.Claims, error) {
	var key [sha256.Size]byte
	copy(key[:], tokenHash)
	claims, ok := r.byHash[key]
	if !ok {
		return agentauth.Claims{}, agentauth.ErrTokenUnknown
	}
	return cloneClaims(claims), nil
}

func (r *fakeRepository) put(rawToken string, claims agentauth.Claims) {
	hash := sha256.Sum256([]byte(rawToken))
	r.byHash[hash] = cloneClaims(claims)
}

func cloneClaims(claims agentauth.Claims) agentauth.Claims {
	claims.AllowedTools = append([]agentauth.Tool(nil), claims.AllowedTools...)
	return claims
}

func validIssueRequest() agentauth.IssueRequest {
	return agentauth.IssueRequest{
		UserID:         "user-id",
		AgentSessionID: "session-id",
		AgentRunID:     "run-id",
		AllowedTools: []agentauth.Tool{
			agentauth.ToolTaskGet,
			agentauth.ToolTaskUpdate,
			agentauth.ToolTaskGet,
		},
		TTL: time.Minute,
	}
}

func issueRequestWithoutUser() agentauth.IssueRequest {
	request := validIssueRequest()
	request.UserID = " "
	return request
}

func issueRequestWithoutSession() agentauth.IssueRequest {
	request := validIssueRequest()
	request.AgentSessionID = " "
	return request
}

func issueRequestWithoutRun() agentauth.IssueRequest {
	request := validIssueRequest()
	request.AgentRunID = " "
	return request
}

func issueRequestWithoutTools() agentauth.IssueRequest {
	request := validIssueRequest()
	request.AllowedTools = nil
	return request
}

func issueRequestWithTools(tools ...agentauth.Tool) agentauth.IssueRequest {
	request := validIssueRequest()
	request.AllowedTools = tools
	return request
}

func issueRequestWithTTL(ttl time.Duration) agentauth.IssueRequest {
	request := validIssueRequest()
	request.TTL = ttl
	return request
}

func claimsWithExpiry(expiresAt time.Time) *agentauth.Claims {
	return &agentauth.Claims{
		UserID:         "user-id",
		AgentSessionID: "session-id",
		AgentRunID:     "run-id",
		AllowedTools:   []agentauth.Tool{agentauth.ToolTaskGet},
		ExpiresAt:      expiresAt,
	}
}
