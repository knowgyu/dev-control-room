package app

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func scanRepo(ctx context.Context, path string) RepositoryState {
	state := RepositoryState{Path: path, ScannedAt: time.Now().UTC()}
	if output, err := git(ctx, path, "rev-parse", "--is-inside-work-tree"); err != nil || strings.TrimSpace(output) != "true" {
		state.Error = "not a readable Git worktree"
		return state
	}

	state.Branch, _ = git(ctx, path, "branch", "--show-current")
	state.Branch = strings.TrimSpace(state.Branch)
	state.Origin, _ = git(ctx, path, "remote", "get-url", "origin")
	state.Origin = strings.TrimSpace(state.Origin)

	status, err := git(ctx, path, "status", "--porcelain=v1")
	if err != nil {
		state.Error = err.Error()
		return state
	}
	state.Dirty = strings.TrimSpace(status) != ""

	counts, err := git(ctx, path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		fields := strings.Fields(counts)
		if len(fields) == 2 {
			state.Ahead, _ = strconv.Atoi(fields[0])
			state.Behind, _ = strconv.Atoi(fields[1])
		}
	}
	return state
}

func git(parent context.Context, path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	commandArgs := append([]string{"-C", path}, args...)
	output, err := exec.CommandContext(ctx, "git", commandArgs...).CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("git command timed out")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}
	return string(output), nil
}
