package assurance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

// QualityRunnerSelectionState is deliberately separate from a Quality Run
// state. Selection only describes whether a reviewed runner can be used.
type QualityRunnerSelectionState string

const (
	QualityRunnerSelectionAvailable   QualityRunnerSelectionState = "available"
	QualityRunnerSelectionUnavailable QualityRunnerSelectionState = "unavailable"
	QualityRunnerSelectionBlocked     QualityRunnerSelectionState = "blocked"
)

const (
	QualityRunnerGoVetID                = "quality.go.vet"
	QualityRunnerGoVetConfigID          = "quality.go.vet.v1"
	QualityRunnerGoMutationID           = "quality.go.mutation"
	QualityRunnerGoMutationConfigID     = "quality.go.mutation.v1"
	QualityRunnerGoPropertyID           = "quality.go.property"
	QualityRunnerGoPropertyConfigID     = "quality.go.property.v1"
	QualityRunnerGoFuzzID               = "quality.go.fuzz"
	QualityRunnerGoFuzzConfigID         = "quality.go.fuzz.v1"
	QualityRunnerGoE2EID                = "quality.go.e2e"
	QualityRunnerGoE2EConfigID          = "quality.go.e2e.v1"
	QualityRunnerGoTestCoverageID       = "quality.go.test_coverage"
	QualityRunnerGoTestCoverageConfigID = "quality.go.test_coverage.v1"
	QualityRunnerBlockedRunnerID        = "quality.blocked"
	QualityRunnerDefinitionVersion      = "quality-runner.v1"
	QualityRunnerGoToolID               = "go.exe"
	QualityRunnerMutationToolID         = "go-mutesting.exe"
	QualityRunnerGoVetTimeout           = 2 * time.Minute
	QualityRunnerGoMutationTimeout      = 5 * time.Minute
	QualityRunnerGoPropertyTimeout      = 2 * time.Minute
	QualityRunnerGoFuzzTimeout          = 30 * time.Second
	QualityRunnerGoTargetedE2ETimeout   = 2 * time.Minute
	QualityRunnerGoTestCoverageTimeout  = 2 * time.Minute
	QualityRunnerReasonNoAdapter        = "runner.adapter_unreviewed"
	QualityRunnerReasonGoUnavailable    = "tool.go_unavailable"
	QualityRunnerReasonGoUntrusted      = "tool.go_untrusted"
	QualityRunnerReasonGoModMissing     = "worktree.go_mod_missing"
	QualityRunnerReasonGoModInvalid     = "worktree.go_mod_invalid"
	QualityRunnerReasonCoverageMissing  = "runner.coverage_path_missing"
	QualityRunnerReasonCoverageUnsafe   = "runner.coverage_path_unsafe"
	QualityRunnerReasonMutationMissing  = "tool.go_mutesting_unavailable"
	QualityRunnerReasonMutationUnsafe   = "tool.go_mutesting_untrusted"
	QualityRunnerReasonTargetMissing    = "target.go_test_missing"
	QualityRunnerReasonTargetAmbiguous  = "target.go_test_ambiguous"
	QualityRunnerReasonSourceInvalid    = "worktree.go_source_invalid"
	QualityRunnerReasonSourceBounds     = "worktree.go_source_bounds_exceeded"
	QualityRunnerReasonSourceSymlink    = "worktree.go_source_symlink_rejected"
)

const (
	qualityRunnerMaxGoFiles       = 2048
	qualityRunnerMaxSourceBytes   = 8 << 20
	qualityRunnerMaxSourceFile    = 512 << 10
	qualityRunnerMaxTargetMatches = 64
	qualityRunnerMaxGoModBytes    = 64 << 10
	qualityRunnerFuzzTime         = "10s"
)

var ErrQualityTechniqueNotRegistered = errors.New("quality technique is not registered")

// QualityRunnerSelectionRequest is the complete caller-controlled selection
// surface. Commands, arguments, tool names, and timeouts are registry-owned.
type QualityRunnerSelectionRequest struct {
	TechniqueID  string `json:"techniqueId"`
	WorktreeRoot string `json:"worktreeRoot"`
	CoveragePath string `json:"coveragePath,omitempty"`
}

// QualityRunnerDefinition is reviewed metadata for one registry entry. It is
// not a command template and cannot be supplied to Select by callers.
type QualityRunnerDefinition struct {
	RunnerID          string        `json:"runnerId"`
	DefinitionVersion string        `json:"definitionVersion"`
	TechniqueID       string        `json:"techniqueId"`
	ToolID            string        `json:"toolId"`
	ConfigID          string        `json:"configId"`
	Timeout           time.Duration `json:"timeout"`
}

// Validate prevents a definition from being used unless it is one of the
// exact, stable definitions owned by the registry.
func (d QualityRunnerDefinition) Validate() error {
	for _, item := range []struct {
		name  string
		value string
	}{
		{"runner ID", d.RunnerID},
		{"definition version", d.DefinitionVersion},
		{"technique ID", d.TechniqueID},
		{"tool ID", d.ToolID},
		{"config ID", d.ConfigID},
	} {
		if !validStableQualityRunnerIdentifier(item.value) {
			return fmt.Errorf("quality runner %s is invalid", item.name)
		}
	}
	if d.DefinitionVersion != QualityRunnerDefinitionVersion {
		return errors.New("quality runner definition version is not reviewed")
	}
	registered, ok := qualityRunnerRegistry[d.TechniqueID]
	if !ok || d != registered.definition {
		return errors.New("quality runner definition is not registered")
	}
	if d.Timeout < 0 || d.Timeout%time.Second != 0 {
		return errors.New("quality runner timeout is invalid")
	}
	return nil
}

// QualityRunnerMetadata is stable, concise evidence metadata. Resolved argv
// remains in Command so it can be recorded without ever becoming a shell
// command string.
type QualityRunnerMetadata struct {
	RunnerID          string `json:"runnerId"`
	DefinitionVersion string `json:"definitionVersion"`
	TechniqueID       string `json:"techniqueId"`
	ToolID            string `json:"toolId"`
	ConfigID          string `json:"configId"`
	ConfigDigest      string `json:"configDigest"`
}

// QualityRunnerUnavailableReason is suitable for embedding in later
// QualityRun evidence without exposing implementation errors or command text.
type QualityRunnerUnavailableReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// QualityRunnerSelection is a pure selection result. It contains no process
// handle and selection never starts a process.
type QualityRunnerSelection struct {
	TechniqueID  string                          `json:"techniqueId"`
	State        QualityRunnerSelectionState     `json:"state"`
	Definition   QualityRunnerDefinition         `json:"definition"`
	WorktreeRoot string                          `json:"worktreeRoot"`
	Command      TypedCommand                    `json:"command"`
	Metadata     QualityRunnerMetadata           `json:"metadata"`
	Unavailable  *QualityRunnerUnavailableReason `json:"unavailable,omitempty"`
}

// Validate checks the complete selection envelope without touching the
// filesystem or starting a process.
func (s QualityRunnerSelection) Validate() error {
	if err := s.Definition.Validate(); err != nil {
		return err
	}
	if s.TechniqueID != s.Definition.TechniqueID || !validQualityTechniqueID(s.TechniqueID) {
		return errors.New("quality runner selection technique ID does not match its definition")
	}
	if s.Metadata.RunnerID != s.Definition.RunnerID || s.Metadata.DefinitionVersion != s.Definition.DefinitionVersion || s.Metadata.TechniqueID != s.TechniqueID || s.Metadata.ToolID != s.Definition.ToolID || s.Metadata.ConfigID != s.Definition.ConfigID {
		return errors.New("quality runner selection metadata does not match its definition")
	}
	if s.Metadata.ConfigDigest != qualityRunnerConfigDigest(s.Definition, s.Command) {
		return errors.New("quality runner selection config digest does not match its definition")
	}
	switch s.State {
	case QualityRunnerSelectionAvailable:
		if s.Unavailable != nil || s.Definition.Timeout <= 0 {
			return errors.New("available quality runner selection has invalid availability metadata")
		}
		return ValidateQualityRunnerCommand(s.Command)
	case QualityRunnerSelectionUnavailable, QualityRunnerSelectionBlocked:
		if !validQualityRunnerUnavailableReason(s.Unavailable) || s.Command.Executable != "" || len(s.Command.Arguments) != 0 {
			return errors.New("unavailable quality runner selection has invalid blocked command metadata")
		}
		return nil
	default:
		return errors.New("quality runner selection state is invalid")
	}
}

type qualityRunnerRegistration struct {
	definition QualityRunnerDefinition
}

// This is the only technique-to-runner registry. Keep entries additive and
// reviewed; an unknown string never falls through to a generic command.
var qualityRunnerRegistry = map[string]qualityRunnerRegistration{
	domain.QualityTechniqueStaticSecurity: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoVetID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueStaticSecurity, ToolID: QualityRunnerGoToolID,
		ConfigID: QualityRunnerGoVetConfigID, Timeout: QualityRunnerGoVetTimeout,
	}},
	domain.QualityTechniqueMutation: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoMutationID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueMutation, ToolID: QualityRunnerMutationToolID,
		ConfigID: QualityRunnerGoMutationConfigID, Timeout: QualityRunnerGoMutationTimeout,
	}},
	domain.QualityTechniqueProperty: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoPropertyID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueProperty, ToolID: QualityRunnerGoToolID,
		ConfigID: QualityRunnerGoPropertyConfigID, Timeout: QualityRunnerGoPropertyTimeout,
	}},
	domain.QualityTechniqueFuzz: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoFuzzID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueFuzz, ToolID: QualityRunnerGoToolID,
		ConfigID: QualityRunnerGoFuzzConfigID, Timeout: QualityRunnerGoFuzzTimeout,
	}},
	domain.QualityTechniqueTargetedE2E: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoE2EID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueTargetedE2E, ToolID: QualityRunnerGoToolID,
		ConfigID: QualityRunnerGoE2EConfigID, Timeout: QualityRunnerGoTargetedE2ETimeout,
	}},
	domain.QualityTechniqueGoTestCoverage: {definition: QualityRunnerDefinition{
		RunnerID: QualityRunnerGoTestCoverageID, DefinitionVersion: QualityRunnerDefinitionVersion,
		TechniqueID: domain.QualityTechniqueGoTestCoverage, ToolID: QualityRunnerGoToolID,
		ConfigID: QualityRunnerGoTestCoverageConfigID, Timeout: QualityRunnerGoTestCoverageTimeout,
	}},
}

type qualityRunnerLookPath func(string) (string, error)
type qualityRunnerStat func(string) (os.FileInfo, error)

// QualityRunnerRegistry owns the fixed registry and read-only verification
// dependencies. Its unexported seams are for package tests; callers cannot
// inject a command or alter a registered definition.
type QualityRunnerRegistry struct {
	lookPath qualityRunnerLookPath
	stat     qualityRunnerStat
	lstat    qualityRunnerStat
}

func NewQualityRunnerRegistry() QualityRunnerRegistry {
	return QualityRunnerRegistry{lookPath: exec.LookPath, stat: os.Stat, lstat: os.Lstat}
}

func (r QualityRunnerRegistry) dependencies() (qualityRunnerLookPath, qualityRunnerStat, qualityRunnerStat) {
	lookPath, stat, lstat := r.lookPath, r.stat, r.lstat
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if stat == nil {
		stat = os.Stat
	}
	if lstat == nil {
		if r.stat != nil {
			lstat = r.stat
		} else {
			lstat = os.Lstat
		}
	}
	return lookPath, stat, lstat
}

// Select resolves only a registered technique. It performs filesystem,
// executable identity, and bounded source checks, but never invokes a tool.
func (r QualityRunnerRegistry) Select(request QualityRunnerSelectionRequest) (QualityRunnerSelection, error) {
	if !validQualityTechniqueID(request.TechniqueID) {
		return QualityRunnerSelection{}, fmt.Errorf("%w: %q", ErrQualityTechniqueNotRegistered, request.TechniqueID)
	}
	registration, ok := qualityRunnerRegistry[request.TechniqueID]
	if !ok {
		return QualityRunnerSelection{}, fmt.Errorf("%w: %q", ErrQualityTechniqueNotRegistered, request.TechniqueID)
	}
	if err := registration.definition.Validate(); err != nil {
		return QualityRunnerSelection{}, fmt.Errorf("quality runner registry is invalid: %w", err)
	}
	lookPath, stat, lstat := r.dependencies()
	if err := validateQualityWorktreeRoot(request.WorktreeRoot, stat, lstat); err != nil {
		return QualityRunnerSelection{}, err
	}
	selection := newQualityRunnerSelection(request, registration.definition)

	goMod := filepath.Join(request.WorktreeRoot, "go.mod")
	goModInfo, err := lstat(goMod)
	if err != nil || goModInfo == nil || goModInfo.Mode()&os.ModeSymlink != 0 || !goModInfo.Mode().IsRegular() {
		return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonGoModMissing, Detail: "selected worktree has no regular go.mod"})
	}
	if err := validateQualityGoModule(goMod); err != nil {
		return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonGoModInvalid, Detail: "selected worktree go.mod is not a valid module declaration"})
	}

	switch request.TechniqueID {
	case domain.QualityTechniqueStaticSecurity:
		goPath, reason := resolveNativeQualityTool(lookPath, lstat, QualityRunnerGoToolID, QualityRunnerReasonGoUnavailable, QualityRunnerReasonGoUntrusted)
		if reason != nil {
			return unavailableQualityRunnerSelection(selection, reason)
		}
		selection.Command = TypedCommand{Executable: goPath, Arguments: []string{"vet", "-mod=readonly", "./..."}}
	case domain.QualityTechniqueMutation:
		mutationPath, reason := resolveNativeQualityTool(lookPath, lstat, QualityRunnerMutationToolID, QualityRunnerReasonMutationMissing, QualityRunnerReasonMutationUnsafe)
		if reason != nil {
			return unavailableQualityRunnerSelection(selection, reason)
		}
		selection.Command = TypedCommand{Executable: mutationPath, Arguments: []string{"./..."}}
	case domain.QualityTechniqueGoTestCoverage:
		if strings.TrimSpace(request.CoveragePath) == "" {
			return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonCoverageMissing, Detail: "coverage runner requires a server-owned absolute .out path"})
		}
		if !isValidAbsolutePath(request.CoveragePath) || !strings.EqualFold(filepath.Ext(request.CoveragePath), ".out") {
			return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonCoverageUnsafe, Detail: "coverage runner requires a server-owned absolute .out path"})
		}
		goPath, reason := resolveNativeQualityTool(lookPath, lstat, QualityRunnerGoToolID, QualityRunnerReasonGoUnavailable, QualityRunnerReasonGoUntrusted)
		if reason != nil {
			return unavailableQualityRunnerSelection(selection, reason)
		}
		selection.Command = qualityCoverageCommand(goPath, request.CoveragePath)
	case domain.QualityTechniqueProperty, domain.QualityTechniqueFuzz, domain.QualityTechniqueTargetedE2E:
		prefix := qualityTargetPrefix(request.TechniqueID)
		targets, reason := discoverQualityGoTargets(request.WorktreeRoot, prefix, lstat)
		if reason != nil {
			return unavailableQualityRunnerSelection(selection, reason)
		}
		if len(targets) == 0 {
			return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonTargetMissing, Detail: "no unique reviewed Go test target was found"})
		}
		if len(targets) != 1 {
			return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonTargetAmbiguous, Detail: "more than one reviewed Go test target was found"})
		}
		goPath, goReason := resolveNativeQualityTool(lookPath, lstat, QualityRunnerGoToolID, QualityRunnerReasonGoUnavailable, QualityRunnerReasonGoUntrusted)
		if goReason != nil {
			return unavailableQualityRunnerSelection(selection, goReason)
		}
		selection.Command = qualityTargetCommand(request.TechniqueID, goPath, targets[0])
	default:
		return QualityRunnerSelection{}, fmt.Errorf("%w: %q", ErrQualityTechniqueNotRegistered, request.TechniqueID)
	}

	if err := ValidateQualityRunnerCommand(selection.Command); err != nil {
		return unavailableQualityRunnerSelection(selection, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonGoUntrusted, Detail: "registered native command failed typed validation"})
	}
	selection.Metadata.ConfigDigest = qualityRunnerConfigDigest(selection.Definition, selection.Command)
	selection.State = QualityRunnerSelectionAvailable
	if err := selection.Validate(); err != nil {
		return QualityRunnerSelection{}, fmt.Errorf("quality runner selection is invalid: %w", err)
	}
	return selection, nil
}

func newQualityRunnerSelection(request QualityRunnerSelectionRequest, definition QualityRunnerDefinition) QualityRunnerSelection {
	selection := QualityRunnerSelection{
		TechniqueID: request.TechniqueID, State: QualityRunnerSelectionUnavailable,
		Definition: definition, WorktreeRoot: request.WorktreeRoot,
		Command: TypedCommand{Arguments: []string{}},
	}
	selection.Metadata = QualityRunnerMetadata{
		RunnerID: definition.RunnerID, DefinitionVersion: definition.DefinitionVersion,
		TechniqueID: definition.TechniqueID, ToolID: definition.ToolID,
		ConfigID:     definition.ConfigID,
		ConfigDigest: qualityRunnerConfigDigest(definition, selection.Command),
	}
	return selection
}

func unavailableQualityRunnerSelection(selection QualityRunnerSelection, reason *QualityRunnerUnavailableReason) (QualityRunnerSelection, error) {
	selection.State = QualityRunnerSelectionUnavailable
	selection.Unavailable = cloneQualityRunnerReason(reason)
	if err := selection.Validate(); err != nil {
		return QualityRunnerSelection{}, fmt.Errorf("quality runner selection is invalid: %w", err)
	}
	return selection, nil
}

// ValidateQualityRunnerCommand validates every command shape currently
// registered by this package. It rejects shell surfaces and arbitrary argv.
func ValidateQualityRunnerCommand(command TypedCommand) error {
	if err := command.Validate(); err != nil {
		return err
	}
	base := nativeExecutableBase(command.Executable)
	switch base {
	case QualityRunnerGoToolID:
		if !isVerifiedNativeGoPath(command.Executable) {
			return errors.New("quality runner executable must be an absolute native go.exe")
		}
		if len(command.Arguments) == 3 && command.Arguments[0] == "vet" && command.Arguments[1] == "-mod=readonly" && command.Arguments[2] == "./..." {
			return nil
		}
		if len(command.Arguments) == 6 && command.Arguments[0] == "test" && command.Arguments[1] == "-mod=readonly" && command.Arguments[2] == "-count=1" && command.Arguments[3] == "-covermode=set" && strings.HasPrefix(command.Arguments[4], "-coverprofile=") && command.Arguments[5] == "./..." {
			path := strings.TrimPrefix(command.Arguments[4], "-coverprofile=")
			if isValidAbsolutePath(path) && strings.EqualFold(filepath.Ext(path), ".out") {
				return nil
			}
		}
		if len(command.Arguments) == 6 && command.Arguments[0] == "test" && command.Arguments[1] == "-mod=readonly" && validGoPackageArgument(command.Arguments[2]) && command.Arguments[3] == "-run" && command.Arguments[5] == "-count=1" {
			if validExactGoTestRegex(command.Arguments[4], "TestProperty") || validExactGoTestRegex(command.Arguments[4], "TestE2E") {
				return nil
			}
		}
		if len(command.Arguments) == 8 && command.Arguments[0] == "test" && command.Arguments[1] == "-mod=readonly" && validGoPackageArgument(command.Arguments[2]) && command.Arguments[3] == "-run" && command.Arguments[4] == "^$" && command.Arguments[5] == "-fuzz" && validExactGoTestRegex(command.Arguments[6], "Fuzz") && command.Arguments[7] == "-fuzztime="+qualityRunnerFuzzTime {
			return nil
		}
		return errors.New("quality runner argv is not a reviewed native Go command")
	case QualityRunnerMutationToolID:
		if !isVerifiedNativeQualityExecutable(command.Executable, QualityRunnerMutationToolID) {
			return errors.New("quality runner executable must be an absolute native go-mutesting.exe")
		}
		if len(command.Arguments) == 1 && command.Arguments[0] == "./..." {
			return nil
		}
		return errors.New("quality runner argv is not the reviewed go-mutesting command")
	default:
		return errors.New("quality runner executable is not registered")
	}
}

func resolveNativeQualityTool(lookPath qualityRunnerLookPath, lstat qualityRunnerStat, toolID, missingCode, unsafeCode string) (string, *QualityRunnerUnavailableReason) {
	toolPath, err := lookPath(toolID)
	if err != nil || strings.TrimSpace(toolPath) == "" {
		return "", &QualityRunnerUnavailableReason{Code: missingCode, Detail: "reviewed native tool was not found"}
	}
	if !isVerifiedNativeQualityExecutable(toolPath, toolID) {
		return "", &QualityRunnerUnavailableReason{Code: unsafeCode, Detail: "resolved tool is not an absolute native executable"}
	}
	info, err := lstat(toolPath)
	if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", &QualityRunnerUnavailableReason{Code: unsafeCode, Detail: "resolved native tool is not a regular file"}
	}
	return toolPath, nil
}

type qualityGoTarget struct {
	Name       string
	PackageArg string
	SourcePath string
}

func qualityTargetPrefix(technique string) string {
	switch technique {
	case domain.QualityTechniqueProperty:
		return "TestProperty"
	case domain.QualityTechniqueFuzz:
		return "Fuzz"
	case domain.QualityTechniqueTargetedE2E:
		return "TestE2E"
	default:
		return ""
	}
}

func qualityTargetCommand(technique, goPath string, target qualityGoTarget) TypedCommand {
	switch technique {
	case domain.QualityTechniqueFuzz:
		return TypedCommand{Executable: goPath, Arguments: []string{"test", "-mod=readonly", target.PackageArg, "-run", "^$", "-fuzz", "^" + target.Name + "$", "-fuzztime=" + qualityRunnerFuzzTime}}
	case domain.QualityTechniqueProperty, domain.QualityTechniqueTargetedE2E:
		return TypedCommand{Executable: goPath, Arguments: []string{"test", "-mod=readonly", target.PackageArg, "-run", "^" + target.Name + "$", "-count=1"}}
	default:
		return TypedCommand{Arguments: []string{}}
	}
}

func qualityCoverageCommand(goPath, profilePath string) TypedCommand {
	return TypedCommand{Executable: goPath, Arguments: []string{"test", "-mod=readonly", "-count=1", "-covermode=set", "-coverprofile=" + profilePath, "./..."}}
}

func validateQualityGoModule(path string) error {
	data, err := readBoundedQualitySource(path, qualityRunnerMaxGoModBytes)
	if err != nil {
		return err
	}
	withoutComments, err := stripQualityGoModComments(data)
	if err != nil {
		return err
	}
	for _, rawLine := range strings.Split(withoutComments, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" || !validGoModulePath(fields[1]) {
			return errors.New("go.mod module directive is invalid")
		}
		return nil
	}
	return errors.New("go.mod has no module directive")
}

func stripQualityGoModComments(data []byte) (string, error) {
	var cleaned strings.Builder
	cleaned.Grow(len(data))

	for i := 0; i < len(data); {
		switch {
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '/':
			cleaned.WriteByte(' ')
			i += 2
			for i < len(data) && data[i] != '\n' {
				i++
			}
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '*':
			cleaned.WriteByte(' ')
			i += 2
			terminated := false
			for i < len(data) {
				if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
					i += 2
					terminated = true
					break
				}
				if data[i] == '\n' {
					cleaned.WriteByte('\n')
				}
				i++
			}
			if !terminated {
				return "", errors.New("go.mod block comment is unterminated")
			}
		default:
			cleaned.WriteByte(data[i])
			i++
		}
	}

	return cleaned.String(), nil
}

func validGoModulePath(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "\\\t\r\n") && !strings.Contains(value, "/../") && !strings.HasPrefix(value, "../") && !strings.HasSuffix(value, "/..")
}

var (
	errQualitySourceBounds  = errors.New("quality source bounds exceeded")
	errQualitySourceSymlink = errors.New("quality source symlink rejected")
)

func discoverQualityGoTargets(root, prefix string, lstat qualityRunnerStat) ([]qualityGoTarget, *QualityRunnerUnavailableReason) {
	files := make([]string, 0, 32)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return errors.New("quality source inspection failed")
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || !safeQualityRelativePath(rel) {
			return errors.New("quality source path escaped worktree")
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == ".git" || entry.Name() == "vendor") {
				return fs.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return errQualitySourceSymlink
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errQualitySourceSymlink
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		info, infoErr := lstat(path)
		if infoErr != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("quality source file is not regular")
		}
		if len(files) >= qualityRunnerMaxGoFiles {
			return errQualitySourceBounds
		}
		files = append(files, path)
		return nil
	})
	if walkErr != nil {
		switch {
		case errors.Is(walkErr, errQualitySourceBounds):
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceBounds, Detail: "Go source discovery exceeded its fixed bounds"}
		case errors.Is(walkErr, errQualitySourceSymlink):
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceSymlink, Detail: "Go source discovery rejected a symlink"}
		default:
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceInvalid, Detail: "Go source discovery could not inspect the worktree"}
		}
	}
	sort.Strings(files)
	targets := make([]qualityGoTarget, 0, 4)
	totalBytes := 0
	fset := token.NewFileSet()
	for _, path := range files {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, readErr := readBoundedQualitySource(path, qualityRunnerMaxSourceFile)
		if readErr != nil {
			if errors.Is(readErr, errQualitySourceBounds) {
				return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceBounds, Detail: "Go source discovery exceeded its fixed bounds"}
			}
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceInvalid, Detail: "Go test source could not be read"}
		}
		totalBytes += len(data)
		if totalBytes > qualityRunnerMaxSourceBytes {
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceBounds, Detail: "Go source discovery exceeded its fixed bounds"}
		}
		if isGeneratedQualityGoSource(data) {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, path, data, parser.ParseComments)
		if parseErr != nil {
			return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceInvalid, Detail: "Go test source is not valid Go"}
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Name == nil || !validGoTestFunctionName(function.Name.Name, prefix) || !validGoTestFunctionSignature(function, prefix) {
				continue
			}
			packageArg, packageErr := qualityPackageArgument(root, filepath.Dir(path))
			if packageErr != nil {
				return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceInvalid, Detail: "Go test package path is outside the worktree"}
			}
			targets = append(targets, qualityGoTarget{Name: function.Name.Name, PackageArg: packageArg, SourcePath: path})
			if len(targets) > qualityRunnerMaxTargetMatches {
				return nil, &QualityRunnerUnavailableReason{Code: QualityRunnerReasonSourceBounds, Detail: "Go target discovery exceeded its fixed bounds"}
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].PackageArg != targets[j].PackageArg {
			return targets[i].PackageArg < targets[j].PackageArg
		}
		if targets[i].Name != targets[j].Name {
			return targets[i].Name < targets[j].Name
		}
		return targets[i].SourcePath < targets[j].SourcePath
	})
	return targets, nil
}

func readBoundedQualitySource(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errQualitySourceBounds
	}
	return data, nil
}

func isGeneratedQualityGoSource(data []byte) bool {
	for _, line := range bytes.SplitN(data, []byte("\n"), 8) {
		trimmed := strings.TrimSpace(string(line))
		if strings.HasPrefix(trimmed, "// Code generated ") && strings.Contains(trimmed, " DO NOT EDIT.") {
			return true
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") {
			return false
		}
	}
	return false
}

func validGoTestFunctionName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) || len(name) <= len(prefix) || !validGoIdentifier(name) {
		return false
	}
	first := name[len(prefix)]
	return first < 'a' || first > 'z'
}

func validGoIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if index == 0 {
			if !(char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
				return false
			}
			continue
		}
		if !(char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func validGoTestFunctionSignature(function *ast.FuncDecl, prefix string) bool {
	if function.Type == nil || function.Body == nil || function.Type.Results != nil || function.Type.TypeParams != nil || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return false
	}
	if len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	parameter := function.Type.Params.List[0].Type
	star, ok := parameter.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.X == nil || selector.Sel == nil {
		return false
	}
	packageIdent, packageOK := selector.X.(*ast.Ident)
	if !packageOK || packageIdent.Name != "testing" {
		return false
	}
	want := "T"
	if prefix == "Fuzz" {
		want = "F"
	}
	return selector.Sel.Name == want
}

func qualityPackageArgument(root, directory string) (string, error) {
	relative, err := filepath.Rel(root, directory)
	if err != nil || !safeQualityRelativePath(relative) {
		return "", errors.New("package path escaped worktree")
	}
	if relative == "." {
		return "./.", nil
	}
	argument := "./" + filepath.ToSlash(relative)
	if !validGoPackageArgument(argument) {
		return "", errors.New("package path is not a safe Go package argument")
	}
	return argument, nil
}

func validateQualityWorktreeRoot(root string, stat, lstat qualityRunnerStat) error {
	if !isValidAbsolutePath(root) {
		return errors.New("selected worktree root must be an absolute path")
	}
	if qualityPathContainsSymlink(root, lstat) {
		return errors.New("selected worktree root contains a symlink")
	}
	info, err := lstat(root)
	if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("selected worktree root is not an existing directory")
	}
	if stat == nil {
		return errors.New("selected worktree root could not be verified")
	}
	return nil
}

func qualityPathContainsSymlink(path string, lstat qualityRunnerStat) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(clean, volume)
	current := volume
	if strings.HasPrefix(remainder, string(filepath.Separator)) {
		current += string(filepath.Separator)
		remainder = strings.TrimLeft(remainder, string(filepath.Separator))
	}
	if current == "" && filepath.IsAbs(clean) {
		current = string(filepath.Separator)
	}
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		if current == string(filepath.Separator) || strings.HasSuffix(current, string(filepath.Separator)) {
			current += part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := lstat(current)
		if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func isVerifiedNativeGoPath(path string) bool {
	return isVerifiedNativeQualityExecutable(path, QualityRunnerGoToolID)
}

func isVerifiedNativeQualityExecutable(path, toolID string) bool {
	if !isValidAbsolutePath(path) {
		return false
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	if windowsAbsolutePath.MatchString(path) && len(path) > 3 && strings.Contains(path[3:], ":") {
		return false
	}
	return strings.EqualFold(nativeExecutableBase(normalized), toolID)
}

func nativeExecutableBase(path string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if index := strings.LastIndexByte(normalized, '/'); index >= 0 {
		normalized = normalized[index+1:]
	}
	return strings.ToLower(normalized)
}

func isValidAbsolutePath(path string) bool {
	if path == "" || path != strings.TrimSpace(path) || strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	if !filepath.IsAbs(path) && !windowsAbsolutePath.MatchString(path) {
		return false
	}
	for _, part := range strings.Split(strings.ReplaceAll(path, "\\", "/"), "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

func safeQualityRelativePath(path string) bool {
	if path == "." || path == "" {
		return true
	}
	if filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validGoPackageArgument(value string) bool {
	if value == "./." {
		return true
	}
	if !strings.HasPrefix(value, "./") || strings.ContainsAny(value, "\\:*?[]\x00\r\n") || strings.Contains(value, "...") || strings.HasPrefix(strings.TrimPrefix(value, "./"), "-") {
		return false
	}
	return safeQualityRelativePath(strings.TrimPrefix(value, "./"))
}

func validExactGoTestRegex(value, prefix string) bool {
	if len(value) <= 2 || value[0] != '^' || value[len(value)-1] != '$' {
		return false
	}
	return validGoTestFunctionName(value[1:len(value)-1], prefix)
}

func validQualityTechniqueID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func validStableQualityRunnerIdentifier(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '.' && char != '_' && char != '-' && char != '/' {
			return false
		}
	}
	return true
}

func validQualityRunnerUnavailableReason(reason *QualityRunnerUnavailableReason) bool {
	if reason == nil || !validStableQualityRunnerIdentifier(reason.Code) || len(reason.Code) > 120 || reason.Detail == "" || len(reason.Detail) > 240 || strings.ContainsAny(reason.Detail, "\x00\r\n") {
		return false
	}
	return true
}

func qualityRunnerConfigDigest(definition QualityRunnerDefinition, command TypedCommand) string {
	payload := struct {
		RunnerID          string       `json:"runnerId"`
		DefinitionVersion string       `json:"definitionVersion"`
		TechniqueID       string       `json:"techniqueId"`
		ToolID            string       `json:"toolId"`
		ConfigID          string       `json:"configId"`
		TimeoutSeconds    int64        `json:"timeoutSeconds"`
		Command           TypedCommand `json:"command"`
	}{
		RunnerID: definition.RunnerID, DefinitionVersion: definition.DefinitionVersion,
		TechniqueID: definition.TechniqueID, ToolID: definition.ToolID,
		ConfigID: definition.ConfigID, TimeoutSeconds: int64(definition.Timeout / time.Second),
		Command: command,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func cloneQualityRunnerReason(reason *QualityRunnerUnavailableReason) *QualityRunnerUnavailableReason {
	if reason == nil {
		return nil
	}
	copy := *reason
	return &copy
}
