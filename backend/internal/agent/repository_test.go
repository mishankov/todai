package agent_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/platforma-dev/platforma/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/execution"
)

func TestRepositoryPersistsConversationEventsAndLifecycleActivity(t *testing.T) {
	db, repository, activityRepository := testAgentRepository(t)
	ctx := context.Background()
	userScope := execution.UserScope("user-id", "correlation-id")

	session, err := repository.CreateSession(ctx, userScope)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	message, run, history, err := repository.CreateMessageRun(
		ctx, userScope, session.ID, agent.MessageInput{Content: " Triage inbox "},
	)
	if err != nil {
		t.Fatalf("create message run: %v", err)
	}
	if message.Content != "Triage inbox" || run.Status != agent.RunStatusQueued {
		t.Errorf("message/run = %#v / %#v", message, run)
	}
	if len(history) != 0 {
		t.Errorf("initial history = %#v, want empty", history)
	}

	runtimeScope := agentRunScope(run)
	started := recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventRunStarted, Sequence: 1, Payload: map[string]any{},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventToolStarted, Sequence: 2,
		Payload: map[string]any{"toolCallId": "call-1", "toolName": "task_get", "arguments": map[string]any{"taskId": "task-1"}},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventToolCompleted, Sequence: 3,
		Payload: map[string]any{
			"toolCallId": "call-1", "toolName": "task_get", "isError": false,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"id":"task-1"}`}}},
		},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventHistoryMessage, Sequence: 4,
		Payload: map[string]any{"message": agent.HistoryMessage{
			Role:      agent.HistoryRoleAssistant,
			Content:   []agent.HistoryContent{{Type: "toolCall", ID: "call-1", Name: "task_get", Arguments: map[string]any{"taskId": "task-1"}}},
			Timestamp: 1,
		}},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventHistoryMessage, Sequence: 5,
		Payload: map[string]any{"message": agent.HistoryMessage{
			Role: agent.HistoryRoleToolResult, ToolCallID: "call-1", ToolName: "task_get",
			Content: []agent.HistoryContent{{Type: "text", Text: `{"id":"task-1"}`}},
			Details: map[string]any{"status": 200}, Timestamp: 2,
		}},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventMessageDelta, Sequence: 6, Payload: map[string]any{"delta": "Inbox "},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventMessageDelta, Sequence: 7, Payload: map[string]any{"delta": "is clear"},
	})
	recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventHistoryMessage, Sequence: 8,
		Payload: map[string]any{"message": agent.HistoryMessage{
			Role:    agent.HistoryRoleAssistant,
			Content: []agent.HistoryContent{{Type: "text", Text: "Inbox is clear"}}, Timestamp: 3,
		}},
	})
	completed := recordRuntimeEvent(t, repository, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventRunCompleted, Sequence: 9, Payload: map[string]any{},
	})
	if completed.StreamOffset <= started.StreamOffset {
		t.Errorf("stream offsets = %d then %d", started.StreamOffset, completed.StreamOffset)
	}

	conversation, messages, runs, lastStreamOffset, err := repository.GetConversation(ctx, "user-id", session.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if conversation.ID != session.ID || len(messages) != 2 || messages[1].Content != "Inbox is clear" {
		t.Errorf("conversation/messages = %#v / %#v", conversation, messages)
	}
	if len(runs) != 1 || runs[0].Status != agent.RunStatusCompleted || runs[0].CompletedAt == nil {
		t.Errorf("runs = %#v", runs)
	}
	if lastStreamOffset != completed.StreamOffset {
		t.Errorf("last stream offset = %d, want %d", lastStreamOffset, completed.StreamOffset)
	}

	replayed, err := repository.ListRunEvents(ctx, "user-id", session.ID, started.StreamOffset, 100)
	if err != nil {
		t.Fatalf("list replay events: %v", err)
	}
	if len(replayed) != 8 || replayed[0].Sequence != 2 || replayed[7].Sequence != 9 {
		t.Errorf("replayed events = %#v", replayed)
	}
	if _, err := repository.ListRunEvents(ctx, "other-user", session.ID, 0, 100); !errors.Is(err, agent.ErrSessionNotFound) {
		t.Errorf("other user ListRunEvents() error = %v", err)
	}
	if _, err := repository.RecordRuntimeEvent(ctx, runtimeScope, run.ID, agent.RuntimeEvent{
		Type: agent.EventMessageDelta, Sequence: 10, Payload: map[string]any{"delta": "late"},
	}); !errors.Is(err, agent.ErrRunFinished) {
		t.Errorf("late event error = %v, want %v", err, agent.ErrRunFinished)
	}
	_, failedRun, history, err := repository.CreateMessageRun(
		ctx, userScope, session.ID, agent.MessageInput{Content: "Try again"},
	)
	if err != nil {
		t.Fatalf("create failed message run: %v", err)
	}
	if len(history) != 4 || history[1].Role != agent.HistoryRoleAssistant ||
		history[1].Content[0].Type != "toolCall" || history[2].Role != agent.HistoryRoleToolResult ||
		history[2].Content[0].Text != `{"id":"task-1"}` || history[3].Content[0].Text != "Inbox is clear" {
		t.Errorf("persisted history = %#v", history)
	}
	if history[0].Content[0].Text != "Triage inbox" {
		t.Errorf("first chat history message = %q", history[0].Content[0].Text)
	}
	recordRuntimeEvent(t, repository, agentRunScope(failedRun), failedRun.ID, agent.RuntimeEvent{
		Type: agent.EventRunFailed, Sequence: 1,
		Payload: map[string]any{"error": map[string]any{"code": "provider_error", "message": "Provider unavailable"}},
	})
	_, _, updatedRuns, _, err := repository.GetConversation(ctx, "user-id", session.ID)
	if err != nil {
		t.Fatalf("get failed conversation: %v", err)
	}
	if len(updatedRuns) != 2 || updatedRuns[1].Error == nil || *updatedRuns[1].Error != "Provider unavailable" {
		t.Errorf("failed run = %#v", updatedRuns)
	}

	activityEvents, err := activityRepository.List(ctx, "user-id", 50)
	if err != nil {
		t.Fatalf("list lifecycle activity: %v", err)
	}
	if len(activityEvents) != 8 {
		t.Fatalf("lifecycle activity count = %d, want 8", len(activityEvents))
	}
	if activityEvents[0].Type != agent.EventRunFailed ||
		activityEvents[len(activityEvents)-1].Type != "agent.session.created" {
		t.Errorf("lifecycle activity = %#v", activityEvents)
	}

	_ = db
}

func TestRepositoryKeepsContextRunsOutOfConversationHistory(t *testing.T) {
	db, repository, _ := testAgentRepository(t)
	ctx := context.Background()
	scope := execution.UserScope("user-id", "correlation-id")

	conversation, err := repository.CreateSession(ctx, scope)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if _, _, _, err := repository.CreateMessageRun(
		ctx, scope, conversation.ID, agent.MessageInput{Content: "Keep this chat isolated"},
	); err != nil {
		t.Fatalf("create conversation run: %v", err)
	}

	messageContext := agent.MessageContext{
		Type: agent.ContextTask, TaskID: "11111111-1111-4111-8111-111111111111",
		Action: agent.ContextActionDecompose,
	}
	actionRun, err := repository.CreateContextRun(ctx, scope, messageContext)
	if err != nil {
		t.Fatalf("create contextual run: %v", err)
	}
	if actionRun.Kind != agent.ExecutionAction || actionRun.ContextType == nil ||
		actionRun.ContextID == nil || actionRun.ContextAction == nil ||
		*actionRun.ContextID != messageContext.TaskID {
		t.Errorf("contextual run = %#v", actionRun)
	}
	actionScope := agentRunScope(actionRun)
	recordRuntimeEvent(t, repository, actionScope, actionRun.ID, agent.RuntimeEvent{
		Type: agent.EventMessageDelta, Sequence: 1, Payload: map[string]any{"delta": "Created subtasks"},
	})
	recordRuntimeEvent(t, repository, actionScope, actionRun.ID, agent.RuntimeEvent{
		Type: agent.EventHistoryMessage, Sequence: 2,
		Payload: map[string]any{"message": agent.HistoryMessage{
			Role: agent.HistoryRoleToolResult, ToolCallID: "call-1", ToolName: "task_create",
			Content:   []agent.HistoryContent{{Type: "text", Text: `{"id":"child-id"}`}},
			Timestamp: 1,
		}},
	})
	recordRuntimeEvent(t, repository, actionScope, actionRun.ID, agent.RuntimeEvent{
		Type: agent.EventRunCompleted, Sequence: 3, Payload: map[string]any{},
	})

	_, messages, runs, lastOffset, err := repository.GetConversation(ctx, "user-id", conversation.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "Keep this chat isolated" || len(runs) != 1 || lastOffset != 0 {
		t.Errorf("conversation leaked contextual run: messages=%#v runs=%#v offset=%d", messages, runs, lastOffset)
	}
	_, _, history, err := repository.CreateMessageRun(
		ctx, scope, conversation.ID, agent.MessageInput{Content: "Continue the chat"},
	)
	if err != nil {
		t.Fatalf("continue conversation: %v", err)
	}
	if len(history) != 1 || history[0].Content[0].Text != "Keep this chat isolated" {
		t.Errorf("conversation history = %#v", history)
	}

	var actionMessageCount int
	if err := db.GetContext(ctx, &actionMessageCount, `
		SELECT COUNT(*) FROM agent_messages WHERE session_id = $1
	`, actionRun.SessionID); err != nil {
		t.Fatalf("count contextual messages: %v", err)
	}
	if actionMessageCount != 0 {
		t.Errorf("contextual agent message count = %d, want 0", actionMessageCount)
	}
	if _, _, _, _, err := repository.GetConversation(
		ctx, "user-id", actionRun.SessionID,
	); !errors.Is(err, agent.ErrSessionNotFound) {
		t.Errorf("contextual execution exposed as conversation: %v", err)
	}
	events, err := repository.ListContextRunEvents(ctx, "user-id", actionRun.ID, 0, 100)
	if err != nil || len(events) != 3 {
		t.Errorf("contextual events = %#v, error = %v", events, err)
	}
	if _, err := repository.ListContextRunEvents(
		ctx, "other-user", actionRun.ID, 0, 100,
	); !errors.Is(err, agent.ErrRunNotFound) {
		t.Errorf("other user contextual events error = %v", err)
	}
}

func TestRepositoryRejectsInvalidRuntimeEventsBeforePersistence(t *testing.T) {
	repository := agent.NewRepository(nil, nil)
	scope := execution.UserScope("user-id", "correlation-id")

	for _, event := range []agent.RuntimeEvent{
		{Type: "runtime.raw", Sequence: 1, Payload: map[string]any{}},
		{Type: agent.EventRunStarted, Sequence: 0, Payload: map[string]any{}},
		{Type: agent.EventRunStarted, Sequence: 1, Payload: []string{"not", "object"}},
		{Type: agent.EventHistoryMessage, Sequence: 1, Payload: map[string]any{
			"message": agent.HistoryMessage{
				Role:    agent.HistoryRoleUser,
				Content: []agent.HistoryContent{{Type: "text", Text: "forged"}}, Timestamp: 1,
			},
		}},
	} {
		if _, err := repository.RecordRuntimeEvent(
			context.Background(), scope, "run-id", event,
		); !errors.Is(err, agent.ErrInvalidEvent) {
			t.Errorf("RecordRuntimeEvent(%#v) error = %v", event, err)
		}
	}
}

func testAgentRepository(t *testing.T) (*sqlx.DB, *agent.Repository, *activity.Repository) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	container, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("todai"),
		postgres.WithUsername("todai"),
		postgres.WithPassword("todai"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}
	testcontainers.CleanupContainer(t, container)

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}
	platformaDatabase, err := database.New(databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = platformaDatabase.Connection().Close() })
	activityRepository := activity.NewRepository(platformaDatabase.Connection())
	repository := agent.NewRepository(platformaDatabase.Connection(), activityRepository)
	platformaDatabase.RegisterRepository("activity_repository", activityRepository)
	platformaDatabase.RegisterRepository("agent_repository", repository)
	if err := platformaDatabase.Migrate(ctx); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return platformaDatabase.Connection(), repository, activityRepository
}

func recordRuntimeEvent(
	t *testing.T,
	repository *agent.Repository,
	scope execution.Scope,
	runID string,
	event agent.RuntimeEvent,
) agent.RunEvent {
	t.Helper()
	persisted, err := repository.RecordRuntimeEvent(context.Background(), scope, runID, event)
	if err != nil {
		t.Fatalf("record runtime event: %v", err)
	}
	return persisted
}

func agentRunScope(run agent.Run) execution.Scope {
	actorID := run.SessionID
	runID := run.ID
	return execution.Scope{
		UserID: run.UserID, ActorType: execution.ActorBuiltInAgent, ActorID: &actorID,
		Source: execution.SourceSystem, CorrelationID: run.CorrelationID, AgentRunID: &runID,
	}
}
