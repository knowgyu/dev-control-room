package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

type assurancePathReferences struct {
	lexical  []string
	resolved []string
}

type assuranceCleanupRoot interface {
	Lstat(string) (os.FileInfo, error)
	Open(string) (*os.File, error)
	Remove(string) error
	RemoveAll(string) error
	Stat(string) (os.FileInfo, error)
	Close() error
}

type assuranceHomeSnapshot struct {
	path string
	info os.FileInfo
}

// cleanupOrphanedAssuranceFiles repairs only the crash window between an
// assurance file-system commit and its database commit. The database remains
// authoritative: every referenced path is preserved, and every deletion is
// confined to the canonical application home.
func (a *App) cleanupOrphanedAssuranceFiles(ctx context.Context) error {
	if !assuranceCleanupRootSupported() {
		// Go 1.23 has no os.Root equivalent. Do not fall back to an
		// unanchored path delete; defer janitor cleanup until a runtime with
		// directory-handle support is available.
		return nil
	}
	home, err := snapshotCanonicalApplicationHome(a.home)
	if err != nil {
		return fmt.Errorf("resolve application home for assurance cleanup: %w", err)
	}
	artifacts, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return fmt.Errorf("list assurance artifacts for startup cleanup: %w", err)
	}
	proposals, err := a.AssuranceProposals(ctx, "")
	if err != nil {
		return fmt.Errorf("list assurance proposals for startup cleanup: %w", err)
	}
	invocations, err := a.AgentInvocations(ctx)
	if err != nil {
		return fmt.Errorf("list agent invocations for startup cleanup: %w", err)
	}
	references := newAssurancePathReferences(len(artifacts) + len(proposals) + len(invocations))
	for _, artifact := range artifacts {
		references.add(artifact.Spec.Path)
	}
	for _, proposal := range proposals {
		references.add(proposal.Spec.IsolationPath)
	}
	for _, invocation := range invocations {
		if invocationMayHavePreparedArtifact(invocation.Spec.State) {
			references.add(preparedAgentInvocationArtifactPath(home.path, invocation.Metadata.ID))
		}
	}
	if err := cleanupOrphanedAssuranceArtifactsAt(home, references); err != nil {
		return fmt.Errorf("clean assurance artifact directory: %w", err)
	}
	if err := cleanupOrphanedAssuranceProposalsAt(home, references); err != nil {
		return fmt.Errorf("clean assurance proposal directories: %w", err)
	}
	return nil
}

func snapshotCanonicalApplicationHome(home string) (assuranceHomeSnapshot, error) {
	if strings.TrimSpace(home) == "" {
		return assuranceHomeSnapshot{}, errors.New("application home is empty")
	}
	absolute, err := filepath.Abs(filepath.Clean(home))
	if err != nil {
		return assuranceHomeSnapshot{}, fmt.Errorf("resolve application home: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return assuranceHomeSnapshot{}, fmt.Errorf("resolve application home symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return assuranceHomeSnapshot{}, fmt.Errorf("inspect application home: %w", err)
	}
	if !info.IsDir() {
		return assuranceHomeSnapshot{}, errors.New("application home is not a directory")
	}
	// Windows lazily loads the file identity used by os.SameFile. Force that
	// lookup while the canonical pathname still names the original directory.
	if !os.SameFile(info, info) {
		return assuranceHomeSnapshot{}, errors.New("identify application home")
	}
	return assuranceHomeSnapshot{path: filepath.Clean(resolved), info: info}, nil
}

func newAssurancePathReferences(capacity int) assurancePathReferences {
	return assurancePathReferences{
		lexical:  make([]string, 0, capacity),
		resolved: make([]string, 0, capacity),
	}
}

func (r *assurancePathReferences) add(path string) {
	lexical, resolved, ok := comparableAssurancePath(path)
	if !ok {
		return
	}
	for _, existing := range r.lexical {
		if assurancePathEqual(existing, lexical) {
			return
		}
	}
	r.lexical = append(r.lexical, lexical)
	r.resolved = append(r.resolved, resolved)
}

func (r assurancePathReferences) overlaps(path string) bool {
	lexical, resolved, ok := comparableAssurancePath(path)
	if !ok {
		return false
	}
	for index, reference := range r.lexical {
		if assurancePathsOverlap(lexical, reference) || assurancePathsOverlap(resolved, r.resolved[index]) {
			return true
		}
	}
	return false
}

func comparableAssurancePath(path string) (string, string, bool) {
	if path == "" || strings.ContainsRune(path, '\x00') {
		return "", "", false
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", "", false
	}
	lexical := filepath.Clean(absolute)
	resolved := lexical
	if value, err := filepath.EvalSymlinks(lexical); err == nil {
		resolved = filepath.Clean(value)
	}
	return lexical, resolved, true
}

func assurancePathEqual(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func assurancePathsOverlap(left, right string) bool {
	return assurancePathEqual(left, right) || pathWithin(left, right) || pathWithin(right, left)
}

func managedAssuranceDirectory(home string, parts ...string) (string, bool, error) {
	directory := filepath.Join(append([]string{home}, parts...)...)
	if !pathWithin(home, directory) {
		return "", false, errors.New("assurance cleanup directory escapes application home")
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", false, errors.New("assurance cleanup directory is not a local directory")
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", false, err
	}
	if !pathWithin(home, resolved) {
		return "", false, errors.New("assurance cleanup directory resolves outside application home")
	}
	return directory, true, nil
}

func cleanupOrphanedAssuranceArtifacts(home string, references assurancePathReferences) (returnErr error) {
	snapshot, err := snapshotCanonicalApplicationHome(home)
	if err != nil {
		return err
	}
	return cleanupOrphanedAssuranceArtifactsAt(snapshot, references)
}

func cleanupOrphanedAssuranceArtifactsAt(home assuranceHomeSnapshot, references assurancePathReferences) (returnErr error) {
	root, err := openVerifiedAssuranceCleanupRootAt(home)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close assurance cleanup root: %w", closeErr))
		}
	}()

	directory, found, err := managedAssuranceDirectory(home.path, "artifacts", "assurance")
	if err != nil || !found {
		return err
	}
	relativeDirectory, err := assuranceCleanupRelativePath(home.path, directory)
	if err != nil {
		return err
	}
	entries, err := readAssuranceCleanupDirectory(root, relativeDirectory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if references.overlaps(path) {
			continue
		}
		relativePath, relativeErr := assuranceCleanupRelativePath(home.path, path)
		if relativeErr != nil {
			return relativeErr
		}
		info, err := root.Lstat(relativePath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect orphaned assurance artifact %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if err := removeManagedAssuranceEntry(root, home.path, path); err != nil {
			return fmt.Errorf("remove orphaned assurance artifact %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func cleanupOrphanedAssuranceProposals(home string, references assurancePathReferences) (returnErr error) {
	snapshot, err := snapshotCanonicalApplicationHome(home)
	if err != nil {
		return err
	}
	return cleanupOrphanedAssuranceProposalsAt(snapshot, references)
}

func cleanupOrphanedAssuranceProposalsAt(home assuranceHomeSnapshot, references assurancePathReferences) (returnErr error) {
	root, err := openVerifiedAssuranceCleanupRootAt(home)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close assurance cleanup root: %w", closeErr))
		}
	}()

	directory, found, err := managedAssuranceDirectory(home.path, "assurance", "proposals")
	if err != nil || !found {
		return err
	}
	relativeDirectory, err := assuranceCleanupRelativePath(home.path, directory)
	if err != nil {
		return err
	}
	entries, err := readAssuranceCleanupDirectory(root, relativeDirectory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if references.overlaps(path) {
			continue
		}
		relativePath, relativeErr := assuranceCleanupRelativePath(home.path, path)
		if relativeErr != nil {
			return relativeErr
		}
		info, err := root.Lstat(relativePath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect orphaned assurance proposal %q: %w", entry.Name(), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := removeManagedAssuranceEntry(root, home.path, path); err != nil {
				return fmt.Errorf("remove orphaned assurance proposal link %q: %w", entry.Name(), err)
			}
			continue
		}
		if !info.IsDir() {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve orphaned assurance proposal %q: %w", entry.Name(), err)
		}
		if !pathWithin(directory, resolved) {
			continue
		}
		// Root.RemoveAll resolves every path component from a directory handle
		// and refuses links which would escape the application home. A rename or
		// replacement after the checks above therefore fails closed.
		if err := root.RemoveAll(relativePath); err != nil {
			return fmt.Errorf("remove orphaned assurance proposal %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func openVerifiedAssuranceCleanupRoot(home string) (assuranceCleanupRoot, error) {
	snapshot, err := snapshotCanonicalApplicationHome(home)
	if err != nil {
		return nil, fmt.Errorf("verify application home for assurance cleanup: %w", err)
	}
	return openVerifiedAssuranceCleanupRootAt(snapshot)
}

func openVerifiedAssuranceCleanupRootAt(home assuranceHomeSnapshot) (assuranceCleanupRoot, error) {
	root, err := openAssuranceCleanupRoot(home.path)
	if err != nil {
		return nil, fmt.Errorf("open application home for assurance cleanup: %w", err)
	}
	current, err := os.Stat(home.path)
	if err != nil {
		return nil, closeAssuranceCleanupRootOnError(root, fmt.Errorf("inspect application home replacement: %w", err))
	}
	rootInfo, err := root.Stat(".")
	if err != nil {
		return nil, closeAssuranceCleanupRootOnError(root, fmt.Errorf("inspect opened application home for assurance cleanup: %w", err))
	}
	if !os.SameFile(rootInfo, home.info) || !os.SameFile(current, home.info) {
		return nil, closeAssuranceCleanupRootOnError(root, errors.New("application home changed during assurance cleanup validation"))
	}
	return root, nil
}

func closeAssuranceCleanupRootOnError(root assuranceCleanupRoot, err error) error {
	if closeErr := root.Close(); closeErr != nil {
		return errors.Join(err, fmt.Errorf("close application home after assurance cleanup validation failure: %w", closeErr))
	}
	return err
}

func assuranceCleanupRelativePath(home, path string) (string, error) {
	if !pathWithin(home, path) {
		return "", errors.New("assurance cleanup path escapes application home")
	}
	relative, err := filepath.Rel(home, path)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return "", errors.New("assurance cleanup path is not a child of application home")
	}
	return filepath.ToSlash(relative), nil
}

func readAssuranceCleanupDirectory(root assuranceCleanupRoot, relative string) ([]os.DirEntry, error) {
	directory, err := root.Open(relative)
	if err != nil {
		return nil, fmt.Errorf("open assurance cleanup directory: %w", err)
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read assurance cleanup directory: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close assurance cleanup directory: %w", closeErr)
	}
	return entries, nil
}

func removeManagedAssuranceEntry(root assuranceCleanupRoot, home, path string) error {
	if !pathWithin(home, path) {
		return errors.New("assurance cleanup entry escapes application home")
	}
	relative, err := assuranceCleanupRelativePath(home, path)
	if err != nil {
		return err
	}
	info, err := root.Lstat(relative)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return errors.New("assurance cleanup entry is not a file or symlink")
	}
	if err := root.Remove(relative); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func cleanupCandidateDigest(candidate domain.CleanupCandidate) (string, error) {
	// Observation time proves freshness at the boundary, but is not target
	// identity; binding it would make the mandatory re-observation stale every
	// time without any state change.
	candidate.Spec.ObservedAt = time.Time{}
	data, err := json.Marshal(candidate)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (a *App) PlanCleanup(ctx context.Context, input CleanupPlanInput) (CleanupPlan, error) {
	if strings.TrimSpace(input.CandidateID) == "" || strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.RepositoryID) == "" || strings.TrimSpace(input.WorktreeID) == "" {
		return CleanupPlan{}, contract.InvalidInput("candidateId, projectId, repositoryId, and worktreeId are required")
	}
	if err := a.RunScan(ctx, "cleanup-plan"); err != nil {
		return CleanupPlan{}, err
	}
	candidate, err := a.findCleanupCandidate(ctx, input.ProjectID, input.CandidateID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return CleanupPlan{}, err
	}
	if candidate.Spec.Decision != domain.CleanupReviewable {
		return CleanupPlan{}, contract.CodedError{Code: contract.ErrorConflict, Message: "cleanup candidate is not reviewable"}
	}
	digest, err := cleanupCandidateDigest(candidate)
	if err != nil {
		return CleanupPlan{}, err
	}
	planIDSum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", candidate.Metadata.ID, time.Now().UnixNano())))
	plan, err := a.PlanAction(ctx, ActionPlanInput{ID: "cleanup-plan-" + hex.EncodeToString(planIDSum[:])[:48], Name: "연결 Worktree 정리", ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: "cleanup.destructive", Inputs: map[string]string{"candidate_id": candidate.Metadata.ID, "candidate_digest": digest}})
	if err != nil {
		return CleanupPlan{}, err
	}
	return CleanupPlan{ActionPlan: plan, Candidate: candidate, Digest: digest, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) CleanupPlan(ctx context.Context, planID string) (CleanupPlan, error) {
	plan, err := a.store.GetActionPlan(ctx, planID)
	if err != nil {
		return CleanupPlan{}, classifyActionError(err)
	}
	if plan.Spec.ActionType != "cleanup.destructive" {
		return CleanupPlan{}, contract.InvalidInput("action plan is not a cleanup plan")
	}
	candidateID := plan.Spec.Inputs["candidate_id"]
	candidate, err := a.findCleanupCandidate(ctx, plan.Spec.ProjectID, candidateID, plan.Spec.RepositoryID, plan.Spec.WorktreeID)
	if err != nil {
		return CleanupPlan{}, err
	}
	digest, err := cleanupCandidateDigest(candidate)
	if err != nil {
		return CleanupPlan{}, err
	}
	if digest != plan.Spec.Inputs["candidate_digest"] {
		return CleanupPlan{}, contract.CodedError{Code: contract.ErrorConflict, Message: "cleanup candidate changed; the plan is stale"}
	}
	return CleanupPlan{ActionPlan: plan, Candidate: candidate, Digest: digest, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) findCleanupCandidate(ctx context.Context, projectID, candidateID, repositoryID, worktreeID string) (domain.CleanupCandidate, error) {
	items, err := a.CleanupCandidates(ctx, projectID)
	if err != nil {
		return domain.CleanupCandidate{}, err
	}
	for _, item := range items {
		if item.Metadata.ID == candidateID && item.Spec.RepositoryID == repositoryID && item.Spec.WorktreeID == worktreeID {
			return item, nil
		}
	}
	return domain.CleanupCandidate{}, contract.CodedError{Code: contract.ErrorConflict, Message: "cleanup candidate is no longer observed; the plan is stale"}
}

func (a *App) CleanupResult(_ context.Context, planID string) (CleanupResult, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, result := range a.config.CleanupResults {
		if result.PlanID == planID {
			return result, nil
		}
	}
	return CleanupResult{}, contract.NotFound("cleanup result not found")
}

func (a *App) ExecuteCleanup(ctx context.Context, planID, holder, idempotencyKey string) (CleanupResult, error) {
	if err := a.RunScan(ctx, "cleanup-execute-precheck"); err != nil {
		return CleanupResult{}, err
	}
	plan, err := a.CleanupPlan(ctx, planID)
	if err != nil {
		return CleanupResult{}, err
	}
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	if err != nil {
		return CleanupResult{}, classifyActionError(err)
	}
	defer func() { _ = a.broker.Release(context.Background(), admission) }()
	if err := a.RunScan(ctx, "cleanup-execute-recheck"); err != nil {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", err.Error())
	}
	current, err := a.findCleanupCandidate(ctx, plan.ActionPlan.Spec.ProjectID, plan.Candidate.Metadata.ID, plan.ActionPlan.Spec.RepositoryID, plan.ActionPlan.Spec.WorktreeID)
	if err != nil {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", "cleanup candidate changed before execution")
	}
	digest, _ := cleanupCandidateDigest(current)
	if digest != plan.Digest || current.Spec.Decision != domain.CleanupReviewable {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", "cleanup candidate changed before execution")
	}
	repository, err := a.Repository(ctx, plan.ActionPlan.Spec.ProjectID, plan.ActionPlan.Spec.RepositoryID)
	if err != nil {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", "repository is no longer registered")
	}
	if filepath.Clean(repository.Spec.Path) == filepath.Clean(current.Spec.CanonicalPath) {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", "primary repository path cannot be cleaned")
	}
	if _, err := os.Stat(current.Spec.CanonicalPath); err != nil {
		return a.finishCleanup(ctx, plan, holder, "precheck_failed", "cleanup Worktree path is unavailable")
	}
	started := time.Now().UTC()
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(plan.ActionPlan, "cleanup_started", holder, started, "started"))
	runner := collector.ProcessRunner{Timeout: 60 * time.Second, OutputMax: 64 << 10}
	removed, runErr := runner.Run(ctx, "git", []string{"worktree", "remove", "--", current.Spec.CanonicalPath}, repository.Spec.Path)
	if runErr != nil || removed.ExitCode != 0 {
		return a.finishCleanup(ctx, plan, holder, "failed", "Git Worktree removal did not complete")
	}
	deleted, runErr := runner.Run(ctx, "git", []string{"branch", "-d", "--", current.Spec.Branch}, repository.Spec.Path)
	if runErr != nil || deleted.ExitCode != 0 {
		return a.finishCleanup(ctx, plan, holder, "failed", "Git branch removal did not complete")
	}
	if err := a.RunScan(ctx, "cleanup-postcheck"); err != nil {
		return a.finishCleanup(ctx, plan, holder, "postcheck_failed", "cleanup completed but postcheck failed")
	}
	return a.finishCleanup(ctx, plan, holder, "succeeded", "")
}

func (a *App) finishCleanup(ctx context.Context, plan CleanupPlan, holder, status, failure string) (CleanupResult, error) {
	result := CleanupResult{PlanID: plan.ActionPlan.Metadata.ID, CandidateID: plan.Candidate.Metadata.ID, Status: status, Path: plan.Candidate.Spec.CanonicalPath, Branch: plan.Candidate.Spec.Branch, Failure: failure, CompletedAt: time.Now().UTC()}
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(plan.ActionPlan, "cleanup_"+status, holder, result.CompletedAt, status))
	if err := a.saveCleanupResult(result); err != nil {
		return result, err
	}
	if status != "succeeded" {
		return result, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: failure}
	}
	return result, nil
}

func (a *App) saveCleanupResult(result CleanupResult) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	items := append([]CleanupResult(nil), a.config.CleanupResults...)
	updated := false
	for index := range items {
		if items[index].PlanID == result.PlanID {
			items[index] = result
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, result)
	}
	if len(items) > 128 {
		items = items[len(items)-128:]
	}
	previous := a.config
	a.config.CleanupResults = items
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}
