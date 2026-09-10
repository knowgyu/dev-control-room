package qualitysetup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrInvalidLanguages reports a language selection outside the supported set.
var ErrInvalidLanguages = errors.New("invalid languages")

const (
	maxDepth         = 3
	maxEntries       = 2000
	maxEligibleFiles = 64
	maxEvidenceBytes = 128 * 1024

	statusExisting    = "existing_configuration"
	statusSetup       = "setup_needed"
	statusUnsupported = "unsupported"
	statusAmbiguous   = "ambiguous"
)

var supportedLanguages = map[string]struct{}{
	"python":     {},
	"javascript": {},
	"typescript": {},
	"go":         {},
}

var languageOrder = []string{"python", "javascript", "typescript", "go"}

var excludedDirectories = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	".venv":        {},
	"venv":         {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"artifacts":    {},
	"testdata":     {},
	".cache":       {},
	"coverage":     {},
	".next":        {},
	".nuxt":        {},
}

var primaryEvidenceNames = map[string]string{
	"go.mod":           "manifest",
	"pyproject.toml":   "manifest",
	"requirements.txt": "manifest",
	"setup.cfg":        "manifest",
	"package.json":     "manifest",
	"tsconfig.json":    "configuration",
}

// Report is the bounded, read-only setup inspection result. Project, repository,
// worktree, and head identity is owned by the application layer.
type Report struct {
	ProjectID         string      `json:"projectId"`
	RepositoryID      string      `json:"repositoryId"`
	WorktreeID        string      `json:"worktreeId"`
	Head              string      `json:"head"`
	Digest            string      `json:"digest"`
	DetectedLanguages []string    `json:"detectedLanguages"`
	SelectedLanguages []string    `json:"selectedLanguages"`
	Components        []Component `json:"components"`
	Warnings          []string    `json:"warnings"`
	Partial           bool        `json:"partial"`
}

// Component describes one manifest/configuration root relative to the selected
// checkout.
type Component struct {
	ID             string     `json:"id"`
	Path           string     `json:"path"`
	Languages      []string   `json:"languages"`
	Frameworks     []string   `json:"frameworks"`
	PackageManager string     `json:"packageManager"`
	Evidence       []Evidence `json:"evidence"`
	Checks         []Check    `json:"checks"`
}

// Evidence identifies a recognized file observed during inspection.
type Evidence struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// Check is a display-only setup recommendation. It is never executed by this
// package, so Executable is always false.
type Check struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	Reason         string   `json:"reason"`
	CommandPreview string   `json:"commandPreview"`
	SetupPreview   []string `json:"setupPreview"`
	Executable     bool     `json:"executable"`
}

type evidenceFile struct {
	path     string
	kind     string
	data     []byte
	complete bool
}

type componentData struct {
	path             string
	evidence         []Evidence
	languages        map[string]bool
	frameworks       map[string]bool
	managers         map[string]bool
	packageManager   string
	managerAmbiguous bool
	managerUnknown   bool
	parseAmbiguous   bool
	ruffConfigured   bool
	pytestConfigured bool
	eslintConfigured bool
	vitestConfigured bool
	scripts          map[string]string
}

var lockManagers = map[string]string{
	"package-lock.json": "npm", "npm-shrinkwrap.json": "npm",
	"pnpm-lock.yaml": "pnpm", "yarn.lock": "yarn",
	"bun.lock": "bun", "bun.lockb": "bun",
	"uv.lock": "uv", "poetry.lock": "poetry",
}

var errUnsafeFile = errors.New("link, special file, or changed filesystem entry")

// Inspect discovers display-only setup evidence in a caller-validated checkout.
// Limits apply before directory allocation and file reads. Cancellation is checked
// between filesystem operations; it cannot interrupt a stalled OS filesystem call.
func Inspect(ctx context.Context, root string, languages []string) (Report, error) {
	report := emptyReport()
	requested, err := normalizeLanguages(languages)
	if err != nil {
		return report, err
	}
	if err := ctx.Err(); err != nil {
		addWarning(&report, "inspection canceled", true)
		return report, err
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return report, errors.New("cannot resolve inspection root")
	}
	if err := checkAncestors(rootPath); err != nil {
		return report, fmt.Errorf("inspect root: %w", err)
	}
	evidence, err := collectEvidence(ctx, rootPath, &report)
	report.Digest = digestEvidence(evidence)
	selection := requested
	if len(selection) == 0 {
		selection = languageOrder
	}
	report.Components, report.DetectedLanguages = makeComponents(evidence, selection, &report)
	report.SelectedLanguages = append([]string{}, requested...)
	if len(requested) == 0 {
		report.SelectedLanguages = append([]string{}, report.DetectedLanguages...)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		addWarning(&report, "inspection canceled or interrupted", true)
	}
	report.Warnings = normalizeWarnings(report.Warnings)
	return report, err
}

func emptyReport() Report {
	return Report{
		DetectedLanguages: []string{}, SelectedLanguages: []string{},
		Components: []Component{}, Warnings: []string{},
	}
}

func normalizeLanguages(languages []string) ([]string, error) {
	seen := map[string]bool{}
	for _, language := range languages {
		language = strings.ToLower(strings.TrimSpace(language))
		if _, ok := supportedLanguages[language]; !ok {
			return []string{}, ErrInvalidLanguages
		}
		seen[language] = true
	}
	return orderedSet(seen, languageOrder), nil
}

// Inspect does not accept a root reached through a link, including an ancestor.
// This is a Go 1.23 portable revalidation, not an atomic filesystem sandbox.
func checkAncestors(path string) error {
	for {
		info, err := os.Lstat(path)
		if err != nil {
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				return pathErr.Err
			}
			return errors.New("cannot inspect directory")
		}
		if !info.IsDir() || info.Mode()&os.ModeType != os.ModeDir {
			return errUnsafeFile
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func openObserved(path string) (*os.File, error) {
	if err := checkAncestors(filepath.Dir(path)); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, errUnsafeFile
	}
	kind := before.Mode() & os.ModeType
	if kind != 0 && kind != os.ModeDir {
		return nil, errUnsafeFile
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errUnsafeFile
	}
	opened, statErr := file.Stat()
	after, lstatErr := os.Lstat(path)
	valid := statErr == nil && lstatErr == nil
	if valid {
		valid = os.SameFile(before, opened) && os.SameFile(opened, after) &&
			after.Mode()&os.ModeType == kind
	}
	if !valid {
		return nil, errors.Join(errUnsafeFile, file.Close())
	}
	return file, nil
}

func collectEvidence(ctx context.Context, root string, report *Report) ([]evidenceFile, error) {
	evidence := []evidenceFile{}
	remaining := maxEntries
	files := 0
	var visit func(string, int) error
	visit = func(relative string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if remaining == 0 {
			addWarning(report, "directory entry limit reached (2000)", true)
			return nil
		}
		directory, err := openObserved(filepath.Join(root, relative))
		if err != nil {
			addWarning(report, "unreadable or unsafe directory: "+relative, true)
			return nil
		}
		entries, readErr := readDirectory(ctx, directory, &remaining)
		closeErr := directory.Close()
		if readErr != nil || closeErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			addWarning(report, "directory listing incomplete: "+relative, true)
		}
		if remaining == 0 {
			addWarning(report, "directory entry limit reached (2000)", true)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := filepath.ToSlash(filepath.Join(relative, entry.Name()))
			if entry.IsDir() {
				if _, skip := excludedDirectories[strings.ToLower(entry.Name())]; skip {
					addWarning(report, "excluded child directories were not inspected", false)
					continue
				}
				if depth == maxDepth {
					addWarning(report, "maximum inspection depth reached (3)", true)
					continue
				}
				if err := visit(path, depth+1); err != nil {
					return err
				}
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				addWarning(report, "skipped symlink: "+path, true)
				continue
			}
			if entry.Type()&os.ModeType != 0 {
				addWarning(report, "skipped special file: "+path, true)
				continue
			}
			kind, recognized := evidenceKind(strings.ToLower(entry.Name()))
			if !recognized {
				continue
			}
			if files == maxEligibleFiles {
				addWarning(report, "eligible file limit reached (64)", true)
				continue
			}
			files++
			data, complete, readErr := readEvidence(ctx, filepath.Join(root, path))
			evidence = append(evidence, evidenceFile{path: path, kind: kind, data: data, complete: complete})
			if readErr != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				addWarning(report, "unreadable or unsafe evidence: "+path, true)
			}
			if readErr == nil && !complete {
				addWarning(report, "evidence exceeds 128 KiB or changed while reading: "+path, true)
			}
		}
		return nil
	}
	err := visit(".", 0)
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].path < evidence[j].path })
	return evidence, err
}

// ReadDir(-1) and WalkDir allocate an entire directory before enforcing a limit.
func readDirectory(ctx context.Context, file *os.File, remaining *int) ([]os.DirEntry, error) {
	entries := []os.DirEntry{}
	for *remaining > 0 {
		if err := ctx.Err(); err != nil {
			return entries, err
		}
		batch, err := file.ReadDir(min(128, *remaining))
		*remaining -= len(batch)
		entries = append(entries, batch...)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
	}
	return entries, nil
}

func readEvidence(ctx context.Context, path string) (data []byte, complete bool, err error) {
	if err := ctx.Err(); err != nil {
		return []byte{}, false, err
	}
	file, err := openObserved(path)
	if err != nil {
		return []byte{}, false, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return []byte{}, false, errUnsafeFile
	}
	data = []byte{}
	buffer := make([]byte, 8192)
	for len(data) < maxEvidenceBytes {
		if err := ctx.Err(); err != nil {
			return data, false, err
		}
		n, readErr := file.Read(buffer[:min(len(buffer), maxEvidenceBytes-len(data))])
		data = append(data, buffer[:n]...)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return data, false, readErr
		}
		if n == 0 {
			return data, false, io.ErrNoProgress
		}
	}
	after, err := file.Stat()
	if err != nil {
		return data, false, errUnsafeFile
	}
	complete = int64(len(data)) == before.Size() && before.Size() == after.Size() &&
		before.ModTime() == after.ModTime()
	return data, complete, ctx.Err()
}

func evidenceKind(name string) (string, bool) {
	if kind, ok := primaryEvidenceNames[name]; ok {
		return kind, true
	}
	if _, ok := lockManagers[name]; ok {
		return "lockfile", true
	}
	if name == "ruff.toml" || name == ".ruff.toml" || name == "pytest.ini" {
		return "configuration", true
	}
	if isESLintConfig(name) || isVitestConfig(name) {
		return "configuration", true
	}
	return "", false
}

func isESLintConfig(name string) bool {
	switch name {
	case ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.cjs", ".eslintrc.yaml", ".eslintrc.yml",
		"eslint.config.js", "eslint.config.cjs", "eslint.config.mjs",
		"eslint.config.ts", "eslint.config.mts", "eslint.config.cts":
		return true
	default:
		return false
	}
}

func isVitestConfig(name string) bool {
	switch name {
	case "vitest.config.js", "vitest.config.cjs", "vitest.config.mjs",
		"vitest.config.ts", "vitest.config.mts", "vitest.config.cts":
		return true
	default:
		return false
	}
}

func makeComponents(evidence []evidenceFile, selected []string, report *Report) ([]Component, []string) {
	components := map[string]*componentData{}
	for _, file := range evidence {
		path := filepath.ToSlash(filepath.Dir(file.path))
		component := components[path]
		if component == nil {
			component = &componentData{
				path: path, evidence: []Evidence{}, languages: map[string]bool{},
				frameworks: map[string]bool{}, managers: map[string]bool{}, scripts: map[string]string{},
			}
			components[path] = component
		}
		component.evidence = append(component.evidence, Evidence{Path: file.path, Kind: file.kind})
		analyzeFile(component, file, report)
	}
	paths := make([]string, 0, len(components))
	for path := range components {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	detected := map[string]bool{}
	result := make([]Component, 0, len(paths))
	for _, path := range paths {
		component := components[path]
		managers := sortedSet(component.managers)
		switch len(managers) {
		case 0:
		case 1:
			component.packageManager = managers[0]
		default:
			component.managerAmbiguous = true
			addWarning(report, "conflicting package manager evidence: "+path, false)
		}
		for language := range component.languages {
			detected[language] = true
		}
		identity := sha256.Sum256([]byte(path))
		result = append(result, Component{
			ID: "component-" + hex.EncodeToString(identity[:]), Path: path,
			Languages:  orderedSet(component.languages, languageOrder),
			Frameworks: sortedSet(component.frameworks), PackageManager: component.packageManager,
			Evidence: component.evidence, Checks: makeChecks(component, selected),
		})
	}
	return result, orderedSet(detected, languageOrder)
}

func analyzeFile(component *componentData, file evidenceFile, report *Report) {
	name := strings.ToLower(filepath.Base(file.path))
	manager, lockfile := lockManagers[name]
	if lockfile {
		component.managers[manager] = true
		language := "javascript"
		if manager == "uv" || manager == "poetry" {
			language = "python"
		}
		component.languages[language] = true
	}
	switch name {
	case "go.mod":
		component.languages["go"] = true
		component.managers["go"] = true
	case "pyproject.toml", "requirements.txt", "setup.cfg", "ruff.toml", ".ruff.toml", "pytest.ini":
		component.languages["python"] = true
	case "package.json":
		component.languages["javascript"] = true
	case "tsconfig.json":
		component.languages["typescript"] = true
	default:
		if isESLintConfig(name) || isVitestConfig(name) {
			component.languages["javascript"] = true
			if strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".mts") || strings.HasSuffix(name, ".cts") {
				component.languages["typescript"] = true
			}
		}
	}
	if !file.complete {
		component.parseAmbiguous = true
		return
	}
	switch {
	case name == "package.json":
		analyzePackageJSON(component, file, report)
	case name == "pyproject.toml":
		doc := parseTOMLSubset(file.data)
		if doc.limited {
			component.parseAmbiguous = true
			addWarning(report, "unsupported or malformed TOML; declaration detection limited: "+file.path, true)
			return
		}
		for section := range doc.sections {
			if section == "tool.ruff" || strings.HasPrefix(section, "tool.ruff.") {
				component.ruffConfigured = true
			}
			if section == "tool.pytest.ini_options" {
				component.pytestConfigured = true
			}
			for _, manager := range []string{"uv", "poetry"} {
				if section == "tool."+manager || strings.HasPrefix(section, "tool."+manager+".") {
					component.managers[manager] = true
				}
			}
		}
		for key, value := range doc.values {
			array := key == "project.dependencies" || strings.HasPrefix(key, "project.optional-dependencies.") ||
				strings.HasPrefix(key, "dependency-groups.")
			if array {
				specs, ok := stringArray(value)
				if !ok {
					component.parseAmbiguous = true
					addWarning(report, "unsupported dependency declaration: "+file.path, true)
					continue
				}
				for _, spec := range specs {
					if dependencyName(spec) == "fastapi" {
						component.frameworks["FastAPI"] = true
					}
				}
			}
			poetryDependency := strings.HasPrefix(key, "tool.poetry.dependencies.") ||
				strings.HasPrefix(key, "tool.poetry.group.")
			if poetryDependency && strings.HasSuffix(key, ".dependencies.fastapi") {
				if _, ok := simpleString(value); ok {
					component.frameworks["FastAPI"] = true
				}
			}
			if key == "project.dynamic" {
				component.parseAmbiguous = true
				addWarning(report, "dynamic project metadata is not resolved: "+file.path, true)
			}
		}
	case name == "requirements.txt":
		for _, raw := range strings.Split(string(file.data), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			name := dependencyName(line)
			if name == "" {
				component.parseAmbiguous = true
				addWarning(report, "unsupported requirements syntax; references are not followed: "+file.path, true)
			}
			if name == "fastapi" {
				component.frameworks["FastAPI"] = true
			}
		}
	case name == "setup.cfg":
		analyzeSetupCFG(component, file, report)
	case name == "tsconfig.json":
		// TypeScript accepts JSONC. Presence identifies TypeScript; resolving or
		// validating compiler options and extends references is outside M1.
	case name == "ruff.toml" || name == ".ruff.toml":
		if parseTOMLSubset(file.data).limited {
			component.parseAmbiguous = true
			addWarning(report, "unsupported or malformed TOML: "+file.path, true)
		} else {
			component.ruffConfigured = true
		}
	case name == "pytest.ini":
		component.pytestConfigured = true
	case isESLintConfig(name):
		if strings.HasSuffix(name, ".json") && !jsonObject(file.data) {
			component.parseAmbiguous = true
			addWarning(report, "malformed JSON: "+file.path, true)
		} else {
			component.eslintConfigured = true
		}
	case isVitestConfig(name):
		component.vitestConfigured = true
	}
}

// JSON configuration is decoded natively. Executable configuration is only
// identified by its allowlisted filename, never loaded or evaluated.
func jsonObject(data []byte) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(data, &value) == nil && value != nil
}

func analyzePackageJSON(component *componentData, file evidenceFile, report *Report) {
	var pkg struct {
		PackageManager  string            `json:"packageManager"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
		ESLintConfig    json.RawMessage   `json:"eslintConfig"`
	}
	if !jsonObject(file.data) || json.Unmarshal(file.data, &pkg) != nil {
		component.parseAmbiguous = true
		addWarning(report, "malformed JSON or unsupported package metadata: "+file.path, true)
		return
	}
	if pkg.PackageManager != "" {
		manager := strings.SplitN(pkg.PackageManager, "@", 2)[0]
		switch manager {
		case "npm", "pnpm", "yarn", "bun":
			component.managers[manager] = true
		default:
			component.managerUnknown = true
			addWarning(report, "unsupported declared package manager: "+file.path, false)
		}
	}
	for _, dependencies := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
		if dependencies["vue"] != "" {
			component.frameworks["Vue"] = true
		}
		if dependencies["typescript"] != "" {
			component.languages["typescript"] = true
		}
	}
	if len(pkg.ESLintConfig) > 0 && string(pkg.ESLintConfig) != "null" {
		if jsonObject(pkg.ESLintConfig) {
			component.eslintConfigured = true
		} else {
			component.parseAmbiguous = true
			addWarning(report, "unsupported eslintConfig metadata: "+file.path, true)
		}
	}
	// Only fixed script names are exposed, and only an exact leading tool token
	// identifies a tool. Neither raw script text nor environment values leave here.
	for _, name := range []string{"lint", "lint:check", "test:unit", "test", "test:ci"} {
		command := strings.Fields(pkg.Scripts[name])
		if len(command) == 0 {
			continue
		}
		switch command[0] {
		case "eslint", "vitest":
			if _, exists := component.scripts[command[0]]; !exists {
				component.scripts[command[0]] = name
			}
		default:
			component.scripts["script:"+name] = name
		}
	}
}

type tomlSubset struct {
	sections map[string]bool
	values   map[string]string
	limited  bool
}

var bareTOMLKey = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)
var simpleTOMLNumber = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

// This deliberately recognizes a subset, not a full TOML parser. Multiline
// strings, quoted/dotted-key ambiguity, inline tables and non-string arrays
// produce a warning and no inferred declarations from this file.
func parseTOMLSubset(data []byte) tomlSubset {
	doc := tomlSubset{sections: map[string]bool{}, values: map[string]string{}}
	section := ""
	lines := strings.Split(string(data), "\n")
	for index := 0; index < len(lines); index++ {
		line, ok := cleanTOMLLine(lines[index])
		if !ok {
			doc.limited = true
			return doc
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				doc.limited = true
				return doc
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if !bareTOMLKey.MatchString(section) || doc.sections[section] {
				doc.limited = true
				return doc
			}
			doc.sections[section] = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !bareTOMLKey.MatchString(key) || strings.Contains(key, ".") {
			doc.limited = true
			return doc
		}
		if strings.HasPrefix(value, "[") {
			var array strings.Builder
			array.WriteString(value)
			for !strings.HasSuffix(value, "]") && index+1 < len(lines) {
				index++
				next, ok := cleanTOMLLine(lines[index])
				if !ok {
					doc.limited = true
					return doc
				}
				array.WriteByte('\n')
				array.WriteString(next)
				value = next
			}
			value = array.String()
		}
		_, isString := simpleString(value)
		_, isArray := stringArray(value)
		scalar := value == "true" || value == "false" || simpleTOMLNumber.MatchString(value)
		if !isString && !isArray && !scalar {
			doc.limited = true
			return doc
		}
		key = section + "." + key
		if _, duplicate := doc.values[key]; duplicate {
			doc.limited = true
			return doc
		}
		doc.values[key] = value
	}
	return doc
}

func cleanTOMLLine(line string) (string, bool) {
	var quote byte
	escaped := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		if quote == 0 && char == '#' {
			line = line[:index]
			break
		}
		if quote == '"' && escaped {
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == '"' || char == '\'' {
			if quote == 0 {
				if strings.HasPrefix(line[index:], strings.Repeat(string(char), 3)) {
					return "", false
				}
				quote = char
			} else if quote == char {
				quote = 0
			}
		}
	}
	return strings.TrimSpace(line), quote == 0
}

func simpleString(value string) (string, bool) {
	if len(value) < 2 || value[len(value)-1] != value[0] {
		return "", false
	}
	quote := value[0]
	if quote != '\'' && quote != '"' {
		return "", false
	}
	body := value[1 : len(value)-1]
	// Escapes are intentionally unsupported, rather than decoded with Go's
	// subtly different string-literal grammar.
	if strings.ContainsAny(body, "\\\n\r"+string(quote)) {
		return "", false
	}
	return body, true
}

func stringArray(value string) ([]string, bool) {
	items := []string{}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return items, false
	}
	value = strings.TrimSpace(value[1 : len(value)-1])
	for value != "" {
		quote := value[0]
		if quote != '"' && quote != '\'' {
			return []string{}, false
		}
		end := strings.IndexByte(value[1:], quote)
		if end < 0 {
			return []string{}, false
		}
		end += 2
		item, ok := simpleString(value[:end])
		if !ok {
			return []string{}, false
		}
		items = append(items, item)
		value = strings.TrimSpace(value[end:])
		if value == "" {
			break
		}
		if value[0] != ',' {
			return []string{}, false
		}
		value = strings.TrimSpace(value[1:])
	}
	return items, true
}

// Only direct named requirements count. Options, includes, URLs, environment
// interpolation and arbitrary quoted text are neither followed nor searched.
func dependencyName(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec[0] == '-' || strings.Contains(spec, "://") || strings.Contains(spec, "\\") {
		return ""
	}
	end := 0
	for end < len(spec) {
		c := spec[end]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return ""
	}
	rest := strings.TrimSpace(spec[end:])
	if rest != "" && !strings.ContainsRune("[<>=!~;#", rune(rest[0])) {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(spec[:end], "_", "-"))
}

func analyzeSetupCFG(component *componentData, file evidenceFile, report *Report) {
	section := ""
	installRequires := false
	limited := false
	fastAPI := false
	pytest := false
	for _, raw := range strings.Split(string(file.data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			indented := len(raw) > len(strings.TrimLeft(raw, " \t"))
			if indented || !strings.HasSuffix(line, "]") {
				limited = true
				break
			}
			section = line[1 : len(line)-1]
			installRequires = false
			pytest = pytest || section == "tool:pytest"
			continue
		}
		indented := len(raw) > len(strings.TrimLeft(raw, " \t"))
		if installRequires && indented {
			name := dependencyName(line)
			limited = limited || name == ""
			fastAPI = fastAPI || name == "fastapi"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			key, value, ok = strings.Cut(line, ":")
		}
		if !ok {
			limited = true
			continue
		}
		installRequires = section == "options" && strings.TrimSpace(key) == "install_requires"
		if installRequires && strings.TrimSpace(value) != "" {
			name := dependencyName(value)
			limited = limited || name == ""
			fastAPI = fastAPI || name == "fastapi"
		}
	}
	if limited {
		component.parseAmbiguous = true
		addWarning(report, "unsupported or malformed setup.cfg; detection limited: "+file.path, true)
		return
	}
	component.pytestConfigured = component.pytestConfigured || pytest
	if fastAPI {
		component.frameworks["FastAPI"] = true
	}
}

func makeChecks(component *componentData, selected []string) []Check {
	checks := []Check{}
	if component.languages["python"] && selectedHas(selected, "python") {
		for _, tool := range []struct {
			id         string
			label      string
			configured bool
		}{
			{id: "ruff", label: "Ruff", configured: component.ruffConfigured},
			{id: "pytest", label: "pytest", configured: component.pytestConfigured},
		} {
			status, reason := checkStatus(component, tool.configured)
			command, setup := pythonPreview(component.packageManager, tool.id)
			checks = append(checks, newCheck(
				tool.id, tool.label, status, reason, command, setup,
			))
		}
	}
	javascript := component.languages["javascript"] && selectedHas(selected, "javascript")
	typescript := component.languages["typescript"] && selectedHas(selected, "typescript")
	if javascript || typescript {
		for _, tool := range []struct {
			id         string
			label      string
			configured bool
		}{
			{id: "eslint", label: "ESLint", configured: component.eslintConfigured},
			{id: "vitest", label: "Vitest", configured: component.vitestConfigured},
		} {
			configured := tool.configured || component.scripts[tool.id] != ""
			status, reason := checkStatus(component, configured)
			command := nodeCommand(component, tool.id)
			setup := nodeSetup(component.packageManager, tool.id)
			checks = append(checks, newCheck(
				tool.id, tool.label, status, reason, command, setup,
			))
		}
		for _, name := range []string{"lint", "lint:check", "test:unit", "test", "test:ci"} {
			if component.scripts["script:"+name] == "" {
				continue
			}
			status, reason := checkStatus(component, true)
			reason += "; existing script is user code and its tool is not inferred"
			checks = append(checks, newCheck(
				"script:"+name, "기존 "+name+" 스크립트", status, reason,
				nodeRunScript(component.packageManager, name), []string{},
			))
		}
	}
	if component.languages["go"] && selectedHas(selected, "go") {
		status, reason := checkStatus(component, false)
		checks = append(checks, newCheck(
			"go-test", "Go tests", status, reason+"; go.mod observed, test presence and success are not assessed",
			"go test ./...", []string{"프로젝트 Go 버전과 go.mod를 먼저 확인하세요."},
		))
	}
	return checks
}

func newCheck(id, label, status, reason, command string, setup []string) Check {
	if status == statusAmbiguous || status == statusUnsupported {
		command = ""
		setup = []string{"누락·충돌·미지원 설정을 확인한 뒤 설치 방법을 선택하세요."}
	}
	if status == statusExisting {
		setup = []string{}
	}
	return Check{
		ID: id, Label: label, Status: status, Reason: reason,
		CommandPreview: command, SetupPreview: setup, Executable: false,
	}
}

func checkStatus(component *componentData, configured bool) (string, string) {
	if component.parseAmbiguous || component.managerAmbiguous {
		return statusAmbiguous, "evidence is incomplete, unsupported or conflicting; runtime readiness is not checked"
	}
	if component.managerUnknown {
		return statusUnsupported, "declared package manager is unsupported; runtime readiness is not checked"
	}
	if configured {
		return statusExisting, "configuration or script observed; runtime readiness is not checked"
	}
	return statusSetup, "configuration not found within inspected scope; tests and runtime readiness are not assessed"
}

func pythonPreview(manager, tool string) (string, []string) {
	args := tool
	if tool == "ruff" {
		args += " check ."
	}
	switch manager {
	case "uv":
		return "uv run --offline --no-sync python -m " + args, []string{
			"이 구성 요소 폴더에서 uv 환경을 확인하세요.",
			"uv add --dev " + tool,
			"적용 전 pyproject.toml·uv.lock 변경을 검토하세요.",
		}
	case "poetry":
		return "poetry run python -m " + args, []string{
			"이 구성 요소 폴더에서 Poetry 환경을 확인하세요.",
			"poetry add --group dev " + tool,
			"적용 전 pyproject.toml·poetry.lock 변경을 검토하세요.",
		}
	default:
		return ".\\.venv\\Scripts\\python.exe -m " + args, []string{
			"이 구성 요소 폴더의 Python·가상환경을 확인하세요.",
			"가상환경이 없을 때만 생성하세요.",
			"py -m venv .venv",
			".\\.venv\\Scripts\\python.exe -m pip install " + tool,
			"기존 의존성 설정에 개발 도구를 기록하세요.",
		}
	}
}

func nodeCommand(component *componentData, tool string) string {
	if script := component.scripts[tool]; script != "" {
		return nodeRunScript(component.packageManager, script)
	}
	args := " ."
	if tool == "vitest" {
		args = " run"
	}
	// Direct previews name an installed project-local binary. No downloading
	// launcher is needed when the installed binary is present.
	return ".\\node_modules\\.bin\\" + tool + ".cmd" + args
}

func nodeRunScript(manager, script string) string {
	switch manager {
	case "npm", "pnpm", "bun":
		return manager + " run " + script
	case "yarn":
		return "yarn run " + script
	default:
		return ""
	}
}

func nodeSetup(manager, tool string) []string {
	var install string
	switch manager {
	case "npm":
		install = "npm install --save-dev " + tool
	case "pnpm":
		install = "pnpm add --save-dev " + tool
	case "yarn":
		install = "yarn add --dev " + tool
	case "bun":
		install = "bun add --dev " + tool
	default:
		return []string{
			"이 구성 요소의 패키지 관리자와 Node 환경을 확인하세요.",
			tool + " 추가 전 기존 검사·테스트 설정을 확인하세요.",
		}
	}
	return []string{
		"이 구성 요소 폴더의 Node 환경과 기존 설정을 확인하세요.",
		install,
		"package.json·잠금 파일 변경을 검토하고 " + tool + " 설정을 확인하세요.",
	}
}

func selectedHas(selected []string, language string) bool {
	for _, value := range selected {
		if value == language {
			return true
		}
	}
	return false
}

func orderedSet(values map[string]bool, order []string) []string {
	result := []string{}
	for _, value := range order {
		if values[value] {
			result = append(result, value)
		}
	}
	return result
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func digestEvidence(evidence []evidenceFile) string {
	sortedEvidence := append([]evidenceFile{}, evidence...)
	sort.Slice(sortedEvidence, func(i, j int) bool {
		return sortedEvidence[i].path < sortedEvidence[j].path
	})
	var records strings.Builder
	records.WriteString("qualitysetup-v1\n")
	for _, file := range sortedEvidence {
		content := sha256.Sum256(file.data)
		records.WriteString(strconv.Itoa(len(file.path)) + ":" + file.path)
		records.WriteString(hex.EncodeToString(content[:]))
		records.WriteString(strconv.FormatBool(file.complete))
		records.WriteByte('\n')
	}
	hash := sha256.Sum256([]byte(records.String()))
	return hex.EncodeToString(hash[:])
}

func addWarning(report *Report, warning string, partial bool) {
	report.Warnings = append(report.Warnings, warning)
	if partial {
		report.Partial = true
	}
}

func normalizeWarnings(warnings []string) []string {
	seen := make(map[string]bool, len(warnings))
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if seen[warning] {
			continue
		}
		seen[warning] = true
		result = append(result, warning)
	}
	sort.Strings(result)
	return result
}
