package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenConcurrentInitializationIsSerialized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	const workers = 8

	start := make(chan struct{})
	results := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			db, err := Open(context.Background(), path)
			if err != nil {
				results <- err
				return
			}
			defer db.Close()
			var version int
			if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
				results <- err
				return
			}
			if version != CurrentSchemaVersion {
				results <- fmt.Errorf("schema version = %d, want %d", version, CurrentSchemaVersion)
				return
			}
			results <- nil
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent Open failed: %v", err)
		}
	}
}

func TestOpenReturnsContextDeadlineWhenStorageLockWaitExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := acquireStorageLock(context.Background(), databaseLockPath(absolutePath))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = Open(ctx, path)
	if err == nil {
		t.Fatal("Open succeeded while the storage lock was held")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open error = %T %v, want context deadline", err, err)
	}
	if strings.Contains(err.Error(), absolutePath) || strings.Contains(strings.ToLower(err.Error()), "pragma") || strings.Contains(strings.ToLower(err.Error()), "select") {
		t.Fatalf("storage busy error exposed path or SQL detail: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("bounded storage wait took %s", elapsed)
	}
}

func TestStorageLockPreservesControllingContextDeadlineAtSchedulerBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := acquireStorageLock(context.Background(), databaseLockPath(absolutePath))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	// Model the narrow boundary where the clock passes a context deadline before
	// the context cancellation signal is observable by the lock polling loop.
	ctx := delayedDeadlineContext{
		Context:  context.Background(),
		deadline: time.Now().Add(-time.Millisecond),
		done:     make(chan struct{}),
	}
	_, err = acquireStorageLockWithin(ctx, databaseLockPath(absolutePath), time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock error = %T %v, want context deadline", err, err)
	}
}

type delayedDeadlineContext struct {
	context.Context
	deadline time.Time
	done     <-chan struct{}
}

func (c delayedDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func (c delayedDeadlineContext) Done() <-chan struct{} { return c.done }

func (delayedDeadlineContext) Err() error { return nil }

func TestStorageLockReturnsTypedBusyAfterBoundedWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := acquireStorageLock(context.Background(), databaseLockPath(absolutePath))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	_, err = acquireStorageLockWithin(context.Background(), databaseLockPath(absolutePath), 50*time.Millisecond)
	if err == nil {
		t.Fatal("storage lock acquisition succeeded while the lock was held")
	}
	var busy *StorageBusyError
	if !errors.As(err, &busy) || !IsStorageBusy(err) {
		t.Fatalf("storage lock error = %T %v, want StorageBusyError", err, err)
	}
	if !IsStorageLockBusy(err) {
		t.Fatalf("storage lock error = %T %v, want pre-execution lock-busy classification", err, err)
	}
}

func TestStorageProcessRetryOnlyReplaysPreExecutionLockBusy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	attempts := 0
	err := retryStorageProcessOperation(ctx, func(context.Context) error {
		attempts++
		if attempts == 1 {
			return &StorageBusyError{operation: "storage access", lockBusy: true}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("pre-execution retry error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("pre-execution attempts = %d, want 2", attempts)
	}

	attempts = 0
	executionBusy := &StorageBusyError{operation: "storage access"}
	err = retryStorageProcessOperation(ctx, func(context.Context) error {
		attempts++
		return executionBusy
	})
	if !errors.Is(err, executionBusy) {
		t.Fatalf("execution-busy error = %v, want original error", err)
	}
	if attempts != 1 {
		t.Fatalf("execution-busy attempts = %d, want 1", attempts)
	}
}

func TestMigrateDirectCallHonorsStorageLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := acquireStorageLock(context.Background(), databaseLockPath(absolutePath))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := Migrate(ctx, db); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("direct Migrate error = %T %v, want context deadline while lock is held", err, err)
	}
}

func TestStorageServerAndCLIProcessesSerialize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	readyPath := filepath.Join(t.TempDir(), "server.ready")
	server := startStorageProcess(t, path, "server", 48, readyPath)
	defer stopStorageProcess(server)

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			stdout, stderr := storageProcessOutput(server)
			t.Fatalf("storage server did not become ready; stdout=%q stderr=%q", stdout, stderr)
		}
		time.Sleep(25 * time.Millisecond)
	}

	const clients = 8
	clientProcesses := make([]*exec.Cmd, 0, clients)
	for range clients {
		clientProcesses = append(clientProcesses, startStorageProcess(t, path, "cli", 24, ""))
	}
	for index, client := range clientProcesses {
		if err := waitStorageProcess(client, 30*time.Second); err != nil {
			stdout, stderr := storageProcessOutput(client)
			t.Fatalf("CLI storage process %d failed: %v; stdout=%q stderr=%q", index, err, stdout, stderr)
		}
	}
	if err := waitStorageProcess(server, 30*time.Second); err != nil {
		stdout, stderr := storageProcessOutput(server)
		t.Fatalf("storage server process failed: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}

	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM projects WHERE id LIKE 'storage-%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	want := clients*24 + 48
	if count != want {
		t.Fatalf("persisted projects = %d, want %d", count, want)
	}
}

func TestStorageProcessHelper(t *testing.T) {
	if os.Getenv("DEVROOM_STORAGE_HELPER") != "1" {
		return
	}
	path := os.Getenv("DEVROOM_STORAGE_PATH")
	role := os.Getenv("DEVROOM_STORAGE_ROLE")
	iterations, err := strconv.Atoi(os.Getenv("DEVROOM_STORAGE_ITERATIONS"))
	if err != nil || iterations < 1 {
		t.Fatalf("invalid storage helper iterations")
	}
	db, err := openStorageProcessDatabase(path)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer db.Close()
	if readyPath := os.Getenv("DEVROOM_STORAGE_READY_FILE"); readyPath != "" {
		if err := os.WriteFile(readyPath, []byte("ready\n"), 0o600); err != nil {
			t.Fatalf("write storage helper readiness: %v", err)
		}
	}
	for index := 0; index < iterations; index++ {
		id := fmt.Sprintf("storage-%s-%d-%d", role, os.Getpid(), index)
		if err := runStorageProcessOperation(func(ctx context.Context) error {
			_, err := db.ExecContext(ctx, `
INSERT INTO projects(id, api_version, kind, name, spec_json)
VALUES (?, 'devroom/v1alpha1', 'Project', ?, '{}')`, id, id)
			return err
		}); err != nil {
			t.Fatalf("write storage item %d: %v", index, err)
		}
		var count int
		if err := runStorageProcessOperation(func(ctx context.Context) error {
			return db.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE id LIKE 'storage-%'`).Scan(&count)
		}); err != nil {
			t.Fatalf("read storage item %d: %v", index, err)
		}
	}
}

const (
	storageProcessOpenDeadline     = 20 * time.Second
	storageProcessOpenRetryBackoff = 25 * time.Millisecond
	storageProcessOperationTimeout = 20 * time.Second
)

// Once readiness is published, the server immediately starts its write/read
// loop. A client can lose the cross-process lock race during that loop and
// Open can return typed busy after its bounded production wait. Retry only
// pre-execution lock busy; an ambiguous SQLite execution-time busy is never
// replayed.
func openStorageProcessDatabase(path string) (*sql.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storageProcessOpenDeadline)
	defer cancel()

	for {
		db, err := Open(ctx, path)
		if err == nil {
			return db, nil
		}
		if !IsStorageLockBusy(err) {
			return nil, err
		}

		timer := time.NewTimer(storageProcessOpenRetryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func runStorageProcessOperation(operation func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), storageProcessOperationTimeout)
	defer cancel()
	return retryStorageProcessOperation(ctx, operation)
}

func retryStorageProcessOperation(ctx context.Context, operation func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		err := operation(ctx)
		if err == nil || !IsStorageLockBusy(err) {
			return err
		}
		timer := time.NewTimer(storageProcessOpenRetryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func startStorageProcess(t *testing.T, path, role string, iterations int, readyPath string) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestStorageProcessHelper$", "-test.v")
	command.Env = append(os.Environ(),
		"DEVROOM_STORAGE_HELPER=1",
		"DEVROOM_STORAGE_PATH="+path,
		"DEVROOM_STORAGE_ROLE="+role,
		"DEVROOM_STORAGE_ITERATIONS="+strconv.Itoa(iterations),
		"DEVROOM_STORAGE_READY_FILE="+readyPath,
	)
	command.Stdout = &bytes.Buffer{}
	command.Stderr = &bytes.Buffer{}
	if err := command.Start(); err != nil {
		t.Fatalf("start storage %s process: %v", role, err)
	}
	t.Cleanup(func() { stopStorageProcess(command) })
	return command
}

func waitStorageProcess(command *exec.Cmd, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		_ = command.Process.Kill()
		select {
		case <-result:
			return fmt.Errorf("process timed out after %s", timeout)
		case <-time.After(time.Second):
			return fmt.Errorf("process timed out after %s and did not exit after kill", timeout)
		}
	}
}

func stopStorageProcess(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Kill()
	_ = command.Wait()
}

func storageProcessOutput(command *exec.Cmd) (string, string) {
	stdout, _ := command.Stdout.(*bytes.Buffer)
	stderr, _ := command.Stderr.(*bytes.Buffer)
	if stdout == nil || stderr == nil {
		return "", ""
	}
	return stdout.String(), stderr.String()
}
