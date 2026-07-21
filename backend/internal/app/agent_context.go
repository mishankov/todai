package app

import (
	"context"
	"errors"

	"github.com/mishankov/todai/backend/internal/agent"
	"github.com/mishankov/todai/backend/internal/task"
)

type taskContextReader interface {
	Get(context.Context, string, string) (task.Task, error)
}

type agentContextValidator struct {
	tasks taskContextReader
}

func (v agentContextValidator) ValidateMessageContext(
	ctx context.Context,
	userID string,
	messageContext agent.MessageContext,
) error {
	if err := messageContext.Validate(); err != nil {
		return err
	}
	if _, err := v.tasks.Get(ctx, userID, messageContext.TaskID); errors.Is(err, task.ErrTaskNotFound) {
		return agent.ErrMessageContextNotFound
	} else if err != nil {
		return err
	}
	return nil
}
