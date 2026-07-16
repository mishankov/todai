package app_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestApplicationStartsServesAPIAndStops(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	coverageDir := coverageDirectory(t)
	binary := buildBinary(t, ctx)
	database := startPostgres(t, ctx)
	databaseURL, err := database.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	port := availablePort(t)
	var logs synchronizedBuffer
	command := exec.Command(binary, "run")
	command.Env = append(
		os.Environ(),
		"GOCOVERDIR="+coverageDir,
		"TODAI_DATABASE_URL="+databaseURL,
		"TODAI_HTTP_PORT="+port,
	)
	command.Stdout = &logs
	command.Stderr = &logs

	if err := command.Start(); err != nil {
		t.Fatalf("start application: %v", err)
	}

	processDone := make(chan error, 1)
	go func() {
		processDone <- command.Wait()
	}()

	processStopped := false
	t.Cleanup(func() {
		if processStopped {
			return
		}

		_ = command.Process.Kill()
		<-processDone
	})

	client := &http.Client{Timeout: time.Second}
	baseURL := "http://127.0.0.1:" + port
	waitForStatus(t, client, http.MethodGet, baseURL+"/health", http.StatusOK, &logs)
	assertStatus(t, client, http.MethodGet, baseURL+"/api/protected/ping", http.StatusUnauthorized)
	assertStatus(t, client, http.MethodPost, baseURL+"/api/auth/register", http.StatusNotFound)
	assertStatus(t, client, http.MethodGet, baseURL+"/protected/ping", http.StatusNotFound)

	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt application: %v\n%s", err, logs.String())
	}

	select {
	case err := <-processDone:
		processStopped = true
		if err != nil {
			t.Fatalf("application stopped with error: %v\n%s", err, logs.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("application did not stop gracefully\n%s", logs.String())
	}

	assertCoverageData(t, coverageDir)
}

func buildBinary(t *testing.T, ctx context.Context) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "todai")
	command := exec.CommandContext(ctx, "go", "build", "-race", "-cover", "-o", binary, "./cmd/todai")
	command.Dir = moduleRoot(t)

	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build application: %v\n%s", err, output)
	}

	return binary
}

func coverageDirectory(t *testing.T) string {
	t.Helper()

	directory := os.Getenv("TODAI_INTEGRATION_COVERAGE_DIR")
	if directory == "" {
		directory = t.TempDir()
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create integration coverage directory: %v", err)
	}

	return directory
}

func assertCoverageData(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read integration coverage directory: %v", err)
	}

	var metadataFound bool
	var countersFound bool
	for _, entry := range entries {
		metadataFound = metadataFound || strings.HasPrefix(entry.Name(), "covmeta.")
		countersFound = countersFound || strings.HasPrefix(entry.Name(), "covcounters.")
	}

	if !metadataFound || !countersFound {
		t.Fatalf("integration coverage data is incomplete in %s", directory)
	}
}

func startPostgres(t *testing.T, ctx context.Context) *postgres.PostgresContainer {
	t.Helper()

	container, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("todai"),
		postgres.WithUsername("todai"),
		postgres.WithPassword("todai"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}

	testcontainers.CleanupContainer(t, container)

	return container
}

func availablePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve HTTP port: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release HTTP port: %v", err)
	}

	return strconv.Itoa(port)
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}

func waitForStatus(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	want int,
	logs *synchronizedBuffer,
) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		status, err := requestStatus(client, method, url)
		if err == nil && status == want {
			return
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status = %d, want %d", status, want)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("wait for %s: %v\n%s", url, lastErr, logs.String())
}

func assertStatus(t *testing.T, client *http.Client, method, url string, want int) {
	t.Helper()

	status, err := requestStatus(client, method, url)
	if err != nil {
		t.Fatalf("request %s: %v", url, err)
	}
	if status != want {
		t.Fatalf("status for %s = %d, want %d", url, status, want)
	}
}

func requestStatus(client *http.Client, method, url string) (int, error) {
	request, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("send request: %w", err)
	}

	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return 0, fmt.Errorf("read response: %w", readErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("close response: %w", closeErr)
	}

	return response.StatusCode, nil
}

type synchronizedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.String()
}
