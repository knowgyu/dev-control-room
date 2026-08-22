package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/app"
	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestWriteCLIErrorDoesNotExposeInternalDetails(t *testing.T) {
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	code := writeCLIError(errors.New("open C:\\private\\secret-canary: SQL details"))
	_ = writer.Close()
	os.Stderr = original
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if code != int(contract.ExitInternal) || strings.Contains(string(data), "secret-canary") || strings.Contains(string(data), "SQL details") || !strings.Contains(string(data), `"message":"internal error"`) {
		t.Fatalf("unexpected CLI error output: code=%d output=%s", code, data)
	}
}

func TestVersionJSONReportsCurrentMilestone(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runVersionTo([]string{"--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("version exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]string]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data == nil || (*envelope.Data)["milestone"] != "2" || (*envelope.Data)["version"] != version {
		t.Fatalf("unexpected version envelope: %#v", envelope)
	}
}

func TestProjectListJSONUsesStableEnvelope(t *testing.T) {
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"project", "list", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("project list exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[[]map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil || len(*envelope.Data) != 0 {
		t.Fatalf("unexpected CLI envelope: %#v", envelope)
	}
	want, err := os.ReadFile("testdata/project_list_empty.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != strings.TrimSpace(string(want)) {
		t.Fatalf("CLI JSON golden mismatch: got %s want %s", stdout.String(), want)
	}
}

func TestEnvironmentDoctorJSONUsesStableEnvelopeAndHidesSecret(t *testing.T) {
	const canary = "secret-canary-value"
	if err := os.Setenv("DEVROOM_CLI_SECRET", canary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_CLI_SECRET") })
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"env", "doctor", "--home", home, "--json"}, &stdout, &stderr)
	if code != int(contract.ExitUnavailable) {
		t.Fatalf("env doctor exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil || strings.Contains(stdout.String(), canary) || strings.Contains(stderr.String(), canary) {
		t.Fatalf("invalid or secret-bearing environment envelope: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestProjectWorktreeListJSONUsesReadOnlyEnvelope(t *testing.T) {
	home := t.TempDir()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Dir(filepath.Dir(workingDirectory))
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), app.AddProjectInput{Name: "worktrees", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	_ = service.Close()
	var stdout, stderr bytes.Buffer
	if code := run([]string{"project", "worktree", "list", project.Metadata.ID, "repo-1", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("worktree CLI failed: %d %s", code, stderr.String())
	}
	var envelope contract.Envelope[[]map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil || len(*envelope.Data) != 1 {
		t.Fatalf("worktree CLI envelope = %s (%v)", stdout.String(), err)
	}
}
