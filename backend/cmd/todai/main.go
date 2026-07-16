package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/platforma-dev/platforma/application"

	"github.com/mishankov/todai/backend/internal/app"
	"github.com/mishankov/todai/backend/internal/config"
)

func main() {
	if err := run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin *os.File, stdout, stderr *os.File) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) > 1 && args[1] == "bootstrap-user" {
		return runBootstrap(ctx, cfg, args[2:], stdin, stdout, stderr)
	}

	productApp, resources, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer resources.Database.Connection().Close()

	if err := productApp.Run(ctx); err != nil {
		if errors.Is(err, application.ErrUnknownCommand) {
			return fmt.Errorf("run todai: %w", err)
		}
		return err
	}

	return nil
}
