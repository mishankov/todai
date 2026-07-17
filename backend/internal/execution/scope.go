// Package execution describes who performs an application operation and on
// whose data it is authorized to act.
package execution

import "errors"

// ActorType identifies the kind of principal that initiated an operation.
type ActorType string

const (
	ActorUser          ActorType = "user"
	ActorBuiltInAgent  ActorType = "built_in_agent"
	ActorExternalAgent ActorType = "external_agent"
	ActorSystem        ActorType = "system"
)

// Source identifies the trusted application boundary that initiated an operation.
type Source string

const (
	SourceWeb         Source = "web"
	SourceInternalAPI Source = "internal_api"
	SourceSystem      Source = "system"
)

var (
	// ErrUserIDRequired indicates that an operation has no owner scope.
	ErrUserIDRequired = errors.New("execution user ID is required")
	// ErrInvalidActorType indicates an unsupported actor type.
	ErrInvalidActorType = errors.New("execution actor type is invalid")
	// ErrActorIDRequired indicates that an actor must have an identity.
	ErrActorIDRequired = errors.New("execution actor ID is required")
	// ErrInvalidActorID indicates an explicitly provided empty actor identity.
	ErrInvalidActorID = errors.New("execution actor ID is invalid")
	// ErrInvalidSource indicates an unsupported operation source.
	ErrInvalidSource = errors.New("execution source is invalid")
	// ErrCorrelationIDRequired indicates that an operation cannot be correlated.
	ErrCorrelationIDRequired = errors.New("execution correlation ID is required")
	// ErrInvalidAgentRunID indicates an explicitly provided empty agent run identity.
	ErrInvalidAgentRunID = errors.New("execution agent run ID is invalid")
)

// Scope carries trusted authorization and attribution for one application operation.
type Scope struct {
	UserID        string
	ActorType     ActorType
	ActorID       *string
	Source        Source
	CorrelationID string
	AgentRunID    *string
}

// UserScope constructs attribution for an authenticated web user.
func UserScope(userID, correlationID string) Scope {
	return Scope{
		UserID:        userID,
		ActorType:     ActorUser,
		ActorID:       &userID,
		Source:        SourceWeb,
		CorrelationID: correlationID,
	}
}

// Validate verifies that the scope is complete and uses supported values.
func (s Scope) Validate() error {
	if s.UserID == "" {
		return ErrUserIDRequired
	}

	switch s.ActorType {
	case ActorUser, ActorBuiltInAgent, ActorExternalAgent, ActorSystem:
	default:
		return ErrInvalidActorType
	}
	if s.ActorID != nil && *s.ActorID == "" {
		return ErrInvalidActorID
	}
	if (s.ActorType == ActorUser || s.ActorType == ActorExternalAgent) && s.ActorID == nil {
		return ErrActorIDRequired
	}

	switch s.Source {
	case SourceWeb, SourceInternalAPI, SourceSystem:
	default:
		return ErrInvalidSource
	}
	if s.CorrelationID == "" {
		return ErrCorrelationIDRequired
	}
	if s.AgentRunID != nil && *s.AgentRunID == "" {
		return ErrInvalidAgentRunID
	}

	return nil
}

// ModifiedBy returns the stable actor identifier stored on mutable records.
func (s Scope) ModifiedBy() string {
	if s.ActorID != nil && *s.ActorID != "" {
		return *s.ActorID
	}

	return string(s.ActorType)
}
