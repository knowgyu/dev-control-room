package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
)

const qualityRunCoverageArtifactLimit = 1 << 20

const (
	qualityRunCoverageProfileMissingReason      = "coverage.profile_missing"
	qualityRunCoverageProfileInconclusiveReason = "coverage.profile_inconclusive"
)

func (a *App) newQualityCoverageProfile(runID string) (string, func(), error) {
	directory := filepath.Join(a.home, "runtime", "quality")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", func() {}, err
	}
	info, err := os.Lstat(directory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", func() {}, errors.New("quality coverage directory is not a regular directory")
	}
	path := filepath.Join(directory, runID+".cover.out")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func readBoundedQualityCoverage(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, errors.New("quality coverage profile is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, qualityRunCoverageArtifactLimit+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > qualityRunCoverageArtifactLimit {
		return data[:qualityRunCoverageArtifactLimit], true, nil
	}
	return data, false, nil
}

func qualityRunProcessOutcome(result environment.Result, processErr error) (state, outcome, summary, reason string) {
	if processErr == nil {
		return "", "", "", ""
	}
	if errors.Is(processErr, context.DeadlineExceeded) || strings.Contains(strings.ToLower(processErr.Error()), "timed out") {
		return domain.AssuranceStateTimedOut, domain.QualityRunOutcomeInconclusive, "선택한 Quality Runner가 제한 시간 안에 완료되지 않았습니다.", "runner.timeout"
	}
	if result.ExitCode != 0 {
		return domain.AssuranceStateFailed, domain.QualityRunOutcomeTestsFailed, "Go 테스트가 실패했습니다.", "runner.tests_failed"
	}
	return domain.AssuranceStateFailed, domain.QualityRunOutcomeInconclusive, "선택한 Quality Runner 결과를 확정할 수 없습니다.", "runner.inconclusive"
}

func qualityCoverageSummary(summary assurance.GoCoverageSummary, artifactID string) *domain.QualityCoverage {
	return &domain.QualityCoverage{
		Mode: summary.Mode, FileCount: summary.FileCount,
		TotalStatements: summary.TotalStatements, CoveredStatements: summary.CoveredStatements,
		Percent: summary.Percent, ProfileArtifactID: artifactID,
	}
}
