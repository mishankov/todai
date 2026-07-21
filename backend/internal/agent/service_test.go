package agent_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/execution"
)

func TestPostMessageRunsRuntimeAndPersistsStableEvents(t *testing.T) {
	repository := newFakeRepository()
	repository.history = []agent.HistoryMessage{{
		Role: agent.HistoryRoleUser, Content: []agent.HistoryContent{{Type: "text", Text: "Earlier"}}, Timestamp: 1,
	}}
	runtime := runtimeFunc(func(
		ctx context.Context,
		request agent.RunRequest,
		emit func(context.Context, agent.RuntimeEvent) error,
	) error {
		if request.UserID != "user-id" || request.SessionID != "session-id" ||
			request.RunID != "run-id" || request.Message != "Triage inbox" || len(request.History) != 1 {
			t.Errorf("run request = %#v", request)
		}
		for _, event := range []agent.RuntimeEvent{
			{Type: agent.EventRunStarted, Sequence: 1, Payload: map[string]any{}},
			{Type: agent.EventMessageDelta, Sequence: 2, Payload: map[string]any{"delta": "Done"}},
			{Type: agent.EventRunCompleted, Sequence: 3, Payload: map[string]any{}},
		} {
			if err := emit(ctx, event); err != nil {
				return err
			}
		}
		return nil
	})
	service := newService(repository, runtime)

	posted, err := service.PostMessage(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"project-id", "session-id", agent.MessageInput{Content: " Triage inbox "},
	)
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	if posted.Message.ID != "message-id" || posted.Run.ID != "run-id" {
		t.Errorf("posted = %#v", posted)
	}
	waitForFinish(t, repository)

	events := repository.runtimeEvents()
	if len(events) != 3 {
		t.Fatalf("runtime event count = %d, want 3", len(events))
	}
	for index, event := range events {
		if event.Sequence != int64(index+1) {
			t.Errorf("event %d sequence = %d", index, event.Sequence)
		}
	}
}

func TestPostMessageRequiresAccessibleProject(t *testing.T) {
	repository := newFakeRepository()
	runtimeCalled := false
	service := agent.NewService(
		repository,
		runtimeFunc(func(
			context.Context,
			agent.RunRequest,
			func(context.Context, agent.RuntimeEvent) error,
		) error {
			runtimeCalled = true
			return nil
		}),
		tokenIssuerFunc(func(
			context.Context,
			agentauth.IssueRequest,
		) (agentauth.IssuedToken, error) {
			return agentauth.IssuedToken{Token: "scoped-token"}, nil
		}),
		agent.ServiceConfig{ProjectValidator: projectValidatorFunc(func(
			context.Context,
			string,
			string,
		) error {
			return agent.ErrProjectNotFound
		})},
	)

	_, err := service.PostMessage(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"other-project", "session-id", agent.MessageInput{Content: "Run"},
	)
	if !errors.Is(err, agent.ErrProjectNotFound) {
		t.Fatalf("error = %v, want %v", err, agent.ErrProjectNotFound)
	}
	if runtimeCalled {
		t.Fatal("runtime was called for an inaccessible project")
	}
}

func TestStartContextRunUsesPrivateExecutionWithEmptyHistory(t *testing.T) {
	repository := newFakeRepository()
	repository.history = []agent.HistoryMessage{{
		Role: agent.HistoryRoleUser, Content: []agent.HistoryContent{{Type: "text", Text: "Chat history"}}, Timestamp: 1,
	}}
	contextReceived := make(chan agent.MessageContext, 1)
	runtimeRequest := make(chan agent.RunRequest, 1)
	service := agent.NewService(
		repository,
		runtimeFunc(func(
			_ context.Context,
			request agent.RunRequest,
			_ func(context.Context, agent.RuntimeEvent) error,
		) error {
			runtimeRequest <- request
			return nil
		}),
		tokenIssuerFunc(func(
			context.Context, agentauth.IssueRequest,
		) (agentauth.IssuedToken, error) {
			return agentauth.IssuedToken{Token: "scoped-token"}, nil
		}),
		agent.ServiceConfig{
			Runtime: "pi", InternalURL: "http://127.0.0.1:8080", TokenTTL: time.Minute,
			ContextValidator: contextValidatorFunc(func(
				_ context.Context, userID, projectID string, messageContext agent.MessageContext,
			) error {
				if userID != "user-id" || projectID != "project-id" {
					t.Errorf("context scope = (%q, %q)", userID, projectID)
				}
				contextReceived <- messageContext
				return nil
			}),
			ProjectValidator: projectValidatorFunc(func(context.Context, string, string) error {
				return nil
			}),
		},
	)
	wantContext := agent.MessageContext{
		Type: agent.ContextTask, TaskID: "11111111-1111-4111-8111-111111111111",
		Action: agent.ContextActionDecompose,
	}

	started, err := service.StartContextRun(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"project-id", wantContext,
	)
	if err != nil {
		t.Fatalf("start contextual run: %v", err)
	}
	if got := <-contextReceived; got != wantContext {
		t.Errorf("validated context = %#v, want %#v", got, wantContext)
	}
	request := <-runtimeRequest
	if request.SessionID != "private-execution-id" || request.RunID != started.ID ||
		request.Message != "Decompose the attached task into clear actionable direct subtasks." ||
		len(request.History) != 0 {
		t.Errorf("contextual runtime request = %#v", request)
	}
	if request.Context == nil || *request.Context != wantContext {
		t.Errorf("runtime context = %#v, want %#v", request.Context, wantContext)
	}
	waitForFinish(t, repository)
}

func TestPostMessageScopesToolAccessToTheDurableRun(t *testing.T) {
	repository := newFakeRepository()
	issuer := &recordingTokenIssuer{revoked: make(chan struct{})}
	requestReceived := make(chan agent.RunRequest, 1)
	service := agent.NewService(repository, runtimeFunc(func(
		_ context.Context,
		request agent.RunRequest,
		_ func(context.Context, agent.RuntimeEvent) error,
	) error {
		requestReceived <- request
		return nil
	}), issuer, agent.ServiceConfig{
		Runtime: "pi", InternalURL: "http://127.0.0.1:8080", TokenTTL: 5 * time.Minute,
		AllowedTools: []agentauth.Tool{agentauth.ToolTaskGet, agentauth.ToolTaskUpdate},
		AgentDir:     "/tmp/pi", Provider: "openai-codex", Model: "model-id",
		Preferences: preferencesResolverFunc(func(context.Context, string, string) (string, string, string, error) {
			return "Europe/Moscow", "selected-model", "high", nil
		}),
		ProjectValidator: projectValidatorFunc(func(context.Context, string, string) error {
			return nil
		}),
	})

	if _, err := service.PostMessage(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"project-id", "session-id", agent.MessageInput{Content: "Run"},
	); err != nil {
		t.Fatalf("post message: %v", err)
	}
	request := <-requestReceived
	waitForFinish(t, repository)
	select {
	case <-issuer.revoked:
	case <-time.After(time.Second):
		t.Fatal("token was not revoked")
	}

	if issuer.issue.UserID != "user-id" || issuer.issue.AgentSessionID != "session-id" ||
		issuer.issue.ProjectID != "project-id" || issuer.issue.AgentRunID != "run-id" ||
		issuer.issue.TTL != 5*time.Minute {
		t.Errorf("token request = %#v", issuer.issue)
	}
	if request.ProjectID != "project-id" || request.AccessToken != "scoped-token" ||
		request.InternalURL != "http://127.0.0.1:8080" ||
		request.Runtime != "pi" || len(request.AllowedTools) != 2 ||
		request.Timezone != "Europe/Moscow" || request.Model != "selected-model" ||
		request.ThinkingEffort != "high" {
		t.Errorf("runtime request = %#v", request)
	}
	if issuer.revokedUser != "user-id" || issuer.revokedRun != "run-id" {
		t.Errorf("revoked = (%q, %q)", issuer.revokedUser, issuer.revokedRun)
	}
}

func TestRuntimeFailureProducesOneTerminalFailure(t *testing.T) {
	repository := newFakeRepository()
	service := newService(repository, runtimeFunc(func(
		context.Context,
		agent.RunRequest,
		func(context.Context, agent.RuntimeEvent) error,
	) error {
		return errors.New("runner exited")
	}))

	if _, err := service.PostMessage(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"project-id", "session-id", agent.MessageInput{Content: "Run"},
	); err != nil {
		t.Fatalf("post message: %v", err)
	}
	waitForFinish(t, repository)
	if eventType := repository.terminalEvent(); eventType != agent.EventRunFailed {
		t.Errorf("terminal event = %q, want %q", eventType, agent.EventRunFailed)
	}
}

func TestAbortCancelsRuntimeAndRecordsTerminalEventOnce(t *testing.T) {
	repository := newFakeRepository()
	started := make(chan struct{})
	service := newService(repository, runtimeFunc(func(
		ctx context.Context,
		_ agent.RunRequest,
		_ func(context.Context, agent.RuntimeEvent) error,
	) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}))
	if _, err := service.PostMessage(
		context.Background(), execution.UserScope("user-id", "correlation-id"),
		"project-id", "session-id", agent.MessageInput{Content: "Run"},
	); err != nil {
		t.Fatalf("post message: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runtime did not start")
	}

	finished, err := service.Abort(
		context.Background(), execution.UserScope("user-id", "abort-correlation"),
		"project-id", "run-id",
	)
	if err != nil {
		t.Fatalf("abort run: %v", err)
	}
	if finished.Status != agent.RunStatusAborted {
		t.Errorf("status = %q, want %q", finished.Status, agent.RunStatusAborted)
	}
	waitForFinish(t, repository)
	if count := repository.terminalCount(); count != 1 {
		t.Errorf("terminal event count = %d, want 1", count)
	}
}

type runtimeFunc func(
	context.Context,
	agent.RunRequest,
	func(context.Context, agent.RuntimeEvent) error,
) error

func (f runtimeFunc) Run(
	ctx context.Context,
	request agent.RunRequest,
	emit func(context.Context, agent.RuntimeEvent) error,
) error {
	return f(ctx, request, emit)
}

type tokenIssuerFunc func(context.Context, agentauth.IssueRequest) (agentauth.IssuedToken, error)

type preferencesResolverFunc func(context.Context, string, string) (string, string, string, error)

type contextValidatorFunc func(context.Context, string, string, agent.MessageContext) error

type projectValidatorFunc func(context.Context, string, string) error

func (f contextValidatorFunc) ValidateMessageContext(
	ctx context.Context,
	userID string,
	projectID string,
	messageContext agent.MessageContext,
) error {
	return f(ctx, userID, projectID, messageContext)
}

func (f preferencesResolverFunc) ResolveAgent(
	ctx context.Context,
	userID string,
	projectID string,
) (string, string, string, error) {
	return f(ctx, userID, projectID)
}

func (f projectValidatorFunc) ValidateProject(
	ctx context.Context,
	userID string,
	projectID string,
) error {
	return f(ctx, userID, projectID)
}

func (f tokenIssuerFunc) Issue(
	ctx context.Context,
	request agentauth.IssueRequest,
) (agentauth.IssuedToken, error) {
	return f(ctx, request)
}

func (f tokenIssuerFunc) RevokeRun(context.Context, string, string) error { return nil }

type recordingTokenIssuer struct {
	issue       agentauth.IssueRequest
	revokedUser string
	revokedRun  string
	revoked     chan struct{}
}

func (i *recordingTokenIssuer) Issue(
	_ context.Context,
	request agentauth.IssueRequest,
) (agentauth.IssuedToken, error) {
	i.issue = request
	return agentauth.IssuedToken{Token: "scoped-token"}, nil
}

func (i *recordingTokenIssuer) RevokeRun(_ context.Context, userID, runID string) error {
	i.revokedUser = userID
	i.revokedRun = runID
	close(i.revoked)
	return nil
}

func newService(repository *fakeRepository, runtime agent.Runtime) *agent.Service {
	issuer := tokenIssuerFunc(func(
		_ context.Context,
		request agentauth.IssueRequest,
	) (agentauth.IssuedToken, error) {
		return agentauth.IssuedToken{Token: "scoped-token"}, nil
	})
	return agent.NewService(repository, runtime, issuer, agent.ServiceConfig{
		Runtime: "fake", InternalURL: "http://127.0.0.1:8080",
		TokenTTL: time.Minute, AllowedTools: []agentauth.Tool{agentauth.ToolTaskGet},
		ProjectValidator: projectValidatorFunc(func(context.Context, string, string) error {
			return nil
		}),
	})
}

type fakeRepository struct {
	mu       sync.Mutex
	run      agent.Run
	events   []agent.RuntimeEvent
	terminal string
	done     chan struct{}
	doneOnce sync.Once
	history  []agent.HistoryMessage
	context  *agent.MessageContext
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		run: agent.Run{
			ID: "run-id", SessionID: "session-id", UserID: "user-id",
			Status: agent.RunStatusQueued, CorrelationID: "correlation-id",
		},
		done: make(chan struct{}),
	}
}

func (r *fakeRepository) CreateSession(context.Context, execution.Scope) (agent.Session, error) {
	return agent.Session{ID: "session-id"}, nil
}

func (r *fakeRepository) GetConversation(
	context.Context,
	string,
	string,
) (agent.Session, []agent.Message, []agent.Run, int64, error) {
	return agent.Session{ID: "session-id"}, []agent.Message{}, []agent.Run{r.run}, 0, nil
}

func (r *fakeRepository) CreateMessageRun(
	_ context.Context,
	_ execution.Scope,
	projectID string,
	_ string,
	input agent.MessageInput,
) (agent.Message, agent.Run, []agent.HistoryMessage, error) {
	return agent.Message{
		ID: "message-id", Content: strings.TrimSpace(input.Content),
	}, withRunProject(r.run, projectID), r.history, nil
}

func (r *fakeRepository) CreateContextRun(
	_ context.Context,
	_ execution.Scope,
	projectID string,
	messageContext agent.MessageContext,
) (agent.Run, error) {
	r.context = &messageContext
	r.run = agent.Run{
		ID: "run-id", SessionID: "private-execution-id", UserID: "user-id",
		ProjectID: &projectID,
		Kind:      agent.ExecutionAction, Status: agent.RunStatusQueued, CorrelationID: "correlation-id",
	}
	return r.run, nil
}

func (r *fakeRepository) GetRun(context.Context, string, string, string) (agent.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run, nil
}

func (r *fakeRepository) ListRunEvents(
	context.Context,
	string,
	string,
	int64,
	int,
) ([]agent.RunEvent, error) {
	return []agent.RunEvent{}, nil
}

func (r *fakeRepository) ListContextRunEvents(
	context.Context,
	string,
	string,
	string,
	int64,
	int,
) ([]agent.RunEvent, error) {
	return []agent.RunEvent{}, nil
}

func withRunProject(run agent.Run, projectID string) agent.Run {
	run.ProjectID = &projectID
	return run
}

func (r *fakeRepository) RecordRuntimeEvent(
	_ context.Context,
	_ execution.Scope,
	_ string,
	event agent.RuntimeEvent,
) (agent.RunEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	if event.Type == agent.EventRunCompleted || event.Type == agent.EventRunFailed ||
		event.Type == agent.EventRunAborted {
		r.finishLocked(event.Type)
	}
	return agent.RunEvent{Sequence: event.Sequence, Type: event.Type}, nil
}

func (r *fakeRepository) FinishRun(
	_ context.Context,
	_ execution.Scope,
	_ string,
	eventType string,
	_ any,
) (agent.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finishLocked(eventType)
	r.doneOnce.Do(func() { close(r.done) })
	return r.run, nil
}

func (r *fakeRepository) finishLocked(eventType string) {
	if r.terminal != "" {
		return
	}
	r.terminal = eventType
	switch eventType {
	case agent.EventRunCompleted:
		r.run.Status = agent.RunStatusCompleted
	case agent.EventRunFailed:
		r.run.Status = agent.RunStatusFailed
	case agent.EventRunAborted:
		r.run.Status = agent.RunStatusAborted
	}
}

func (r *fakeRepository) runtimeEvents() []agent.RuntimeEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]agent.RuntimeEvent(nil), r.events...)
}

func (r *fakeRepository) terminalEvent() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.terminal
}

func (r *fakeRepository) terminalCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.terminal == "" {
		return 0
	}
	return 1
}

func waitForFinish(t *testing.T, repository *fakeRepository) {
	t.Helper()
	select {
	case <-repository.done:
	case <-time.After(time.Second):
		t.Fatal("run did not finish")
	}
}
