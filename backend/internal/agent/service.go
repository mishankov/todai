package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/execution"
)

const eventPersistenceTimeout = 5 * time.Second

var (
	// ErrServiceStopped indicates that the application is shutting down.
	ErrServiceStopped = errors.New("agent service is stopped")
	// ErrMessageContextNotFound avoids starting a run for an inaccessible product resource.
	ErrMessageContextNotFound = errors.New("agent message context not found")
	// ErrContextValidatorRequired prevents unverified structured context from reaching a runtime.
	ErrContextValidatorRequired = errors.New("agent message context validator is not configured")
)

type repository interface {
	CreateSession(context.Context, execution.Scope) (Session, error)
	GetConversation(context.Context, string, string) (Session, []Message, []Run, int64, error)
	CreateMessageRun(context.Context, execution.Scope, string, MessageInput) (Message, Run, []HistoryMessage, error)
	CreateContextRun(context.Context, execution.Scope, MessageContext) (Run, error)
	GetRun(context.Context, string, string) (Run, error)
	ListRunEvents(context.Context, string, string, int64, int) ([]RunEvent, error)
	ListContextRunEvents(context.Context, string, string, int64, int) ([]RunEvent, error)
	RecordRuntimeEvent(context.Context, execution.Scope, string, RuntimeEvent) (RunEvent, error)
	FinishRun(context.Context, execution.Scope, string, string, any) (Run, error)
}

type tokenIssuer interface {
	Issue(context.Context, agentauth.IssueRequest) (agentauth.IssuedToken, error)
	RevokeRun(context.Context, string, string) error
}

// PreferencesResolver supplies the effective user preferences for a new run.
type PreferencesResolver interface {
	ResolveAgent(context.Context, string) (
		timezone string, model string, thinkingEffort string, err error,
	)
}

// ContextValidator verifies that a structured product resource belongs to the current user.
type ContextValidator interface {
	ValidateMessageContext(context.Context, string, MessageContext) error
}

// ServiceConfig controls the isolated runtime without exposing credentials globally.
type ServiceConfig struct {
	Runtime          string
	InternalURL      string
	TokenTTL         time.Duration
	AllowedTools     []agentauth.Tool
	AgentDir         string
	Provider         string
	Model            string
	ThinkingEffort   string
	Preferences      PreferencesResolver
	ContextValidator ContextValidator
}

// Conversation contains a session and its canonical messages and runs.
type Conversation struct {
	Session          Session   `json:"session"`
	Messages         []Message `json:"messages"`
	Runs             []Run     `json:"runs"`
	LastStreamOffset int64     `json:"lastStreamOffset"`
}

// PostedMessage contains the accepted user message and newly queued run.
type PostedMessage struct {
	Message Message `json:"message"`
	Run     Run     `json:"run"`
}

type activeRun struct {
	cancel context.CancelFunc
}

// Service coordinates durable agent state with asynchronous runtime execution.
type Service struct {
	repository repository
	runtime    Runtime
	tokens     tokenIssuer
	config     ServiceConfig

	mu       sync.Mutex
	root     context.Context
	active   map[string]activeRun
	stopped  bool
	waitRuns sync.WaitGroup
}

// NewService constructs an agent application service.
func NewService(repository repository, runtime Runtime, tokens tokenIssuer, config ServiceConfig) *Service {
	return &Service{
		repository: repository,
		runtime:    runtime,
		tokens:     tokens,
		config:     config,
		root:       context.Background(),
		active:     make(map[string]activeRun),
	}
}

// Run owns the application root context and gracefully cancels active runs on shutdown.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	s.root = ctx
	s.mu.Unlock()

	<-ctx.Done()
	s.mu.Lock()
	s.stopped = true
	for _, active := range s.active {
		active.cancel()
	}
	s.mu.Unlock()
	s.waitRuns.Wait()
	return nil
}

// CreateSession starts a new user-owned conversation.
func (s *Service) CreateSession(ctx context.Context, scope execution.Scope) (Session, error) {
	created, err := s.repository.CreateSession(ctx, scope)
	if err != nil {
		return Session{}, fmt.Errorf("create agent session: %w", err)
	}
	return created, nil
}

// GetSession returns canonical conversation state scoped to its user.
func (s *Service) GetSession(ctx context.Context, userID, sessionID string) (Conversation, error) {
	session, messages, runs, lastStreamOffset, err := s.repository.GetConversation(ctx, userID, sessionID)
	if err != nil {
		return Conversation{}, fmt.Errorf("get agent session: %w", err)
	}
	return Conversation{
		Session: session, Messages: messages, Runs: runs, LastStreamOffset: lastStreamOffset,
	}, nil
}

// StartContextRun starts an isolated action that cannot read or append to chat history.
func (s *Service) StartContextRun(
	ctx context.Context,
	scope execution.Scope,
	messageContext MessageContext,
) (Run, error) {
	if err := s.validateRunContext(ctx, scope.UserID, &messageContext); err != nil {
		return Run{}, err
	}
	timezone, model, thinkingEffort, err := s.resolvePreferences(ctx, scope.UserID)
	if err != nil {
		return Run{}, err
	}
	run, err := s.repository.CreateContextRun(ctx, scope, messageContext)
	if err != nil {
		return Run{}, fmt.Errorf("queue contextual agent run: %w", err)
	}
	if err := s.launchRun(
		ctx, run, contextPrompt(messageContext), &messageContext, nil,
		timezone, model, thinkingEffort,
	); err != nil {
		return Run{}, err
	}
	return run, nil
}

// PostMessage durably queues a run and starts it asynchronously.
func (s *Service) PostMessage(
	ctx context.Context,
	scope execution.Scope,
	sessionID string,
	input MessageInput,
) (PostedMessage, error) {
	if s.runtime == nil {
		return PostedMessage{}, errors.New("agent runtime is not configured")
	}
	timezone, model, thinkingEffort, err := s.resolvePreferences(ctx, scope.UserID)
	if err != nil {
		return PostedMessage{}, err
	}
	message, run, history, err := s.repository.CreateMessageRun(ctx, scope, sessionID, input)
	if err != nil {
		return PostedMessage{}, fmt.Errorf("queue agent message: %w", err)
	}
	if err := s.launchRun(
		ctx, run, message.Content, nil, history,
		timezone, model, thinkingEffort,
	); err != nil {
		return PostedMessage{}, err
	}
	return PostedMessage{Message: message, Run: run}, nil
}

func (s *Service) launchRun(
	ctx context.Context,
	run Run,
	message string,
	messageContext *MessageContext,
	history []HistoryMessage,
	timezone string,
	model string,
	thinkingEffort string,
) error {
	if s.tokens == nil {
		return errors.New("agent token issuer is not configured")
	}
	issued, err := s.tokens.Issue(ctx, agentauth.IssueRequest{
		UserID: run.UserID, AgentSessionID: run.SessionID, AgentRunID: run.ID,
		AllowedTools: s.config.AllowedTools, TTL: s.config.TokenTTL,
	})
	if err != nil {
		finishScope := runtimeScope(run)
		_, _ = s.repository.FinishRun(ctx, finishScope, run.ID, EventRunFailed, map[string]any{
			"error": "issue scoped agent token",
		})
		return fmt.Errorf("issue scoped agent token: %w", err)
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		_ = s.tokens.RevokeRun(ctx, run.UserID, run.ID)
		finishScope := runtimeScope(run)
		_, _ = s.repository.FinishRun(ctx, finishScope, run.ID, EventRunFailed, map[string]any{
			"error": ErrServiceStopped.Error(),
		})
		return ErrServiceStopped
	}
	runCtx, cancel := context.WithCancel(s.root)
	s.active[run.ID] = activeRun{cancel: cancel}
	s.waitRuns.Add(1)
	s.mu.Unlock()

	go s.execute(
		runCtx, run, message, messageContext, history, issued.Token,
		timezone, model, thinkingEffort,
	)
	return nil
}

func (s *Service) validateRunContext(
	ctx context.Context,
	userID string,
	messageContext *MessageContext,
) error {
	if s.runtime == nil {
		return errors.New("agent runtime is not configured")
	}
	if messageContext == nil {
		return nil
	}
	if err := messageContext.Validate(); err != nil {
		return err
	}
	if s.config.ContextValidator == nil {
		return ErrContextValidatorRequired
	}
	if err := s.config.ContextValidator.ValidateMessageContext(ctx, userID, *messageContext); err != nil {
		return fmt.Errorf("validate agent run context: %w", err)
	}
	return nil
}

func (s *Service) resolvePreferences(
	ctx context.Context,
	userID string,
) (string, string, string, error) {
	timezone := ""
	model := s.config.Model
	thinkingEffort := s.config.ThinkingEffort
	if s.config.Preferences == nil {
		return timezone, model, thinkingEffort, nil
	}
	timezone, model, thinkingEffort, err := s.config.Preferences.ResolveAgent(ctx, userID)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve agent preferences: %w", err)
	}
	return timezone, model, thinkingEffort, nil
}

func contextPrompt(messageContext MessageContext) string {
	switch messageContext.Action {
	case ContextActionDecompose:
		return "Decompose the attached task into clear actionable direct subtasks."
	default:
		return "Perform the attached task action."
	}
}

// ListEvents returns persisted events after a durable SSE cursor.
func (s *Service) ListEvents(
	ctx context.Context,
	userID string,
	sessionID string,
	after int64,
	limit int,
) ([]RunEvent, error) {
	events, err := s.repository.ListRunEvents(ctx, userID, sessionID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent events: %w", err)
	}
	return events, nil
}

// ListContextRunEvents returns persisted events for one isolated contextual run.
func (s *Service) ListContextRunEvents(
	ctx context.Context,
	userID string,
	runID string,
	after int64,
	limit int,
) ([]RunEvent, error) {
	events, err := s.repository.ListContextRunEvents(ctx, userID, runID, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list contextual agent run events: %w", err)
	}
	return events, nil
}

// Abort cancels an active run and durably records a single terminal event.
func (s *Service) Abort(
	ctx context.Context,
	scope execution.Scope,
	runID string,
) (Run, error) {
	_, err := s.repository.GetRun(ctx, scope.UserID, runID)
	if err != nil {
		return Run{}, fmt.Errorf("get agent run for abort: %w", err)
	}

	s.mu.Lock()
	active, ok := s.active[runID]
	s.mu.Unlock()
	if ok {
		active.cancel()
	}
	finished, err := s.repository.FinishRun(ctx, scope, runID, EventRunAborted, map[string]any{
		"reason": "user_requested",
	})
	if err != nil {
		return Run{}, fmt.Errorf("abort agent run: %w", err)
	}
	return finished, nil
}

func (s *Service) execute(
	ctx context.Context,
	run Run,
	message string,
	messageContext *MessageContext,
	history []HistoryMessage,
	accessToken string,
	timezone string,
	model string,
	thinkingEffort string,
) {
	defer s.waitRuns.Done()
	defer func() {
		revokeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventPersistenceTimeout)
		defer cancel()
		if err := s.tokens.RevokeRun(revokeCtx, run.UserID, run.ID); err != nil {
			log.ErrorContext(revokeCtx, "revoke agent run token", "run_id", run.ID, "error", err)
		}
	}()
	defer func() {
		s.mu.Lock()
		delete(s.active, run.ID)
		s.mu.Unlock()
	}()

	scope := runtimeScope(run)
	allowedTools := make([]string, len(s.config.AllowedTools))
	for index, tool := range s.config.AllowedTools {
		allowedTools[index] = string(tool)
	}
	err := s.runtime.Run(ctx, RunRequest{
		UserID: run.UserID, SessionID: run.SessionID, RunID: run.ID, Message: message,
		Context: messageContext, History: history,
		Runtime: s.config.Runtime, InternalURL: s.config.InternalURL, AccessToken: accessToken,
		AllowedTools: allowedTools, AgentDir: s.config.AgentDir,
		Provider: s.config.Provider, Model: model, Timezone: timezone,
		ThinkingEffort: thinkingEffort,
	}, func(eventCtx context.Context, event RuntimeEvent) error {
		if eventCtx == nil {
			eventCtx = ctx
		}
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(eventCtx), eventPersistenceTimeout)
		defer cancel()
		_, err := s.repository.RecordRuntimeEvent(persistCtx, scope, run.ID, event)
		return err
	})

	eventType := EventRunCompleted
	payload := map[string]any{}
	if ctx.Err() != nil {
		eventType = EventRunAborted
		payload["reason"] = "cancelled"
	} else if err != nil && !errors.Is(err, ErrRunFinished) {
		eventType = EventRunFailed
		payload["error"] = err.Error()
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), eventPersistenceTimeout)
	defer cancel()
	if _, finishErr := s.repository.FinishRun(persistCtx, scope, run.ID, eventType, payload); finishErr != nil {
		log.ErrorContext(persistCtx, "finish agent run", "run_id", run.ID, "error", finishErr)
	}
}

func runtimeScope(run Run) execution.Scope {
	actorID := run.SessionID
	runID := run.ID
	return execution.Scope{
		UserID:        run.UserID,
		ActorType:     execution.ActorBuiltInAgent,
		ActorID:       &actorID,
		Source:        execution.SourceSystem,
		CorrelationID: run.CorrelationID,
		AgentRunID:    &runID,
	}
}
