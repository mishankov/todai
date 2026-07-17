// Package fakeagent provides a deterministic agent harness for exercising task tools without an LLM.
package fakeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mishankov/todai/backend/internal/agentauth"
)

// Step is one deterministic tool invocation. Output may point to a response DTO.
type Step struct {
	Tool   agentauth.Tool
	Input  any
	Output any
}

// Agent replays a fixed sequence of tool calls through the real internal HTTP boundary.
type Agent struct {
	baseURL string
	token   string
	client  *http.Client
}

// New constructs a deterministic fake agent.
func New(baseURL, token string, client *http.Client) *Agent {
	if client == nil {
		client = http.DefaultClient
	}
	return &Agent{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  client,
	}
}

// Run executes each step in order and stops at the first failed tool call.
func (a *Agent) Run(ctx context.Context, steps []Step) error {
	for index, step := range steps {
		if err := a.Call(ctx, step.Tool, step.Input, step.Output); err != nil {
			return fmt.Errorf("run fake agent step %d (%s): %w", index+1, step.Tool, err)
		}
	}

	return nil
}

// Call invokes one internal tool with the configured bearer token.
func (a *Agent) Call(ctx context.Context, tool agentauth.Tool, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode tool input: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.baseURL+"/internal/tools/"+string(tool),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create tool request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("send tool request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, readErr := io.ReadAll(io.LimitReader(response.Body, 8*1024))
		if readErr != nil {
			return fmt.Errorf("tool response status %d", response.StatusCode)
		}
		return &HTTPError{Status: response.StatusCode, Message: strings.TrimSpace(string(message))}
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode tool response: %w", err)
	}

	return nil
}

// HTTPError reports a non-successful internal tool response.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("tool response status %d: %s", e.Status, e.Message)
}
