package piruntime_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/piruntime"
)

func TestRuntimeAdaptsDeterministicRunnerEvents(t *testing.T) {
	runtime := helperRuntime(t, "happy")
	events := make([]agent.RuntimeEvent, 0)

	err := runtime.Run(context.Background(), testRunRequest(), func(
		_ context.Context,
		event agent.RuntimeEvent,
	) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("event count = %d, want 4: %#v", len(events), events)
	}
	if events[0].Type != agent.EventRunStarted || events[0].Sequence != 1 {
		t.Errorf("started event = %#v", events[0])
	}
	startedPayload, ok := events[0].Payload.(map[string]any)
	if !ok || startedPayload["runtime"] != "fake" || startedPayload["runtimeVersion"] != "0.1.0" {
		t.Errorf("started payload = %#v", events[0].Payload)
	}
	if events[1].Type != agent.EventMessageDelta || events[1].Sequence != 2 {
		t.Errorf("delta event = %#v", events[1])
	}
	if events[2].Type != agent.EventHistoryMessage || events[2].Sequence != 3 {
		t.Errorf("history event = %#v", events[2])
	}
	if events[3].Type != agent.EventRunCompleted || events[3].Sequence != 4 {
		t.Errorf("completed event = %#v", events[3])
	}
}

func TestRuntimeRunsCompiledTypeScriptRunner(t *testing.T) {
	entry := "../../../pi-runner/dist/cli/main.js"
	if _, err := os.Stat(entry); errors.Is(err, os.ErrNotExist) {
		t.Skip("compiled pi-runner is not available")
	} else if err != nil {
		t.Fatalf("stat compiled runner: %v", err)
	}
	runtime := piruntime.New(piruntime.Config{
		Executable: "node", Args: []string{entry}, StartupTimeout: 5 * time.Second,
	})
	events := make([]agent.RuntimeEvent, 0)
	request := testRunRequest()
	request.History = nil
	err := runtime.Run(context.Background(), request, func(
		_ context.Context,
		event agent.RuntimeEvent,
	) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) != 4 || events[0].Type != agent.EventRunStarted ||
		events[1].Type != agent.EventMessageDelta || events[2].Type != agent.EventHistoryMessage ||
		events[3].Type != agent.EventRunCompleted {
		t.Errorf("events = %#v", events)
	}
}

func TestRuntimeAcceptsPartialJSONLLines(t *testing.T) {
	runtime := helperRuntime(t, "partial")
	events := make([]agent.RuntimeEvent, 0)

	err := runtime.Run(context.Background(), testRunRequest(), func(
		_ context.Context,
		event agent.RuntimeEvent,
	) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) != 4 || events[3].Type != agent.EventRunCompleted {
		t.Errorf("events = %#v", events)
	}
}

func TestRuntimeAdaptsToolLifecycleWithArgumentsAndResults(t *testing.T) {
	runtime := helperRuntime(t, "tools")
	events := make([]agent.RuntimeEvent, 0)

	if err := runtime.Run(context.Background(), testRunRequest(), func(
		_ context.Context,
		event agent.RuntimeEvent,
	) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) != 6 || events[1].Type != agent.EventToolStarted ||
		events[2].Type != agent.EventToolCompleted {
		t.Fatalf("events = %#v", events)
	}
	payload, ok := events[2].Payload.(map[string]any)
	if !ok || payload["toolName"] != "task_get" || payload["isError"] != false {
		t.Errorf("tool payload = %#v", events[2].Payload)
	}
	startedPayload := events[1].Payload.(map[string]any)
	arguments := startedPayload["arguments"].(map[string]any)
	result := payload["result"].(map[string]any)
	if arguments["taskId"] != "task-1" || result["details"] == nil {
		t.Errorf("tool arguments/result = %#v / %#v", arguments, result)
	}
}

func TestRuntimeRejectsInvalidRunnerOutput(t *testing.T) {
	runtime := helperRuntime(t, "invalid")

	err := runtime.Run(
		context.Background(), testRunRequest(),
		func(context.Context, agent.RuntimeEvent) error { return nil },
	)
	if !errors.Is(err, piruntime.ErrInvalidProtocol) {
		t.Fatalf("Run() error = %v, want ErrInvalidProtocol", err)
	}
}

func TestRuntimeDoesNotInheritBackendSecrets(t *testing.T) {
	t.Setenv("TODAI_DATABASE_URL", "postgres://secret")
	runtime := helperRuntime(t, "environment")

	if err := runtime.Run(
		context.Background(), testRunRequest(),
		func(context.Context, agent.RuntimeEvent) error { return nil },
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRuntimeRequestsAbortAndPersistsTerminalEvent(t *testing.T) {
	runtime := helperRuntime(t, "abort")
	ctx, cancel := context.WithCancel(context.Background())
	events := make([]agent.RuntimeEvent, 0)

	err := runtime.Run(ctx, testRunRequest(), func(
		_ context.Context,
		event agent.RuntimeEvent,
	) error {
		events = append(events, event)
		if event.Type == agent.EventRunStarted {
			cancel()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) != 2 || events[1].Type != agent.EventRunAborted || events[1].Sequence != 2 {
		t.Errorf("events = %#v", events)
	}
}

func TestRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("TODAI_RUNNER_HELPER") != "1" {
		return
	}

	scenario := os.Getenv("TODAI_RUNNER_SCENARIO")
	if scenario == "environment" && os.Getenv("TODAI_DATABASE_URL") != "" {
		os.Exit(7)
	}
	writeProtocolLine(map[string]any{
		"protocol": "todai.runner", "version": 4, "type": "runner.ready",
		"runtime": map[string]any{"name": "fake", "version": "0.1.0"},
	})
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(2)
	}
	var start map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &start); err != nil || start["type"] != "run.start" {
		os.Exit(3)
	}
	toolAccess, ok := start["toolAccess"].(map[string]any)
	history, historyOK := start["history"].([]any)
	pi, piOK := start["pi"].(map[string]any)
	messageContext, contextOK := start["context"].(map[string]any)
	if !ok || toolAccess["token"] != "scoped-token" || start["runtimeName"] != "fake" ||
		!historyOK || len(history) != 1 || !piOK || pi["timezone"] != "Europe/Moscow" ||
		pi["model"] != "selected-model" || pi["thinkingEffort"] != "high" || !contextOK ||
		messageContext["type"] != "task" || messageContext["action"] != "decompose" {
		os.Exit(6)
	}
	runID, _ := start["runId"].(string)
	writeProtocolLine(map[string]any{
		"protocol": "todai.runner", "version": 4, "type": "run.started",
		"runId": runID, "sequence": 1, "model": "selected-model", "thinkingEffort": "high",
	})
	if scenario == "invalid" {
		_, _ = fmt.Fprintln(os.Stdout, "this is not JSON")
		os.Exit(0)
	}
	if scenario == "abort" {
		if !scanner.Scan() {
			os.Exit(4)
		}
		var abort map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &abort); err != nil || abort["type"] != "run.abort" {
			os.Exit(5)
		}
		writeProtocolLine(map[string]any{
			"protocol": "todai.runner", "version": 4, "type": "run.aborted",
			"runId": runID, "sequence": 2,
		})
		os.Exit(0)
	}

	nextSequence := 2
	if scenario == "tools" {
		writeProtocolLine(map[string]any{
			"protocol": "todai.runner", "version": 4, "type": "tool.started",
			"runId": runID, "sequence": nextSequence, "toolCallId": "call-1", "toolName": "task_get",
			"arguments": map[string]any{"taskId": "task-1"},
		})
		nextSequence++
		writeProtocolLine(map[string]any{
			"protocol": "todai.runner", "version": 4, "type": "tool.completed",
			"runId": runID, "sequence": nextSequence, "toolCallId": "call-1", "toolName": "task_get", "isError": false,
			"result": map[string]any{"content": []any{map[string]any{"type": "text", "text": `{\"id\":\"task-1\"}`}}, "details": map[string]any{"status": 200}},
		})
		nextSequence++
	}
	delta := protocolLine(map[string]any{
		"protocol": "todai.runner", "version": 4, "type": "assistant.delta",
		"runId": runID, "sequence": nextSequence, "messageId": "message-" + runID,
		"delta": "Deterministic response",
	})
	if scenario == "partial" {
		middle := len(delta) / 2
		_, _ = os.Stdout.WriteString(delta[:middle])
		time.Sleep(10 * time.Millisecond)
		_, _ = os.Stdout.WriteString(delta[middle:])
	} else {
		_, _ = os.Stdout.WriteString(delta)
	}
	writeProtocolLine(map[string]any{
		"protocol": "todai.runner", "version": 4, "type": "history.message",
		"runId": runID, "sequence": nextSequence + 1,
		"historyMessage": map[string]any{
			"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "Deterministic response"}},
			"timestamp": 1,
		},
	})
	writeProtocolLine(map[string]any{
		"protocol": "todai.runner", "version": 4, "type": "run.completed",
		"runId": runID, "sequence": nextSequence + 2,
	})
	os.Exit(0)
}

func helperRuntime(t *testing.T, scenario string) *piruntime.Runtime {
	t.Helper()
	return piruntime.New(piruntime.Config{
		Executable: os.Args[0],
		Args:       []string{"-test.run=TestRuntimeHelperProcess"},
		Environment: []string{
			"TODAI_RUNNER_HELPER=1",
			"TODAI_RUNNER_SCENARIO=" + scenario,
		},
		StartupTimeout: time.Second,
		AbortTimeout:   time.Second,
	})
}

func testRunRequest() agent.RunRequest {
	return agent.RunRequest{
		UserID: "user-id", SessionID: "session-id", RunID: "run-id", Message: "Plan my day",
		Context: &agent.MessageContext{
			Type: agent.ContextTask, TaskID: "11111111-1111-4111-8111-111111111111",
			Action: agent.ContextActionDecompose,
		},
		History: []agent.HistoryMessage{{
			Role: agent.HistoryRoleUser, Content: []agent.HistoryContent{{Type: "text", Text: "Earlier"}}, Timestamp: 1,
		}},
		Runtime: "fake", InternalURL: "http://127.0.0.1:8080", AccessToken: "scoped-token",
		AllowedTools: []string{"task_get"}, Model: "selected-model", Timezone: "Europe/Moscow",
		ThinkingEffort: "high",
	}
}

func writeProtocolLine(value map[string]any) {
	_, _ = os.Stdout.WriteString(protocolLine(value))
}

func protocolLine(value map[string]any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(encoded)) + "\n"
}
