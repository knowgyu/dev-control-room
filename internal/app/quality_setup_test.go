package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/qualitysetup"
)

func TestQualitySetupRejectsMissingIDs(t *testing.T) {
	// No store or collector: missing IDs must be rejected before either is used.
	service := &App{}
	for _, field := range []string{"project", "repository", "worktree"} {
		t.Run(field, func(t *testing.T) {
			ids := map[string]string{"project": "project", "repository": "repo-1", "worktree": "primary"}
			ids[field] = " \t"
			_, err := service.QualitySetup(
				context.Background(), ids["project"], ids["repository"], ids["worktree"], nil,
			)
			if contract.Classify(err).Code != contract.ErrorInvalidInput {
				t.Fatalf("missing ID error = %v, want invalid input", err)
			}
		})
	}
}

func TestQualitySetupHTTPUsesNoStoreEnvelopeForInvalidIDs(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	request := httptest.NewRequest(http.MethodGet, "/api/quality/setup?projectId=&repositoryId=repo-1&worktreeId=primary", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("quality setup status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("quality setup cache control = %q, want no-store", got)
	}
	var envelope contract.Envelope[qualitysetup.Report]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || envelope.OK || envelope.Error == nil || envelope.Error.Code != contract.ErrorInvalidInput {
		t.Fatalf("quality setup envelope = %#v", envelope)
	}
}

func TestQualitySetupMasksInspectorTextButPreservesBindingEvidence(t *testing.T) {
	const secret = "quality-setup-secret-canary"
	// Include enum values as known secrets to prove that they remain intact.
	service := &App{masker: masking.New([]string{secret, "python", "manifest", "setup_needed"}, nil)}
	report := qualitysetup.Report{
		ProjectID:         secret,
		RepositoryID:      secret,
		WorktreeID:        secret,
		Head:              secret,
		Digest:            secret,
		DetectedLanguages: []string{"python", "go"},
		SelectedLanguages: []string{"python"},
		Warnings:          []string{secret, "safe warning"},
		Partial:           true,
		Components: []qualitysetup.Component{{
			ID:             secret,
			Path:           secret,
			Languages:      []string{"python"},
			Frameworks:     []string{secret, "FastAPI"},
			PackageManager: secret,
			Evidence:       []qualitysetup.Evidence{{Path: secret, Kind: "manifest"}},
			Checks: []qualitysetup.Check{{
				ID:             secret,
				Label:          secret,
				Status:         "setup_needed",
				Reason:         secret,
				CommandPreview: secret,
				SetupPreview:   []string{secret, "safe preview"},
			}},
		}},
	}
	service.maskQualitySetupReport(&report)

	component := report.Components[0]
	check := component.Checks[0]
	for field, text := range map[string]string{
		"warning":         report.Warnings[0],
		"component path":  component.Path,
		"framework":       component.Frameworks[0],
		"package manager": component.PackageManager,
		"evidence path":   component.Evidence[0].Path,
		"label":           check.Label,
		"reason":          check.Reason,
		"command preview": check.CommandPreview,
		"setup preview":   check.SetupPreview[0],
	} {
		if text != masking.Replacement {
			t.Errorf("%s = %q, want redaction", field, text)
		}
	}
	for field, value := range map[string]string{
		"project ID": report.ProjectID, "repository ID": report.RepositoryID,
		"worktree ID": report.WorktreeID, "head": report.Head, "digest": report.Digest,
		"component ID": component.ID, "check ID": check.ID,
	} {
		if value != secret {
			t.Errorf("%s changed to %q", field, value)
		}
	}
	for field, languages := range map[string][]string{
		"detected": report.DetectedLanguages[:1], "selected": report.SelectedLanguages,
		"component": component.Languages,
	} {
		if !slices.Equal(languages, []string{"python"}) {
			t.Errorf("%s languages changed to %v", field, languages)
		}
	}
	if component.Evidence[0].Kind != "manifest" || check.Status != "setup_needed" {
		t.Fatalf("classification enums changed: %#v", component)
	}
	if check.Executable || !report.Partial {
		t.Fatal("report or execution semantics changed during masking")
	}
	if report.Warnings[1] != "safe warning" || check.SetupPreview[1] != "safe preview" {
		t.Fatal("safe text changed during masking")
	}
}

func TestQualitySetupScopesReportToFreshRegisteredWorktree(t *testing.T) {
	const secret = "quality-setup-path-canary"
	ctx := context.Background()
	repository := tempGoGitRepository(t, "quality-setup")
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, repository, "worktree", "add", "-b", "setup-linked", linked)
	componentPath := filepath.Join(linked, secret)
	if err := os.Mkdir(componentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentPath, "pyproject.toml"), []byte("[project]\nname = \"backend\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := newWithMasker(t.TempDir(), "127.0.0.1:38471", masking.New([]string{secret}, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality Setup", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	worktrees, err := service.Worktrees(ctx, project.Metadata.ID, "repo-1")
	if err != nil {
		t.Fatal(err)
	}
	var worktreeID, storedHead string
	for _, worktree := range worktrees {
		if !worktree.Spec.Primary {
			worktreeID, storedHead = worktree.Metadata.ID, worktree.Spec.Head
		}
	}
	if worktreeID == "" {
		t.Fatal("linked fixture was not observed")
	}
	gitFixture(t, linked, "commit", "--allow-empty", "-m", "advance selected checkout")
	fresh, err := service.collector.Collect(ctx, linked)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := qualitysetup.Inspect(ctx, linked, []string{"python"})
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{
		"projectId": {project.Metadata.ID}, "repositoryId": {"repo-1"},
		"worktreeId": {worktreeID}, "languages": {" Python,python "},
		"path": {filepath.Join(t.TempDir(), "never-inspect-this-path")},
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/quality/setup?"+query.Encode(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("setup response = %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("successful setup response is cacheable")
	}
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatalf("HTTP presentation text leaked secret: %s", recorder.Body.String())
	}
	var envelope contract.Envelope[qualitysetup.Report]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data == nil || envelope.Schema != contract.EnvelopeSchema || envelope.Error != nil {
		t.Fatalf("success envelope = %#v", envelope)
	}
	report := envelope.Data
	if report.ProjectID != project.Metadata.ID || report.RepositoryID != "repo-1" || report.WorktreeID != worktreeID {
		t.Fatalf("quality setup scope = %#v", report)
	}
	if report.Head != fresh.Head || report.Head == storedHead || report.Digest != expected.Digest || report.Digest == "" {
		t.Fatalf("setup did not retain fresh HEAD and inspector digest: %#v", report)
	}
	if !slices.Equal(report.SelectedLanguages, []string{"python"}) || !slices.Contains(report.DetectedLanguages, "go") {
		t.Fatalf("manual filter lost observed languages: %#v", report)
	}
	if len(report.Components) != len(expected.Components) || len(report.Components) != 2 {
		t.Fatalf("setup changed observed components: %#v", report.Components)
	}
	for i, component := range report.Components {
		if component.ID != expected.Components[i].ID {
			t.Fatal("component binding ID changed during masking")
		}
	}
	persisted, err := service.Worktree(ctx, project.Metadata.ID, "repo-1", worktreeID)
	if err != nil || persisted.Spec.Head != storedHead {
		t.Fatalf("read-only setup changed persisted worktree: %#v, %v", persisted, err)
	}
}

func TestQualitySetupHTTPRejectsUnsupportedLanguage(t *testing.T) {
	repository := tempGoGitRepository(t, "quality-setup-language")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality Setup Language", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/quality/setup?projectId="+project.Metadata.ID+"&repositoryId=repo-1&worktreeId=primary&languages=rust", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unsupported language status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[qualitysetup.Report]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contract.ErrorInvalidInput {
		t.Fatalf("unsupported language envelope = %#v", envelope)
	}
}

func TestQualitySetupCannotEscapeSelectedRepository(t *testing.T) {
	first := tempGoGitRepository(t, "quality-setup-first")
	second := tempGoGitRepository(t, "quality-setup-second")
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, second, "worktree", "add", "-b", "quality-setup-linked", linked)

	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality Setup Scope", Path: first})
	if err != nil {
		t.Fatal(err)
	}
	secondRepository, err := service.AddRepository(context.Background(), AddRepositoryInput{
		ProjectID: project.Metadata.ID,
		ID:        "second",
		Name:      "second",
		Path:      second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondRepository.Metadata.ID != "second" {
		t.Fatalf("second repository = %#v", secondRepository)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	worktrees, err := service.Worktrees(context.Background(), project.Metadata.ID, "second")
	if err != nil {
		t.Fatal(err)
	}
	foreignWorktreeID := ""
	for _, worktree := range worktrees {
		if worktree.Metadata.ID != "primary" {
			foreignWorktreeID = worktree.Metadata.ID
			break
		}
	}
	if foreignWorktreeID == "" {
		t.Fatalf("linked worktree was not registered: %#v", worktrees)
	}

	// Unknown scopes must be rejected before Git or source inspection.
	service.collector = collector.NewGitCollector(qualitySetupGitRunner(func() error {
		t.Fatal("Git ran for an unknown or foreign scope")
		return nil
	}))
	for _, tc := range []struct {
		name, projectID, repositoryID, worktreeID string
	}{
		{name: "foreign worktree", projectID: project.Metadata.ID, repositoryID: "repo-1", worktreeID: foreignWorktreeID},
		{name: "unknown repository", projectID: project.Metadata.ID, repositoryID: "missing", worktreeID: "primary"},
		{name: "unknown project", projectID: "missing", repositoryID: "second", worktreeID: foreignWorktreeID},
		{name: "path as ID", projectID: project.Metadata.ID, repositoryID: "repo-1", worktreeID: linked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := url.Values{
				"projectId": {tc.projectID}, "repositoryId": {tc.repositoryID}, "worktreeId": {tc.worktreeID},
			}
			recorder := httptest.NewRecorder()
			service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/quality/setup?"+query.Encode(), nil))
			assertQualitySetupHTTPError(t, recorder, http.StatusNotFound, contract.ErrorNotFound)
		})
	}
}

func TestQualitySetupRejectsInvalidLanguagesBeforeScopeLookup(t *testing.T) {
	// A nil store makes any premature lookup fail the test.
	service := &App{}
	for _, raw := range []string{"rust", "python,,go", "python,", strings.Repeat("x", 129), strings.Repeat("go,", 64) + "go"} {
		t.Run(raw[:min(len(raw), 20)], func(t *testing.T) {
			_, err := service.QualitySetup(context.Background(), "project", "repo-1", "primary", strings.Split(raw, ","))
			if contract.Classify(err).Code != contract.ErrorInvalidInput {
				t.Fatalf("invalid languages error = %v", err)
			}
			recorder := httptest.NewRecorder()
			query := "projectId=project&repositoryId=repo-1&worktreeId=primary&languages=" + url.QueryEscape(raw)
			service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/quality/setup?"+query, nil))
			assertQualitySetupHTTPError(t, recorder, http.StatusBadRequest, contract.ErrorInvalidInput)
		})
	}
	for _, tc := range []struct {
		name, raw string
		want      []string
	}{
		{name: "auto", raw: "", want: []string{}},
		{name: "all four unique", raw: "python,javascript,typescript,go,python", want: []string{"python", "javascript", "typescript", "go"}},
		{name: "exact bound", raw: "go" + strings.Repeat(" ", 126), want: []string{"go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseQualitySetupLanguages(tc.raw)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("languages = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestQualitySetupHTTPRevalidationErrorsAreSafe(t *testing.T) {
	const secret = "quality-setup-error-canary"
	ctx := context.Background()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(ctx, AddProjectInput{Name: "Setup Errors", Path: tempGitRepository(t, "setup-errors")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(ctx, "manual"); err != nil {
		t.Fatal(err)
	}
	path := "/api/quality/setup?projectId=" + project.Metadata.ID + "&repositoryId=repo-1&worktreeId=primary"
	worktree, err := service.Worktree(ctx, project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	worktree.Spec.AssociationFingerprint = "sha256:changed"
	data, err := json.Marshal(worktree)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.store.DB().Exec(
		"UPDATE worktrees SET object_json = ? WHERE project_id = ? AND repository_id = ? AND id = ?",
		string(data), project.Metadata.ID, "repo-1", "primary",
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	assertQualitySetupHTTPError(t, recorder, http.StatusConflict, contract.ErrorConflict)
	if !strings.Contains(recorder.Body.String(), "refresh") {
		t.Fatal("stale worktree error does not ask for refresh")
	}
	service.collector = collector.NewGitCollector(qualitySetupGitRunner(func() error {
		return errors.New("private filesystem error: " + secret)
	}))
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	assertQualitySetupHTTPError(t, recorder, http.StatusServiceUnavailable, contract.ErrorUnavailable)
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatalf("Git error leaked: %s", recorder.Body.String())
	}
}

type qualitySetupGitRunner func() error

func (run qualitySetupGitRunner) Run(context.Context, string, []string, string) (collector.CommandResult, error) {
	return collector.CommandResult{}, run()
}

func assertQualitySetupHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, code contract.ErrorCode) {
	t.Helper()
	if response.Code != status || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("setup error response = %d %v: %s", response.Code, response.Header(), response.Body.String())
	}
	var envelope contract.Envelope[qualitysetup.Report]
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || envelope.OK || envelope.Data != nil || envelope.Error == nil {
		t.Fatalf("invalid error envelope: %#v", envelope)
	}
	if envelope.Error.Code != code || envelope.Error.Details != nil {
		t.Fatalf("unexpected error contract: %#v", envelope.Error)
	}
}
