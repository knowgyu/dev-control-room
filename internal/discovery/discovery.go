// Package discovery reads a deliberately small, fixed set of repository entry
// points. It never executes discovered commands or follows paths outside root.
package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxSourceBytes = 1 << 20

type Candidate struct {
	Name         string
	SourcePath   string
	SourceDigest string
	CommandKind  string
	Command      string
}

var workflowRun = regexp.MustCompile(`^\s*-\s*run:\s*([^#][^\r\n]*)\s*$`)
var blockScalarHeader = regexp.MustCompile(`^\s*(-\s*)?[A-Za-z0-9_-]+:\s*[|>][+-]?\s*(#.*)?$`)

// Discover returns package scripts and unambiguous one-line GitHub Actions
// runs. Multiline YAML is intentionally skipped rather than guessed.
func Discover(root string) ([]Candidate, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("selected worktree root is unavailable")
	}
	items := make([]Candidate, 0)
	if packageItems, err := packageScripts(root); err != nil {
		return nil, err
	} else {
		items = append(items, packageItems...)
	}
	workflowItems, err := workflowRuns(root)
	if err != nil {
		return nil, err
	}
	items = append(items, workflowItems...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].SourcePath == items[j].SourcePath {
			return items[i].Command < items[j].Command
		}
		return items[i].SourcePath < items[j].SourcePath
	})
	return items, nil
}

func packageScripts(root string) ([]Candidate, error) {
	data, found, err := readRootFile(root, "package.json")
	if err != nil || !found {
		return nil, err
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil // A malformed manifest is not a trustworthy discovery source.
	}
	names := make([]string, 0, len(manifest.Scripts))
	for name, script := range manifest.Scripts {
		if strings.TrimSpace(name) != "" && strings.TrimSpace(script) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	digest := digest(data)
	items := make([]Candidate, 0, len(names))
	for _, name := range names {
		items = append(items, Candidate{
			Name:         "package script " + name,
			SourcePath:   "package.json",
			SourceDigest: digest,
			CommandKind:  "package_script",
			Command:      "npm run " + name,
		})
	}
	return items, nil
}

func workflowRuns(root string) ([]Candidate, error) {
	paths := make([]string, 0)
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			return nil, err
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	items := make([]Candidate, 0)
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil || !safeRelative(relative) {
			return nil, errors.New("workflow path escapes selected worktree")
		}
		data, found, err := readRootFile(root, filepath.ToSlash(relative))
		if err != nil || !found {
			if err != nil {
				return nil, err
			}
			continue
		}
		stepsIndent := -1
		scalarIndent := -1
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSuffix(line, "\r")
			trimmed := strings.TrimSpace(line)
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if trimmed == "steps:" {
				stepsIndent = indent
				continue
			}
			if stepsIndent >= 0 && trimmed != "" && !strings.HasPrefix(trimmed, "#") && indent <= stepsIndent {
				stepsIndent = -1
				scalarIndent = -1
			}
			if stepsIndent < 0 || indent <= stepsIndent {
				continue
			}
			if scalarIndent >= 0 {
				if trimmed == "" || indent > scalarIndent {
					continue
				}
				scalarIndent = -1
			}
			if blockScalarHeader.MatchString(line) {
				scalarIndent = indent
				continue
			}
			match := workflowRun.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}
			command := strings.TrimSpace(match[1])
			if command == "|" || command == ">" || strings.HasPrefix(command, "|") || strings.HasPrefix(command, ">") {
				scalarIndent = indent
				continue
			}
			if command == "" || strings.ContainsAny(command, "#'\"") {
				continue
			}
			items = append(items, Candidate{
				Name:         "GitHub Actions run",
				SourcePath:   filepath.ToSlash(relative),
				SourceDigest: digest(data),
				CommandKind:  "github_actions_run",
				Command:      command,
			})
		}
	}
	return items, nil
}

func readRootFile(root, relative string) ([]byte, bool, error) {
	if !safeRelative(relative) {
		return nil, false, errors.New("source path escapes selected worktree")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(relative)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !withinRoot(root, path) {
		return nil, false, errors.New("source path escapes selected worktree")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, errors.New("discovery source is not a regular file")
	}
	data, err := readLimited(path)
	return data, err == nil, err
}

func readLimited(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSourceBytes {
		return nil, errors.New("discovery source exceeds 1 MiB")
	}
	return data, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func safeRelative(path string) bool {
	path = filepath.ToSlash(path)
	return path != "" && !strings.HasPrefix(path, "/") && path != ".." && !strings.HasPrefix(path, "../") && !strings.Contains(path, "/../")
}

func withinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && safeRelative(filepath.ToSlash(relative))
}
