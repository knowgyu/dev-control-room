package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

const (
	defaultImpactPeriodDays = 30
	maxImpactPeriodDays     = 365
	artifactQuotaBytes      = int64(512 << 20)
	archiveManifestName     = "assurance-manifest.json"
)

// AssuranceImpactQuery describes the evidence scope for the effect surface.
// Now is intentionally test-only input: callers normally leave it zero so the
// service uses the current UTC clock.
type AssuranceImpactQuery struct {
	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	ProjectID string    `json:"projectId,omitempty"`
	Days      int       `json:"days,omitempty"`
	Now       time.Time `json:"-"`
}

type AssuranceMetric struct {
	Key           string                     `json:"key"`
	Label         string                     `json:"label"`
	Value         *float64                   `json:"value,omitempty"`
	Unit          string                     `json:"unit,omitempty"`
	State         string                     `json:"state"`
	SampleCount   int                        `json:"sampleCount"`
	EvidenceCount int                        `json:"evidenceCount"`
	Comparison    *AssuranceMetricComparison `json:"comparison,omitempty"`
}

type AssuranceMetricComparison struct {
	Kind          string   `json:"kind"`
	PreviousValue *float64 `json:"previousValue,omitempty"`
	Delta         *float64 `json:"delta,omitempty"`
	DeltaPercent  *float64 `json:"deltaPercent,omitempty"`
	State         string   `json:"state"`
}

type AssuranceTrendPoint struct {
	StartAt          time.Time `json:"startAt"`
	EndAt            time.Time `json:"endAt"`
	AgentInvocations int       `json:"agentInvocations"`
	SuccessfulAgents int       `json:"successfulAgents"`
	QualityRuns      int       `json:"qualityRuns"`
	SuccessfulRuns   int       `json:"successfulRuns"`
	Effects          int       `json:"effects"`
	VerifiedEffects  int       `json:"verifiedEffects"`
	Artifacts        int       `json:"artifacts"`
}

type AssuranceDataQuality struct {
	RecordsTotal        int       `json:"recordsTotal"`
	MeasuredEffects     int       `json:"measuredEffects"`
	UserEstimated       int       `json:"userEstimatedEffects"`
	AIInference         int       `json:"aiInferenceEffects"`
	UnavailableEffects  int       `json:"unavailableEffects"`
	UnattributedEffects int       `json:"unattributedEffects"`
	MissingEvidence     int       `json:"missingEvidence"`
	MissingArtifacts    int       `json:"missingArtifacts"`
	BaselineState       string    `json:"baselineState"`
	LastEvidenceAt      time.Time `json:"lastEvidenceAt,omitempty"`
}

type AssuranceTraceabilitySummary struct {
	EffectsTotal      int    `json:"effectsTotal"`
	CompleteEffects   int    `json:"completeEffects"`
	PartialEffects    int    `json:"partialEffects"`
	UnresolvedEffects int    `json:"unresolvedEffects"`
	LinkedArtifacts   int    `json:"linkedArtifacts"`
	MissingArtifacts  int    `json:"missingArtifacts"`
	Status            string `json:"status"`
}

type AssuranceImpactDashboard struct {
	GeneratedAt  time.Time                    `json:"generatedAt"`
	StartAt      time.Time                    `json:"startAt"`
	EndAt        time.Time                    `json:"endAt"`
	PeriodDays   int                          `json:"periodDays"`
	Provider     string                       `json:"providerFilter,omitempty"`
	Model        string                       `json:"modelFilter,omitempty"`
	ProjectID    string                       `json:"projectFilter,omitempty"`
	Comparison   string                       `json:"comparison"`
	Metrics      []AssuranceMetric            `json:"metrics"`
	Trend        []AssuranceTrendPoint        `json:"trend"`
	DataQuality  AssuranceDataQuality         `json:"dataQuality"`
	Traceability AssuranceTraceabilitySummary `json:"traceability"`
}

type AssuranceTraceNode struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Label       string     `json:"label"`
	TraceID     string     `json:"traceId,omitempty"`
	State       string     `json:"state,omitempty"`
	Scope       string     `json:"scope,omitempty"`
	Head        string     `json:"head,omitempty"`
	Digest      string     `json:"digest,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type AssuranceTraceLink struct {
	FromID   string `json:"fromId"`
	ToID     string `json:"toId"`
	Relation string `json:"relation"`
}

// AssuranceArtifactRef is safe to share: the operator's local storage path is
// deliberately not part of a trace or exported impact report.
type AssuranceArtifactRef struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	MIME       string `json:"mime"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Retention  string `json:"retention"`
	TraceID    string `json:"traceId,omitempty"`
	Present    bool   `json:"present"`
}

type AssuranceTrace struct {
	GeneratedAt time.Time              `json:"generatedAt"`
	Effect      domain.Effect          `json:"effect"`
	Nodes       []AssuranceTraceNode   `json:"nodes"`
	Links       []AssuranceTraceLink   `json:"links"`
	Artifacts   []AssuranceArtifactRef `json:"artifacts"`
	MissingRefs []string               `json:"missingRefs,omitempty"`
	Complete    bool                   `json:"complete"`
}

type ArtifactStorageSummary struct {
	GeneratedAt   time.Time `json:"generatedAt"`
	QuotaBytes    int64     `json:"quotaBytes"`
	UsedBytes     int64     `json:"usedBytes"`
	ActiveBytes   int64     `json:"activeBytes"`
	ArchivedBytes int64     `json:"archivedBytes"`
	PinnedBytes   int64     `json:"pinnedBytes"`
	ArtifactCount int       `json:"artifactCount"`
	MissingCount  int       `json:"missingCount"`
	DeletedCount  int       `json:"deletedCount"`
}

type AssuranceReportQuery struct {
	Format    string `json:"format,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	Days      int    `json:"days,omitempty"`
}

type AssuranceReportExport struct {
	Filename    string
	ContentType string
	Body        []byte
}

type assuranceObservation struct {
	Value         float64
	Available     bool
	SampleCount   int
	EvidenceCount int
}

type assuranceImpactCounts struct {
	Invocations       int
	SuccessfulAgents  int
	QualityRuns       int
	SuccessfulRuns    int
	Effects           int
	AdoptedEffects    int
	ReverifiedEffects int
	VerifiedEffects   int
	CompleteEffects   int
	Artifacts         int
	KnownTimeSaved    float64
	KnownTimeUnit     string
	KnownTimeCount    int
	KnownTimeMixed    bool
}

type assuranceInvocationContext struct {
	Provider string
	Model    string
}

func (a *App) AssuranceImpact(ctx context.Context, query AssuranceImpactQuery) (AssuranceImpactDashboard, error) {
	end := query.Now
	if end.IsZero() {
		end = time.Now().UTC()
	} else {
		end = end.UTC()
	}
	days := query.Days
	if days <= 0 {
		days = defaultImpactPeriodDays
	}
	if days > maxImpactPeriodDays {
		days = maxImpactPeriodDays
	}
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	previousStart := start.Add(-time.Duration(days) * 24 * time.Hour)

	invocations, err := a.AgentInvocations(ctx)
	if err != nil {
		return AssuranceImpactDashboard{}, err
	}
	runs, err := a.QualityRuns(ctx)
	if err != nil {
		return AssuranceImpactDashboard{}, err
	}
	effects, err := a.AssuranceEffects(ctx)
	if err != nil {
		return AssuranceImpactDashboard{}, err
	}
	artifacts, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return AssuranceImpactDashboard{}, err
	}

	provider, model, projectID := strings.TrimSpace(query.Provider), strings.TrimSpace(query.Model), strings.TrimSpace(query.ProjectID)
	invocationMap := make(map[string]domain.AgentInvocation, len(invocations))
	for _, item := range invocations {
		invocationMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			invocationMap[item.Spec.TraceID] = item
		}
	}
	runMap := make(map[string]domain.QualityRun, len(runs))
	for _, item := range runs {
		runMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			runMap[item.Spec.TraceID] = item
		}
	}
	artifactMap := make(map[string]domain.Artifact, len(artifacts))
	for _, item := range artifacts {
		artifactMap[item.Metadata.ID] = item
	}

	current := assuranceImpactCounts{}
	previous := assuranceImpactCounts{}
	selectedArtifactIDs := make(map[string]bool)
	trendStart := start
	trendStep := 24 * time.Hour
	if days > 31 {
		trendStep = 7 * 24 * time.Hour
	}
	trend := make([]AssuranceTrendPoint, 0, int(end.Sub(start)/trendStep)+1)
	for trendStart.Before(end) {
		trendEnd := trendStart.Add(trendStep)
		if trendEnd.After(end) {
			trendEnd = end
		}
		trend = append(trend, AssuranceTrendPoint{StartAt: trendStart, EndAt: trendEnd})
		if !trendEnd.After(trendStart) {
			break
		}
		trendStart = trendEnd
	}

	for _, item := range invocations {
		if !assuranceInvocationMatches(item, provider, model, projectID, runMap) {
			continue
		}
		at := item.Spec.StartedAt
		switch {
		case inImpactWindow(at, start, end):
			current.Invocations++
			markArtifactIDs(selectedArtifactIDs, item.Spec.ArtifactIDs)
			if item.Spec.State == domain.AssuranceStateSucceeded {
				current.SuccessfulAgents++
			}
			addInvocationTrend(trend, item, start, trendStep)
		case inImpactWindow(at, previousStart, start):
			previous.Invocations++
			markArtifactIDs(selectedArtifactIDs, item.Spec.ArtifactIDs)
			if item.Spec.State == domain.AssuranceStateSucceeded {
				previous.SuccessfulAgents++
			}
		}
	}

	for _, item := range runs {
		if !assuranceRunMatches(item, provider, model, projectID) {
			continue
		}
		at := item.Spec.StartedAt
		switch {
		case inImpactWindow(at, start, end):
			current.QualityRuns++
			markArtifactIDs(selectedArtifactIDs, item.Spec.ArtifactIDs)
			if item.Spec.State == domain.AssuranceStateSucceeded {
				current.SuccessfulRuns++
			}
			addRunTrend(trend, item, start, trendStep)
		case inImpactWindow(at, previousStart, start):
			previous.QualityRuns++
			markArtifactIDs(selectedArtifactIDs, item.Spec.ArtifactIDs)
			if item.Spec.State == domain.AssuranceStateSucceeded {
				previous.SuccessfulRuns++
			}
		}
	}

	dataQuality := AssuranceDataQuality{}
	currentEffects := make([]domain.Effect, 0, len(effects))
	findingRefs := make(map[string]bool)
	for _, item := range effects {
		if projectID != "" && item.Spec.ProjectID != projectID {
			continue
		}
		itemProvider, itemModel, attributed := effectProviderModel(item, invocationMap, runMap)
		if provider != "" && (!attributed || itemProvider != provider) {
			dataQuality.UnattributedEffects++
			continue
		}
		if model != "" && (!attributed || itemModel != model) {
			dataQuality.UnattributedEffects++
			continue
		}
		if inImpactWindow(item.Spec.CreatedAt, start, end) {
			currentEffects = append(currentEffects, item)
			markArtifactIDs(selectedArtifactIDs, item.Spec.EvidenceIDs)
			if findingID := strings.TrimSpace(item.Spec.SourceFindingID); findingID != "" && !findingRefs[findingID] {
				if _, err := a.store.GetFinding(ctx, findingID); err == nil {
					findingRefs[findingID] = true
				} else if !errors.Is(err, sql.ErrNoRows) {
					return AssuranceImpactDashboard{}, err
				}
			}
			addEffectCounts(&current, item, artifactMap, invocationMap, runMap, findingRefs)
			addEffectTrend(trend, item, start, trendStep, artifactMap, invocationMap, runMap, findingRefs)
		} else if inImpactWindow(item.Spec.CreatedAt, previousStart, start) {
			markArtifactIDs(selectedArtifactIDs, item.Spec.EvidenceIDs)
			if findingID := strings.TrimSpace(item.Spec.SourceFindingID); findingID != "" && !findingRefs[findingID] {
				if _, err := a.store.GetFinding(ctx, findingID); err == nil {
					findingRefs[findingID] = true
				} else if !errors.Is(err, sql.ErrNoRows) {
					return AssuranceImpactDashboard{}, err
				}
			}
			addEffectCounts(&previous, item, artifactMap, invocationMap, runMap, findingRefs)
		}
	}

	for _, item := range artifacts {
		if (provider != "" || model != "" || projectID != "") && !selectedArtifactIDs[item.Metadata.ID] {
			continue
		}
		if inImpactWindow(item.Spec.CreatedAt, start, end) {
			current.Artifacts++
			addArtifactTrend(trend, item, start, trendStep)
		} else if inImpactWindow(item.Spec.CreatedAt, previousStart, start) {
			previous.Artifacts++
		}
	}

	dataQuality.RecordsTotal = current.Invocations + current.QualityRuns + current.Effects + current.Artifacts
	for _, item := range currentEffects {
		switch item.Spec.Kind {
		case domain.EffectMeasured, domain.EffectPreventedRegression:
			dataQuality.MeasuredEffects++
		case domain.EffectUserEstimated:
			dataQuality.UserEstimated++
		case domain.EffectAIInference:
			dataQuality.AIInference++
		case domain.EffectUnavailable:
			dataQuality.UnavailableEffects++
		}
		if item.Spec.CreatedAt.After(dataQuality.LastEvidenceAt) {
			dataQuality.LastEvidenceAt = item.Spec.CreatedAt
		}
	}
	dataQuality.MissingEvidence = currentEffectsMissingEvidence(currentEffects, artifactMap)
	dataQuality.MissingArtifacts = missingArtifactCount(currentEffects, artifactMap)
	if previous.Invocations+previous.QualityRuns+previous.Effects+previous.Artifacts > 0 {
		dataQuality.BaselineState = "previous_equal_period"
	} else {
		dataQuality.BaselineState = "unavailable"
	}
	traceability := summarizeTraceability(currentEffects, invocationMap, runMap, artifactMap, findingRefs)

	metrics := []AssuranceMetric{
		impactCountMetric("quality_runs", "Quality Run", current.QualityRuns, previous.QualityRuns, current.QualityRuns, previous.QualityRuns),
		impactRateMetric("quality_success_rate", "Quality 성공률", current.SuccessfulRuns, current.QualityRuns, previous.SuccessfulRuns, previous.QualityRuns, current.QualityRuns),
		impactRateMetric("agent_success_rate", "Agent 성공률", current.SuccessfulAgents, current.Invocations, previous.SuccessfulAgents, previous.Invocations, current.Invocations),
		impactCountMetric("verified_effects", "검증된 효과", current.VerifiedEffects, previous.VerifiedEffects, current.Effects, previous.Effects),
		impactRateMetric("effect_adoption_rate", "효과 채택률", current.AdoptedEffects, current.Effects, previous.AdoptedEffects, previous.Effects, current.Effects),
		impactRateMetric("reverification_rate", "재검증률", current.ReverifiedEffects, current.Effects, previous.ReverifiedEffects, previous.Effects, current.Effects),
		impactRateMetric("evidence_coverage", "근거 완결성", current.CompleteEffects, current.Effects, previous.CompleteEffects, previous.Effects, current.Effects),
	}
	metrics = append(metrics, timeSavedMetric(current, previous))

	return AssuranceImpactDashboard{
		GeneratedAt:  time.Now().UTC(),
		StartAt:      start,
		EndAt:        end,
		PeriodDays:   days,
		Provider:     provider,
		Model:        model,
		ProjectID:    projectID,
		Comparison:   "previous_equal_period",
		Metrics:      metrics,
		Trend:        trend,
		DataQuality:  dataQuality,
		Traceability: traceability,
	}, nil
}

func (a *App) AssuranceTrace(ctx context.Context, effectID string) (AssuranceTrace, error) {
	effects, err := a.AssuranceEffects(ctx)
	if err != nil {
		return AssuranceTrace{}, err
	}
	var effect domain.Effect
	for _, item := range effects {
		if item.Metadata.ID == effectID {
			effect = item
			break
		}
	}
	if effect.Metadata.ID == "" {
		return AssuranceTrace{}, contract.NotFound("assurance effect not found")
	}
	runs, err := a.QualityRuns(ctx)
	if err != nil {
		return AssuranceTrace{}, err
	}
	invocations, err := a.AgentInvocations(ctx)
	if err != nil {
		return AssuranceTrace{}, err
	}
	artifacts, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return AssuranceTrace{}, err
	}
	findings, err := a.Findings(ctx, effect.Spec.ProjectID, effect.Spec.RepositoryID)
	if err != nil {
		return AssuranceTrace{}, err
	}
	runMap := make(map[string]domain.QualityRun, len(runs))
	runTraceMap := make(map[string]domain.QualityRun, len(runs))
	for _, item := range runs {
		runMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			runTraceMap[item.Spec.TraceID] = item
		}
	}
	invocationMap := make(map[string]domain.AgentInvocation, len(invocations))
	invocationTraceMap := make(map[string]domain.AgentInvocation, len(invocations))
	for _, item := range invocations {
		invocationMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			invocationTraceMap[item.Spec.TraceID] = item
		}
	}
	artifactMap := make(map[string]domain.Artifact, len(artifacts))
	artifactTraceMap := make(map[string]domain.Artifact, len(artifacts))
	for _, item := range artifacts {
		artifactMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			artifactTraceMap[item.Spec.TraceID] = item
		}
	}

	result := AssuranceTrace{GeneratedAt: time.Now().UTC(), Effect: effect, Nodes: []AssuranceTraceNode{}, Links: []AssuranceTraceLink{}, Artifacts: []AssuranceArtifactRef{}, MissingRefs: []string{}}
	result.Nodes = append(result.Nodes, AssuranceTraceNode{ID: effect.Metadata.ID, Kind: domain.EffectKind, Label: effect.Spec.Label, TraceID: effect.Spec.TraceID, State: effect.Spec.Kind, Scope: assuranceScopeLabel(effect.Spec.ProjectID, effect.Spec.RepositoryID, effect.Spec.WorktreeID), Digest: effect.Spec.Fingerprint})
	seenNodes := map[string]bool{effect.Metadata.ID: true}
	seenArtifacts := map[string]bool{}
	addNode := func(node AssuranceTraceNode) {
		if node.ID == "" || seenNodes[node.ID] {
			return
		}
		seenNodes[node.ID] = true
		result.Nodes = append(result.Nodes, node)
	}
	addLink := func(fromID, toID, relation string) {
		if fromID == "" || toID == "" || fromID == toID {
			return
		}
		for _, link := range result.Links {
			if link.FromID == fromID && link.ToID == toID && link.Relation == relation {
				return
			}
		}
		result.Links = append(result.Links, AssuranceTraceLink{FromID: fromID, ToID: toID, Relation: relation})
	}

	run, runOK := runMap[effect.Spec.SourceRunID]
	if !runOK {
		run, runOK = runTraceMap[effect.Spec.SourceRunID]
	}
	invocation, invocationOK := invocationMap[effect.Spec.SourceRunID]
	if !invocationOK {
		invocation, invocationOK = invocationTraceMap[effect.Spec.SourceRunID]
	}
	if runOK {
		addNode(AssuranceTraceNode{ID: run.Metadata.ID, Kind: domain.QualityRunKind, Label: run.Spec.Technique, TraceID: run.Spec.TraceID, State: run.Spec.State, Scope: assuranceScopeLabel(run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID), Head: run.Spec.Head, Digest: run.Spec.ConfigDigest, StartedAt: timePtr(run.Spec.StartedAt), CompletedAt: run.Spec.CompletedAt})
		addLink(run.Metadata.ID, effect.Metadata.ID, "source_run")
		for _, artifactID := range run.Spec.ArtifactIDs {
			addArtifactTrace(&result, artifactMap, seenArtifacts, artifactID, run.Metadata.ID, "run_evidence")
		}
	} else if invocationOK {
		addNode(AssuranceTraceNode{ID: invocation.Metadata.ID, Kind: domain.AgentInvocationKind, Label: invocation.Spec.Provider, TraceID: invocation.Spec.TraceID, State: invocation.Spec.State, Scope: assuranceScopeLabel(invocation.Spec.ProjectID, invocation.Spec.RepositoryID, invocation.Spec.WorktreeID), Head: invocation.Spec.Head, StartedAt: timePtr(invocation.Spec.StartedAt), CompletedAt: invocation.Spec.CompletedAt})
		addLink(invocation.Metadata.ID, effect.Metadata.ID, "source_invocation")
		for _, artifactID := range invocation.Spec.ArtifactIDs {
			addArtifactTrace(&result, artifactMap, seenArtifacts, artifactID, invocation.Metadata.ID, "invocation_evidence")
		}
	} else if effect.Spec.SourceRunID != "" {
		result.MissingRefs = append(result.MissingRefs, effect.Spec.SourceRunID)
	}

	if effect.Spec.SourceFindingID != "" {
		for _, finding := range findings {
			if finding.Metadata.ID != effect.Spec.SourceFindingID {
				continue
			}
			addNode(AssuranceTraceNode{ID: finding.Metadata.ID, Kind: domain.FindingKind, Label: finding.Spec.Summary, State: string(finding.Spec.State), Scope: assuranceScopeLabel(finding.Spec.ProjectID, finding.Spec.RepositoryID, "")})
			addLink(finding.Metadata.ID, effect.Metadata.ID, "source_finding")
			break
		}
		if !seenNodes[effect.Spec.SourceFindingID] {
			result.MissingRefs = append(result.MissingRefs, effect.Spec.SourceFindingID)
		}
	}
	for _, artifactID := range effect.Spec.EvidenceIDs {
		addArtifactTrace(&result, artifactMap, seenArtifacts, artifactID, effect.Metadata.ID, "effect_evidence")
	}
	for _, traceID := range effect.Spec.TraceIDs {
		if traceID == effect.Metadata.ID || seenNodes[traceID] || seenArtifacts[traceID] {
			continue
		}
		run, runOK := runMap[traceID]
		if !runOK {
			run, runOK = runTraceMap[traceID]
		}
		if runOK {
			addNode(AssuranceTraceNode{ID: run.Metadata.ID, Kind: domain.QualityRunKind, Label: run.Spec.Technique, TraceID: run.Spec.TraceID, State: run.Spec.State, Scope: assuranceScopeLabel(run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID), Head: run.Spec.Head, Digest: run.Spec.ConfigDigest, StartedAt: timePtr(run.Spec.StartedAt), CompletedAt: run.Spec.CompletedAt})
			addLink(run.Metadata.ID, effect.Metadata.ID, "trace")
			continue
		}
		invocation, invocationOK := invocationMap[traceID]
		if !invocationOK {
			invocation, invocationOK = invocationTraceMap[traceID]
		}
		if invocationOK {
			addNode(AssuranceTraceNode{ID: invocation.Metadata.ID, Kind: domain.AgentInvocationKind, Label: invocation.Spec.Provider, TraceID: invocation.Spec.TraceID, State: invocation.Spec.State, Scope: assuranceScopeLabel(invocation.Spec.ProjectID, invocation.Spec.RepositoryID, invocation.Spec.WorktreeID), Head: invocation.Spec.Head, StartedAt: timePtr(invocation.Spec.StartedAt), CompletedAt: invocation.Spec.CompletedAt})
			addLink(invocation.Metadata.ID, effect.Metadata.ID, "trace")
			continue
		}
		artifactID := traceID
		if _, ok := artifactMap[artifactID]; !ok {
			if artifact, ok := artifactTraceMap[traceID]; ok {
				artifactID = artifact.Metadata.ID
			}
		}
		if _, ok := artifactMap[artifactID]; ok {
			addArtifactTrace(&result, artifactMap, seenArtifacts, artifactID, effect.Metadata.ID, "trace_evidence")
			continue
		}
		result.MissingRefs = append(result.MissingRefs, traceID)
	}
	sort.Strings(result.MissingRefs)
	hasSource := runOK || invocationOK || seenNodes[effect.Spec.SourceFindingID]
	reverificationLinked := traceReferenceExists(effect.Spec.ReverificationRunID, runMap, runTraceMap, invocationMap, invocationTraceMap)
	result.Complete = len(result.MissingRefs) == 0 && effectEvidenceComplete(effect, artifactMap) && hasSource && effectAdoptionMetadataComplete(effect) && reverificationLinked
	return result, nil
}

func (a *App) AssuranceArtifactStorage(ctx context.Context) (ArtifactStorageSummary, error) {
	items, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return ArtifactStorageSummary{}, err
	}
	result := ArtifactStorageSummary{GeneratedAt: time.Now().UTC(), QuotaBytes: artifactQuotaBytes, ArtifactCount: len(items)}
	for _, item := range items {
		switch item.Spec.Retention {
		case domain.ArtifactRetentionDeleted:
			result.DeletedCount++
			continue
		case domain.ArtifactRetentionPinned:
			result.PinnedBytes += item.Spec.Size
		case domain.ArtifactRetentionArchived:
			result.ArchivedBytes += item.Spec.Size
		default:
			result.ActiveBytes += item.Spec.Size
		}
		result.UsedBytes += item.Spec.Size
		if !artifactPresent(item) {
			result.MissingCount++
		}
	}
	return result, nil
}

func (a *App) ExportAssuranceReport(ctx context.Context, query AssuranceReportQuery) (AssuranceReportExport, error) {
	format := strings.ToLower(strings.TrimSpace(query.Format))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		return AssuranceReportExport{}, contract.InvalidInput("assurance report format must be json or csv")
	}
	impact, err := a.AssuranceImpact(ctx, AssuranceImpactQuery{Provider: query.Provider, Model: query.Model, ProjectID: query.ProjectID, Days: query.Days})
	if err != nil {
		return AssuranceReportExport{}, err
	}
	if format == "csv" {
		body, err := marshalImpactCSV(impact)
		if err != nil {
			return AssuranceReportExport{}, err
		}
		return AssuranceReportExport{Filename: "dev-control-room-assurance.csv", ContentType: "text/csv; charset=utf-8", Body: body}, nil
	}
	effects, err := a.AssuranceEffects(ctx)
	if err != nil {
		return AssuranceReportExport{}, err
	}
	runs, err := a.QualityRuns(ctx)
	if err != nil {
		return AssuranceReportExport{}, err
	}
	invocations, err := a.AgentInvocations(ctx)
	if err != nil {
		return AssuranceReportExport{}, err
	}
	artifacts, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return AssuranceReportExport{}, err
	}
	storage, err := a.AssuranceArtifactStorage(ctx)
	if err != nil {
		return AssuranceReportExport{}, err
	}
	invocationMap := make(map[string]domain.AgentInvocation, len(invocations))
	for _, item := range invocations {
		invocationMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			invocationMap[item.Spec.TraceID] = item
		}
	}
	runMap := make(map[string]domain.QualityRun, len(runs))
	for _, item := range runs {
		runMap[item.Metadata.ID] = item
		if item.Spec.TraceID != "" {
			runMap[item.Spec.TraceID] = item
		}
	}
	scopedEffects := filterReportEffects(effects, impact, query, invocationMap, runMap)
	scopedArtifacts := reportArtifacts(scopedEffects, invocationMap, runMap, artifacts)
	report := struct {
		Schema      string                   `json:"schema"`
		GeneratedAt time.Time                `json:"generatedAt"`
		Impact      AssuranceImpactDashboard `json:"impact"`
		Effects     []domain.Effect          `json:"effects"`
		Artifacts   []AssuranceArtifactRef   `json:"artifacts"`
		Storage     ArtifactStorageSummary   `json:"storage"`
	}{Schema: "devroom/assurance-report/v1", GeneratedAt: time.Now().UTC(), Impact: impact, Effects: scopedEffects, Artifacts: mapArtifactRefs(scopedArtifacts), Storage: storage}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return AssuranceReportExport{}, err
	}
	body = append(body, '\n')
	return AssuranceReportExport{Filename: "dev-control-room-assurance.json", ContentType: "application/json; charset=utf-8", Body: body}, nil
}

func marshalImpactCSV(impact AssuranceImpactDashboard) ([]byte, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write([]string{"metric", "label", "value", "unit", "state", "sample_count", "evidence_count", "previous_value", "delta", "delta_percent"}); err != nil {
		return nil, err
	}
	for _, metric := range impact.Metrics {
		row := []string{metric.Key, metric.Label, "", metric.Unit, metric.State, strconv.Itoa(metric.SampleCount), strconv.Itoa(metric.EvidenceCount), "", "", ""}
		if metric.Value != nil {
			row[2] = strconv.FormatFloat(*metric.Value, 'f', -1, 64)
		}
		if metric.Comparison != nil {
			if metric.Comparison.PreviousValue != nil {
				row[7] = strconv.FormatFloat(*metric.Comparison.PreviousValue, 'f', -1, 64)
			}
			if metric.Comparison.Delta != nil {
				row[8] = strconv.FormatFloat(*metric.Comparison.Delta, 'f', -1, 64)
			}
			if metric.Comparison.DeltaPercent != nil {
				row[9] = strconv.FormatFloat(*metric.Comparison.DeltaPercent, 'f', -1, 64)
			}
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

func filterReportEffects(effects []domain.Effect, impact AssuranceImpactDashboard, query AssuranceReportQuery, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun) []domain.Effect {
	provider, model, projectID := strings.TrimSpace(query.Provider), strings.TrimSpace(query.Model), strings.TrimSpace(query.ProjectID)
	result := make([]domain.Effect, 0, len(effects))
	for _, item := range effects {
		if !inImpactWindow(item.Spec.CreatedAt, impact.StartAt, impact.EndAt) {
			continue
		}
		if projectID != "" && item.Spec.ProjectID != projectID {
			continue
		}
		itemProvider, itemModel, attributed := effectProviderModel(item, invocations, runs)
		if provider != "" && (!attributed || itemProvider != provider) {
			continue
		}
		if model != "" && (!attributed || itemModel != model) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func reportArtifacts(effects []domain.Effect, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun, artifacts []domain.Artifact) []domain.Artifact {
	wanted := make(map[string]bool)
	for _, effect := range effects {
		markArtifactIDs(wanted, effect.Spec.EvidenceIDs)
		for _, traceID := range effect.Spec.TraceIDs {
			if artifact, ok := findArtifactByIDOrTrace(artifacts, traceID); ok {
				wanted[artifact.Metadata.ID] = true
			}
		}
		if invocation, ok := invocations[effect.Spec.SourceRunID]; ok {
			markArtifactIDs(wanted, invocation.Spec.ArtifactIDs)
		}
		if run, ok := runs[effect.Spec.SourceRunID]; ok {
			markArtifactIDs(wanted, run.Spec.ArtifactIDs)
		}
	}
	result := make([]domain.Artifact, 0, len(wanted))
	for _, artifact := range artifacts {
		if wanted[artifact.Metadata.ID] {
			result = append(result, artifact)
		}
	}
	return result
}

func findArtifactByIDOrTrace(artifacts []domain.Artifact, ref string) (domain.Artifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Metadata.ID == ref || artifact.Spec.TraceID == ref {
			return artifact, true
		}
	}
	return domain.Artifact{}, false
}

func impactCountMetric(key, label string, current, previous, evidence, previousEvidence int) AssuranceMetric {
	cur := assuranceObservation{Value: float64(current), Available: evidence > 0, SampleCount: current, EvidenceCount: evidence}
	prev := assuranceObservation{Value: float64(previous), Available: previousEvidence > 0, SampleCount: previous, EvidenceCount: previousEvidence}
	return metricWithComparison(key, label, cur, prev, "count")
}

func impactRateMetric(key, label string, numerator, denominator, previousNumerator, previousDenominator, evidence int) AssuranceMetric {
	cur := assuranceObservation{Available: denominator > 0, SampleCount: denominator, EvidenceCount: evidence}
	if cur.Available {
		cur.Value = float64(numerator) * 100 / float64(denominator)
	}
	prev := assuranceObservation{Available: previousDenominator > 0, SampleCount: previousDenominator}
	if prev.Available {
		prev.Value = float64(previousNumerator) * 100 / float64(previousDenominator)
	}
	return metricWithComparison(key, label, cur, prev, "%")
}

func timeSavedMetric(current, previous assuranceImpactCounts) AssuranceMetric {
	if current.KnownTimeCount == 0 || current.KnownTimeMixed {
		return AssuranceMetric{
			Key: "time_saved", Label: "기록된 시간 절감", Unit: current.KnownTimeUnit,
			State: "unavailable", SampleCount: current.KnownTimeCount, EvidenceCount: current.KnownTimeCount,
			Comparison: &AssuranceMetricComparison{Kind: "previous_equal_period", State: "unavailable"},
		}
	}
	currentObservation := assuranceObservation{Value: current.KnownTimeSaved, Available: true, SampleCount: current.KnownTimeCount, EvidenceCount: current.KnownTimeCount}
	previousAvailable := previous.KnownTimeCount > 0 && !previous.KnownTimeMixed && previous.KnownTimeUnit == current.KnownTimeUnit
	previousObservation := assuranceObservation{Value: previous.KnownTimeSaved, Available: previousAvailable, SampleCount: previous.KnownTimeCount, EvidenceCount: previous.KnownTimeCount}
	return metricWithComparison("time_saved", "기록된 시간 절감", currentObservation, previousObservation, current.KnownTimeUnit)
}

func metricWithComparison(key, label string, current, previous assuranceObservation, unit string) AssuranceMetric {
	metric := AssuranceMetric{Key: key, Label: label, Unit: unit, State: "unavailable", SampleCount: current.SampleCount, EvidenceCount: current.EvidenceCount}
	if current.Available {
		value := current.Value
		metric.Value = &value
		metric.State = "measured"
	}
	comparison := &AssuranceMetricComparison{Kind: "previous_equal_period", State: "unavailable"}
	if previous.Available {
		previousValue := previous.Value
		comparison.PreviousValue = &previousValue
		if current.Available {
			delta := current.Value - previous.Value
			comparison.Delta = &delta
			switch {
			case delta > 0:
				comparison.State = "increase"
			case delta < 0:
				comparison.State = "decrease"
			default:
				comparison.State = "neutral"
			}
			if previous.Value != 0 {
				percent := delta / previous.Value * 100
				comparison.DeltaPercent = &percent
			}
		}
	}
	metric.Comparison = comparison
	return metric
}

func assuranceInvocationMatches(item domain.AgentInvocation, provider, model, projectID string, runs map[string]domain.QualityRun) bool {
	if provider != "" && item.Spec.Provider != provider {
		return false
	}
	if model != "" && item.Spec.RequestedModel != model && item.Spec.ResolvedModel != model {
		return false
	}
	if projectID == "" {
		return true
	}
	if item.Spec.ProjectID != "" {
		return item.Spec.ProjectID == projectID
	}
	for _, run := range runs {
		for _, invocationID := range run.Spec.InvocationIDs {
			if invocationID == item.Metadata.ID && run.Spec.ProjectID == projectID {
				return true
			}
		}
	}
	return projectID == ""
}

func markArtifactIDs(selected map[string]bool, ids []string) {
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			selected[id] = true
		}
	}
}

func assuranceRunMatches(item domain.QualityRun, provider, model, projectID string) bool {
	if projectID != "" && item.Spec.ProjectID != projectID {
		return false
	}
	if provider != "" {
		value, _ := item.Spec.Evidence["provider"].(string)
		if value != provider {
			return false
		}
	}
	if model != "" {
		value, _ := item.Spec.Evidence["model"].(string)
		if value != model {
			return false
		}
	}
	return true
}

func effectProviderModel(item domain.Effect, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun) (string, string, bool) {
	if invocation, ok := invocations[item.Spec.SourceRunID]; ok {
		return invocation.Spec.Provider, firstNonEmpty(invocation.Spec.ResolvedModel, invocation.Spec.RequestedModel), true
	}
	if run, ok := runs[item.Spec.SourceRunID]; ok {
		provider, _ := run.Spec.Evidence["provider"].(string)
		model, _ := run.Spec.Evidence["model"].(string)
		return provider, model, provider != "" || model != ""
	}
	return "", "", false
}

func addEffectCounts(counts *assuranceImpactCounts, item domain.Effect, artifacts map[string]domain.Artifact, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun, findingRefs map[string]bool) {
	counts.Effects++
	if item.Spec.Adopted {
		counts.AdoptedEffects++
	}
	if item.Spec.Reverified {
		counts.ReverifiedEffects++
	}
	complete := effectEvidenceComplete(item, artifacts)
	if complete {
		counts.CompleteEffects++
	}
	if complete && effectSourceLinked(item, invocations, runs, findingRefs) && effectAdoptionComplete(item, invocations, runs) && (item.Spec.Kind == domain.EffectMeasured || item.Spec.Kind == domain.EffectPreventedRegression) {
		counts.VerifiedEffects++
	}
	if item.Spec.MetricKey != "time_saved" || !item.Spec.ValueKnown || item.Spec.Unit == "" || (item.Spec.Kind != domain.EffectMeasured && item.Spec.Kind != domain.EffectUserEstimated) {
		return
	}
	unit := strings.ToLower(strings.TrimSpace(item.Spec.Unit))
	if counts.KnownTimeCount == 0 {
		counts.KnownTimeUnit = unit
		counts.KnownTimeSaved = item.Spec.Value
		counts.KnownTimeCount = 1
		return
	}
	counts.KnownTimeCount++
	if counts.KnownTimeUnit != unit {
		counts.KnownTimeMixed = true
		return
	}
	if !counts.KnownTimeMixed {
		counts.KnownTimeSaved += item.Spec.Value
	}
}

func effectSourceLinked(item domain.Effect, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun, findingRefs map[string]bool) bool {
	if item.Spec.SourceFindingID != "" {
		return findingRefs[item.Spec.SourceFindingID]
	}
	if item.Spec.SourceRunID == "" {
		return false
	}
	if _, ok := invocations[item.Spec.SourceRunID]; ok {
		return true
	}
	_, ok := runs[item.Spec.SourceRunID]
	return ok
}

func effectAdoptionComplete(item domain.Effect, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun) bool {
	if !effectAdoptionMetadataComplete(item) {
		return false
	}
	_, runExists := runs[item.Spec.ReverificationRunID]
	_, invocationExists := invocations[item.Spec.ReverificationRunID]
	return runExists || invocationExists
}

func effectAdoptionMetadataComplete(item domain.Effect) bool {
	return item.Spec.Adopted && item.Spec.Reverified && strings.TrimSpace(item.Spec.AdoptedCommit) != "" && strings.TrimSpace(item.Spec.ReverifiedCommit) != "" && strings.TrimSpace(item.Spec.ReverificationRunID) != ""
}

func traceReferenceExists(ref string, runs, runTraces map[string]domain.QualityRun, invocations, invocationTraces map[string]domain.AgentInvocation) bool {
	if strings.TrimSpace(ref) == "" {
		return false
	}
	if _, ok := runs[ref]; ok {
		return true
	}
	if _, ok := runTraces[ref]; ok {
		return true
	}
	if _, ok := invocations[ref]; ok {
		return true
	}
	_, ok := invocationTraces[ref]
	return ok
}

func effectEvidenceComplete(item domain.Effect, artifacts map[string]domain.Artifact) bool {
	if len(item.Spec.EvidenceIDs) == 0 {
		return false
	}
	for _, id := range item.Spec.EvidenceIDs {
		artifact, ok := artifacts[id]
		if !ok || artifact.Spec.Retention == domain.ArtifactRetentionDeleted || !artifactPresent(artifact) {
			return false
		}
	}
	return true
}

func currentEffectsMissingEvidence(effects []domain.Effect, artifacts map[string]domain.Artifact) int {
	missing := 0
	for _, item := range effects {
		if !effectEvidenceComplete(item, artifacts) {
			missing++
		}
	}
	return missing
}

func missingArtifactCount(effects []domain.Effect, artifacts map[string]domain.Artifact) int {
	missing := 0
	seen := map[string]bool{}
	for _, item := range effects {
		for _, id := range item.Spec.EvidenceIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			artifact, ok := artifacts[id]
			if !ok || artifact.Spec.Retention == domain.ArtifactRetentionDeleted || !artifactPresent(artifact) {
				missing++
			}
		}
	}
	return missing
}

func summarizeTraceability(effects []domain.Effect, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun, artifacts map[string]domain.Artifact, findingRefs map[string]bool) AssuranceTraceabilitySummary {
	result := AssuranceTraceabilitySummary{EffectsTotal: len(effects)}
	for _, item := range effects {
		hasSource := item.Spec.SourceFindingID != "" && findingRefs[item.Spec.SourceFindingID]
		if item.Spec.SourceRunID != "" {
			_, hasRun := runs[item.Spec.SourceRunID]
			_, hasInvocation := invocations[item.Spec.SourceRunID]
			hasSource = hasRun || hasInvocation
		}
		linked := 0
		for _, id := range item.Spec.EvidenceIDs {
			if artifact, ok := artifacts[id]; ok && artifact.Spec.Retention != domain.ArtifactRetentionDeleted && artifactPresent(artifact) {
				linked++
			} else {
				result.MissingArtifacts++
			}
		}
		result.LinkedArtifacts += linked
		complete := hasSource && len(item.Spec.EvidenceIDs) > 0 && linked == len(item.Spec.EvidenceIDs) && effectAdoptionComplete(item, invocations, runs)
		partial := hasSource || len(item.Spec.EvidenceIDs) > 0 || len(item.Spec.TraceIDs) > 0
		switch {
		case complete:
			result.CompleteEffects++
		case partial:
			result.PartialEffects++
		default:
			result.UnresolvedEffects++
		}
	}
	switch {
	case result.EffectsTotal == 0:
		result.Status = "unavailable"
	case result.CompleteEffects == result.EffectsTotal:
		result.Status = "complete"
	case result.CompleteEffects > 0 || result.PartialEffects > 0:
		result.Status = "partial"
	default:
		result.Status = "unresolved"
	}
	return result
}

func addInvocationTrend(trend []AssuranceTrendPoint, item domain.AgentInvocation, start time.Time, step time.Duration) {
	index := impactTrendIndex(item.Spec.StartedAt, start, step)
	if index < 0 || index >= len(trend) {
		return
	}
	trend[index].AgentInvocations++
	if item.Spec.State == domain.AssuranceStateSucceeded {
		trend[index].SuccessfulAgents++
	}
}

func addRunTrend(trend []AssuranceTrendPoint, item domain.QualityRun, start time.Time, step time.Duration) {
	index := impactTrendIndex(item.Spec.StartedAt, start, step)
	if index < 0 || index >= len(trend) {
		return
	}
	trend[index].QualityRuns++
	if item.Spec.State == domain.AssuranceStateSucceeded {
		trend[index].SuccessfulRuns++
	}
}

func addEffectTrend(trend []AssuranceTrendPoint, item domain.Effect, start time.Time, step time.Duration, artifacts map[string]domain.Artifact, invocations map[string]domain.AgentInvocation, runs map[string]domain.QualityRun, findingRefs map[string]bool) {
	index := impactTrendIndex(item.Spec.CreatedAt, start, step)
	if index < 0 || index >= len(trend) {
		return
	}
	trend[index].Effects++
	if effectEvidenceComplete(item, artifacts) && effectSourceLinked(item, invocations, runs, findingRefs) && effectAdoptionComplete(item, invocations, runs) && (item.Spec.Kind == domain.EffectMeasured || item.Spec.Kind == domain.EffectPreventedRegression) {
		trend[index].VerifiedEffects++
	}
}

func addArtifactTrend(trend []AssuranceTrendPoint, item domain.Artifact, start time.Time, step time.Duration) {
	index := impactTrendIndex(item.Spec.CreatedAt, start, step)
	if index >= 0 && index < len(trend) {
		trend[index].Artifacts++
	}
}

func impactTrendIndex(at, start time.Time, step time.Duration) int {
	if at.Before(start) {
		return -1
	}
	return int(at.Sub(start) / step)
}

func inImpactWindow(at, start, end time.Time) bool {
	return !at.IsZero() && !at.Before(start) && at.Before(end)
}

func assuranceScopeLabel(projectID, repositoryID, worktreeID string) string {
	return strings.Join(filterNonEmpty(projectID, repositoryID, worktreeID), " / ")
}

func filterNonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func addArtifactTrace(result *AssuranceTrace, artifacts map[string]domain.Artifact, seen map[string]bool, artifactID, fromID, relation string) {
	artifact, ok := artifacts[artifactID]
	if !ok {
		result.MissingRefs = appendUniqueStrings(result.MissingRefs, artifactID)
		return
	}
	if !seen[artifactID] {
		seen[artifactID] = true
		result.Artifacts = append(result.Artifacts, assuranceArtifactRef(artifact))
		result.Nodes = append(result.Nodes, AssuranceTraceNode{ID: artifact.Metadata.ID, Kind: domain.ArtifactKind, Label: artifact.Metadata.Name, TraceID: artifact.Spec.TraceID, State: artifact.Spec.Retention, Digest: artifact.Spec.SHA256})
	}
	if !artifactPresent(artifact) {
		result.MissingRefs = appendUniqueStrings(result.MissingRefs, artifactID)
	}
	result.Links = appendUniqueTraceLink(result.Links, AssuranceTraceLink{FromID: fromID, ToID: artifactID, Relation: relation})
}

func assuranceArtifactRef(item domain.Artifact) AssuranceArtifactRef {
	return AssuranceArtifactRef{ID: item.Metadata.ID, Name: item.Metadata.Name, SourceType: item.Spec.SourceType, SourceID: item.Spec.SourceID, MIME: item.Spec.MIME, Size: item.Spec.Size, SHA256: item.Spec.SHA256, Retention: item.Spec.Retention, TraceID: item.Spec.TraceID, Present: artifactPresent(item)}
}

func artifactPresent(item domain.Artifact) bool {
	if item.Spec.Retention == domain.ArtifactRetentionDeleted {
		return false
	}
	if artifactFileHasSize(item.Spec.Path, item.Spec.Size) {
		return true
	}
	if strings.TrimSpace(item.Spec.ArchivePath) == "" {
		return false
	}
	path, size, err := archivedArtifactPath(item)
	return err == nil && artifactFileHasSize(path, size)
}

func artifactFileHasSize(path string, expectedSize int64) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Size() == expectedSize
}

func mapArtifactRefs(items []domain.Artifact) []AssuranceArtifactRef {
	result := make([]AssuranceArtifactRef, 0, len(items))
	for _, item := range items {
		result = append(result, assuranceArtifactRef(item))
	}
	return result
}

func appendUniqueTraceLink(links []AssuranceTraceLink, candidate AssuranceTraceLink) []AssuranceTraceLink {
	for _, link := range links {
		if link == candidate {
			return links
		}
	}
	return append(links, candidate)
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// AssuranceArchiveManifest is intentionally a separate local file. It lets a
// later restore verify an archive without trusting a mutable database row.
type AssuranceArchiveManifest struct {
	Schema     string                         `json:"schema"`
	CreatedAt  time.Time                      `json:"createdAt"`
	ArtifactID []AssuranceArchiveManifestItem `json:"artifacts"`
}

type AssuranceArchiveManifestItem struct {
	ArtifactID string `json:"artifactId"`
	Filename   string `json:"filename"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	MIME       string `json:"mime"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
}

func readArchiveManifest(path string, expectedHash string) (AssuranceArchiveManifest, error) {
	data, err := readRegularFile(path)
	if err != nil {
		return AssuranceArchiveManifest{}, err
	}
	if expectedHash != "" && digestBytes(data) != expectedHash {
		return AssuranceArchiveManifest{}, errors.New("assurance archive manifest hash mismatch")
	}
	var manifest AssuranceArchiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return AssuranceArchiveManifest{}, err
	}
	if manifest.Schema != "devroom/assurance-archive/v1" || len(manifest.ArtifactID) == 0 {
		return AssuranceArchiveManifest{}, errors.New("assurance archive manifest is invalid")
	}
	return manifest, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
