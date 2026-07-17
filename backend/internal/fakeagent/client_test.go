package fakeagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/fakeagent"
)

func TestAgentReplaysToolPlanWithBearerToken(t *testing.T) {
	t.Parallel()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer scoped-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		calls = append(calls, request.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-id"})
	}))
	t.Cleanup(server.Close)

	var created struct {
		ID string `json:"id"`
	}
	var found struct {
		ID string `json:"id"`
	}
	agent := fakeagent.New(server.URL, "scoped-token", server.Client())
	err := agent.Run(context.Background(), []fakeagent.Step{
		{Tool: agentauth.ToolTaskCreate, Input: map[string]any{"title": "Plan day"}, Output: &created},
		{Tool: agentauth.ToolTaskGet, Input: map[string]any{"taskId": "task-id"}, Output: &found},
	})
	if err != nil {
		t.Fatalf("run fake agent: %v", err)
	}
	if created.ID != "task-id" || found.ID != "task-id" {
		t.Errorf("outputs = (%q, %q), want task-id", created.ID, found.ID)
	}
	if len(calls) != 2 || calls[0] != "/internal/tools/task_create" ||
		calls[1] != "/internal/tools/task_get" {
		t.Errorf("calls = %#v", calls)
	}
}

func TestAgentReturnsToolHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "version conflict", http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	err := fakeagent.New(server.URL, "token", server.Client()).Call(
		context.Background(), agentauth.ToolTaskUpdate, map[string]any{}, nil,
	)
	var httpErr *fakeagent.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict {
		t.Fatalf("error = %v, want conflict HTTPError", err)
	}
}
