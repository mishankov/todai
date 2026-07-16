package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/mishankov/todai/backend/internal/app"
	"github.com/mishankov/todai/backend/internal/bootstrap"
	"github.com/mishankov/todai/backend/internal/config"
)

func runBootstrap(ctx context.Context, cfg config.Config, args []string, stdin *os.File, stdout, stderr io.Writer) (runErr error) {
	flags := flag.NewFlagSet("bootstrap-user", flag.ContinueOnError)
	flags.SetOutput(stderr)
	username := flags.String("username", os.Getenv("TODAI_BOOTSTRAP_USERNAME"), "username for the single user")
	passwordStdin := flags.Bool("password-stdin", false, "read the password from standard input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("username is required; pass --username or set TODAI_BOOTSTRAP_USERNAME")
	}

	password, err := readPassword(stdin, stdout, *passwordStdin)
	if err != nil {
		return err
	}

	_, resources, err := app.New(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := resources.Database.Connection().Close(); err != nil && runErr == nil {
			runErr = fmt.Errorf("close database: %w", err)
		}
	}()

	service := bootstrap.New(resources.Database.Connection(), resources.Auth.Service)
	if err := service.CreateUser(ctx, *username, password); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "Created user %q.\n", *username)
	return err
}

func readPassword(stdin *os.File, stdout io.Writer, fromStdin bool) (string, error) {
	if fromStdin {
		scanner := bufio.NewScanner(stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("read password: %w", err)
			}
			return "", errors.New("password was not provided on standard input")
		}

		password := strings.TrimSuffix(scanner.Text(), "\r")
		if password == "" {
			return "", errors.New("password must not be empty")
		}
		return password, nil
	}

	fd := int(stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("standard input is not a terminal; use --password-stdin")
	}

	password, err := promptSecret(fd, stdout, "Password: ")
	if err != nil {
		return "", err
	}
	confirmation, err := promptSecret(fd, stdout, "Confirm password: ")
	if err != nil {
		return "", err
	}
	if password != confirmation {
		return "", errors.New("passwords do not match")
	}
	if password == "" {
		return "", errors.New("password must not be empty")
	}

	return password, nil
}

func promptSecret(fd int, output io.Writer, prompt string) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	secret, err := term.ReadPassword(fd)
	if _, writeErr := fmt.Fprintln(output); writeErr != nil {
		return "", writeErr
	}
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return string(secret), nil
}
