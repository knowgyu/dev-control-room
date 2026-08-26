package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func (a *App) SetAssuranceArtifactRetention(ctx context.Context, id, retention string) (domain.Artifact, error) {
	retention = strings.TrimSpace(retention)
	if retention != domain.ArtifactRetentionActive && retention != domain.ArtifactRetentionPinned {
		return domain.Artifact{}, contract.InvalidInput("artifact retention must be active or pinned")
	}
	item, err := a.assuranceArtifact(ctx, id)
	if err != nil {
		return domain.Artifact{}, err
	}
	if item.Spec.Retention == domain.ArtifactRetentionDeleted {
		return domain.Artifact{}, contract.Conflict("deleted artifact cannot change retention")
	}
	if retention == domain.ArtifactRetentionActive {
		data, readErr := readRegularFile(item.Spec.Path)
		if readErr != nil {
			return domain.Artifact{}, contract.Conflict("artifact must be restored before unpinning")
		}
		if int64(len(data)) != item.Spec.Size || artifactHash(data) != item.Spec.SHA256 {
			return domain.Artifact{}, contract.Conflict("local artifact hash does not match its manifest")
		}
		item.Spec.PinnedAt = nil
		item.Spec.PinReason = ""
	} else if item.Spec.Retention != domain.ArtifactRetentionPinned {
		data, readErr := readRegularFile(item.Spec.Path)
		if readErr != nil {
			if strings.TrimSpace(item.Spec.ArchivePath) == "" {
				return domain.Artifact{}, contract.Conflict("artifact must be restored or archived before pinning")
			}
			archivePath, archiveSize, archiveErr := archivedArtifactPath(item)
			if archiveErr != nil {
				return domain.Artifact{}, contract.Conflict("artifact archive cannot be verified before pinning")
			}
			data, archiveErr = readRegularFile(archivePath)
			if archiveErr != nil || int64(len(data)) != archiveSize {
				return domain.Artifact{}, contract.Conflict("artifact archive cannot be verified before pinning")
			}
		}
		if int64(len(data)) != item.Spec.Size || artifactHash(data) != item.Spec.SHA256 {
			return domain.Artifact{}, contract.Conflict("artifact hash does not match its manifest")
		}
		now := time.Now().UTC()
		item.Spec.PinnedAt = &now
	}
	item.Spec.Retention = retention
	if err := a.store.UpdateAssuranceArtifact(ctx, item); err != nil {
		return domain.Artifact{}, err
	}
	return item, nil
}

func (a *App) RestoreAssuranceArtifact(ctx context.Context, id string) (domain.Artifact, error) {
	item, err := a.assuranceArtifact(ctx, id)
	if err != nil {
		return domain.Artifact{}, err
	}
	if item.Spec.Retention == domain.ArtifactRetentionDeleted {
		return domain.Artifact{}, contract.Conflict("deleted artifact cannot be restored")
	}
	if data, readErr := readRegularFile(item.Spec.Path); readErr == nil {
		if artifactHash(data) != item.Spec.SHA256 {
			return domain.Artifact{}, contract.Conflict("local artifact hash does not match its manifest")
		}
		return a.markArtifactRestored(ctx, item)
	}
	if strings.TrimSpace(item.Spec.ArchivePath) == "" {
		return domain.Artifact{}, contract.Unavailable("artifact archive is not available")
	}

	sourcePath, expectedSize, err := archivedArtifactPath(item)
	if err != nil {
		return domain.Artifact{}, contract.Unavailable("artifact archive manifest is invalid")
	}
	data, err := readRegularFile(sourcePath)
	if err != nil {
		return domain.Artifact{}, contract.Unavailable("artifact archive is not available")
	}
	if int64(len(data)) != expectedSize || int64(len(data)) != item.Spec.Size || artifactHash(data) != item.Spec.SHA256 {
		return domain.Artifact{}, contract.Conflict("artifact archive hash does not match its manifest")
	}
	if err := restoreArtifactFile(item.Spec.Path, data); err != nil {
		return domain.Artifact{}, err
	}
	return a.markArtifactRestored(ctx, item)
}

func (a *App) assuranceArtifact(ctx context.Context, id string) (domain.Artifact, error) {
	items, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return domain.Artifact{}, err
	}
	for _, item := range items {
		if item.Metadata.ID == id {
			return item, nil
		}
	}
	return domain.Artifact{}, contract.NotFound("assurance artifact not found")
}

func (a *App) markArtifactRestored(ctx context.Context, item domain.Artifact) (domain.Artifact, error) {
	now := time.Now().UTC()
	item.Spec.RestoredAt = &now
	if item.Spec.Retention != domain.ArtifactRetentionPinned {
		item.Spec.Retention = domain.ArtifactRetentionActive
	}
	if err := a.store.UpdateAssuranceArtifact(ctx, item); err != nil {
		return domain.Artifact{}, err
	}
	return item, nil
}

func archivedArtifactPath(item domain.Artifact) (string, int64, error) {
	root, err := filepath.Abs(filepath.Clean(item.Spec.ArchivePath))
	if err != nil || root == "." {
		return "", 0, errors.New("archive path is invalid")
	}
	if item.Spec.ArchiveManifest != "" {
		manifestPath, err := safeArchiveChild(root, item.Spec.ArchiveManifest)
		if err != nil {
			return "", 0, err
		}
		manifest, err := readArchiveManifest(manifestPath, item.Spec.ArchiveSHA256)
		if err != nil {
			return "", 0, err
		}
		for _, entry := range manifest.ArtifactID {
			if entry.ArtifactID != item.Metadata.ID || entry.SHA256 != item.Spec.SHA256 || entry.Size != item.Spec.Size {
				continue
			}
			path, err := safeArchiveChild(root, entry.Filename)
			if err != nil {
				return "", 0, err
			}
			return path, entry.Size, nil
		}
		return "", 0, errors.New("artifact is missing from archive manifest")
	}
	// Archives created before the manifest contract are still recoverable, but
	// only through the server-owned basename of the original artifact path.
	path, err := safeArchiveChild(root, filepath.Base(item.Spec.Path))
	return path, item.Spec.Size, err
}

func safeArchiveChild(root, child string) (string, error) {
	child = filepath.Clean(child)
	if child == "." || filepath.IsAbs(child) || filepath.Base(child) != child || child == ".." || strings.Contains(child, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", errors.New("archive child path is invalid")
	}
	path := filepath.Join(root, child)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("archive child escapes archive root")
	}
	return path, nil
}

func restoreArtifactFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".artifact-restore-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return contract.Conflict("local artifact path is not a regular file")
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if artifactHash(existing) != artifactHash(data) {
			return contract.Conflict("local artifact already exists with a different hash")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("path is not a regular file")
	}
	return os.ReadFile(path)
}

func artifactHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
