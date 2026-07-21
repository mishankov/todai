package piruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/agent"
)

const (
	defaultStartupTimeout = 5 * time.Second
	defaultAbortTimeout   = 2 * time.Second
	defaultMaximumLine    = 1024 * 1024
)

// Config describes how the isolated runner process is launched.
type Config struct {
	Executable     string
	Args           []string
	Directory      string
	Environment    []string
	StartupTimeout time.Duration
	RunTimeout     time.Duration
	AbortTimeout   time.Duration
	MaximumLine    int
}

// Runtime launches one isolated runner process for each agent run.
type Runtime struct {
	config Config
}

// New constructs a process-backed runner adapter.
func New(config Config) *Runtime {
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = defaultStartupTimeout
	}
	if config.AbortTimeout <= 0 {
		config.AbortTimeout = defaultAbortTimeout
	}
	if config.MaximumLine <= 0 {
		config.MaximumLine = defaultMaximumLine
	}
	return &Runtime{config: config}
}

// Run executes one request and persists each adapted product event through emit.
func (r *Runtime) Run(
	ctx context.Context,
	request agent.RunRequest,
	emit func(context.Context, agent.RuntimeEvent) error,
) error {
	if strings.TrimSpace(r.config.Executable) == "" {
		return errors.New("runner executable is required")
	}
	if r.config.RunTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.RunTimeout)
		defer cancel()
	}

	command := exec.Command(r.config.Executable, r.config.Args...)
	command.Dir = r.config.Directory
	command.Env = runnerEnvironment(r.config.Environment)
	stdin, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("open runner stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open runner stdout: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("open runner stderr: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start runner: %w", err)
	}

	processDone := make(chan error, 1)
	go func() { processDone <- command.Wait() }()
	go logRunnerStderr(ctx, stderr)

	records := scanRecords(stdout, r.config.MaximumLine)
	ready, err := waitForReady(ctx, records, r.config.StartupTimeout)
	if err != nil {
		stopProcess(command, stdin, processDone)
		return err
	}

	start := envelope{
		Type:        "run.start",
		RequestID:   "start-" + request.RunID,
		SessionID:   request.SessionID,
		RunID:       request.RunID,
		Message:     request.Message,
		Context:     request.Context,
		History:     append([]agent.HistoryMessage{}, request.History...),
		RuntimeName: request.Runtime,
		ToolAccess: &toolAccess{
			BaseURL: request.InternalURL, Token: request.AccessToken,
			AllowedTools: append([]string(nil), request.AllowedTools...),
		},
		Pi: &piConfig{
			AgentDir: request.AgentDir, Provider: request.Provider,
			Model: request.Model, Timezone: request.Timezone,
			ThinkingEffort: request.ThinkingEffort,
		},
	}
	if err := writeEnvelope(stdin, start); err != nil {
		stopProcess(command, stdin, processDone)
		return err
	}

	runtimePayload := map[string]any{
		"runtime": ready.Runtime.Name, "runtimeVersion": ready.Runtime.Version,
		"model": request.Model, "thinkingEffort": request.ThinkingEffort,
	}
	expectedSequence := int64(1)
	terminal := false
	for !terminal {
		select {
		case <-ctx.Done():
			if err := writeEnvelope(stdin, envelope{
				Type: "run.abort", RequestID: "abort-" + request.RunID, RunID: request.RunID,
			}); err != nil {
				stopProcess(command, stdin, processDone)
				return ctx.Err()
			}
			abortTimer := time.NewTimer(r.config.AbortTimeout)
			err := consumeUntilTerminal(
				context.WithoutCancel(ctx), records, abortTimer.C,
				request.RunID, &expectedSequence, runtimePayload, emit,
			)
			abortTimer.Stop()
			stopProcess(command, stdin, processDone)
			if err != nil {
				return fmt.Errorf("abort runner: %w", err)
			}
			return nil
		case record, ok := <-records:
			if !ok {
				stopProcess(command, stdin, processDone)
				return ErrUnexpectedExit
			}
			if record.err != nil {
				stopProcess(command, stdin, processDone)
				return record.err
			}
			adapted, isTerminal, err := adaptEvent(
				record.envelope, request.RunID, expectedSequence, runtimePayload,
			)
			if err != nil {
				stopProcess(command, stdin, processDone)
				return err
			}
			if err := emit(ctx, adapted); err != nil {
				stopProcess(command, stdin, processDone)
				return fmt.Errorf("persist runner event: %w", err)
			}
			expectedSequence++
			terminal = isTerminal
		}
	}

	_ = stdin.Close()
	select {
	case err := <-processDone:
		if err != nil {
			return fmt.Errorf("wait for runner: %w", err)
		}
	case <-time.After(r.config.AbortTimeout):
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		<-processDone
	}

	return nil
}

type scannedRecord struct {
	envelope envelope
	err      error
}

func scanRecords(reader io.Reader, maximumLine int) <-chan scannedRecord {
	records := make(chan scannedRecord)
	go func() {
		defer close(records)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), maximumLine)
		for scanner.Scan() {
			decoded, err := decodeEnvelope(scanner.Bytes())
			records <- scannedRecord{envelope: decoded, err: err}
			if err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			records <- scannedRecord{err: fmt.Errorf("%w: read JSONL: %v", ErrInvalidProtocol, err)}
		}
	}()
	return records
}

func waitForReady(
	ctx context.Context,
	records <-chan scannedRecord,
	timeout time.Duration,
) (envelope, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return envelope{}, ctx.Err()
	case <-timer.C:
		return envelope{}, errors.New("runner startup timed out")
	case record, ok := <-records:
		if !ok {
			return envelope{}, errors.New("runner produced no ready event")
		}
		if record.err != nil {
			return envelope{}, record.err
		}
		if record.envelope.Type != "runner.ready" || record.envelope.Runtime == nil ||
			strings.TrimSpace(record.envelope.Runtime.Name) == "" ||
			strings.TrimSpace(record.envelope.Runtime.Version) == "" {
			return envelope{}, fmt.Errorf("%w: expected runner.ready", ErrInvalidProtocol)
		}
		return record.envelope, nil
	}
}

func adaptEvent(
	value envelope,
	runID string,
	expectedSequence int64,
	runtimePayload map[string]any,
) (agent.RuntimeEvent, bool, error) {
	if value.RunID != runID {
		return agent.RuntimeEvent{}, false, fmt.Errorf(
			"%w: event run %q does not match %q", ErrInvalidProtocol, value.RunID, runID,
		)
	}
	if value.Sequence != expectedSequence {
		return agent.RuntimeEvent{}, false, fmt.Errorf(
			"%w: event sequence %d, want %d", ErrInvalidProtocol, value.Sequence, expectedSequence,
		)
	}

	payload := make(map[string]any)
	terminal := false
	productType := ""
	switch value.Type {
	case "run.started":
		productType = "agent.run.started"
		for key, item := range runtimePayload {
			payload[key] = item
		}
		if strings.TrimSpace(value.Model) != "" {
			payload["model"] = value.Model
		}
		if strings.TrimSpace(value.ThinkingEffort) != "" {
			payload["thinkingEffort"] = value.ThinkingEffort
		}
	case "assistant.delta":
		if strings.TrimSpace(value.MessageID) == "" {
			return agent.RuntimeEvent{}, false, fmt.Errorf("%w: messageId is required", ErrInvalidProtocol)
		}
		productType = "agent.message.delta"
		payload["messageId"] = value.MessageID
		payload["delta"] = value.Delta
	case "tool.started":
		if strings.TrimSpace(value.ToolCallID) == "" || strings.TrimSpace(value.ToolName) == "" {
			return agent.RuntimeEvent{}, false, fmt.Errorf("%w: tool identity is required", ErrInvalidProtocol)
		}
		productType = agent.EventToolStarted
		payload["toolCallId"] = value.ToolCallID
		payload["toolName"] = value.ToolName
		arguments, err := decodeProtocolObject(value.Arguments, "tool arguments")
		if err != nil {
			return agent.RuntimeEvent{}, false, err
		}
		payload["arguments"] = arguments
	case "tool.completed":
		if strings.TrimSpace(value.ToolCallID) == "" || strings.TrimSpace(value.ToolName) == "" {
			return agent.RuntimeEvent{}, false, fmt.Errorf("%w: tool identity is required", ErrInvalidProtocol)
		}
		productType = agent.EventToolCompleted
		payload["toolCallId"] = value.ToolCallID
		payload["toolName"] = value.ToolName
		payload["isError"] = value.IsError
		result, err := decodeProtocolObject(value.Result, "tool result")
		if err != nil {
			return agent.RuntimeEvent{}, false, err
		}
		payload["result"] = result
	case "history.message":
		if value.HistoryMessage == nil {
			return agent.RuntimeEvent{}, false, fmt.Errorf("%w: historyMessage is required", ErrInvalidProtocol)
		}
		productType = agent.EventHistoryMessage
		payload["message"] = value.HistoryMessage
	case "run.completed":
		productType = "agent.run.completed"
		terminal = true
	case "run.failed":
		if value.Error == nil || strings.TrimSpace(value.Error.Code) == "" {
			return agent.RuntimeEvent{}, false, fmt.Errorf("%w: run error is required", ErrInvalidProtocol)
		}
		productType = "agent.run.failed"
		payload["error"] = value.Error
		terminal = true
	case "run.aborted":
		productType = "agent.run.aborted"
		terminal = true
	default:
		return agent.RuntimeEvent{}, false, fmt.Errorf(
			"%w: unsupported event type %q", ErrInvalidProtocol, value.Type,
		)
	}

	return agent.RuntimeEvent{Type: productType, Sequence: value.Sequence, Payload: payload}, terminal, nil
}

func decodeProtocolObject(value json.RawMessage, name string) (map[string]any, error) {
	var decoded map[string]any
	if len(value) == 0 || json.Unmarshal(value, &decoded) != nil || decoded == nil {
		return nil, fmt.Errorf("%w: %s must be an object", ErrInvalidProtocol, name)
	}
	return decoded, nil
}

func consumeUntilTerminal(
	ctx context.Context,
	records <-chan scannedRecord,
	timeout <-chan time.Time,
	runID string,
	expectedSequence *int64,
	runtimePayload map[string]any,
	emit func(context.Context, agent.RuntimeEvent) error,
) error {
	for {
		select {
		case <-timeout:
			return errors.New("runner abort timed out")
		case record, ok := <-records:
			if !ok {
				return ErrUnexpectedExit
			}
			if record.err != nil {
				return record.err
			}
			adapted, terminal, err := adaptEvent(
				record.envelope, runID, *expectedSequence, runtimePayload,
			)
			if err != nil {
				return err
			}
			if err := emit(ctx, adapted); err != nil {
				return err
			}
			*expectedSequence++
			if terminal {
				return nil
			}
		}
	}
}

func writeEnvelope(writer io.Writer, value envelope) error {
	encoded, err := encodeEnvelope(value)
	if err != nil {
		return err
	}
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("write runner command: %w", err)
	}
	return nil
}

func logRunnerStderr(ctx context.Context, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		log.InfoContext(ctx, "runner output", "message", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.ErrorContext(ctx, "read runner stderr", "error", err)
	}
}

func runnerEnvironment(extra []string) []string {
	keys := []string{
		"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR",
		"NODE_EXTRA_CA_CERTS", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
	}
	environment := make([]string, 0, len(keys)+len(extra))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return append(environment, extra...)
}

func stopProcess(command *exec.Cmd, stdin io.Closer, processDone <-chan error) {
	_ = stdin.Close()
	if command.Process != nil {
		_ = command.Process.Kill()
	}
	select {
	case <-processDone:
	case <-time.After(time.Second):
	}
}
