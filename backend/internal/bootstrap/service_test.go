package bootstrap

import (
	"context"
	"errors"
	"testing"
)

type fakeCounter struct {
	count int
	err   error
}

func (f fakeCounter) CountUsers(context.Context) (int, error) {
	return f.count, f.err
}

type fakeCreator struct {
	called bool
	err    error
}

func (f *fakeCreator) CreateWithLoginAndPassword(context.Context, string, string) error {
	f.called = true
	return f.err
}

func TestCreateUser(t *testing.T) {
	t.Parallel()

	creator := &fakeCreator{}
	service := newService(fakeCounter{}, creator)

	if err := service.CreateUser(t.Context(), "admin", "secret-password"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if !creator.called {
		t.Fatal("user creator was not called")
	}
}

func TestCreateUserRejectsInitializedInstallation(t *testing.T) {
	t.Parallel()

	creator := &fakeCreator{}
	service := newService(fakeCounter{count: 1}, creator)

	err := service.CreateUser(t.Context(), "admin", "secret-password")
	if !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("error = %v, want ErrAlreadyInitialized", err)
	}
	if creator.called {
		t.Fatal("user creator was called for an initialized installation")
	}
}

func TestCreateUserReturnsCountError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	service := newService(fakeCounter{err: wantErr}, &fakeCreator{})

	err := service.CreateUser(t.Context(), "admin", "secret-password")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
