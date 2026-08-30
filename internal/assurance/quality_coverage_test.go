package assurance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
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

func qualityCoverageGoWorktree(t *testing.T) string {
	t.Helper()
	root := qualityCoverageTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.23.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func qualityCoverageFakeTool(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(qualityCoverageTempDir(t), name)
	if err := os.WriteFile(path, []byte("fixture executable identity"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestQualityCoverageFixturesStayOutsideRepositoryWhenGOTMPDIRIsLocal(t *testing.T) {
	repositoryRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTMPDIR", repositoryRoot)

	directory := qualityCoverageTempDir(t)
	if relative, err := filepath.Rel(repositoryRoot, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
		t.Fatalf("quality coverage temporary directory %q is inside repository %q", directory, repositoryRoot)
	}
}

func TestParseGoCoverageProfileTable(t *testing.T) {
	tests := []struct {
		name        string
		profile     string
		wantFiles   int
		wantTotal   int
		wantCovered int
		wantPercent float64
		wantErr     bool
	}{
		{name: "set profile", profile: "mode: set\npkg/a.go:1.1,2.2 2 1\npkg/a.go:3.1,4.2 1 0\npkg/b.go:1.1,2.2 3 1\n", wantFiles: 2, wantTotal: 6, wantCovered: 5, wantPercent: 83.33333333333333},
		{name: "count profile", profile: "mode: count\npkg/a.go:1.1,2.2 1 2\n", wantFiles: 1, wantTotal: 1, wantCovered: 1, wantPercent: 100},
		{name: "header only", profile: "mode: atomic\n", wantPercent: 0},
		{name: "malformed record", profile: "mode: set\npkg/a.go:1.1,2.2 nope 1\n", wantErr: true},
		{name: "missing mode", profile: "pkg/a.go:1.1,2.2 1 1\n", wantErr: true},
		{name: "invalid mode", profile: "mode: bogus\n", wantErr: true},
		{name: "invalid location", profile: "mode: set\npkg/a.go:0.1,2.2 1 1\n", wantErr: true},
		{name: "invalid count", profile: "mode: set\npkg/a.go:1.1,2.2 1 -1\n", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseGoCoverageProfile([]byte(test.profile))
			if test.wantErr {
				if err == nil {
					t.Fatal("malformed profile was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.FileCount != test.wantFiles || got.TotalStatements != test.wantTotal || got.CoveredStatements != test.wantCovered || got.Percent != test.wantPercent {
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}

func TestParseGoCoverageProfileRejectsUnboundedAndInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "too large", data: []byte("mode: set\n" + strings.Repeat("x", qualityCoverageMaxProfileBytes))},
		{name: "invalid utf8", data: []byte{'m', 'o', 'd', 'e', ':', ' ', 's', 'e', 't', '\n', 0xff}},
		{name: "duplicate header", data: []byte("mode: set\nmode: set\n")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseGoCoverageProfile(test.data); err == nil {
				t.Fatal("invalid coverage profile was accepted")
			}
		})
	}
}

func TestNativeGoCoverageCommandRunsAgainstTemporaryModule(t *testing.T) {
	goPath, err := exec.LookPath(QualityRunnerGoToolID)
	if err != nil {
		t.Skipf("native %s is unavailable: %v", QualityRunnerGoToolID, err)
	}
	root := qualityCoverageTempDir(t)
	writeCoverageFixtureFile(t, root, "go.mod", "module fixture.example/coverage\n\ngo 1.23\n")
	writeCoverageFixtureFile(t, root, "covered.go", `package fixture

func Covered() int { return 42 }
`)
	writeCoverageFixtureFile(t, root, "covered_test.go", `package fixture

import "testing"

func TestCovered(t *testing.T) {
	if Covered() != 42 {
		t.Fatal("unexpected fixture value")
	}
}
`)
	profilePath := filepath.Join(qualityCoverageTempDir(t), "coverage.out")
	selection, err := NewQualityRunnerRegistry().Select(QualityRunnerSelectionRequest{
		TechniqueID:  domain.QualityTechniqueGoTestCoverage,
		WorktreeRoot: root,
		CoveragePath: profilePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"test", "-mod=readonly", "-count=1", "-covermode=set", "-coverprofile=" + profilePath, "./..."}
	if selection.State != QualityRunnerSelectionAvailable || selection.Command.Executable != goPath || !reflect.DeepEqual(selection.Command.Arguments, wantArgs) {
		t.Fatalf("coverage selection = %#v, want argv %#v", selection, wantArgs)
	}
	goCache := qualityCoverageTempDir(t)
	env := append(environment.SafeEnvironment(nil), "GOCACHE="+goCache, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
	result, err := (environment.ProcessRunner{OutputLimit: 128 << 10}).RunInDirectory(context.Background(), selection.Command.Executable, selection.Command.Arguments, env, root, time.Minute)
	if err != nil {
		t.Fatalf("native coverage command failed: %v (%s)", err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("native coverage command exit code = %d", result.ExitCode)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := ParseGoCoverageProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Mode != "set" || summary.FileCount != 1 || summary.TotalStatements == 0 || summary.CoveredStatements == 0 {
		t.Fatalf("native coverage summary = %#v", summary)
	}
}

func TestQualityRunnerCoverageSelectionRejectsUnsafeProfilePaths(t *testing.T) {
	root := qualityCoverageGoWorktree(t)
	goPath := qualityCoverageFakeTool(t, QualityRunnerGoToolID)
	registry := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath})
	tests := []struct {
		name string
		path string
		code string
	}{
		{name: "missing", code: QualityRunnerReasonCoverageMissing},
		{name: "relative", path: "coverage.out", code: QualityRunnerReasonCoverageUnsafe},
		{name: "wrong extension", path: filepath.Join(qualityCoverageTempDir(t), "coverage.txt"), code: QualityRunnerReasonCoverageUnsafe},
		{name: "parent traversal", path: root + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "coverage.out", code: QualityRunnerReasonCoverageUnsafe},
		{name: "newline", path: filepath.Join(qualityCoverageTempDir(t), "coverage.out") + "\n", code: QualityRunnerReasonCoverageUnsafe},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection, err := registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueGoTestCoverage, WorktreeRoot: root, CoveragePath: test.path})
			if err != nil {
				t.Fatal(err)
			}
			if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable == nil || selection.Unavailable.Code != test.code || selection.Command.Executable != "" || len(selection.Command.Arguments) != 0 {
				t.Fatalf("unsafe coverage selection = %#v", selection)
			}
		})
	}
}

func writeCoverageFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
