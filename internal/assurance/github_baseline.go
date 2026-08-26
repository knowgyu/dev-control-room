package assurance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxGitHubBaselinePayloadBytes bounds fixture and provider output before it
// is decoded. Only the compact evidence below survives parsing.
const MaxGitHubBaselinePayloadBytes = 256 << 10

const (
	GitHubBaselineSourceBranchProtection GitHubBaselineSource = "github.branch_protection"
	GitHubBaselineSourceBranchRules      GitHubBaselineSource = "github.branch_rules"

	// These aliases keep the source vocabulary discoverable without creating
	// additional accepted provider paths.
	GitHubBaselineSourceLegacy    = GitHubBaselineSourceBranchProtection
	GitHubBaselineSourceActive    = GitHubBaselineSourceBranchRules
	GitHubBaselineSourceID        = string(GitHubBaselineSourceBranchProtection)
	githubBaselineBranchPath      = "repos/%s/%s/branches/%s"
	githubBaselineBranchRulesPath = "repos/%s/%s/rules/branches/%s"
)

// GitHubBaselineSource identifies one reviewed GitHub endpoint. It is not an
// endpoint or argv template supplied by a caller.
type GitHubBaselineSource string

// GitHubBaselineInput is the complete, typed input for the one supported
// read-only GitHub baseline request. It deliberately has no arbitrary argv or
// endpoint field.
type GitHubBaselineInput struct {
	Source       GitHubBaselineSource
	GHPath       string
	Owner        string
	Repository   string
	TargetBranch string
}

// GitHubBaselineInvocation is data only. It is an argv description, not an
// execution handle; this package has no process or network execution path.
type GitHubBaselineInvocation struct {
	Executable string
	Arguments  []string
}

// BuildGitHubBaselineInvocation creates exactly one fixed, read-only gh argv.
// The branch is encoded as one URL path segment, so a branch such as
// feature/windows is never mistaken for another REST path segment.
func BuildGitHubBaselineInvocation(input GitHubBaselineInput) (GitHubBaselineInvocation, error) {
	if err := validateGHPath(input.GHPath); err != nil {
		return GitHubBaselineInvocation{}, err
	}
	if err := validateOwner(input.Owner); err != nil {
		return GitHubBaselineInvocation{}, err
	}
	if err := validateRepository(input.Repository); err != nil {
		return GitHubBaselineInvocation{}, err
	}
	if err := validateBranch(input.TargetBranch); err != nil {
		return GitHubBaselineInvocation{}, err
	}

	source := input.Source
	if source == "" {
		source = GitHubBaselineSourceBranchProtection
	}
	if err := validateGitHubBaselineSource(source); err != nil {
		return GitHubBaselineInvocation{}, err
	}

	owner := url.PathEscape(input.Owner)
	repository := url.PathEscape(input.Repository)
	branch := url.PathEscape(input.TargetBranch)
	path := ""
	arguments := []string{"api", "--method", "GET", "--hostname", "github.com"}
	switch source {
	case GitHubBaselineSourceBranchProtection:
		path = fmt.Sprintf(githubBaselineBranchPath, owner, repository, branch)
	case GitHubBaselineSourceBranchRules:
		path = fmt.Sprintf(githubBaselineBranchRulesPath, owner, repository, branch)
	default:
		return GitHubBaselineInvocation{}, errors.New("GitHub baseline source is not registered")
	}
	arguments = append(arguments, path)
	invocation := GitHubBaselineInvocation{
		Executable: input.GHPath,
		Arguments:  arguments,
	}
	return invocation, nil
}

// BuildGitHubBaselineInvocationForSource is a convenience wrapper for a
// caller that selects one of the reviewed sources separately from the other
// request fields.
func BuildGitHubBaselineInvocationForSource(input GitHubBaselineInput, source GitHubBaselineSource) (GitHubBaselineInvocation, error) {
	input.Source = source
	return BuildGitHubBaselineInvocation(input)
}

// Availability says whether the provider supplied authoritative required
// status-check evidence. Unknown is intentionally different from available
// with zero required contexts.
type BaselineAvailability string

const (
	BaselineAvailable   BaselineAvailability = "available"
	BaselineUnknown     BaselineAvailability = "unknown"
	BaselineUnavailable BaselineAvailability = "unavailable"
)

// GitHubBaselineMetadata is safe caller-supplied metadata. SourceID is an
// identifier, never a URL or a credential-bearing string. ObservedAt is not
// used in the source digest.
type GitHubBaselineMetadata struct {
	SourceID   string
	ObservedAt time.Time
}

// GitHubBaselineResponse is the minimum response envelope needed to classify
// provider availability. Headers and raw response metadata are intentionally
// absent.
type GitHubBaselineResponse struct {
	StatusCode int
	Body       []byte
}

// GitHubBaselineEvidence is the compact, integration-ready representation of
// provider-authoritative status-check requirements. Every RequiredContexts
// value is required by the provider; no local equivalent is inferred here.
type GitHubBaselineEvidence struct {
	Availability     BaselineAvailability `json:"availability"`
	RequiredContexts []string             `json:"requiredContexts"`
	Reason           string               `json:"reason"`
	SourceID         string               `json:"sourceID"`
	ObservedAt       time.Time            `json:"observedAt"`
	SourceDigest     string               `json:"sourceDigest"`
}

// ParseGitHubBaseline parses a successful fixture from any supported GitHub
// baseline source. Source-specific parsing is available below when the caller
// knows which fixed endpoint produced the response.
func ParseGitHubBaseline(payload []byte, metadata GitHubBaselineMetadata) (GitHubBaselineEvidence, error) {
	return ParseGitHubBaselineResponseForSource(GitHubBaselineResponse{StatusCode: 200, Body: payload}, "", metadata)
}

// ParseGitHubBaselineResponse maps only the successful provider response into
// compact evidence. 404, authentication failures, and other non-200 results
// remain explicit unavailable evidence; they never become an empty local
// baseline. A syntactically invalid or over-limit successful payload is an
// error so callers cannot persist an apparently authoritative partial result.
func ParseGitHubBaselineResponse(response GitHubBaselineResponse, metadata GitHubBaselineMetadata) (GitHubBaselineEvidence, error) {
	return ParseGitHubBaselineResponseForSource(response, "", metadata)
}

// ParseGitHubBaselineForSource parses the response shape owned by source.
// Rejecting a valid response from a different endpoint as unknown prevents a
// caller from accidentally attributing ruleset evidence to legacy protection.
func ParseGitHubBaselineForSource(payload []byte, source GitHubBaselineSource, metadata GitHubBaselineMetadata) (GitHubBaselineEvidence, error) {
	return ParseGitHubBaselineResponseForSource(GitHubBaselineResponse{StatusCode: 200, Body: payload}, source, metadata)
}

// ParseGitHubBaselineResponseForSource maps one response from a reviewed
// endpoint into compact evidence. It performs no provider or process I/O.
func ParseGitHubBaselineResponseForSource(response GitHubBaselineResponse, source GitHubBaselineSource, metadata GitHubBaselineMetadata) (GitHubBaselineEvidence, error) {
	source, err := normalizeGitHubBaselineSource(source)
	if err != nil {
		return GitHubBaselineEvidence{}, err
	}
	metadataSourceWasProvided := metadata.SourceID != ""
	metadata, err = normalizeMetadata(metadata, source)
	if err != nil {
		return GitHubBaselineEvidence{}, err
	}

	if response.StatusCode <= 0 || response.StatusCode > 599 {
		return GitHubBaselineEvidence{}, errors.New("GitHub baseline response status is invalid")
	}
	if response.StatusCode != 200 {
		return newGitHubBaselineEvidence(
			BaselineUnavailable,
			nil,
			fmt.Sprintf("http_status_%d", response.StatusCode),
			metadata,
		), nil
	}
	if len(response.Body) > MaxGitHubBaselinePayloadBytes {
		return GitHubBaselineEvidence{}, errors.New("GitHub baseline payload exceeded the bounded limit")
	}

	availability, contexts, reason, detectedSource, err := parseGitHubBaselinePayload(response.Body, source)
	if err != nil {
		return GitHubBaselineEvidence{}, err
	}
	if source == "" && !metadataSourceWasProvided {
		metadata.SourceID = string(detectedSource)
		metadata, err = normalizeMetadata(metadata, detectedSource)
		if err != nil {
			return GitHubBaselineEvidence{}, err
		}
	}
	return newGitHubBaselineEvidence(availability, contexts, reason, metadata), nil
}

func normalizeGitHubBaselineSource(source GitHubBaselineSource) (GitHubBaselineSource, error) {
	switch source {
	case "", GitHubBaselineSourceBranchProtection, GitHubBaselineSourceBranchRules:
		return source, nil
	default:
		return "", errors.New("GitHub baseline source is not registered")
	}
}

func validateGitHubBaselineSource(source GitHubBaselineSource) error {
	if source == "" {
		return nil
	}
	_, err := normalizeGitHubBaselineSource(source)
	return err
}

func newGitHubBaselineEvidence(availability BaselineAvailability, contexts []string, reason string, metadata GitHubBaselineMetadata) GitHubBaselineEvidence {
	if contexts == nil {
		contexts = []string{}
	}
	contexts, _ = normalizeContexts(contexts)
	evidence := GitHubBaselineEvidence{
		Availability:     availability,
		RequiredContexts: contexts,
		Reason:           reason,
		SourceID:         metadata.SourceID,
		ObservedAt:       metadata.ObservedAt,
	}
	evidence.SourceDigest = digestGitHubBaselineEvidence(evidence)
	return evidence
}

type normalizedGitHubBaselineEvidence struct {
	Availability     BaselineAvailability `json:"availability"`
	RequiredContexts []string             `json:"requiredContexts"`
	Reason           string               `json:"reason"`
}

func digestGitHubBaselineEvidence(evidence GitHubBaselineEvidence) string {
	normalized := normalizedGitHubBaselineEvidence{
		Availability:     evidence.Availability,
		RequiredContexts: evidence.RequiredContexts,
		Reason:           evidence.Reason,
	}
	encoded, _ := json.Marshal(normalized)
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// MergeGitHubBaselineEvidence reconciles independent authoritative GitHub
// sources. Required contexts are unioned only when every source is available.
// If any source is unknown or unavailable, the result remains non-authoritative
// and carries no contexts, so a failed refresh can never look like "no checks".
func MergeGitHubBaselineEvidence(evidence ...GitHubBaselineEvidence) GitHubBaselineEvidence {
	merged := GitHubBaselineEvidence{
		Availability:     BaselineUnknown,
		RequiredContexts: []string{},
		Reason:           "no_authoritative_sources",
		SourceID:         "github.authoritative_merge",
	}
	if len(evidence) == 0 {
		merged.SourceDigest = digestGitHubBaselineEvidence(merged)
		return merged
	}

	contextSet := make(map[string]struct{})
	anyUnavailable := false
	anyUnknown := false
	for _, source := range evidence {
		switch source.Availability {
		case BaselineAvailable:
			for _, context := range source.RequiredContexts {
				contextSet[context] = struct{}{}
			}
		case BaselineUnavailable:
			anyUnavailable = true
		case BaselineUnknown:
			anyUnknown = true
		default:
			anyUnknown = true
		}
		if source.ObservedAt.After(merged.ObservedAt) {
			merged.ObservedAt = source.ObservedAt
		}
	}

	switch {
	case anyUnavailable:
		merged.Availability = BaselineUnavailable
		merged.Reason = "authoritative_source_unavailable"
	case anyUnknown:
		merged.Availability = BaselineUnknown
		merged.Reason = "authoritative_source_unknown"
	default:
		merged.Availability = BaselineAvailable
		merged.Reason = ""
		merged.RequiredContexts = make([]string, 0, len(contextSet))
		for context := range contextSet {
			merged.RequiredContexts = append(merged.RequiredContexts, context)
		}
		merged.RequiredContexts, _ = normalizeContexts(merged.RequiredContexts)
	}
	merged.SourceDigest = digestGitHubBaselineEvidence(merged)
	return merged
}

// ReconcileGitHubBaselineEvidence is the descriptive alias used by baseline
// callers that treat this operation as a reconciliation pass.
func ReconcileGitHubBaselineEvidence(evidence ...GitHubBaselineEvidence) GitHubBaselineEvidence {
	return MergeGitHubBaselineEvidence(evidence...)
}

func normalizeMetadata(metadata GitHubBaselineMetadata, source GitHubBaselineSource) (GitHubBaselineMetadata, error) {
	if metadata.SourceID == "" {
		if source == "" {
			source = GitHubBaselineSourceBranchProtection
		}
		metadata.SourceID = string(source)
	}
	if err := validateSourceID(metadata.SourceID); err != nil {
		return GitHubBaselineMetadata{}, err
	}
	if !metadata.ObservedAt.IsZero() {
		metadata.ObservedAt = metadata.ObservedAt.UTC()
	}
	return metadata, nil
}

func parseGitHubBaselinePayload(payload []byte, requestedSource GitHubBaselineSource) (BaselineAvailability, []string, string, GitHubBaselineSource, error) {
	if len(payload) == 0 {
		return "", nil, "", "", errors.New("GitHub baseline payload is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	var document any
	if err := decoder.Decode(&document); err != nil {
		return "", nil, "", "", errors.New("GitHub baseline payload is invalid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", nil, "", "", errors.New("GitHub baseline payload contains more than one JSON value")
	}

	source := requestedSource
	if source == "" {
		source = detectGitHubBaselineSource(document)
	}
	if source == "" {
		return BaselineUnknown, []string{}, "unsupported_provider_shape", "", nil
	}

	var (
		availability BaselineAvailability
		contexts     []string
		reason       string
		valid        bool
	)
	switch source {
	case GitHubBaselineSourceBranchProtection:
		availability, contexts, reason, valid = parseLegacyProtection(document)
	case GitHubBaselineSourceBranchRules:
		availability, contexts, reason, valid = parseActiveBranchRules(document)
	default:
		return "", nil, "", "", errors.New("GitHub baseline source is not registered")
	}
	if !valid {
		return "", nil, "", "", errors.New("GitHub baseline payload parser returned an invalid state")
	}
	return availability, contexts, reason, source, nil
}

func detectGitHubBaselineSource(document any) GitHubBaselineSource {
	object, ok := document.(map[string]any)
	if ok {
		if _, found := object["protected"]; found {
			return GitHubBaselineSourceBranchProtection
		}
		if _, found := object["rules"]; found {
			return GitHubBaselineSourceBranchRules
		}
		return ""
	}

	entries, ok := document.([]any)
	if !ok || len(entries) == 0 {
		return ""
	}
	for _, entry := range entries {
		object, ok := entry.(map[string]any)
		if !ok {
			return ""
		}
		if _, found := object["type"]; !found {
			return ""
		}
	}
	return GitHubBaselineSourceBranchRules
}

func parseLegacyProtection(document any) (BaselineAvailability, []string, string, bool) {
	object, ok := document.(map[string]any)
	if !ok {
		return BaselineUnknown, []string{}, "branch_protection_state_unsupported", true
	}
	protected, ok := object["protected"].(bool)
	if !ok {
		return BaselineUnknown, []string{}, "branch_protection_state_unsupported", true
	}
	if !protected {
		return BaselineAvailable, []string{}, "", true
	}

	protection, found := object["protection"]
	if !found || protection == nil {
		return BaselineUnknown, []string{}, "branch_protection_unsupported", true
	}

	encoded, err := json.Marshal(protection)
	if err != nil {
		return BaselineUnknown, []string{}, "branch_protection_unsupported", true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil || fields == nil {
		return BaselineUnknown, []string{}, "branch_protection_unsupported", true
	}
	rawEnabled, found := fields["enabled"]
	if !found {
		return BaselineUnknown, []string{}, "branch_protection_enabled_unsupported", true
	}
	var enabled bool
	if err := json.Unmarshal(rawEnabled, &enabled); err != nil {
		return BaselineUnknown, []string{}, "branch_protection_enabled_unsupported", true
	}
	rawChecks, found := fields["required_status_checks"]
	if !found {
		return BaselineUnknown, []string{}, "required_status_checks_unsupported", true
	}
	contexts, recognized, valid := parseProtectionStatusChecks(rawChecks)
	if !valid || !recognized {
		return BaselineUnknown, []string{}, "required_status_checks_unsupported", true
	}
	contexts, valid = normalizeContexts(contexts)
	if !valid {
		return BaselineUnknown, []string{}, "required_status_contexts_unsupported", true
	}
	if !enabled {
		if len(contexts) != 0 {
			return BaselineUnknown, []string{}, "branch_protection_disabled_with_checks", true
		}
		return BaselineAvailable, []string{}, "", true
	}
	return BaselineAvailable, contexts, "", true
}

func parseActiveBranchRules(document any) (BaselineAvailability, []string, string, bool) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return BaselineUnknown, []string{}, "rules_unsupported", true
	}
	if object, ok := document.(map[string]any); ok {
		raw, found := object["rules"]
		if !found {
			return BaselineUnknown, []string{}, "rules_unsupported", true
		}
		encoded, err = json.Marshal(raw)
		if err != nil {
			return BaselineUnknown, []string{}, "rules_unsupported", true
		}
	}
	contexts, recognized, valid := parseRuleList(encoded)
	if !valid || !recognized {
		return BaselineUnknown, []string{}, "rules_unsupported", true
	}
	contexts, valid = normalizeContexts(contexts)
	if !valid {
		return BaselineUnknown, []string{}, "required_status_contexts_unsupported", true
	}
	return BaselineAvailable, contexts, "", true
}

func parseProtectionStatusChecks(raw json.RawMessage) ([]string, bool, bool) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return []string{}, true, true
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false, false
	}
	contexts := []string{}
	recognized := false
	if rawContexts, ok := fields["contexts"]; ok {
		values, valid := decodeStringArray(rawContexts)
		if !valid {
			return nil, false, false
		}
		contexts = append(contexts, values...)
		recognized = true
	}
	if rawChecks, ok := fields["checks"]; ok {
		values, valid := decodeCheckContexts(rawChecks)
		if !valid {
			return nil, false, false
		}
		contexts = append(contexts, values...)
		recognized = true
	}
	return contexts, recognized, true
}

func decodeStringArray(raw json.RawMessage) ([]string, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []string{}, true
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, false
	}
	return values, true
}

func decodeCheckContexts(raw json.RawMessage) ([]string, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []string{}, true
	}
	var checks []json.RawMessage
	if err := json.Unmarshal(raw, &checks); err != nil || checks == nil {
		return nil, false
	}
	contexts := make([]string, 0, len(checks))
	for _, rawCheck := range checks {
		var check map[string]json.RawMessage
		if err := json.Unmarshal(rawCheck, &check); err != nil || check == nil {
			return nil, false
		}
		rawContext, ok := check["context"]
		if !ok || bytes.Equal(bytes.TrimSpace(rawContext), []byte("null")) {
			return nil, false
		}
		var context string
		if err := json.Unmarshal(rawContext, &context); err != nil {
			return nil, false
		}
		if strings.TrimSpace(context) == "" {
			return nil, false
		}
		contexts = append(contexts, context)
	}
	return contexts, true
}

func parseRuleList(raw json.RawMessage) ([]string, bool, bool) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, false, false
	}
	if len(entries) == 0 {
		return []string{}, true, true
	}

	contexts := []string{}
	recognized := false
	for _, entry := range entries {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(entry, &object); err != nil || object == nil {
			return nil, false, false
		}
		if nested, ok := object["rules"]; ok {
			nestedContexts, nestedRecognized, valid := parseRuleList(nested)
			if !valid {
				return nil, false, false
			}
			contexts = append(contexts, nestedContexts...)
			recognized = recognized || nestedRecognized
			continue
		}

		rawType, ok := object["type"]
		if !ok {
			return nil, false, false
		}
		var ruleType string
		if err := json.Unmarshal(rawType, &ruleType); err != nil || ruleType == "" {
			return nil, false, false
		}
		recognized = true
		if ruleType != "required_status_checks" {
			continue
		}

		rawParameters, ok := object["parameters"]
		if !ok {
			return nil, false, false
		}
		parameterContexts, valid := parseRuleParameters(rawParameters)
		if !valid {
			return nil, false, false
		}
		contexts = append(contexts, parameterContexts...)
	}
	return contexts, recognized, true
}

func parseRuleParameters(raw json.RawMessage) ([]string, bool) {
	var parameters map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parameters); err != nil || parameters == nil {
		return nil, false
	}
	if rawChecks, ok := parameters["required_status_checks"]; ok {
		return decodeCheckContexts(rawChecks)
	}
	if rawContexts, ok := parameters["contexts"]; ok {
		return decodeStringArray(rawContexts)
	}
	if rawChecks, ok := parameters["checks"]; ok {
		return decodeCheckContexts(rawChecks)
	}
	return nil, false
}

func normalizeContexts(values []string) ([]string, bool) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !utf8.ValidString(value) || len(value) > 512 {
			return nil, false
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return nil, false
			}
		}
		seen[value] = struct{}{}
		if len(seen) > 256 {
			return nil, false
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, true
}

func validateGHPath(path string) error {
	if !safeText(path, false) || path != strings.TrimSpace(path) || containsShellSurface(path) {
		return errors.New("gh path contains unsafe text")
	}
	if len(path) < 8 || path[1] != ':' || !isASCIIAlpha(rune(path[0])) || (path[2] != '\\' && path[2] != '/') {
		return errors.New("gh path must be an absolute Windows path")
	}
	if strings.Contains(path[2:], ":") {
		return errors.New("gh path contains an alternate stream")
	}
	normalized := strings.ReplaceAll(path, "/", "\\")
	parts := strings.Split(normalized[3:], "\\")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return errors.New("gh path contains an ambiguous component")
		}
	}
	if !strings.EqualFold(parts[len(parts)-1], "gh.exe") {
		return errors.New("gh path must name gh.exe")
	}
	return nil
}

func validateOwner(owner string) error {
	if !safeText(owner, true) || len(owner) == 0 || len(owner) > 39 {
		return errors.New("GitHub owner is invalid")
	}
	for index, r := range owner {
		if !isASCIIAlphaNum(r) && (r != '-' || index == 0 || index == len(owner)-1 || (index > 0 && owner[index-1] == '-')) {
			return errors.New("GitHub owner is invalid")
		}
	}
	return nil
}

func validateRepository(repository string) error {
	if !safeText(repository, true) || len(repository) == 0 || len(repository) > 100 {
		return errors.New("GitHub repository is invalid")
	}
	if strings.HasSuffix(strings.ToLower(repository), ".git") {
		return errors.New("GitHub repository must not include a .git suffix")
	}
	for index, r := range repository {
		allowed := isASCIIAlphaNum(r) || r == '-' || r == '_' || r == '.'
		if !allowed || (index == 0 && !isASCIIAlphaNum(r)) || (index == len(repository)-1 && !isASCIIAlphaNum(r)) {
			return errors.New("GitHub repository is invalid")
		}
	}
	return nil
}

func validateBranch(branch string) error {
	if !safeText(branch, true) || branch == "" || len(branch) > 255 || branch != strings.TrimSpace(branch) {
		return errors.New("GitHub target branch is invalid")
	}
	if branch == "@" || strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "refs/") ||
		strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") ||
		strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.Contains(branch, "\\") ||
		strings.ContainsAny(branch, "~^:?*[]") || containsShellSurface(branch) ||
		strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return errors.New("GitHub target branch is invalid")
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return errors.New("GitHub target branch is invalid")
		}
	}
	return nil
}

func validateSourceID(sourceID string) error {
	if !safeText(sourceID, true) || len(sourceID) == 0 || len(sourceID) > 64 {
		return errors.New("GitHub baseline source identifier is invalid")
	}
	for index, r := range sourceID {
		allowed := isASCIIAlphaNum(r) || strings.ContainsRune("._:-", r)
		if !allowed || (index == 0 && !isASCIIAlphaNum(r)) {
			return errors.New("GitHub baseline source identifier is invalid")
		}
	}
	return nil
}

func safeText(value string, rejectWhitespace bool) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || (rejectWhitespace && unicode.IsSpace(r)) {
			return false
		}
	}
	return true
}

func containsShellSurface(value string) bool {
	return strings.ContainsAny(value, "\"'`&|<>^%;$()[]{}!#")
}

func isASCIIAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIAlphaNum(r rune) bool {
	return isASCIIAlpha(r) || (r >= '0' && r <= '9')
}
