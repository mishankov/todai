package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/platforma-dev/platforma/application"
	"github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/app"
	"github.com/mishankov/todai/backend/internal/config"
)

func main() {
	ctx := context.Background()
	log.SetDefault(log.New(os.Stdout, "text", log.LevelInfo, nil).With("component", "backend"))
	if err := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.ErrorContext(ctx, "application finished with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin *os.File, stdout, stderr *os.File) (runErr error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.SetDefault(log.New(stdout, cfg.LogFormat, log.LevelInfo, nil).With("component", "backend"))

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
