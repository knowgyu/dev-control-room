package assurance

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestQualityRunnerRegistrySelectsReviewedGoVetForRegisteredGoWorktree(t *testing.T) {
	root := qualityGoWorktree(t)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	lookups := []string{}
	registry := QualityRunnerRegistry{
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			if name != QualityRunnerGoToolID {
				t.Fatalf("selection looked up unregistered tool %q", name)
			}
			return goPath, nil
		},
		stat:  os.Stat,
		lstat: os.Lstat,
	}
	selection, err := registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionAvailable {
		t.Fatalf("selection state = %#v", selection)
	}
	if selection.Command.Executable != goPath || !reflect.DeepEqual(selection.Command.Arguments, []string{"vet", "-mod=readonly", "./..."}) {
		t.Fatalf("typed command = %#v", selection.Command)
	}
	if selection.Definition.RunnerID != QualityRunnerGoVetID || selection.Definition.ConfigID != QualityRunnerGoVetConfigID || selection.Definition.Timeout != QualityRunnerGoVetTimeout {
		t.Fatalf("definition = %#v", selection.Definition)
	}
	if selection.Unavailable != nil || selection.Metadata.ConfigDigest == "" || selection.Metadata.RunnerID != QualityRunnerGoVetID {
		t.Fatalf("metadata/unavailable = %#v / %#v", selection.Metadata, selection.Unavailable)
	}
	if !reflect.DeepEqual(lookups, []string{QualityRunnerGoToolID}) {
		t.Fatalf("lookups = %#v", lookups)
	}
}

func TestQualityRunnerRegistrySelectsNativeGoCoverageWithFixedProfilePath(t *testing.T) {
	root := qualityGoWorktree(t)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	profilePath := filepath.Join(t.TempDir(), "coverage.out")
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{
		TechniqueID: domain.QualityTechniqueGoTestCoverage, WorktreeRoot: root, CoveragePath: profilePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test", "-mod=readonly", "-count=1", "-covermode=set", "-coverprofile=" + profilePath, "./..."}
	if selection.State != QualityRunnerSelectionAvailable || selection.Definition.RunnerID != QualityRunnerGoTestCoverageID || !reflect.DeepEqual(selection.Command.Arguments, want) {
		t.Fatalf("coverage selection = %#v, want argv %#v", selection, want)
	}
	if selection.Definition.Timeout != QualityRunnerGoTestCoverageTimeout || selection.Metadata.ConfigDigest == "" {
		t.Fatalf("coverage definition/metadata = %#v / %#v", selection.Definition, selection.Metadata)
	}
}

func TestQualityRunnerRegistrySelectsActualGoPropertyTarget(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, filepath.Join("internal", "checks", "property_test.go"), `package checks

import "testing"

func TestPropertyRoundTrip(t *testing.T) {}
`)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionAvailable || selection.Definition.RunnerID != QualityRunnerGoPropertyID {
		t.Fatalf("property selection = %#v", selection)
	}
	want := []string{"test", "-mod=readonly", "./internal/checks", "-run", "^TestPropertyRoundTrip$", "-count=1"}
	if !reflect.DeepEqual(selection.Command.Arguments, want) {
		t.Fatalf("property argv = %#v, want %#v", selection.Command.Arguments, want)
	}
	if selection.Definition.Timeout != QualityRunnerGoPropertyTimeout || selection.Metadata.ConfigDigest == "" {
		t.Fatalf("property definition/metadata = %#v / %#v", selection.Definition, selection.Metadata)
	}
}

func TestQualityRunnerRegistrySelectsActualGoFuzzTarget(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, "fuzz_test.go", `package fixture

import "testing"

func FuzzParser(f *testing.F) {}
`)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueFuzz, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test", "-mod=readonly", "./.", "-run", "^$", "-fuzz", "^FuzzParser$", "-fuzztime=10s"}
	if selection.State != QualityRunnerSelectionAvailable || !reflect.DeepEqual(selection.Command.Arguments, want) {
		t.Fatalf("fuzz selection = %#v", selection)
	}
	if selection.Definition.RunnerID != QualityRunnerGoFuzzID || selection.Definition.Timeout != QualityRunnerGoFuzzTimeout {
		t.Fatalf("fuzz definition = %#v", selection.Definition)
	}
}

func TestQualityRunnerRegistrySelectsActualGoE2ETarget(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, "e2e_test.go", `package fixture

import "testing"

func TestE2EHealthCheck(t *testing.T) {}
`)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueTargetedE2E, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test", "-mod=readonly", "./.", "-run", "^TestE2EHealthCheck$", "-count=1"}
	if selection.State != QualityRunnerSelectionAvailable || !reflect.DeepEqual(selection.Command.Arguments, want) {
		t.Fatalf("E2E selection = %#v", selection)
	}
	if selection.Definition.RunnerID != QualityRunnerGoE2EID || selection.Definition.Timeout != QualityRunnerGoTargetedE2ETimeout {
		t.Fatalf("E2E definition = %#v", selection.Definition)
	}
}

func TestQualityRunnerMutationSelectsOnlyVerifiedNativeTool(t *testing.T) {
	root := qualityGoWorktree(t)
	mutationPath := qualityFakeTool(t, QualityRunnerMutationToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerMutationToolID: mutationPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueMutation, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionAvailable || selection.Definition.RunnerID != QualityRunnerGoMutationID {
		t.Fatalf("mutation selection = %#v", selection)
	}
	if selection.Command.Executable != mutationPath || !reflect.DeepEqual(selection.Command.Arguments, []string{"./..."}) {
		t.Fatalf("mutation command = %#v", selection.Command)
	}
	if selection.Definition.Timeout != QualityRunnerGoMutationTimeout {
		t.Fatalf("mutation timeout = %s", selection.Definition.Timeout)
	}
}

func TestQualityRunnerSelectionReportsMissingGoAndGoModAsUnavailable(t *testing.T) {
	root := t.TempDir()
	goPath := filepath.Join(t.TempDir(), QualityRunnerGoToolID)
	if err := os.WriteFile(goPath, []byte("fixture executable identity"), 0o700); err != nil {
		t.Fatal(err)
	}
	registry := QualityRunnerRegistry{
		lookPath: func(string) (string, error) { return goPath, nil },
		stat:     os.Stat,
		lstat:    os.Lstat,
	}
	selection, err := registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable == nil || selection.Unavailable.Code != QualityRunnerReasonGoModMissing {
		t.Fatalf("missing go.mod selection = %#v", selection)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingGo := QualityRunnerRegistry{
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
		stat:     os.Stat,
		lstat:    os.Lstat,
	}
	selection, err = missingGo.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable == nil || selection.Unavailable.Code != QualityRunnerReasonGoUnavailable || selection.Command.Executable != "" {
		t.Fatalf("missing go selection = %#v", selection)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selection, err = registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable == nil || selection.Unavailable.Code != QualityRunnerReasonGoModInvalid {
		t.Fatalf("invalid go.mod selection = %#v", selection)
	}
}

func TestValidateQualityGoModuleHandlesCommentsAndRejectsMalformedDirectives(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "inline line comment",
			content: "module example.com/fixture // module comment\n",
		},
		{
			name:    "line comment block before directive",
			content: "// module metadata\n// continues here\nmodule example.com/fixture\n",
		},
		{
			name:    "inline block comment",
			content: "module /* module comment */ example.com/fixture\n",
		},
		{
			name:    "multiline block comment before directive",
			content: "/* module metadata\n   continues here */\nmodule example.com/fixture\n",
		},
		{
			name:    "missing module path",
			content: "module // missing path\n",
			wantErr: true,
		},
		{
			name:    "extra module argument",
			content: "module example.com/fixture unexpected\n",
			wantErr: true,
		},
		{
			name:    "non-module directive",
			content: "go 1.23\n",
			wantErr: true,
		},
		{
			name:    "unsafe module path",
			content: "module ../fixture\n",
			wantErr: true,
		},
		{
			name:    "unterminated block comment",
			content: "module example.com/fixture /* missing terminator\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "go.mod")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			err := validateQualityGoModule(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateQualityGoModule() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestQualityRunnerTargetDiscoveryReportsMissingAndAmbiguousTargets(t *testing.T) {
	root := qualityGoWorktree(t)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	registry := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath})
	selection, err := registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonTargetMissing || selection.Command.Executable != "" {
		t.Fatalf("missing target = %#v", selection)
	}

	qualityWriteFile(t, root, "one_test.go", "package fixture\nimport \"testing\"\nfunc TestPropertyOne(t *testing.T) {}\n")
	qualityWriteFile(t, root, "two_test.go", "package fixture\nimport \"testing\"\nfunc TestPropertyTwo(t *testing.T) {}\n")
	selection, err = registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonTargetAmbiguous || selection.Command.Executable != "" {
		t.Fatalf("ambiguous target = %#v", selection)
	}
}

func TestQualityRunnerDiscoveryExcludesGeneratedGitAndVendorSources(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, "generated_test.go", `// Code generated by fixture; DO NOT EDIT.
package fixture
import "testing"
func TestPropertyGenerated(t *testing.T) {}
`)
	qualityWriteFile(t, root, filepath.Join("vendor", "vendor_test.go"), "package vendor\nimport \"testing\"\nfunc TestPropertyVendor(t *testing.T) {}\n")
	qualityWriteFile(t, root, filepath.Join(".git", "metadata_test.go"), "package git\nimport \"testing\"\nfunc TestPropertyGit(t *testing.T) {}\n")
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonTargetMissing {
		t.Fatalf("excluded target selection = %#v", selection)
	}
}

func TestQualityRunnerDiscoveryRejectsMalformedSourceAndSymlink(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, "broken_test.go", "package fixture\nfunc TestPropertyBroken( {\n")
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonSourceInvalid {
		t.Fatalf("malformed source selection = %#v", selection)
	}

	symlinkRoot := qualityGoWorktree(t)
	qualityWriteFile(t, root, "target_test.go", "package fixture\nimport \"testing\"\nfunc TestPropertyTarget(t *testing.T) {}\n")
	link := filepath.Join(symlinkRoot, "linked_test.go")
	if err := os.Symlink(filepath.Join(root, "target_test.go"), link); err != nil {
		t.Skipf("native symlink creation unavailable: %v", err)
	}
	selection, err = qualityRegistryWithTools(symlinkRoot, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: symlinkRoot})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonSourceSymlink {
		t.Fatalf("symlink source selection = %#v", selection)
	}
}

func TestQualityRunnerDiscoveryRejectsSourceBounds(t *testing.T) {
	root := qualityGoWorktree(t)
	qualityWriteFile(t, root, "oversized_test.go", "package fixture\n// "+strings.Repeat("x", qualityRunnerMaxSourceFile)+"\n")
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	selection, err := qualityRegistryWithTools(root, map[string]string{QualityRunnerGoToolID: goPath}).Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueProperty, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonSourceBounds {
		t.Fatalf("source bounds selection = %#v", selection)
	}
}

func TestQualityRunnerMutationReportsAbsentOrUnsafeToolWithoutSuccess(t *testing.T) {
	root := qualityGoWorktree(t)
	missing := qualityRegistryWithTools(root, nil)
	selection, err := missing.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueMutation, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonMutationMissing || selection.Command.Executable != "" {
		t.Fatalf("missing mutation tool = %#v", selection)
	}

	unsafe := filepath.Join(t.TempDir(), "go-mutesting.cmd")
	if err := os.WriteFile(unsafe, []byte("not a native tool"), 0o700); err != nil {
		t.Fatal(err)
	}
	registry := QualityRunnerRegistry{
		lookPath: func(name string) (string, error) {
			if name == QualityRunnerMutationToolID {
				return unsafe, nil
			}
			return "", errors.New("not configured")
		},
		stat:  os.Stat,
		lstat: os.Lstat,
	}
	selection, err = registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueMutation, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonMutationUnsafe || selection.Command.Executable != "" {
		t.Fatalf("unsafe mutation tool = %#v", selection)
	}
}

func TestQualityRunnerSelectionNeverStartsAProcessAndIsDeterministic(t *testing.T) {
	root := qualityGoWorktree(t)
	goPath := qualityFakeTool(t, QualityRunnerGoToolID)
	lookups := 0
	registry := QualityRunnerRegistry{
		lookPath: func(name string) (string, error) {
			lookups++
			if name != QualityRunnerGoToolID {
				t.Fatalf("unexpected tool lookup %q", name)
			}
			return goPath, nil
		},
		stat:  os.Stat,
		lstat: os.Lstat,
	}
	request := QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root}
	first, err := registry.Select(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Select(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || first.Metadata.ConfigDigest == "" {
		t.Fatalf("selection is not deterministic: %#v != %#v", first, second)
	}
	if lookups != 2 {
		t.Fatalf("selection process/tool lookup count = %d", lookups)
	}
}

func TestQualityRunnerRejectsShellSurfacesArbitraryOptionsAndUnsafePaths(t *testing.T) {
	validGoPath := `C:\Tools\go.exe`
	validMutationPath := `C:\Tools\go-mutesting.exe`
	validCommands := []TypedCommand{
		{Executable: validGoPath, Arguments: []string{"vet", "-mod=readonly", "./..."}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "-count=1", "-covermode=set", "-coverprofile=C:\\tmp\\coverage.out", "./..."}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^TestPropertyRoundTrip$", "-count=1"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^$", "-fuzz", "^FuzzParser$", "-fuzztime=10s"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^TestE2EHealthCheck$", "-count=1"}},
		{Executable: validMutationPath, Arguments: []string{"./..."}},
	}
	for _, command := range validCommands {
		if err := ValidateQualityRunnerCommand(command); err != nil {
			t.Errorf("rejected reviewed command %#v: %v", command, err)
		}
	}
	invalid := []TypedCommand{
		{Executable: `C:\Windows\System32\cmd.exe`, Arguments: []string{"/c", "go vet ./..."}},
		{Executable: `C:\Tools\go.bat`, Arguments: []string{"vet", "./..."}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^TestPropertyRoundTrip$", "-count=1", "--exec", "cmd.exe"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./../escape", "-run", "^TestPropertyRoundTrip$", "-count=1"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "TestPropertyRoundTrip", "-count=1"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^TestPropertyroundTrip$", "-count=1"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^$", "-fuzz", "^FuzzParser$", "-fuzztime=30s"}},
		{Executable: validGoPath, Arguments: []string{"test", "-mod=readonly", "-count=1", "-covermode=set", "-coverprofile=relative.out", "./..."}},
		{Executable: validMutationPath, Arguments: []string{"./...", "--exec", "go test"}},
		{Executable: `C:\Tools\go.exe`, Arguments: []string{"vet", "./...", "&&", "whoami"}},
	}
	for _, command := range invalid {
		if err := ValidateQualityRunnerCommand(command); err == nil {
			t.Errorf("accepted unsafe or unreviewed command %#v", command)
		}
	}
	if err := ValidateQualityRunnerCommand(TypedCommand{Executable: `C:\Tools\go.exe`, Arguments: []string{"test", "-mod=readonly", "./pkg", "-run", "^TestPropertyRoundTrip$", "-count=1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewQualityRunnerRegistry().Select(QualityRunnerSelectionRequest{TechniqueID: "arbitrary-technique", WorktreeRoot: t.TempDir()}); !errors.Is(err, ErrQualityTechniqueNotRegistered) {
		t.Fatalf("arbitrary technique error = %v", err)
	}
	if _, err := NewQualityRunnerRegistry().Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: "relative/root"}); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("relative root error = %v", err)
	}
}

func TestQualityRunnerRejectsSymlinkedNativeTool(t *testing.T) {
	root := qualityGoWorktree(t)
	realTool := qualityFakeTool(t, QualityRunnerGoToolID)
	link := filepath.Join(t.TempDir(), QualityRunnerGoToolID)
	if err := os.Symlink(realTool, link); err != nil {
		t.Skipf("native symlink creation unavailable: %v", err)
	}
	registry := QualityRunnerRegistry{
		lookPath: func(string) (string, error) { return link, nil },
		stat:     os.Stat,
		lstat:    os.Lstat,
	}
	selection, err := registry.Select(QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if selection.State != QualityRunnerSelectionUnavailable || selection.Unavailable.Code != QualityRunnerReasonGoUntrusted {
		t.Fatalf("symlinked native tool = %#v", selection)
	}
}

func TestQualityRunnerUnavailableReasonsAreConciseAndCannotCarryRawErrors(t *testing.T) {
	reasons := []*QualityRunnerUnavailableReason{
		{Code: QualityRunnerReasonTargetMissing, Detail: "no unique reviewed Go test target was found"},
		{Code: QualityRunnerReasonMutationMissing, Detail: "reviewed native tool was not found"},
	}
	for _, reason := range reasons {
		if !validQualityRunnerUnavailableReason(reason) {
			t.Fatalf("valid reason rejected: %#v", reason)
		}
	}
	for _, reason := range []*QualityRunnerUnavailableReason{
		{Code: "runner failed", Detail: "raw process output"},
		{Code: QualityRunnerReasonTargetMissing, Detail: "C:\\secret\\source.go\nraw error"},
		{Code: QualityRunnerReasonTargetMissing, Detail: strings.Repeat("x", 241)},
	} {
		if validQualityRunnerUnavailableReason(reason) {
			t.Fatalf("accepted unsafe reason: %#v", reason)
		}
	}
}

func qualityGoWorktree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.23.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func qualityFakeTool(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fixture executable identity"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func qualityWriteFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func qualityRegistryWithTools(root string, tools map[string]string) QualityRunnerRegistry {
	return QualityRunnerRegistry{
		lookPath: func(name string) (string, error) {
			path, ok := tools[name]
			if !ok {
				return "", errors.New("tool not configured")
			}
			return path, nil
		},
		stat:  os.Stat,
		lstat: os.Lstat,
	}
}
