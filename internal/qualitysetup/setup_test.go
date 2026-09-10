package qualitysetup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInspectMixedBackendFrontend(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "pyproject.toml"), `[project]
name = "service"
dependencies = ["fastapi>=0.100"]

[tool.ruff]
line-length = 100

[tool.pytest.ini_options]
testpaths = ["tests"]
`)
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{
  "packageManager": "pnpm@9.0.0",
  "dependencies": {"vue": "^3.5.0"},
  "scripts": {"lint": "eslint .", "test:unit": "vitest run"}
}`)
	writeFile(t, filepath.Join(root, "frontend", "tsconfig.json"), `{ "compilerOptions": {} }`)
	writeFile(t, filepath.Join(root, "frontend", "eslint.config.js"), "export default []\n")
	writeFile(t, filepath.Join(root, "frontend", "vitest.config.ts"), "export default {}\n")

	report, err := Inspect(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if report.Partial {
		t.Fatalf("Inspect() unexpectedly partial: %v", report.Warnings)
	}
	if got, want := report.DetectedLanguages, []string{"python", "javascript", "typescript"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("detected languages = %#v, want %#v", got, want)
	}
	if len(report.Components) != 2 {
		t.Fatalf("component count = %d, want 2", len(report.Components))
	}

	backend := componentByPath(t, report.Components, "backend")
	if !reflect.DeepEqual(backend.Frameworks, []string{"FastAPI"}) {
		t.Fatalf("backend frameworks = %#v", backend.Frameworks)
	}
	if backend.PackageManager != "" {
		t.Fatalf("backend package manager = %q, want unknown", backend.PackageManager)
	}
	assertCheckStatus(t, backend, "ruff", statusExisting)
	assertCheckStatus(t, backend, "pytest", statusExisting)

	frontend := componentByPath(t, report.Components, "frontend")
	if !reflect.DeepEqual(frontend.Frameworks, []string{"Vue"}) {
		t.Fatalf("frontend frameworks = %#v", frontend.Frameworks)
	}
	if frontend.PackageManager != "pnpm" {
		t.Fatalf("frontend package manager = %q, want pnpm", frontend.PackageManager)
	}
	assertCheckStatus(t, frontend, "eslint", statusExisting)
	assertCheckStatus(t, frontend, "vitest", statusExisting)
	for _, component := range report.Components {
		for _, check := range component.Checks {
			if check.Executable {
				t.Errorf("check %q unexpectedly executable", check.ID)
			}
			if check.SetupPreview == nil {
				t.Errorf("check %q has nil setup preview", check.ID)
			}
		}
	}
}

func TestInspectManualFilterKeepsComponentsAndDigest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "requirements.txt"), "fastapi==0.115\n# fastapi must not be read from comments\n")
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"vue":"^3"}}`)

	auto, err := Inspect(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("automatic Inspect() error = %v", err)
	}
	filtered, err := Inspect(context.Background(), root, []string{"python"})
	if err != nil {
		t.Fatalf("filtered Inspect() error = %v", err)
	}
	if auto.Digest == "" || auto.Digest != filtered.Digest {
		t.Fatalf("digest changed with language filter: auto=%q filtered=%q", auto.Digest, filtered.Digest)
	}
	if !reflect.DeepEqual(filtered.SelectedLanguages, []string{"python"}) {
		t.Fatalf("selected languages = %#v", filtered.SelectedLanguages)
	}
	if len(auto.Components) != len(filtered.Components) {
		t.Fatalf("component count changed: auto=%d filtered=%d", len(auto.Components), len(filtered.Components))
	}
	frontend := componentByPath(t, filtered.Components, "frontend")
	if len(frontend.Checks) != 0 {
		t.Fatalf("filtered frontend checks = %#v, want none", frontend.Checks)
	}
	backend := componentByPath(t, filtered.Components, "backend")
	if len(backend.Checks) != 2 {
		t.Fatalf("filtered backend checks = %d, want 2", len(backend.Checks))
	}
}

func TestInspectInvalidLanguages(t *testing.T) {
	_, err := Inspect(context.Background(), t.TempDir(), []string{"rust"})
	if !errors.Is(err, ErrInvalidLanguages) {
		t.Fatalf("error = %v, want ErrInvalidLanguages", err)
	}
}

func TestInspectMalformedAndCommentOnlyDeclarations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "pyproject.toml"), `# [tool.ruff]
[project
# fastapi
this is unsupported
`)
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":`)
	writeFile(t, filepath.Join(root, "frontend", "tsconfig.json"), `{`)

	report, err := Inspect(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !report.Partial || len(report.Warnings) == 0 {
		t.Fatalf("malformed report = %#v, want partial warnings", report)
	}
	backend := componentByPath(t, report.Components, "backend")
	if len(backend.Frameworks) != 0 {
		t.Fatalf("comment-only FastAPI framework = %#v", backend.Frameworks)
	}
	assertCheckStatus(t, backend, "ruff", statusAmbiguous)
	frontend := componentByPath(t, report.Components, "frontend")
	assertCheckStatus(t, frontend, "eslint", statusAmbiguous)
	assertCheckStatus(t, frontend, "vitest", statusAmbiguous)
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "this is unsupported") || strings.Contains(warning, "dependencies") {
			t.Errorf("warning contains malformed content: %q", warning)
		}
	}
}

func TestInspectExcludesChildrenAndScansSelectedRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{}`)
	for _, directory := range []string{".git", "node_modules", ".venv", "venv", "vendor", "dist", "build", "artifacts", "testdata", ".cache", "coverage", ".next", ".nuxt"} {
		writeFile(t, filepath.Join(root, directory, "package.json"), `{ "dependencies": {"vue":"^3"} }`)
	}

	report, err := Inspect(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.Components) != 1 || report.Components[0].Path != "." {
		t.Fatalf("components = %#v, want only selected root", report.Components)
	}
	if len(report.DetectedLanguages) != 1 || report.DetectedLanguages[0] != "javascript" {
		t.Fatalf("detected languages = %#v", report.DetectedLanguages)
	}
}

func TestInspectSkipsSymlink(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	writeFile(t, filepath.Join(target, "package.json"), `{"dependencies":{"vue":"^3"}}`)
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	report, err := Inspect(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.Components) != 0 {
		t.Fatalf("components = %#v, want none", report.Components)
	}
	if !containsWarning(report.Warnings, "skipped symlink") || !report.Partial {
		t.Fatalf("warnings = %#v, partial = %v", report.Warnings, report.Partial)
	}
}

func TestInspectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := Inspect(ctx, t.TempDir(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if !report.Partial || !containsWarning(report.Warnings, "inspection canceled") {
		t.Fatalf("report = %#v, want partial cancellation warning", report)
	}
}

func TestInspectBounds(t *testing.T) {
	t.Run("file size", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "package.json"), strings.Repeat("x", maxEvidenceBytes+1))
		report, err := Inspect(context.Background(), root, nil)
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if !report.Partial || !containsWarning(report.Warnings, "128 KiB") {
			t.Fatalf("report = %#v, want file-size partial warning", report)
		}
	})

	t.Run("eligible files", func(t *testing.T) {
		root := t.TempDir()
		for index := 0; index < maxEligibleFiles+4; index++ {
			writeFile(t, filepath.Join(root, "component", formatIndex(index), "package.json"), `{}`)
		}
		report, err := Inspect(context.Background(), root, nil)
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if !report.Partial || !containsWarning(report.Warnings, "eligible file limit") {
			t.Fatalf("report = %#v, want eligible-file partial warning", report)
		}
		if len(report.Components) != maxEligibleFiles {
			t.Fatalf("inspected components=%d, want %d", len(report.Components), maxEligibleFiles)
		}
	})

	t.Run("directory entries", func(t *testing.T) {
		root := t.TempDir()
		for index := 0; index < maxEntries+4; index++ {
			writeFile(t, filepath.Join(root, "files", strings.TrimSpace(formatIndex(index))+".txt"), "ignored")
		}
		report, err := Inspect(context.Background(), root, nil)
		if err != nil {
			t.Fatalf("Inspect() error = %v", err)
		}
		if !report.Partial || !containsWarning(report.Warnings, "directory entry limit") {
			t.Fatalf("report = %#v, want entry-limit partial warning", report)
		}
	})
}

func TestReportJSONShape(t *testing.T) {
	report := Report{
		DetectedLanguages: []string{},
		SelectedLanguages: []string{},
		Components: []Component{{
			ID:             ".",
			Path:           ".",
			Languages:      []string{"go"},
			Frameworks:     []string{},
			PackageManager: "go",
			Evidence:       []Evidence{},
			Checks: []Check{{
				ID: "go-test", Label: "Go checks", Status: statusUnsupported,
				Reason: "not assessed", CommandPreview: "go test ./...",
				SetupPreview: []string{}, Executable: false,
			}},
		}},
		Warnings: []string{},
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(encoded)
	for _, field := range []string{"projectId", "repositoryId", "worktreeId", "detectedLanguages", "selectedLanguages", "setupPreview", "executable"} {
		if !strings.Contains(text, `"`+field+`":`) {
			t.Errorf("missing JSON field %q", field)
		}
	}
	if !strings.Contains(text, `"projectId":""`) || !strings.Contains(text, `"executable":false`) {
		t.Fatalf("JSON = %s", text)
	}
	if strings.Contains(text, `"setupPreview":null`) {
		t.Fatalf("JSON contains null setup preview: %s", text)
	}
}

func TestInspectConservativePythonDeclarations(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		fastAPI bool
		partial bool
	}{
		{name: "requirements pin", file: "requirements.txt", content: "FastAPI[standard]>=0.1\n", fastAPI: true},
		{name: "requirements comment", file: "requirements.txt", content: "# fastapi\nhttpx # 'fastapi'\n"},
		{name: "requirements marker", file: "requirements.txt", content: "httpx; extra == 'fastapi'\n"},
		{name: "requirements include", file: "requirements.txt", content: "-r .env\n", partial: true},
		{name: "pyproject multiline", file: "pyproject.toml",
			content: "[project]\ndependencies = [\n 'httpx', # 'fastapi'\n 'fastapi>=0.1',\n]\n", fastAPI: true},
		{name: "pyproject comments", file: "pyproject.toml",
			content: "# [tool.ruff]\n[project]\ndescription = 'fastapi'\ndependencies = ['httpx'] # fastapi\n"},
		{name: "pyproject multiline string", file: "pyproject.toml",
			content: "[project]\ndescription = '''\n[tool.ruff]\ndependencies = ['fastapi']\n'''\n", partial: true},
		{name: "pyproject inline table", file: "pyproject.toml",
			content: "[tool.poetry.dependencies]\nfastapi = {version = '*'}\n", partial: true},
		{name: "pyproject invalid dependency type", file: "pyproject.toml",
			content: "[project]\ndependencies = 'fastapi'\n", partial: true},
		{name: "poetry declaration", file: "pyproject.toml",
			content: "[tool.poetry.dependencies]\nfastapi = '^0.1'\n", fastAPI: true},
		{name: "setup multiline pins", file: "setup.cfg",
			content: "[options]\ninstall_requires =\n    httpx==0.2\n    fastapi==0.1\n", fastAPI: true},
		{name: "setup comment", file: "setup.cfg", content: "[metadata]\ndescription = fastapi\n# fastapi\n"},
		{name: "setup reset dependency scope", file: "setup.cfg",
			content: "[options]\ninstall_requires = httpx\nother =\n    fastapi\n", partial: true},
		{name: "setup continuation pretending section", file: "setup.cfg",
			content: "[metadata]\ndescription =\n    [options]\n    install_requires = fastapi\n", partial: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, tt.file), tt.content)
			report := inspect(t, root, nil)
			component := componentByPath(t, report.Components, ".")
			got := reflect.DeepEqual(component.Frameworks, []string{"FastAPI"})
			if got != tt.fastAPI || report.Partial != tt.partial {
				t.Fatalf("frameworks=%v partial=%v warnings=%v; want fastAPI=%v partial=%v",
					component.Frameworks, report.Partial, report.Warnings, tt.fastAPI, tt.partial)
			}
		})
	}
}

func TestInspectPackageManagerEvidence(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		manager string
		status  string
		check   string
	}{
		{name: "plain pyproject unknown", files: map[string]string{"pyproject.toml": "[project]\nname='demo'\n"},
			status: statusSetup, check: "ruff"},
		{name: "uv lock with requirements", files: map[string]string{
			"uv.lock": "version = 1\n", "requirements.txt": "fastapi\n"},
			manager: "uv", status: statusSetup, check: "ruff"},
		{name: "poetry lock", files: map[string]string{"poetry.lock": ""},
			manager: "poetry", status: statusSetup, check: "ruff"},
		{name: "npm lock", files: map[string]string{"package.json": "{}", "package-lock.json": "{}"},
			manager: "npm", status: statusSetup, check: "eslint"},
		{name: "sticky conflict", files: map[string]string{
			"package-lock.json": "{}", "package.json": "{\"packageManager\":\"pnpm@9\"}",
			"pnpm-lock.yaml": "", "yarn.lock": ""},
			status: statusAmbiguous, check: "eslint"},
		{name: "unsupported manager", files: map[string]string{
			"package.json": "{\"packageManager\":\"unknown@1\"}"},
			status: statusUnsupported, check: "eslint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.files {
				writeFile(t, filepath.Join(root, name), content)
			}
			report := inspect(t, root, nil)
			component := componentByPath(t, report.Components, ".")
			if component.PackageManager != tt.manager {
				t.Fatalf("manager=%q want=%q", component.PackageManager, tt.manager)
			}
			assertCheckStatus(t, component, tt.check, tt.status)
			if tt.status == statusAmbiguous && !containsWarning(report.Warnings, "conflicting package manager") {
				t.Fatal("missing conflict warning")
			}
		})
	}
}

func TestInspectScriptAssociationsAndPreviews(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"),
		"{\"packageManager\":\"pnpm@9\",\"scripts\":{\"lint\":\"echo eslint\",\"test\":\"jest\",\"test:unit\":\"vitest run\"}}")
	component := componentByPath(t, inspect(t, root, nil).Components, ".")
	assertCheckStatus(t, component, "eslint", statusSetup)
	assertCheckStatus(t, component, "script:lint", statusExisting)
	assertCheckStatus(t, component, "script:test", statusExisting)
	assertCheckStatus(t, component, "vitest", statusExisting)
	if command := checkByID(t, component, "vitest").CommandPreview; command != "pnpm run test:unit" {
		t.Fatalf("Vitest command=%q", command)
	}
	writeFile(t, filepath.Join(root, "package.json"),
		"{\"packageManager\":\"pnpm@9\",\"scripts\":{\"lint\":\"node eslint-helper.js\",\"test\":\"echo vitest\"}}")
	writeFile(t, filepath.Join(root, "vitest.config.ts"), "throw new Error('must never execute');")
	component = componentByPath(t, inspect(t, root, nil).Components, ".")
	assertCheckStatus(t, component, "eslint", statusSetup)
	if command := checkByID(t, component, "vitest").CommandPreview; command != ".\\node_modules\\.bin\\vitest.cmd run" {
		t.Fatalf("direct Vitest command=%q", command)
	}
	writeFile(t, filepath.Join(root, "backend", "requirements.txt"), "fastapi\n")
	component = componentByPath(t, inspect(t, root, nil).Components, "backend")
	ruff := checkByID(t, component, "ruff")
	if ruff.CommandPreview != ".\\.venv\\Scripts\\python.exe -m ruff check ." {
		t.Fatalf("Ruff command=%q", ruff.CommandPreview)
	}
	if !containsWarning(ruff.SetupPreview, "py -m venv .venv") {
		t.Fatal("missing local Windows environment setup")
	}
	writeFile(t, filepath.Join(root, "backend", "uv.lock"), "")
	component = componentByPath(t, inspect(t, root, nil).Components, "backend")
	if command := checkByID(t, component, "ruff").CommandPreview; command != "uv run --offline --no-sync python -m ruff check ." {
		t.Fatalf("uv Ruff command=%q", command)
	}
}

func TestInspectJSONCAndNoRawContent(t *testing.T) {
	root := t.TempDir()
	config := "{ // tsconfig is JSONC\n \"extends\":\"./.env\", \"compilerOptions\": {\"strict\":true,},\n}"
	writeFile(t, filepath.Join(root, "tsconfig.json"), config)
	writeFile(t, filepath.Join(root, "package.json"),
		"{\"scripts\":{\"test\":\"echo RAW_SECRET_CANARY\"},\"dependencies\":{\"eslint\":\"9\",\"vitest\":\"3\"}}")
	writeFile(t, filepath.Join(root, ".env"), "RAW_SECRET_CANARY")
	before := inspect(t, root, nil)
	if before.Partial || !reflect.DeepEqual(before.DetectedLanguages, []string{"javascript", "typescript"}) {
		t.Fatalf("JSONC report=%+v", before)
	}
	component := componentByPath(t, before.Components, ".")
	assertCheckStatus(t, component, "eslint", statusSetup)
	assertCheckStatus(t, component, "vitest", statusSetup)
	encoded, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "RAW_SECRET_CANARY") {
		t.Fatal("report leaked raw script or secret content")
	}
	writeFile(t, filepath.Join(root, ".env"), "changed ignored bytes")
	after := inspect(t, root, nil)
	if after.Digest != before.Digest {
		t.Fatal(".env affected evidence digest")
	}
	got, err := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err != nil || string(got) != config {
		t.Fatal("inspection changed configuration")
	}
}

func TestInspectOpaqueIdentityAndEvidenceBytes(t *testing.T) {
	root := t.TempDir()
	path := "private-token-canary"
	writeFile(t, filepath.Join(root, path, "package.json"), "{}")
	first := inspect(t, root, nil)
	filtered := inspect(t, root, []string{"python"})
	component := componentByPath(t, first.Components, path)
	if strings.Contains(component.ID, path) || !strings.HasPrefix(component.ID, "component-") {
		t.Fatalf("unsafe component identity=%q", component.ID)
	}
	if component.ID != filtered.Components[0].ID || first.Digest != filtered.Digest {
		t.Fatal("filter changed identity or digest")
	}
	writeFile(t, filepath.Join(root, path, "package.json"), "{ }")
	changed := inspect(t, root, nil)
	if first.Digest == changed.Digest || changed.Components[0].ID != component.ID {
		t.Fatal("byte changes must change digest but preserve component identity")
	}
	other := t.TempDir()
	writeFile(t, filepath.Join(other, path, "package.json"), "{ }")
	moved := inspect(t, other, nil)
	if moved.Digest != changed.Digest || moved.Components[0].ID != component.ID {
		t.Fatal("absolute root affected relative identity or evidence digest")
	}
}

func TestInspectTruncatedEvidenceAndDepth(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "oversize", "pyproject.toml"),
		"[project]\ndependencies=['fastapi']\n[tool.ruff]\n#"+strings.Repeat("x", maxEvidenceBytes))
	writeFile(t, filepath.Join(root, "a", "b", "c", "requirements.txt"), "fastapi\n")
	writeFile(t, filepath.Join(root, "a", "b", "c", "d", "package.json"), "{}")
	report := inspect(t, root, nil)
	component := componentByPath(t, report.Components, "oversize")
	assertCheckStatus(t, component, "ruff", statusAmbiguous)
	if len(component.Frameworks) != 0 {
		t.Fatal("truncated prefix was used for framework detection")
	}
	componentByPath(t, report.Components, "a/b/c")
	if len(report.Components) != 2 || !containsWarning(report.Warnings, "maximum inspection depth") {
		t.Fatalf("depth boundary report=%+v", report)
	}
	bounded, complete, err := readEvidence(context.Background(), filepath.Join(root, "oversize", "pyproject.toml"))
	if err != nil || complete || len(bounded) != maxEvidenceBytes {
		t.Fatalf("read bound len=%d complete=%v err=%v", len(bounded), complete, err)
	}
	exact := t.TempDir()
	writeFile(t, filepath.Join(exact, "package.json"), "{}"+strings.Repeat(" ", maxEvidenceBytes-2))
	if report := inspect(t, exact, nil); report.Partial {
		t.Fatalf("exact size boundary unexpectedly partial: %v", report.Warnings)
	}
}

func TestInspectExcludedSelectedRootAndAllowlist(t *testing.T) {
	root := filepath.Join(t.TempDir(), "testdata")
	writeFile(t, filepath.Join(root, "go.mod"), "module example.test\n")
	initial := inspect(t, root, nil)
	for _, file := range []string{"setup.py", ".env", "eslint.config.js.bak", "unknown.toml", "vite.config.ts"} {
		writeFile(t, filepath.Join(root, file), "uninspected")
	}
	for directory := range excludedDirectories {
		writeFile(t, filepath.Join(root, directory, "package.json"), "{}")
	}
	report := inspect(t, root, nil)
	if len(report.Components) != 1 || report.Components[0].Path != "." || report.Digest != initial.Digest {
		t.Fatalf("selected root or exclusions changed evidence: %+v", report)
	}
}

func TestInspectSkipsLinkedEvidenceAndRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, target, "outside")
	if err := os.Symlink(target, filepath.Join(root, "package.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	report := inspect(t, root, nil)
	if len(report.Components) != 0 || !report.Partial {
		t.Fatalf("linked evidence report=%+v", report)
	}
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), link, nil); err == nil {
		t.Fatal("selected symlink root accepted")
	}
}

func TestInspectSkipsSpecialEvidence(t *testing.T) {
	root := t.TempDir()
	socket, err := net.Listen("unix", filepath.Join(root, "package.json"))
	if err != nil {
		t.Skipf("filesystem socket unavailable: %v", err)
	}
	defer func() {
		if err := socket.Close(); err != nil {
			t.Error(err)
		}
	}()
	report := inspect(t, root, nil)
	if len(report.Components) != 0 || !report.Partial || !containsWarning(report.Warnings, "special file") {
		t.Fatalf("special-file report=%+v", report)
	}
}

func TestInspectCancellationDuringTraversal(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 8; index++ {
		writeFile(t, filepath.Join(root, formatIndex(index), "package.json"), "{}")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	counted := &cancelContext{Context: ctx, cancel: cancel, remaining: 15}
	report, err := Inspect(counted, root, nil)
	if !errors.Is(err, context.Canceled) || !report.Partial || len(report.Components) >= 8 {
		t.Fatalf("canceled report=%+v err=%v", report, err)
	}
}

type cancelContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (ctx *cancelContext) Err() error {
	ctx.remaining--
	if ctx.remaining == 0 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func inspect(t *testing.T, root string, languages []string) Report {
	t.Helper()
	report, err := Inspect(context.Background(), root, languages)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func checkByID(t *testing.T, component Component, id string) Check {
	t.Helper()
	for _, check := range component.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %q missing", id)
	return Check{}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func componentByPath(t *testing.T, components []Component, path string) Component {
	t.Helper()
	for _, component := range components {
		if component.Path == path {
			return component
		}
	}
	t.Fatalf("component %q not found in %#v", path, components)
	return Component{}
}

func assertCheckStatus(t *testing.T, component Component, id, want string) {
	t.Helper()
	for _, check := range component.Checks {
		if check.ID == id {
			if check.Status != want {
				t.Fatalf("check %q status = %q, want %q", id, check.Status, want)
			}
			return
		}
	}
	t.Fatalf("check %q not found in %#v", id, component.Checks)
}

func containsWarning(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}

func formatIndex(index int) string {
	return fmt.Sprintf("%04d", index)
}
