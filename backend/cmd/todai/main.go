package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/platforma-dev/platforma/application"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/app"
	"github.com/mishankov/todai/backend/internal/config"
)

type logContextKey string

const backendComponentKey logContextKey = "backend-component"

func main() {
	ctx := backendLoggingContext(context.Background())
	log.SetDefault(newBackendLogger(os.Stdout, "text"))
	if err := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.ErrorContext(ctx, "application finished with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin *os.File, stdout, stderr *os.File) (runErr error) {
	ctx = backendLoggingContext(ctx)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.SetDefault(newBackendLogger(stdout, cfg.LogFormat))

	if len(args) > 1 && args[1] == "bootstrap-user" {
		return runBootstrap(ctx, cfg, args[2:], stdin, stdout, stderr)
	}

	productApp, resources, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := resources.Database.Connection().Close(); err != nil && runErr == nil {
			runErr = fmt.Errorf("close database: %w", err)
		}
	}()

	if err := productApp.Run(ctx); err != nil {
		if errors.Is(err, application.ErrUnknownCommand) {
			return fmt.Errorf("run todai: %w", err)
		}
		return err
	}

	return nil
}

func backendLoggingContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, backendComponentKey, "backend")
}

func newBackendLogger(output io.Writer, loggerType string) *log.WideEventLogger {
	// Keep every diagnostic record. Platforma models package-level logs as
	// log.record events, so a tail sampler could discard even ErrorContext calls.
	return log.NewWideEventLogger(
		output,
		nil,
		loggerType,
		map[string]any{"component": backendComponentKey},
	)
}
