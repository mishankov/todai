package app

import (
	"context"
	"errors"

	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
)

type taskContextReader interface {
	Get(context.Context, string, string) (task.Task, error)
}

type agentProjectReader interface {
	Get(context.Context, string, string) (project.Project, error)
	ResolveAgent(context.Context, string, string) (string, string, error)
}

type accountAgentPreferences interface {
	ResolveAgent(context.Context, string) (string, string, string, error)
}

type agentProjectValidator struct {
	projects agentProjectReader
}

func (v agentProjectValidator) ValidateProject(ctx context.Context, userID, projectID string) error {
	if _, err := v.projects.Get(ctx, userID, projectID); errors.Is(err, project.ErrProjectNotFound) {
		return agent.ErrProjectNotFound
	} else if err != nil {
		return err
	}
	return nil
}

type agentPreferencesResolver struct {
	account  accountAgentPreferences
	projects agentProjectReader
}

func (r agentPreferencesResolver) ResolveAgent(
	ctx context.Context,
	userID string,
	projectID string,
) (string, string, string, error) {
	timezone, _, _, err := r.account.ResolveAgent(ctx, userID)
	if err != nil {
		return "", "", "", err
	}
	model, thinkingEffort, err := r.projects.ResolveAgent(ctx, userID, projectID)
	if err != nil {
		return "", "", "", err
	}
	return timezone, model, thinkingEffort, nil
}

type agentContextValidator struct {
	tasks taskContextReader
}

func (v agentContextValidator) ValidateMessageContext(
	ctx context.Context,
	userID string,
	projectID string,
	messageContext agent.MessageContext,
) error {
	if err := messageContext.Validate(); err != nil {
		return err
	}
	found, err := v.tasks.Get(ctx, userID, messageContext.TaskID)
	if errors.Is(err, task.ErrTaskNotFound) {
		return agent.ErrMessageContextNotFound
	} else if err != nil {
		return err
	}
	if found.ProjectID == nil || *found.ProjectID != projectID {
		return agent.ErrMessageContextNotFound
	}
	return nil
}
