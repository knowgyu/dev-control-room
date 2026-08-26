package assurance

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildGitHubBaselineInvocationUsesOneSafeReadOnlyArgv(t *testing.T) {
	input := GitHubBaselineInput{
		GHPath:       `C:\Program Files\GitHub CLI\gh.exe`,
		Owner:        "sample-owner",
		Repository:   "sample-repository",
		TargetBranch: "feature/windows",
	}

	got, err := BuildGitHubBaselineInvocation(input)
	if err != nil {
		t.Fatalf("BuildGitHubBaselineInvocation() error = %v", err)
	}
	want := GitHubBaselineInvocation{
		Executable: input.GHPath,
		Arguments: []string{
			"api",
			"--method",
			"GET",
			"--hostname",
			"github.com",
			"repos/sample-owner/sample-repository/branches/feature%2Fwindows",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("invocation = %#v, want %#v", got, want)
	}
	if got2, err := BuildGitHubBaselineInvocation(input); err != nil || !reflect.DeepEqual(got, got2) {
		t.Fatalf("rebuilding invocation was not deterministic: %#v, %v", got2, err)
	}
}

func TestBuildGitHubBaselineInvocationUsesFixedArgvForActiveBranchRules(t *testing.T) {
	base := GitHubBaselineInput{
		GHPath:       `C:\Program Files\GitHub CLI\gh.exe`,
		Owner:        "sample-owner",
		Repository:   "sample-repository",
		TargetBranch: "release/windows",
	}
	tests := []struct {
		name   string
		source GitHubBaselineSource
		want   []string
	}{
		{
			name:   "active branch rules",
			source: GitHubBaselineSourceBranchRules,
			want: []string{
				"api", "--method", "GET", "--hostname", "github.com",
				"repos/sample-owner/sample-repository/rules/branches/release%2Fwindows",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.Source = test.source
			got, err := BuildGitHubBaselineInvocation(input)
			if err != nil {
				t.Fatalf("BuildGitHubBaselineInvocation() error = %v", err)
			}
			if !reflect.DeepEqual(got.Arguments, test.want) {
				t.Fatalf("argv = %#v, want %#v", got.Arguments, test.want)
			}
		})
	}
}

func TestBuildGitHubBaselineInvocationRejectsUnregisteredSource(t *testing.T) {
	_, err := BuildGitHubBaselineInvocation(GitHubBaselineInput{
		Source:       GitHubBaselineSource("https://example.invalid/anything"),
		GHPath:       `C:\tools\gh.exe`,
		Owner:        "owner",
		Repository:   "repo",
		TargetBranch: "main",
	})
	if err == nil {
		t.Fatal("expected unregistered source to be rejected")
	}
}

func TestBuildGitHubBaselineInvocationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name  string
		input GitHubBaselineInput
	}{
		{name: "bare gh path", input: GitHubBaselineInput{GHPath: "gh.exe", Owner: "owner", Repository: "repo", TargetBranch: "main"}},
		{name: "unix path", input: GitHubBaselineInput{GHPath: `/usr/bin/gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "main"}},
		{name: "drive relative path", input: GitHubBaselineInput{GHPath: `C:tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "main"}},
		{name: "shell path", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe;whoami`, Owner: "owner", Repository: "repo", TargetBranch: "main"}},
		{name: "wrong executable", input: GitHubBaselineInput{GHPath: `C:\tools\gh.cmd`, Owner: "owner", Repository: "repo", TargetBranch: "main"}},
		{name: "owner path", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner/name", Repository: "repo", TargetBranch: "main"}},
		{name: "owner whitespace", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner name", Repository: "repo", TargetBranch: "main"}},
		{name: "repository path", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo/name", TargetBranch: "main"}},
		{name: "repository git suffix", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo.git", TargetBranch: "main"}},
		{name: "empty branch", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: ""}},
		{name: "branch control", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "feature\nfix"}},
		{name: "branch traversal", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "feature/../fix"}},
		{name: "branch query", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "feature?debug"}},
		{name: "branch shell surface", input: GitHubBaselineInput{GHPath: `C:\tools\gh.exe`, Owner: "owner", Repository: "repo", TargetBranch: "feature;whoami"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildGitHubBaselineInvocation(test.input); err == nil {
				t.Fatal("expected unsafe input to be rejected")
			}
		})
	}
}

func TestParseGitHubBaselineRequiredContexts(t *testing.T) {
	payload := []byte(`{
  "protected": true,
  "protection": {
    "enabled": true,
    "required_status_checks": {
      "contexts": ["build", ""],
      "checks": [{"context":"unit"}, {"context":"build"}]
    }
  },
  "node_id": "provider-branch-data-must-drop",
  "url": "provider-location-must-drop"
}`)
	observedAt := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	evidence, err := ParseGitHubBaseline(payload, GitHubBaselineMetadata{SourceID: "fixture.github", ObservedAt: observedAt})
	if err != nil {
		t.Fatalf("ParseGitHubBaseline() error = %v", err)
	}
	if evidence.Availability != BaselineAvailable {
		t.Fatalf("availability = %q, want %q", evidence.Availability, BaselineAvailable)
	}
	if got, want := evidence.RequiredContexts, []string{"build", "unit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("required contexts = %#v, want %#v", got, want)
	}
	if evidence.SourceID != "fixture.github" || !evidence.ObservedAt.Equal(observedAt) {
		t.Fatalf("safe metadata was not retained: %#v", evidence)
	}
	if !strings.HasPrefix(evidence.SourceDigest, "sha256:") || len(evidence.SourceDigest) != len("sha256:")+64 {
		t.Fatalf("source digest = %q, want sha256 hex digest", evidence.SourceDigest)
	}
	encoded := mustJSON(t, evidence)
	if strings.Contains(encoded, "provider-location-must-drop") || strings.Contains(encoded, "provider-branch-data-must-drop") || strings.Contains(encoded, "node_id") {
		t.Fatalf("raw provider payload was retained in compact evidence: %s", encoded)
	}
}

func TestParseGitHubBaselineBranchProtectionStateSemantics(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		available  BaselineAvailability
		contexts   []string
		wantReason bool
	}{
		{
			name:      "known unprotected",
			payload:   `{"protected":false,"protection":"ignored-for-unprotected-branch","node_id":"must-not-retain"}`,
			available: BaselineAvailable,
			contexts:  []string{},
		},
		{
			name:      "disabled protection with empty checks",
			payload:   `{"protected":true,"protection":{"enabled":false,"required_status_checks":{"contexts":[],"checks":[]}}}`,
			available: BaselineAvailable,
			contexts:  []string{},
		},
		{
			name:       "protected branch missing protection",
			payload:    `{"protected":true,"url":"provider-data-must-not-retain"}`,
			available:  BaselineUnknown,
			contexts:   []string{},
			wantReason: true,
		},
		{
			name:       "protected branch unsupported checks",
			payload:    `{"protected":true,"protection":{"enabled":true,"required_status_checks":"not-an-object"}}`,
			available:  BaselineUnknown,
			contexts:   []string{},
			wantReason: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence, err := ParseGitHubBaselineForSource([]byte(test.payload), GitHubBaselineSourceBranchProtection, GitHubBaselineMetadata{})
			if err != nil {
				t.Fatalf("ParseGitHubBaselineForSource() error = %v", err)
			}
			if evidence.Availability != test.available || !reflect.DeepEqual(evidence.RequiredContexts, test.contexts) {
				t.Fatalf("evidence = %#v, want availability=%q contexts=%#v", evidence, test.available, test.contexts)
			}
			if test.wantReason && evidence.Reason == "" {
				t.Fatalf("unknown protection evidence omitted its reason: %#v", evidence)
			}
			if strings.Contains(mustJSON(t, evidence), "must-not-retain") {
				t.Fatalf("raw branch payload was retained: %#v", evidence)
			}
		})
	}
}

func TestParseGitHubBaselineRulesAndDuplicateNormalization(t *testing.T) {
	first := []byte(`{"rules":[{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"ci/test"},{"context":"ci/test"},{"context":"  ci/build  "}]}}]}`)
	second := []byte(`{"rules":[{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"ci/build"},{"context":"ci/test"}]}}]}`)
	metadata := GitHubBaselineMetadata{SourceID: "fixture.rules"}
	one, err := ParseGitHubBaseline(first, metadata)
	if err != nil {
		t.Fatalf("first parse error = %v", err)
	}
	two, err := ParseGitHubBaseline(second, metadata)
	if err != nil {
		t.Fatalf("second parse error = %v", err)
	}
	want := []string{"ci/build", "ci/test"}
	if !reflect.DeepEqual(one.RequiredContexts, want) || !reflect.DeepEqual(two.RequiredContexts, want) {
		t.Fatalf("normalized contexts = %#v and %#v, want %#v", one.RequiredContexts, two.RequiredContexts, want)
	}
	if one.SourceDigest != two.SourceDigest {
		t.Fatalf("equivalent compact evidence digests differ: %q != %q", one.SourceDigest, two.SourceDigest)
	}
}

func TestParseGitHubBaselineForActiveBranchRules(t *testing.T) {
	payload := []byte(`[
  {"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"ci/windows"},{"context":"ci/windows"},{"context":"  ci/linux  "}]}},
  {"type":"non_fast_forward"}
]`)
	evidence, err := ParseGitHubBaselineForSource(payload, GitHubBaselineSourceBranchRules, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("ParseGitHubBaselineForSource() error = %v", err)
	}
	if evidence.Availability != BaselineAvailable {
		t.Fatalf("availability = %q, want %q", evidence.Availability, BaselineAvailable)
	}
	if got, want := evidence.RequiredContexts, []string{"ci/linux", "ci/windows"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("required contexts = %#v, want %#v", got, want)
	}
	if evidence.SourceID != string(GitHubBaselineSourceBranchRules) {
		t.Fatalf("source ID = %q, want %q", evidence.SourceID, GitHubBaselineSourceBranchRules)
	}
}

func TestParseGitHubBaselineRejectsMalformedCheckEntries(t *testing.T) {
	malformedEntries := []struct {
		name   string
		checks string
	}{
		{name: "null entry", checks: `[null]`},
		{name: "missing context", checks: `[{}]`},
		{name: "null context", checks: `[{"context":null}]`},
		{name: "non-string context", checks: `[{"context":42}]`},
		{name: "empty context", checks: `[{"context":""}]`},
		{name: "whitespace context", checks: `[{"context":"   "}]`},
	}

	for _, test := range malformedEntries {
		t.Run("legacy/"+test.name, func(t *testing.T) {
			payload := `{"protected":true,"protection":{"enabled":true,"required_status_checks":{"checks":` + test.checks + `}}}`
			assertGitHubBaselineCheckEntryIsUnknown(t, payload, GitHubBaselineSourceBranchProtection)
		})
		t.Run("active/"+test.name, func(t *testing.T) {
			payload := `[{"type":"required_status_checks","parameters":{"required_status_checks":` + test.checks + `}}]`
			assertGitHubBaselineCheckEntryIsUnknown(t, payload, GitHubBaselineSourceBranchRules)
		})
	}
}

func assertGitHubBaselineCheckEntryIsUnknown(t *testing.T, payload string, source GitHubBaselineSource) {
	t.Helper()
	evidence, err := ParseGitHubBaselineForSource([]byte(payload), source, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("ParseGitHubBaselineForSource() error = %v; malformed checks should become explicit unknown evidence", err)
	}
	if evidence.Availability != BaselineUnknown {
		t.Fatalf("availability = %q, want %q; malformed checks must not become available", evidence.Availability, BaselineUnknown)
	}
	if len(evidence.RequiredContexts) != 0 {
		t.Fatalf("required contexts = %#v, want empty after fail-closed parse", evidence.RequiredContexts)
	}
	if evidence.Reason == "" {
		t.Fatal("fail-closed evidence omitted its reason")
	}
}

func TestRepositoryRulesetListingIsExplicitUnknown(t *testing.T) {
	payload := []byte(`[
  {"id": 42, "name": "provider-metadata-must-not-survive", "target": "branch", "enforcement": "active"}
]`)
	evidence, err := ParseGitHubBaseline(payload, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("ParseGitHubBaseline() error = %v", err)
	}
	if evidence.Availability != BaselineUnknown || len(evidence.RequiredContexts) != 0 || evidence.Reason == "" {
		t.Fatalf("ruleset listing must remain explicit unknown: %#v", evidence)
	}
	if strings.Contains(mustJSON(t, evidence), "provider-metadata-must-not-survive") {
		t.Fatal("ruleset listing metadata was retained")
	}
	if _, err := ParseGitHubBaselineForSource(payload, GitHubBaselineSource("github.repository_rulesets"), GitHubBaselineMetadata{}); err == nil {
		t.Fatal("unimplemented repository-rulesets source must be rejected")
	}
}

func TestMergeGitHubBaselineEvidenceFailsClosedForUnknownOrUnavailable(t *testing.T) {
	available, err := ParseGitHubBaseline([]byte(`{"protected":true,"protection":{"enabled":true,"required_status_checks":{"contexts":["ci/known"]}}}`), GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("available parse error = %v", err)
	}
	unknown, err := ParseGitHubBaseline([]byte(`{"message":"unsupported"}`), GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("unknown parse error = %v", err)
	}
	unavailable, err := ParseGitHubBaselineResponse(GitHubBaselineResponse{StatusCode: 503}, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("unavailable parse error = %v", err)
	}

	for _, test := range []struct {
		name   string
		input  []GitHubBaselineEvidence
		status BaselineAvailability
		reason string
	}{
		{name: "unknown blocks available", input: []GitHubBaselineEvidence{available, unknown}, status: BaselineUnknown, reason: "authoritative_source_unknown"},
		{name: "unavailable blocks available", input: []GitHubBaselineEvidence{available, unavailable}, status: BaselineUnavailable, reason: "authoritative_source_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			merged := MergeGitHubBaselineEvidence(test.input...)
			if merged.Availability != test.status || merged.Reason != test.reason {
				t.Fatalf("merged state = %#v, want status=%q reason=%q", merged, test.status, test.reason)
			}
			if len(merged.RequiredContexts) != 0 {
				t.Fatalf("unsafe partial contexts were retained: %#v", merged.RequiredContexts)
			}
			if merged.SourceDigest == "" {
				t.Fatal("merged evidence has no deterministic digest")
			}
		})
	}
}

func TestMergeGitHubBaselineEvidenceUnionsOnlyAvailableSources(t *testing.T) {
	legacy, err := ParseGitHubBaseline([]byte(`{"protected":true,"protection":{"enabled":true,"required_status_checks":{"contexts":["ci/legacy"]}}}`), GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("legacy parse error = %v", err)
	}
	rules, err := ParseGitHubBaselineForSource([]byte(`[{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"ci/rules"}]}}]`), GitHubBaselineSourceBranchRules, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("rules parse error = %v", err)
	}
	merged := MergeGitHubBaselineEvidence(legacy, rules)
	if merged.Availability != BaselineAvailable {
		t.Fatalf("availability = %q, want %q", merged.Availability, BaselineAvailable)
	}
	if got, want := merged.RequiredContexts, []string{"ci/legacy", "ci/rules"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged contexts = %#v, want %#v", got, want)
	}
	if got := ReconcileGitHubBaselineEvidence(legacy, rules); !reflect.DeepEqual(got, merged) {
		t.Fatalf("reconciliation alias changed result: %#v vs %#v", got, merged)
	}
}

func TestParseGitHubBaselineKeepsUnknownAndUnavailableExplicit(t *testing.T) {
	unknown, err := ParseGitHubBaseline([]byte(`{"message":"unsupported response shape"}`), GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("unknown parse error = %v", err)
	}
	if unknown.Availability != BaselineUnknown || unknown.Reason == "" || unknown.SourceDigest == "" {
		t.Fatalf("unknown evidence was not explicit: %#v", unknown)
	}
	unknownArray, err := ParseGitHubBaseline([]byte(`[]`), GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("unknown array parse error = %v", err)
	}
	if unknownArray.Availability != BaselineUnknown || unknownArray.Reason == "" {
		t.Fatalf("unsupported array was not explicit unknown: %#v", unknownArray)
	}

	unavailable, err := ParseGitHubBaselineResponse(GitHubBaselineResponse{
		StatusCode: 404,
		Body:       []byte(`{"message":"secret-bearing diagnostic must not be retained"}`),
	}, GitHubBaselineMetadata{})
	if err != nil {
		t.Fatalf("unavailable parse error = %v", err)
	}
	if unavailable.Availability != BaselineUnavailable || unavailable.Reason != "http_status_404" || unavailable.SourceDigest == "" {
		t.Fatalf("unavailable evidence was not explicit: %#v", unavailable)
	}
	if strings.Contains(mustJSON(t, unavailable), "secret-bearing") {
		t.Fatal("unavailable evidence retained raw response data")
	}
}

func TestParseGitHubBaselineRejectsInvalidAndUnboundedPayload(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"required_status_checks":`),
		bytes.Repeat([]byte("x"), MaxGitHubBaselinePayloadBytes+1),
	} {
		if _, err := ParseGitHubBaseline(payload, GitHubBaselineMetadata{}); err == nil {
			t.Fatal("expected invalid or oversized payload to fail closed")
		}
	}
}

func TestGitHubBaselineHasNoExecutionSeam(t *testing.T) {
	invocation, err := BuildGitHubBaselineInvocation(GitHubBaselineInput{
		GHPath:       `C:\path\that\need\not\exist\gh.exe`,
		Owner:        "owner",
		Repository:   "repo",
		TargetBranch: "main",
	})
	if err != nil {
		t.Fatalf("build error = %v", err)
	}
	if invocation.Executable == "" || len(invocation.Arguments) != 6 {
		t.Fatalf("unexpected typed invocation: %#v", invocation)
	}
	typeName := reflect.TypeOf(invocation)
	for index := 0; index < typeName.NumMethod(); index++ {
		name := typeName.Method(index).Name
		if strings.Contains(name, "Run") || strings.Contains(name, "Execute") || strings.Contains(name, "Fetch") {
			t.Fatalf("invocation exposes an execution method: %s", name)
		}
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(encoded)
}
