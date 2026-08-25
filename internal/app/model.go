package app

import (
	"regexp"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

type IntegrationKind string

const (
	IntegrationGitHub     IntegrationKind = "github"
	IntegrationJenkins    IntegrationKind = "jenkins"
	IntegrationKubernetes IntegrationKind = "kubernetes"
)

type IntegrationConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Kind          IntegrationKind   `json:"kind"`
	Endpoint      string            `json:"endpoint"`
	CredentialRef string            `json:"credentialRef,omitempty"`
	Values        map[string]string `json:"values,omitempty"`
}

type IntegrationHealth struct {
	ID                  string          `json:"id"`
	Kind                IntegrationKind `json:"kind"`
	Endpoint            string          `json:"endpoint"`
	CredentialReference string          `json:"credentialReference,omitempty"`
	CredentialPresent   bool            `json:"credentialPresent"`
	Status              string          `json:"status"`
	HTTPStatus          int             `json:"httpStatus,omitempty"`
	Message             string          `json:"message"`
	CheckedAt           time.Time       `json:"checkedAt"`
}

type PowerShellRunbookConfig struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	ScriptPath           string   `json:"scriptPath"`
	Parameters           []string `json:"parameters,omitempty"`
	EnvironmentAllowlist []string `json:"environmentAllowlist,omitempty"`
	TimeoutSeconds       int      `json:"timeoutSeconds,omitempty"`
}

// ExternalWorkGroupConfig is a user-local, reusable set of typed external
// targets. It stores endpoint and credential references only; credential
// values and provider response bodies never belong here.
type ExternalWorkGroupConfig struct {
	ID      string                        `json:"id"`
	Name    string                        `json:"name"`
	Targets []ExternalJenkinsTargetConfig `json:"targets"`
}

type ExternalJenkinsTargetConfig struct {
	ID                string            `json:"id"`
	IntegrationID     string            `json:"integrationId"`
	CompletedBuildURL string            `json:"completedBuildUrl"`
	Parameters        map[string]string `json:"parameters,omitempty"`
	FallbackRunbookID string            `json:"fallbackRunbookId,omitempty"`
}

type ExternalJenkinsTargetPlan struct {
	ID                  string            `json:"id"`
	IntegrationID       string            `json:"integrationId"`
	UsernameReference   string            `json:"usernameReference,omitempty"`
	CredentialReference string            `json:"credentialReference,omitempty"`
	BaseURL             string            `json:"baseUrl"`
	Job                 string            `json:"job"`
	BuildEndpoint       string            `json:"buildEndpoint"`
	Parameters          map[string]string `json:"parameters,omitempty"`
}

type ExternalWorkGroupPlan struct {
	ActionPlan domain.ActionPlan           `json:"actionPlan"`
	Group      ExternalWorkGroupConfig     `json:"group"`
	Digest     string                      `json:"digest"`
	Targets    []ExternalJenkinsTargetPlan `json:"targets"`
	CreatedAt  time.Time                   `json:"createdAt"`
}

type ExternalWorkTargetResult struct {
	TargetID    string    `json:"targetId"`
	Status      string    `json:"status"`
	BuildNumber int64     `json:"buildNumber,omitempty"`
	BuildURL    string    `json:"buildUrl,omitempty"`
	Result      string    `json:"result,omitempty"`
	Failure     string    `json:"failure,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
}

type ExternalWorkGroupResult struct {
	PlanID      string                     `json:"planId"`
	Status      string                     `json:"status"`
	Outcomes    []ExternalWorkTargetResult `json:"outcomes"`
	CompletedAt time.Time                  `json:"completedAt"`
}

type ExternalWorkResultRecord struct {
	PlanID string                  `json:"planId"`
	Result ExternalWorkGroupResult `json:"result"`
}

type CleanupPlan struct {
	ActionPlan domain.ActionPlan       `json:"actionPlan"`
	Candidate  domain.CleanupCandidate `json:"candidate"`
	Digest     string                  `json:"digest"`
	CreatedAt  time.Time               `json:"createdAt"`
}

type CleanupResult struct {
	PlanID      string    `json:"planId"`
	CandidateID string    `json:"candidateId"`
	Status      string    `json:"status"`
	Path        string    `json:"path"`
	Branch      string    `json:"branch,omitempty"`
	Failure     string    `json:"failure,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
}

type ReleasePlanInput struct {
	GroupID          string `json:"groupId"`
	Environment      string `json:"environment"`
	ExpectedRevision string `json:"expectedRevision,omitempty"`
	ProjectID        string `json:"projectId"`
	RepositoryID     string `json:"repositoryId"`
	WorktreeID       string `json:"worktreeId"`
}

type ReleasePlan struct {
	ActionPlan       domain.ActionPlan           `json:"actionPlan"`
	Group            ExternalWorkGroupConfig     `json:"group"`
	Environment      string                      `json:"environment"`
	ExpectedRevision string                      `json:"expectedRevision,omitempty"`
	Digest           string                      `json:"digest"`
	Targets          []ExternalJenkinsTargetPlan `json:"targets"`
	Postchecks       []string                    `json:"postchecks"`
	CreatedAt        time.Time                   `json:"createdAt"`
}

type ReleasePostcheckEvidence struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ReleaseResult struct {
	PlanID      string                     `json:"planId"`
	Environment string                     `json:"environment"`
	Status      string                     `json:"status"`
	External    ExternalWorkGroupResult    `json:"external"`
	Postchecks  []ReleasePostcheckEvidence `json:"postchecks"`
	CompletedAt time.Time                  `json:"completedAt"`
}

var integrationValueKey = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
var runbookParameterPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

type Config struct {
	Version                  int                             `json:"version"`
	ScanIntervalSeconds      int                             `json:"scan_interval_seconds"`
	Projects                 []domain.Project                `json:"projects"`
	Environment              []domain.EnvironmentDeclaration `json:"environment,omitempty"`
	Connectors               []domain.ConnectorReference     `json:"connectors,omitempty"`
	Integrations             []IntegrationConfig             `json:"integrations,omitempty"`
	Runbooks                 []PowerShellRunbookConfig       `json:"runbooks,omitempty"`
	ExternalWorkGroups       []ExternalWorkGroupConfig       `json:"externalWorkGroups,omitempty"`
	ExternalWorkResults      []ExternalWorkResultRecord      `json:"externalWorkResults,omitempty"`
	CleanupResults           []CleanupResult                 `json:"cleanupResults,omitempty"`
	ReleaseResults           []ReleaseResult                 `json:"releaseResults,omitempty"`
	AgentProfilesInitialized bool                            `json:"agent_profiles_initialized,omitempty"`
}

type RepositoryState struct {
	ID            string            `json:"id"`
	Path          string            `json:"path"`
	TopLevel      string            `json:"top_level,omitempty"`
	Branch        string            `json:"branch,omitempty"`
	Origin        string            `json:"origin,omitempty"`
	Detached      bool              `json:"detached"`
	Dirty         bool              `json:"dirty"`
	Ahead         int               `json:"ahead"`
	Behind        int               `json:"behind"`
	Upstream      string            `json:"upstream,omitempty"`
	Provider      string            `json:"provider,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	WorktreeCount int               `json:"worktree_count"`
	Worktrees     []domain.Worktree `json:"worktrees,omitempty"`
	UnsafeCleanup bool              `json:"unsafe_cleanup"`
	Error         string            `json:"error,omitempty"`
	ScannedAt     time.Time         `json:"scanned_at"`
}

type ProjectState struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Repos     []RepositoryState `json:"repos"`
	ScannedAt time.Time         `json:"scanned_at"`
}

type Snapshot struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Projects    []ProjectState `json:"projects"`
}

type Event = domain.Event
