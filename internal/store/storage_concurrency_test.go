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
	db, err := Open(context.Background(), path)
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
		if _, err := db.ExecContext(context.Background(), `
INSERT INTO projects(id, api_version, kind, name, spec_json)
VALUES (?, 'devroom/v1alpha1', 'Project', ?, '{}')`, id, id); err != nil {
			t.Fatalf("write storage item %d: %v", index, err)
		}
		var count int
		if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM projects WHERE id LIKE 'storage-%'`).Scan(&count); err != nil {
			t.Fatalf("read storage item %d: %v", index, err)
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
