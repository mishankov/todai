package execution_test

import (
	"errors"
	"testing"

	"github.com/mishankov/todai/backend/internal/execution"
)

func TestUserScopeIsValidAndAttributesTheUser(t *testing.T) {
	t.Parallel()

	scope := execution.UserScope("user-id", "correlation-id")
	if err := scope.Validate(); err != nil {
		t.Fatalf("validate user scope: %v", err)
	}
	if scope.ActorType != execution.ActorUser || scope.Source != execution.SourceWeb {
		t.Errorf("user scope = %#v", scope)
	}
	if scope.ActorID == nil || *scope.ActorID != "user-id" {
		t.Errorf("actor ID = %#v, want user-id", scope.ActorID)
	}
	if scope.ModifiedBy() != "user-id" {
		t.Errorf("modified by = %q, want user-id", scope.ModifiedBy())
	}
}

func TestScopeRequiresCompleteAttribution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope execution.Scope
		want  error
	}{
		{name: "user ID", scope: validScope(), want: execution.ErrUserIDRequired},
		{name: "actor type", scope: withUserID(validScope()), want: execution.ErrInvalidActorType},
		{
			name: "user actor ID",
			scope: execution.Scope{
				UserID: "user-id", ActorType: execution.ActorUser,
				Source: execution.SourceWeb, CorrelationID: "correlation-id",
			},
			want: execution.ErrActorIDRequired,
		},
		{
			name: "source",
			scope: execution.Scope{
				UserID: "user-id", ActorType: execution.ActorSystem,
				CorrelationID: "correlation-id",
			},
			want: execution.ErrInvalidSource,
		},
		{
			name: "correlation ID",
			scope: execution.Scope{
				UserID: "user-id", ActorType: execution.ActorSystem, Source: execution.SourceSystem,
			},
			want: execution.ErrCorrelationIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.scope.Validate(); !errors.Is(err, test.want) {
				t.Errorf("Validate() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestScopeUsesActorTypeWhenActorHasNoID(t *testing.T) {
	t.Parallel()

	scope := execution.Scope{
		UserID:        "user-id",
		ActorType:     execution.ActorSystem,
		Source:        execution.SourceSystem,
		CorrelationID: "correlation-id",
	}
	if err := scope.Validate(); err != nil {
		t.Fatalf("validate system scope: %v", err)
	}
	if scope.ModifiedBy() != "system" {
		t.Errorf("modified by = %q, want system", scope.ModifiedBy())
	}
}

func validScope() execution.Scope {
	return execution.Scope{}
}

func withUserID(scope execution.Scope) execution.Scope {
	scope.UserID = "user-id"
	return scope
}
