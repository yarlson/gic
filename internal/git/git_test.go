package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"cmt/internal/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initRepo(t *testing.T) (string, string) {
	t.Helper()

	repoDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)

	for _, args := range [][]string{
		{"init"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@example.com"},
	} {
		cmd := exec.CommandContext(t.Context(), gitPath, args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, out)
	}

	return repoDir, gitPath
}

func writeExecutable(t *testing.T, dir, name, script string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))

	return filepath.Join(dir, name)
}

func TestClientUsesConfiguredDir(t *testing.T) {
	repoDir, gitPath := initRepo(t)
	client := git.NewClient(repoDir, gitPath)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("content"), 0o644))

	otherDir := t.TempDir()
	t.Chdir(otherDir)

	status, err := client.Status(context.Background())
	require.NoError(t, err)
	assert.Contains(t, status, "tracked.txt")
}

func TestLogReturnsErrNoCommitsYet(t *testing.T) {
	repoDir, gitPath := initRepo(t)
	client := git.NewClient(repoDir, gitPath)

	log, err := client.Log(context.Background())
	require.ErrorIs(t, err, git.ErrNoCommitsYet)
	assert.Empty(t, log)
}

func TestAddAndCommit(t *testing.T) {
	repoDir, gitPath := initRepo(t)
	client := git.NewClient(repoDir, gitPath)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature"), 0o644))
	require.NoError(t, client.Add(context.Background(), "feature.txt"))
	require.NoError(t, client.Commit(context.Background(), "Add feature"))

	log, err := client.Log(context.Background())
	require.NoError(t, err)
	assert.Contains(t, log, "Add feature")

	status, err := client.Status(context.Background())
	require.NoError(t, err)
	assert.Empty(t, status)
}

func TestHasStagedChangesIgnoresUnstagedFiles(t *testing.T) {
	repoDir, gitPath := initRepo(t)
	client := git.NewClient(repoDir, gitPath)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "unstaged.txt"), []byte("unstaged"), 0o644))

	hasStagedChanges, err := client.HasStagedChanges(context.Background())
	require.NoError(t, err)
	assert.False(t, hasStagedChanges)

	require.NoError(t, client.Add(context.Background(), "unstaged.txt"))

	hasStagedChanges, err = client.HasStagedChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, hasStagedChanges)
}

func TestStatusHonorsContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub uses POSIX sh")
	}

	repoDir := t.TempDir()
	stubDir := t.TempDir()
	stubPath := writeExecutable(t, stubDir, "git", "#!/bin/sh\nsleep 5\n")
	client := git.NewClient(repoDir, stubPath)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Status(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
