package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func qualityCoverageTempDir(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("", ".quality-coverage-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("remove quality coverage temporary directory: %v", err)
		}
	})
	return directory
}

func TestQualityCoverageFixturesStayOutsideRepositoryWhenGOTMPDIRIsLocal(t *testing.T) {
	repositoryRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTMPDIR", repositoryRoot)

	directory := tempGoCoverageGitRepository(t, "quality-coverage")
	if pathWithin(repositoryRoot, directory) {
		t.Fatalf("quality coverage temporary directory %q is inside repository %q", directory, repositoryRoot)
	}
}

func TestQualityCoverageProfileCreationAndCleanup(t *testing.T) {
	app := &App{home: qualityCoverageTempDir(t)}
	path, cleanup, err := app.newQualityCoverageProfile("run-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".out" || !strings.HasSuffix(path, filepath.Join("runtime", "quality", "run-fixture.cover.out")) {
		t.Fatalf("coverage profile path = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("coverage profile was not cleaned up: %v", err)
	}
}

func TestReadBoundedQualityCoverage(t *testing.T) {
	directory := qualityCoverageTempDir(t)
	validPath := filepath.Join(directory, "coverage.out")
	valid := []byte("mode: set\nfixture.go:1.1,1.2 1 1\n")
	if err := os.WriteFile(validPath, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	data, truncated, err := readBoundedQualityCoverage(validPath)
	if err != nil || truncated || string(data) != string(valid) {
		t.Fatalf("bounded coverage = %q, truncated=%t, err=%v", data, truncated, err)
	}

	largePath := filepath.Join(directory, "large.out")
	large := []byte(strings.Repeat("x", qualityRunCoverageArtifactLimit+1))
	if err := os.WriteFile(largePath, large, 0o600); err != nil {
		t.Fatal(err)
	}
	data, truncated, err = readBoundedQualityCoverage(largePath)
	if err != nil || !truncated || len(data) != qualityRunCoverageArtifactLimit {
		t.Fatalf("large bounded coverage = len(%d), truncated=%t, err=%v", len(data), truncated, err)
	}

	directoryPath := filepath.Join(directory, "directory.out")
	if err := os.Mkdir(directoryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readBoundedQualityCoverage(directoryPath); err == nil {
		t.Fatal("directory was accepted as a coverage profile")
	}
}
