// Package piruntime adapts the Todai runner JSONL protocol to stable agent events.
package piruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mishankov/todai/backend/internal/agent"
)

const (
	protocolName    = "todai.runner"
	protocolVersion = 3
)

var (
	// ErrInvalidProtocol indicates malformed or incompatible runner output.
	ErrInvalidProtocol = errors.New("invalid runner protocol")
	// ErrUnexpectedExit indicates that the runner stopped before a terminal event.
	ErrUnexpectedExit = errors.New("runner exited before a terminal event")
)

type envelope struct {
	Protocol       string                 `json:"protocol"`
	Version        int                    `json:"version"`
	Type           string                 `json:"type"`
	RequestID      string                 `json:"requestId,omitempty"`
	SessionID      string                 `json:"sessionId,omitempty"`
	RunID          string                 `json:"runId,omitempty"`
	Message        string                 `json:"message,omitempty"`
	History        []agent.HistoryMessage `json:"history"`
	RuntimeName    string                 `json:"runtimeName,omitempty"`
	ToolAccess     *toolAccess            `json:"toolAccess,omitempty"`
	Pi             *piConfig              `json:"pi,omitempty"`
	Sequence       int64                  `json:"sequence,omitempty"`
	MessageID      string                 `json:"messageId,omitempty"`
	Delta          string                 `json:"delta,omitempty"`
	Model          string                 `json:"model,omitempty"`
	ThinkingEffort string                 `json:"thinkingEffort,omitempty"`
	ToolCallID     string                 `json:"toolCallId,omitempty"`
	ToolName       string                 `json:"toolName,omitempty"`
	Arguments      json.RawMessage        `json:"arguments,omitempty"`
	Result         json.RawMessage        `json:"result,omitempty"`
	IsError        bool                   `json:"isError,omitempty"`
	HistoryMessage *agent.HistoryMessage  `json:"historyMessage,omitempty"`
	Runtime        *runtimeInfo           `json:"runtime,omitempty"`
	Error          *protocolError         `json:"error,omitempty"`
	Payload        json.RawMessage        `json:"payload,omitempty"`
}

type toolAccess struct {
	BaseURL      string   `json:"baseUrl"`
	Token        string   `json:"token"`
	AllowedTools []string `json:"allowedTools"`
}

type piConfig struct {
	AgentDir       string `json:"agentDir,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	ThinkingEffort string `json:"thinkingEffort,omitempty"`
}

type runtimeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type protocolError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func decodeEnvelope(line []byte) (envelope, error) {
	var decoded envelope
	if err := json.Unmarshal(line, &decoded); err != nil {
		return envelope{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidProtocol, err)
	}
	if decoded.Protocol != protocolName || decoded.Version != protocolVersion {
		return envelope{}, fmt.Errorf(
			"%w: got protocol %q version %d", ErrInvalidProtocol, decoded.Protocol, decoded.Version,
		)
	}
	if strings.TrimSpace(decoded.Type) == "" {
		return envelope{}, fmt.Errorf("%w: event type is required", ErrInvalidProtocol)
	}

	return decoded, nil
}

func encodeEnvelope(value envelope) ([]byte, error) {
	value.Protocol = protocolName
	value.Version = protocolVersion
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode runner command: %w", err)
	}
	return append(encoded, '\n'), nil
}
