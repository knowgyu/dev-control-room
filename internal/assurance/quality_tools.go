package assurance

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

// QualityToolState describes the locally observable state of one known tool.
// A PATH hit is intentionally not treated as proof that the candidate is a
// usable executable. The inspector never executes a discovered candidate.
type QualityToolState string

const (
	QualityToolStatePresentUnverified QualityToolState = "present_unverified"
	QualityToolStateMissing           QualityToolState = "missing"
	QualityToolStateUntrusted         QualityToolState = "untrusted"

	// QualityToolStateAvailable is retained as a source-compatible alias for
	// callers that used the old name. It no longer serializes as "available".
	QualityToolStateAvailable QualityToolState = QualityToolStatePresentUnverified
)

// QualityCapabilityState describes whether Dev Control Room can use a
// reviewed quality capability with the currently observable prerequisites.
type QualityCapabilityState string

const (
	QualityCapabilityStatePresentUnverified      QualityCapabilityState = "present_unverified"
	QualityCapabilityStateMissing                QualityCapabilityState = "missing"
	QualityCapabilityStateNeedsTarget            QualityCapabilityState = "needs_target"
	QualityCapabilityStateInstalledNotRegistered QualityCapabilityState = "installed_not_registered"
	QualityCapabilityStateUntrusted              QualityCapabilityState = "untrusted"

	// QualityCapabilityStateAvailable is retained as a source-compatible alias
	// for callers that used the old name. It no longer serializes as "available".
	QualityCapabilityStateAvailable QualityCapabilityState = QualityCapabilityStatePresentUnverified
)

type QualityExecutionReadiness string

const QualityExecutionReadinessNotEvaluated QualityExecutionReadiness = "not_evaluated"

const (
	QualityToolReasonMissing           = "tool.missing"
	QualityToolReasonPresentUnverified = "tool.present_unverified"
	QualityToolReasonUntrusted         = "tool.untrusted"
	QualityRunnerReasonUnregistered    = "runner.unregistered"
	QualityTargetReasonNotEvaluated    = "target.not_evaluated"
	QualityExecutionReasonNotEvaluated = "execution.not_evaluated"
)

const (
	QualityToolGovulncheckID = "govulncheck.exe"
	QualityToolStaticcheckID = "staticcheck.exe"
	QualityToolGosecID       = "gosec.exe"
	QualityToolGitleaksID    = "gitleaks.exe"
)

// QualityToolStatus is a safe, read-only description of one PATH candidate.
// A candidate is never executed by this endpoint, so it cannot be reported as
// an actually ready executable.
type QualityToolStatus struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	State              QualityToolState          `json:"state"`
	DiscoveryState     QualityToolState          `json:"discoveryState"`
	ExecutionReadiness QualityExecutionReadiness `json:"executionReadiness"`
	Path               string                    `json:"path,omitempty"`
	Version            string                    `json:"version,omitempty"`
	CheckedAt          time.Time                 `json:"checkedAt"`
	Reason             string                    `json:"reason"`
	ReasonCode         string                    `json:"reasonCode"`
	ReasonCodes        []string                  `json:"reasonCodes"`
	InstallGuidance    string                    `json:"installGuidance"`
}

// QualityCapabilityStatus reports the relationship between a reviewed
// capability and its local prerequisite tool. It does not execute a quality
// run and it does not infer a target that the caller has not selected.
type QualityCapabilityStatus struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	State              QualityCapabilityState    `json:"state"`
	DiscoveryState     QualityToolState          `json:"discoveryState"`
	ExecutionReadiness QualityExecutionReadiness `json:"executionReadiness"`
	ToolID             string                    `json:"toolId"`
	RunnerID           string                    `json:"runnerId,omitempty"`
	CheckedAt          time.Time                 `json:"checkedAt"`
	Reason             string                    `json:"reason"`
	ReasonCode         string                    `json:"reasonCode"`
	ReasonCodes        []string                  `json:"reasonCodes"`
	InstallGuidance    string                    `json:"installGuidance"`
}

// QualityToolsReadModel is the live local capability view. It is intentionally
// not persisted: a PATH change or a newly configured test target must be
// reflected by the next read without presenting stale evidence as current.
type QualityToolsReadModel struct {
	CheckedAt    time.Time                 `json:"checkedAt"`
	Tools        []QualityToolStatus       `json:"tools"`
	Capabilities []QualityCapabilityStatus `json:"capabilities"`
}

type qualityToolDefinition struct {
	ID              string
	Name            string
	InstallGuidance string
}

var qualityToolDefinitions = []qualityToolDefinition{
	{
		ID:              QualityRunnerGoToolID,
		Name:            "Go",
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              QualityRunnerMutationToolID,
		Name:            "go-mutesting",
		InstallGuidance: "검토된 go-mutesting.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              QualityToolGovulncheckID,
		Name:            "govulncheck",
		InstallGuidance: "govulncheck.exe를 별도로 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              QualityToolStaticcheckID,
		Name:            "Staticcheck",
		InstallGuidance: "Staticcheck를 설치하고 staticcheck.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              QualityToolGosecID,
		Name:            "gosec",
		InstallGuidance: "gosec.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              QualityToolGitleaksID,
		Name:            "gitleaks",
		InstallGuidance: "gitleaks.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
}

type qualityCapabilityDefinition struct {
	ID              string
	Name            string
	ToolID          string
	TechniqueID     string
	NeedsTarget     bool
	InstallGuidance string
}

var qualityCapabilityDefinitions = []qualityCapabilityDefinition{
	{
		ID:              "go_test",
		Name:            "Go test",
		ToolID:          QualityRunnerGoToolID,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "go_vet",
		Name:            "Go vet",
		ToolID:          QualityRunnerGoToolID,
		TechniqueID:     domain.QualityTechniqueStaticSecurity,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "go_coverage",
		Name:            "Go coverage",
		ToolID:          QualityRunnerGoToolID,
		TechniqueID:     domain.QualityTechniqueGoTestCoverage,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "mutation",
		Name:            "Mutation testing",
		ToolID:          QualityRunnerMutationToolID,
		TechniqueID:     domain.QualityTechniqueMutation,
		InstallGuidance: "검토된 go-mutesting.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              "property",
		Name:            "Property testing",
		ToolID:          QualityRunnerGoToolID,
		TechniqueID:     domain.QualityTechniqueProperty,
		NeedsTarget:     true,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "fuzz",
		Name:            "Fuzz testing",
		ToolID:          QualityRunnerGoToolID,
		TechniqueID:     domain.QualityTechniqueFuzz,
		NeedsTarget:     true,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "targeted_e2e",
		Name:            "Targeted E2E",
		ToolID:          QualityRunnerGoToolID,
		TechniqueID:     domain.QualityTechniqueTargetedE2E,
		NeedsTarget:     true,
		InstallGuidance: "Go를 설치하고 go.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "govulncheck",
		Name:            "govulncheck",
		ToolID:          QualityToolGovulncheckID,
		InstallGuidance: "govulncheck.exe를 별도로 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              "staticcheck",
		Name:            "Staticcheck",
		ToolID:          QualityToolStaticcheckID,
		InstallGuidance: "Staticcheck를 설치하고 staticcheck.exe가 PATH에 있도록 설정한 뒤 다시 확인하세요.",
	},
	{
		ID:              "gosec",
		Name:            "gosec",
		ToolID:          QualityToolGosecID,
		InstallGuidance: "gosec.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
	{
		ID:              "gitleaks",
		Name:            "gitleaks",
		ToolID:          QualityToolGitleaksID,
		InstallGuidance: "gitleaks.exe를 설치하고 PATH에 추가한 뒤 다시 확인하세요.",
	},
}

// QualityToolInspector owns only fixed, reviewed tool definitions. The
// injected seams are package-private so production callers cannot turn this
// into a generic command or executable registry.
type QualityToolInspector struct {
	lookPath qualityRunnerLookPath
	now      func() time.Time
}

func NewQualityToolInspector() QualityToolInspector {
	return QualityToolInspector{
		lookPath: exec.LookPath,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Snapshot performs only PATH resolution. It never runs a discovered
// candidate, checks a version, or installs or changes a tool.
func (i QualityToolInspector) Snapshot(ctx context.Context) (QualityToolsReadModel, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return QualityToolsReadModel{}, err
	}
	lookPath := i.dependencies()
	checkedAt := time.Now().UTC()
	if i.now != nil {
		checkedAt = i.now().UTC()
	}
	tools := make([]QualityToolStatus, 0, len(qualityToolDefinitions))
	byID := make(map[string]QualityToolStatus, len(qualityToolDefinitions))
	for _, definition := range qualityToolDefinitions {
		status := inspectQualityTool(definition, checkedAt, lookPath)
		tools = append(tools, status)
		byID[status.ID] = status
	}

	capabilities := make([]QualityCapabilityStatus, 0, len(qualityCapabilityDefinitions))
	for _, definition := range qualityCapabilityDefinitions {
		capabilities = append(capabilities, qualityCapabilityStatus(definition, byID, checkedAt))
	}
	return QualityToolsReadModel{CheckedAt: checkedAt, Tools: tools, Capabilities: capabilities}, nil
}

func (i QualityToolInspector) dependencies() qualityRunnerLookPath {
	lookPath := i.lookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return lookPath
}

func inspectQualityTool(
	definition qualityToolDefinition,
	checkedAt time.Time,
	lookPath qualityRunnerLookPath,
) QualityToolStatus {
	status := QualityToolStatus{
		ID:                 definition.ID,
		Name:               definition.Name,
		State:              QualityToolStateMissing,
		DiscoveryState:     QualityToolStateMissing,
		ExecutionReadiness: QualityExecutionReadinessNotEvaluated,
		CheckedAt:          checkedAt,
		Reason:             "PATH에서 " + definition.ID + " 후보를 찾지 못했습니다.",
		ReasonCode:         QualityToolReasonMissing,
		ReasonCodes:        qualityReasonCodes(QualityToolReasonMissing),
		InstallGuidance:    definition.InstallGuidance,
	}
	path, err := lookPath(definition.ID)
	if err != nil || strings.TrimSpace(path) == "" {
		return status
	}
	status.State = QualityToolStatePresentUnverified
	status.DiscoveryState = QualityToolStatePresentUnverified
	status.Path = path
	status.Reason = "PATH에서 " + definition.ID + " 후보를 찾았지만 실행하지 않아 실제 실행 파일 여부와 버전을 확인하지 않았습니다."
	status.ReasonCode = QualityToolReasonPresentUnverified
	status.ReasonCodes = qualityReasonCodes(QualityToolReasonPresentUnverified)
	return status
}

func qualityCapabilityStatus(
	definition qualityCapabilityDefinition,
	tools map[string]QualityToolStatus,
	checkedAt time.Time,
) QualityCapabilityStatus {
	toolID := definition.ToolID
	runnerID := ""
	runner, registered := qualityRunnerRegistry[definition.TechniqueID]
	if registered {
		toolID = runner.definition.ToolID
		runnerID = runner.definition.RunnerID
	}
	status := QualityCapabilityStatus{
		ID:                 definition.ID,
		Name:               definition.Name,
		ToolID:             toolID,
		RunnerID:           runnerID,
		ExecutionReadiness: QualityExecutionReadinessNotEvaluated,
		CheckedAt:          checkedAt,
		InstallGuidance:    definition.InstallGuidance,
	}
	tool, ok := tools[toolID]
	if !ok {
		status.State = QualityCapabilityStateMissing
		status.DiscoveryState = QualityToolStateMissing
		status.Reason = "필수 도구 상태를 확인하지 못했습니다."
		status.ReasonCode = QualityToolReasonMissing
		status.ReasonCodes = qualityReasonCodes(QualityToolReasonMissing)
		return status
	}
	status.DiscoveryState = tool.DiscoveryState
	switch tool.State {
	case QualityToolStateMissing:
		status.State = QualityCapabilityStateMissing
		status.Reason = definition.Name + "에 필요한 " + tool.Name + "이(가) 설치되어 있지 않습니다."
		status.ReasonCode = QualityToolReasonMissing
		status.ReasonCodes = qualityReasonCodes(QualityToolReasonMissing)
	case QualityToolStateUntrusted:
		status.State = QualityCapabilityStateUntrusted
		status.Reason = tool.Name + " 후보를 신뢰할 수 없어 실행하지 않습니다."
		status.ReasonCode = QualityToolReasonUntrusted
		status.ReasonCodes = qualityReasonCodes(QualityToolReasonUntrusted)
		status.InstallGuidance = tool.InstallGuidance
	case QualityToolStatePresentUnverified:
		if !registered {
			status.State = QualityCapabilityStateInstalledNotRegistered
			status.Reason = "도구 후보는 있지만 이 기능에 연결된 검토된 Quality Runner가 등록되지 않았습니다."
			status.ReasonCode = QualityRunnerReasonUnregistered
			status.ReasonCodes = qualityReasonCodes(QualityRunnerReasonUnregistered, QualityToolReasonPresentUnverified)
			status.InstallGuidance = "검토된 runner를 등록한 뒤에만 이 기능을 실행하세요."
		} else if definition.NeedsTarget {
			status.State = QualityCapabilityStateNeedsTarget
			status.Reason = "도구 후보와 검토된 runner는 확인했지만 프로젝트 Worktree와 테스트 대상은 이 조회에서 평가하지 않았습니다."
			status.ReasonCode = QualityTargetReasonNotEvaluated
			status.ReasonCodes = qualityReasonCodes(QualityTargetReasonNotEvaluated, QualityToolReasonPresentUnverified)
			status.InstallGuidance = "작업 범위에서 Worktree와 테스트 대상을 선택한 뒤 다시 확인하세요."
		} else {
			status.State = QualityCapabilityStatePresentUnverified
			status.Reason = "PATH에서 도구 후보와 검토된 runner를 찾았지만 실행하지 않아 실제 실행 가능 여부를 확인하지 않았습니다."
			status.ReasonCode = QualityToolReasonPresentUnverified
			status.ReasonCodes = qualityReasonCodes(QualityToolReasonPresentUnverified)
		}
	default:
		status.State = QualityCapabilityStateMissing
		status.Reason = "필수 도구 상태가 유효하지 않습니다."
		status.ReasonCode = QualityToolReasonMissing
		status.ReasonCodes = qualityReasonCodes(QualityToolReasonMissing)
	}
	return status
}

func qualityReasonCodes(primary string, additional ...string) []string {
	codes := make([]string, 0, 2+len(additional))
	appendCode := func(code string) {
		if code == "" {
			return
		}
		for _, existing := range codes {
			if existing == code {
				return
			}
		}
		codes = append(codes, code)
	}
	appendCode(primary)
	for _, code := range additional {
		appendCode(code)
	}
	appendCode(QualityExecutionReasonNotEvaluated)
	return codes
}
