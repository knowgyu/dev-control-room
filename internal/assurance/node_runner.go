package assurance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/masking"
)

const (
	QualityAdapterESLintID  = "quality.node.eslint"
	QualityAdapterVitestID  = "quality.node.vitest"
	QualityParserESLintJSON = "eslint.json.v1"
	QualityParserVitestJSON = "vitest.json.v1"
)

type NodeQualityRunner struct {
	Capability func(context.Context) Windows11Capability
	Process    QualityProcessRunner
}

func NewNodeQualityRunner() NodeQualityRunner {
	return NodeQualityRunner{Capability: func(ctx context.Context) Windows11Capability { return CheckWindows11Capability(ctx) }, Process: nativeQualityProcessRunner{}}
}

func (r NodeQualityRunner) SelectESLint(request QualityAdapterRequest) (QualityAdapterCommand, error) {
	return r.selectNode(request, QualityAdapterESLintID, QualityParserESLintJSON, "eslint", "eslint.js", []string{"--format", "json", "."})
}

func (r NodeQualityRunner) SelectVitest(request QualityAdapterRequest) (QualityAdapterCommand, error) {
	return r.selectNode(request, QualityAdapterVitestID, QualityParserVitestJSON, "vitest", "vitest.mjs", []string{"run", "--reporter=json"})
}

func (r NodeQualityRunner) RunESLint(ctx context.Context, request QualityAdapterRequest) QualityAdapterResult {
	command, err := r.SelectESLint(request)
	if err != nil {
		return adapterUnavailable(QualityAdapterESLintID, QualityParserESLintJSON, request, err)
	}
	return r.runJSON(ctx, request, command, func(data []byte, masker *masking.Masker) ([]QualityFinding, *QualityTestSummary, error) {
		findings, _, err := ParseESLintJSON(data, masker)
		return findings, nil, err
	})
}

func (r NodeQualityRunner) RunVitest(ctx context.Context, request QualityAdapterRequest) QualityAdapterResult {
	command, err := r.SelectVitest(request)
	if err != nil {
		return adapterUnavailable(QualityAdapterVitestID, QualityParserVitestJSON, request, err)
	}
	result := r.runProcess(ctx, request, command)
	if result.TimedOut || result.ParserError != "" || result.Outcome == QualityOutcomeToolError || result.Outcome == QualityOutcomeInconclusive {
		return result
	}
	summary, err := ParseVitestJSON([]byte(result.rawStdout))
	if err != nil {
		result.Outcome = QualityOutcomeInconclusive
		result.ParserError = "Vitest JSON parser rejected the bounded report"
		return result
	}
	result.Tests = &summary
	if summary.Failed > 0 || result.ExitCode != 0 {
		result.Outcome = QualityOutcomeTestsFailed
	} else {
		result.Outcome = QualityOutcomeClean
	}
	return result
}

func (r NodeQualityRunner) runJSON(ctx context.Context, request QualityAdapterRequest, command QualityAdapterCommand, parse func([]byte, *masking.Masker) ([]QualityFinding, *QualityTestSummary, error)) QualityAdapterResult {
	result := r.runProcess(ctx, request, command)
	if result.TimedOut || result.ParserError != "" || result.Outcome == QualityOutcomeToolError {
		return result
	}
	findings, tests, err := parse([]byte(result.rawStdout), request.Masker)
	if err != nil {
		result.Outcome = QualityOutcomeInconclusive
		result.ParserError = "ESLint JSON parser rejected the bounded report"
		return result
	}
	result.Findings = findings
	result.Tests = tests
	if len(findings) > 0 {
		result.Outcome = QualityOutcomeFindings
	} else if result.ExitCode == 0 {
		result.Outcome = QualityOutcomeClean
	} else {
		result.Outcome = QualityOutcomeToolError
	}
	return result
}

func (r NodeQualityRunner) runProcess(ctx context.Context, request QualityAdapterRequest, command QualityAdapterCommand) QualityAdapterResult {
	result := QualityAdapterResult{AdapterID: command.AdapterID, ParserID: command.ParserID, ConfigDigest: request.ConfigDigest, ComponentRoot: command.ComponentRoot, Command: command.Command, Findings: []QualityFinding{}}
	capability := CheckWindows11Capability
	if r.Capability != nil {
		capability = r.Capability
	}
	if !capability(ctx).Available {
		result.Outcome = QualityOutcomeInconclusive
		result.ParserError = "native Windows 11 capability is unavailable"
		return result
	}
	process := r.Process
	if process == nil {
		process = nativeQualityProcessRunner{}
	}
	output, err := process.Run(ctx, command.Command, command.ComponentRoot, command.Timeout, command.MaxOutputBytes)
	result.ExitCode = output.ExitCode
	result.rawStdout = output.Stdout
	result.Evidence = maskQualityEvidence(output.Stdout, output.Stderr, request.Masker, command.MaxOutputBytes)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
			result.Outcome = QualityOutcomeInconclusive
			result.TimedOut = true
		} else {
			result.Outcome = QualityOutcomeToolError
		}
	}
	return result
}

func (r NodeQualityRunner) selectNode(request QualityAdapterRequest, adapterID, parserID, packageName, expectedScript string, arguments []string) (QualityAdapterCommand, error) {
	root, component, timeout, limit, err := validateQualityAdapterRequest(request, request.NodePath, "node.exe")
	if err != nil {
		return QualityAdapterCommand{}, err
	}
	modules := filepath.Join(component, "node_modules")
	if request.NodeModulesRoot != "" {
		modules = filepath.Clean(request.NodeModulesRoot)
	}
	if !qualityPathWithin(component, modules) || qualityPathContainsSymlink(modules, os.Lstat) {
		return QualityAdapterCommand{}, errors.New("node_modules root is outside or linked from the component")
	}
	packageRoot := filepath.Join(modules, packageName)
	manifestPath := filepath.Join(packageRoot, "package.json")
	info, err := os.Lstat(manifestPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return QualityAdapterCommand{}, errors.New("reviewed project-local Node package is unavailable")
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Bin     any    `json:"bin"`
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil || json.Unmarshal(data, &manifest) != nil || manifest.Name != packageName {
		return QualityAdapterCommand{}, errors.New("Node package identity is not trusted")
	}
	if request.ToolVersion != "" && manifest.Version != request.ToolVersion {
		return QualityAdapterCommand{}, errors.New("Node package version does not match the selected version")
	}
	script, ok := nodePackageBin(manifest.Bin, packageName)
	if !ok || filepath.Base(script) != expectedScript {
		return QualityAdapterCommand{}, errors.New("Node package entry point is not reviewed")
	}
	scriptPath := filepath.Join(packageRoot, filepath.FromSlash(script))
	info, err = os.Lstat(scriptPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return QualityAdapterCommand{}, errors.New("Node package entry point is unavailable")
	}
	return QualityAdapterCommand{AdapterID: adapterID, ParserID: parserID, Command: TypedCommand{Executable: request.NodePath, Arguments: append([]string{scriptPath}, arguments...)}, WorktreeRoot: root, ComponentRoot: component, Timeout: timeout, MaxOutputBytes: limit}, nil
}

func nodePackageBin(value any, packageName string) (string, bool) {
	var candidate string
	switch bin := value.(type) {
	case string:
		candidate = bin
	case map[string]any:
		if item, ok := bin[packageName]; ok {
			candidate, _ = item.(string)
		}
	}
	candidate = strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(candidate), "\\", "/"), "./")
	if candidate == "" || strings.HasPrefix(candidate, "/") || strings.Contains(candidate, "../") || strings.ContainsAny(candidate, "\x00\r\n") {
		return "", false
	}
	return candidate, true
}

func ParseESLintJSON(data []byte, masker *masking.Masker) ([]QualityFinding, *QualityTestSummary, error) {
	var entries []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleID   string `json:"ruleId"`
			Message  string `json:"message"`
			Severity int    `json:"severity"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) > 2000 {
		return nil, nil, errors.New("invalid ESLint JSON")
	}
	findings := []QualityFinding{}
	for _, entry := range entries {
		for _, message := range entry.Messages {
			if strings.TrimSpace(message.Message) == "" || message.Severity < 0 || message.Severity > 2 || message.Line < 0 || message.Column < 0 {
				return nil, nil, errors.New("invalid ESLint finding")
			}
			severity := "warning"
			if message.Severity == 2 {
				severity = "error"
			}
			findings = append(findings, QualityFinding{Rule: maskText(masker, message.RuleID), Message: maskText(masker, message.Message), Severity: severity, File: maskText(masker, entry.FilePath), Line: message.Line, Column: message.Column})
		}
	}
	return findings, nil, nil
}

func ParseVitestJSON(data []byte) (QualityTestSummary, error) {
	var report struct {
		Total       *int              `json:"numTotalTests"`
		Passed      *int              `json:"numPassedTests"`
		Failed      *int              `json:"numFailedTests"`
		Skipped     *int              `json:"numPendingTests"`
		TestResults []json.RawMessage `json:"testResults"`
	}
	if err := json.Unmarshal(data, &report); err != nil || report.Total == nil || report.Passed == nil || report.Failed == nil || report.Skipped == nil || len(report.TestResults) > 2000 {
		return QualityTestSummary{}, errors.New("invalid Vitest JSON")
	}
	if *report.Total < 0 || *report.Passed < 0 || *report.Failed < 0 || *report.Skipped < 0 || *report.Passed+*report.Failed+*report.Skipped > *report.Total {
		return QualityTestSummary{}, errors.New("invalid Vitest test totals")
	}
	return QualityTestSummary{Total: *report.Total, Passed: *report.Passed, Failed: *report.Failed, Skipped: *report.Skipped}, nil
}
