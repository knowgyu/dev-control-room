// Package collector contains bounded, allow-listed observation collectors.
// The only process runner in Milestone 1 is Git; there is deliberately no
// shell or arbitrary executable surface here.
package collector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout   = 8 * time.Second
	DefaultOutputMax = 1 << 20
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type CommandRunner interface {
	Run(context.Context, string, []string, string) (CommandResult, error)
}

type ProcessRunner struct {
	Timeout   time.Duration
	OutputMax int
}

func (r ProcessRunner) Run(parent context.Context, executable string, args []string, directory string) (CommandResult, error) {
	if executable != "git" {
		return CommandResult{}, errors.New("only the git executable is permitted")
	}
	if strings.TrimSpace(directory) == "" || filepath.IsAbs(directory) == false {
		return CommandResult{}, errors.New("git working directory must be an absolute registered path")
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxOutput := r.OutputMax
	if maxOutput <= 0 {
		maxOutput = DefaultOutputMax
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	var stdout, stderr limitedBuffer
	stdout.limit = maxOutput
	stderr.limit = maxOutput
	stdout.cancel = cancel
	stderr.cancel = cancel
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return CommandResult{}, fmt.Errorf("git command timed out after %s", timeout)
		}
		return CommandResult{}, ctx.Err()
	}
	if err != nil {
		return CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode(err)}, err
	}
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		if b.cancel != nil {
			b.cancel()
		}
		return 0, errors.New("git output exceeded bounded limit")
	}
	return b.Buffer.Write(p)
}

func exitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

type RemoteInfo struct {
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Host         string   `json:"host,omitempty"`
	Provider     string   `json:"provider"`
	Capabilities []string `json:"capabilities,omitempty"`
}

const (
	ProviderGitHub       = "github"
	ProviderGitLab       = "gitlab"
	ProviderBitbucket    = "bitbucket"
	ProviderUnknown      = "unknown"
	CapabilityRemoteRead = "remote_read"
)

type Worktree struct {
	ID                     string `json:"id"`
	Path                   string `json:"path"`
	AssociationFingerprint string `json:"associationFingerprint,omitempty"`
	Trust                  string `json:"trust"`
	Primary                bool   `json:"primary"`
	Head                   string `json:"head,omitempty"`
	Branch                 string `json:"branch,omitempty"`
	Bare                   bool   `json:"bare,omitempty"`
	Detached               bool   `json:"detached"`
	Dirty                  bool   `json:"dirty"`
	Untracked              bool   `json:"untracked"`
	Ahead                  int    `json:"ahead"`
	Behind                 int    `json:"behind"`
	Upstream               string `json:"upstream,omitempty"`
	Locked                 bool   `json:"locked"`
	Prunable               bool   `json:"prunable"`
}

type State struct {
	Path                        string       `json:"path"`
	TopLevel                    string       `json:"topLevel,omitempty"`
	Branch                      string       `json:"branch,omitempty"`
	Detached                    bool         `json:"detached"`
	Dirty                       bool         `json:"dirty"`
	Ahead                       int          `json:"ahead"`
	Behind                      int          `json:"behind"`
	Upstream                    string       `json:"upstream,omitempty"`
	Remotes                     []RemoteInfo `json:"remotes,omitempty"`
	Worktrees                   []Worktree   `json:"worktrees,omitempty"`
	WorktreeEnumerationComplete bool         `json:"worktreeEnumerationComplete"`
	UnsafeCleanup               bool         `json:"unsafeCleanup"`
	Error                       string       `json:"error,omitempty"`
	CollectedAt                 time.Time    `json:"collectedAt"`
}

type GitCollector struct {
	Runner CommandRunner
}

func NewGitCollector(runner CommandRunner) GitCollector {
	if runner == nil {
		runner = ProcessRunner{}
	}
	return GitCollector{Runner: runner}
}

func (c GitCollector) Collect(ctx context.Context, path string) (State, error) {
	state := State{Path: path, CollectedAt: time.Now().UTC()}
	var err error
	if _, err := c.required(ctx, path, "rev-parse", "--is-inside-work-tree"); err != nil {
		state.Error = "not a readable Git worktree"
		return state, err
	}
	state.TopLevel, err = c.required(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		state.Error = "Git worktree root could not be read"
		return state, err
	}
	if !sameDirectory(path, state.TopLevel) {
		state.Error = "registered path is not the Git worktree root"
		return state, errors.New("registered path is not the Git worktree root")
	}
	state.Branch, err = c.optional(ctx, path, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || state.Branch == "" {
		state.Detached = true
		state.Branch = ""
	}
	status, err := c.required(ctx, path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		state.Error = "Git status could not be read"
		return state, err
	}
	state.Dirty = strings.TrimSpace(status) != ""
	state.Remotes = c.remotes(ctx, path)
	state.Upstream, _ = c.optional(ctx, path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if state.Upstream != "" {
		if counts, countErr := c.required(ctx, path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"); countErr == nil {
			fields := strings.Fields(counts)
			if len(fields) == 2 {
				state.Ahead, _ = strconv.Atoi(fields[0])
				state.Behind, _ = strconv.Atoi(fields[1])
			}
		}
	}
	state.Worktrees, err = c.worktrees(ctx, path)
	state.WorktreeEnumerationComplete = err == nil
	state.UnsafeCleanup = state.Dirty || state.Detached || state.Ahead > 0 || len(state.Worktrees) > 1
	return state, nil
}

func sameDirectory(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func (c GitCollector) required(ctx context.Context, path string, args ...string) (string, error) {
	result, err := c.Runner.Run(ctx, "git", args, path)
	if err != nil || result.ExitCode != 0 {
		if err == nil {
			err = errors.New("git command failed")
		}
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c GitCollector) optional(ctx context.Context, path string, args ...string) (string, error) {
	result, err := c.Runner.Run(ctx, "git", args, path)
	if err != nil || result.ExitCode != 0 {
		if err == nil {
			err = errors.New("git command failed")
		}
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c GitCollector) remotes(ctx context.Context, path string) []RemoteInfo {
	result, err := c.Runner.Run(ctx, "git", []string{"remote", "-v"}, path)
	if err != nil || result.ExitCode != 0 {
		return []RemoteInfo{}
	}
	remotes := make(map[string]RemoteInfo)
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "(fetch)" {
			continue
		}
		normalized, host, provider, capabilities := NormalizeRemote(fields[1])
		if normalized == "" {
			continue
		}
		remotes[fields[0]] = RemoteInfo{Name: fields[0], URL: normalized, Host: host, Provider: provider, Capabilities: capabilities}
	}
	resultList := make([]RemoteInfo, 0, len(remotes))
	for _, remote := range remotes {
		resultList = append(resultList, remote)
	}
	// map iteration is intentionally normalized for deterministic observations.
	for i := 0; i < len(resultList); i++ {
		for j := i + 1; j < len(resultList); j++ {
			if resultList[j].Name < resultList[i].Name {
				resultList[i], resultList[j] = resultList[j], resultList[i]
			}
		}
	}
	return resultList
}

func (c GitCollector) worktrees(ctx context.Context, path string) ([]Worktree, error) {
	result, err := c.Runner.Run(ctx, "git", []string{"worktree", "list", "--porcelain", "-z"}, path)
	if err != nil || result.ExitCode != 0 {
		return []Worktree{}, err
	}
	return parseWorktrees(result.Stdout), nil
}

// parseWorktrees consumes Git's NUL-delimited porcelain format. Paths may
// contain spaces or newlines, so line splitting is deliberately forbidden.
func parseWorktrees(output string) []Worktree {
	var worktrees []Worktree
	var current *Worktree
	flush := func() {
		if current != nil {
			worktrees = append(worktrees, *current)
			current = nil
		}
	}
	records := strings.Split(output, "\x00")
	// Compatibility for test doubles and older Git output; real collection
	// always requests -z and never uses this branch for path parsing.
	if !strings.Contains(output, "\x00") {
		records = strings.Split(output, "\n")
	}
	for _, line := range records {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			current = &Worktree{Path: strings.TrimPrefix(line, "worktree "), Trust: "unverified"}
		case strings.HasPrefix(line, "HEAD ") && current != nil:
			current.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case strings.HasPrefix(line, "branch ") && current != nil:
			current.Branch = strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, "branch ")), "refs/heads/")
		case line == "bare" && current != nil:
			current.Bare = true
		case line == "locked" && current != nil:
			current.Locked = true
		case strings.HasPrefix(line, "locked ") && current != nil:
			current.Locked = true
		case line == "prunable" && current != nil:
			current.Prunable = true
		case strings.HasPrefix(line, "prunable ") && current != nil:
			current.Prunable = true
		case line == "":
			flush()
		}
	}
	flush()
	return worktrees
}

// WorktreeDetails verifies each advertised worktree against the same Git
// common directory before returning bounded read-only state. It is intentionally
// separate from Collect so callers can preserve repository-level compatibility.
func (c GitCollector) WorktreeDetails(ctx context.Context, registeredPath string, advertised []Worktree) ([]Worktree, bool) {
	registeredCanonical, err := canonicalDirectory(registeredPath)
	if err != nil {
		return advertised, false
	}
	common, err := c.gitDirectory(ctx, registeredCanonical, "--git-common-dir")
	if err != nil {
		return advertised, false
	}
	complete := true
	for i := range advertised {
		item := &advertised[i]
		candidate, candidateErr := canonicalDirectory(item.Path)
		if candidateErr != nil {
			// Git explicitly marks a prunable worktree as absent. It is complete
			// enumeration evidence, but is never entered or status-read.
			if !item.Prunable {
				complete = false
			}
			continue
		}
		top, topErr := c.required(ctx, candidate, "rev-parse", "--show-toplevel")
		candidateCommon, commonErr := c.gitDirectory(ctx, candidate, "--git-common-dir")
		gitDir, gitErr := c.gitDirectory(ctx, candidate, "--git-dir")
		if topErr != nil || commonErr != nil || gitErr != nil || !sameDirectory(candidate, top) || !sameDirectory(common, candidateCommon) {
			complete = false
			continue
		}
		item.Path = candidate
		item.Primary = sameDirectory(candidate, registeredCanonical)
		item.Trust = "verified_read_only"
		item.AssociationFingerprint = associationFingerprint(common, gitDir)
		if item.Primary {
			item.ID = "primary"
		} else {
			item.ID = "linked-" + item.AssociationFingerprint[len("sha256:"):][:32]
		}
		c.populateWorktree(ctx, item)
	}
	return advertised, complete
}

func (c GitCollector) populateWorktree(ctx context.Context, item *Worktree) {
	item.Branch, _ = c.optional(ctx, item.Path, "symbolic-ref", "--short", "-q", "HEAD")
	item.Detached = item.Branch == ""
	item.Head, _ = c.optional(ctx, item.Path, "rev-parse", "HEAD")
	status, err := c.required(ctx, item.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err == nil {
		item.Dirty = strings.TrimSpace(status) != ""
		for _, record := range strings.Split(status, "\x00") {
			if strings.HasPrefix(record, "?? ") {
				item.Untracked = true
			}
		}
	}
	item.Upstream, _ = c.optional(ctx, item.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if item.Upstream != "" {
		if counts, err := c.required(ctx, item.Path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"); err == nil {
			fields := strings.Fields(counts)
			if len(fields) == 2 {
				item.Ahead, _ = strconv.Atoi(fields[0])
				item.Behind, _ = strconv.Atoi(fields[1])
			}
		}
	}
}

func canonicalDirectory(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("worktree path is unavailable")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(canonical), nil
}

func (c GitCollector) gitDirectory(ctx context.Context, path, argument string) (string, error) {
	value, err := c.required(ctx, path, "rev-parse", argument)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(path, value)
	}
	return canonicalDirectory(value)
}

func associationFingerprint(commonDir, gitDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(commonDir) + "\x00" + filepath.Clean(gitDir)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// NormalizeRemote strips credentials, query strings, fragments and the .git
// suffix. It accepts HTTPS, SSH URLs and Git's scp-like syntax without ever
// returning the original credential-bearing value.
func NormalizeRemote(raw string) (normalized, host, provider string, capabilities []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ProviderUnknown, nil
	}
	parseValue := raw
	if strings.Contains(parseValue, ":") && !strings.Contains(parseValue, "://") && strings.Contains(parseValue, "@") {
		parts := strings.SplitN(parseValue, ":", 2)
		parseValue = "ssh://" + parts[0] + "/" + parts[1]
	}
	parsed, err := url.Parse(parseValue)
	if err != nil || parsed.Hostname() == "" {
		return "", "", ProviderUnknown, nil
	}
	host = strings.ToLower(parsed.Hostname())
	path := strings.TrimSuffix(strings.TrimRight(parsed.EscapedPath(), "/"), ".git")
	if path == "" || path == "/" {
		return "", host, ProviderUnknown, nil
	}
	normalized = "https://" + host + path
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		provider = ProviderGitHub
	case host == "gitlab.com" || strings.HasSuffix(host, ".gitlab.com"):
		provider = ProviderGitLab
	case host == "bitbucket.org" || strings.HasSuffix(host, ".bitbucket.org"):
		provider = ProviderBitbucket
	default:
		provider = ProviderUnknown
	}
	capabilities = []string{CapabilityRemoteRead}
	return normalized, host, provider, capabilities
}
