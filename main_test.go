package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"cmt/internal/provider"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "cmt"}
	cmd.Flags().String("provider", "", "")
	cmd.Flags().String("model", "", "")

	return cmd
}

// TestRunErrorsWhenDefaultProviderMissing verifies that `run` exits with a
// clear provider error before any UI fires when the default provider binary is
// unavailable on $PATH.
func TestRunErrorsWhenDefaultProviderMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based PATH setup uses POSIX behavior")
	}

	stubDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	t.Setenv("CMT_PROVIDER", "")

	linkPath := filepath.Join(stubDir, "git")
	require.NoError(t, os.Symlink(gitPath, linkPath))
	t.Setenv("PATH", stubDir)

	err = run(context.Background(), newTestCommand(), "anything", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude")
	assert.Contains(t, err.Error(), "PATH")
}

func TestRunErrorsWhenGitMissing(t *testing.T) {
	t.Setenv("PATH", "")

	err := run(t.Context(), newTestCommand(), "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required executable `git`")
}

func TestRunErrorsForUnsupportedProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based PATH setup uses POSIX behavior")
	}

	stubDir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(stubDir, "git")))
	t.Setenv("PATH", stubDir)
	t.Setenv("CMT_PROVIDER", "unknown")

	err = run(t.Context(), newTestCommand(), "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

// buildCmtBinary compiles the cmt binary into a fresh temp dir and returns
// its path. Used by acceptance tests that need to run `cmt` as a subprocess.
func buildCmtBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()

	binName := "cmt"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	binPath := filepath.Join(binDir, binName)

	repoRoot, err := os.Getwd()
	require.NoError(t, err)

	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binPath, ".")
	cmd.Dir = repoRoot

	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "go build failed: %s", out)

	return binPath
}

func initMainTestRepo(t *testing.T) (string, string) {
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

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "initial.txt"), []byte("initial"), 0o644))

	for _, args := range [][]string{{"add", "initial.txt"}, {"commit", "-m", "Initial commit"}} {
		cmd := exec.CommandContext(t.Context(), gitPath, args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v failed: %s", args, out)
	}

	return repoDir, gitPath
}

func prepareRunTest(t *testing.T) (string, string) {
	t.Helper()

	repoDir, gitPath := initMainTestRepo(t)
	stubDir := t.TempDir()
	require.NoError(t, os.Symlink(gitPath, filepath.Join(stubDir, "git")))
	require.NoError(t, os.WriteFile(filepath.Join(stubDir, "claude"), []byte("#!/bin/sh\n"+
		"if [ \"$1\" = '--help' ]; then\n"+
		"  printf '%s\\n' '--disable-slash-commands' '--no-session-persistence' '--permission-mode' '--allowedTools' '-p, --print'\n"+
		"  exit 0\n"+
		"fi\n"+
		"if [ \"$1\" = 'auth' ] && [ \"$2\" = 'status' ]; then\n"+
		"  printf '%s\\n' '{\"loggedIn\": true}'\n"+
		"  exit 0\n"+
		"fi\n"+
		"printf '%s\\n' 'Describe staged behavior'\n"), 0o755))

	t.Setenv("PATH", stubDir)
	t.Setenv("CMT_PROVIDER", "claude")
	t.Setenv("CMT_MODEL", "")
	t.Chdir(repoDir)

	return repoDir, gitPath
}

func TestRunMessageOnlyWritesGeneratedMessage(t *testing.T) {
	repoDir, gitPath := prepareRunTest(t)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("staged"), 0o644))

	addCmd := exec.CommandContext(t.Context(), gitPath, "add", "staged.txt")
	addCmd.Dir = repoDir
	require.NoError(t, addCmd.Run())

	cmd := newTestCommand()

	var output bytes.Buffer
	cmd.SetOut(&output)

	require.NoError(t, run(t.Context(), cmd, "staged only", true))
	assert.Equal(t, "Describe staged behavior\n", output.String())
}

func TestRunNormalModeCreatesCommit(t *testing.T) {
	repoDir, gitPath := prepareRunTest(t)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature"), 0o644))

	previousAutoApprove := autoApprove
	autoApprove = true

	t.Cleanup(func() { autoApprove = previousAutoApprove })

	require.NoError(t, run(t.Context(), newTestCommand(), "", false))

	message, err := exec.CommandContext(t.Context(), gitPath, "-C", repoDir, "log", "-1", "--pretty=%s").Output()
	require.NoError(t, err)
	assert.Equal(t, "Describe staged behavior\n", string(message))
}

func TestMessageOnlyPrintsMessageWithoutChangingRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("provider stub uses POSIX sh")
	}

	binPath := buildCmtBinary(t)
	repoDir, gitPath := initMainTestRepo(t)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("staged"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "unstaged.txt"), []byte("unstaged"), 0o644))

	addCmd := exec.CommandContext(t.Context(), gitPath, "add", "staged.txt")
	addCmd.Dir = repoDir
	require.NoError(t, addCmd.Run())

	statusBefore, err := exec.CommandContext(t.Context(), gitPath, "-C", repoDir, "status", "--porcelain").Output()
	require.NoError(t, err)
	headBefore, err := exec.CommandContext(t.Context(), gitPath, "-C", repoDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)

	stubDir := t.TempDir()
	require.NoError(t, os.Symlink(gitPath, filepath.Join(stubDir, "git")))
	require.NoError(t, os.WriteFile(filepath.Join(stubDir, "claude"), []byte("#!/bin/sh\n"+
		"if [ \"$1\" = '--help' ]; then\n"+
		"  printf '%s\\n' '--disable-slash-commands' '--no-session-persistence' '--permission-mode' '--allowedTools' '-p, --print'\n"+
		"  exit 0\n"+
		"fi\n"+
		"if [ \"$1\" = 'auth' ] && [ \"$2\" = 'status' ]; then\n"+
		"  printf '%s\\n' '{\"loggedIn\": true}'\n"+
		"  exit 0\n"+
		"fi\n"+
		"printf '%s\\n' 'Describe staged behavior'\n"), 0o755))

	cmd := exec.CommandContext(t.Context(), binPath, "--message-only", "focus on staged behavior")
	cmd.Dir = repoDir
	cmd.Env = []string{"PATH=" + stubDir, "CMT_PROVIDER=claude", "CMT_MODEL="}

	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "cmt --message-only failed: %s", out)
	assert.Equal(t, "Describe staged behavior\n", string(out))

	statusAfter, err := exec.CommandContext(t.Context(), gitPath, "-C", repoDir, "status", "--porcelain").Output()
	require.NoError(t, err)
	assert.Equal(t, statusBefore, statusAfter)

	headAfter, err := exec.CommandContext(t.Context(), gitPath, "-C", repoDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	assert.Equal(t, headBefore, headAfter)
}

func TestMessageOnlyRejectsAutoApproveBeforePreflight(t *testing.T) {
	binPath := buildCmtBinary(t)
	cmd := exec.CommandContext(t.Context(), binPath, "--message-only", "--auto-approve")
	cmd.Env = []string{"PATH="}

	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "--message-only cannot be combined with --auto-approve")
	assert.NotContains(t, string(out), "required executable")
}

// TestVersionCommandsDoNotInvokeProviders verifies that `cmt version` and
// `cmt --version` print build metadata without requiring provider CLIs on
// $PATH.
func TestVersionCommandsDoNotInvokeProviders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation depends on POSIX layout")
	}

	binPath := buildCmtBinary(t)

	for _, args := range [][]string{{"version"}, {"--version"}} {
		cmd := exec.CommandContext(t.Context(), binPath, args...)

		cmd.Env = append(os.Environ(), "PATH=")

		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "cmt %v should succeed without provider CLIs on PATH (out: %s)", args, out)
		assert.Contains(t, string(out), "Version:", "cmt %v should print version metadata", args)
		assert.Contains(t, string(out), "Built:", "cmt %v should print build metadata", args)
	}
}

func TestResolveOptionPrecedence(t *testing.T) {
	cmd := newTestCommand()

	t.Setenv("CMT_PROVIDER", "claude")

	require.NoError(t, cmd.Flags().Set("provider", "codex"))
	assert.Equal(t, "codex", resolveOption(cmd, "provider", "CMT_PROVIDER", provider.DefaultProviderID))
}

func TestResolveOptionFallsBackToEnvThenDefault(t *testing.T) {
	cmd := newTestCommand()

	t.Setenv("CMT_PROVIDER", "")
	t.Setenv("CMT_MODEL", "opus")

	assert.Equal(t, "opus", resolveOption(cmd, "model", "CMT_MODEL", "sonnet"))
	assert.Equal(t, provider.DefaultProviderID, resolveOption(cmd, "provider", "CMT_PROVIDER", provider.DefaultProviderID))
}

func TestProviderDefinitionsExposeProviderSpecificDefaults(t *testing.T) {
	claude, err := provider.Lookup("claude")
	require.NoError(t, err)
	assert.Equal(t, "sonnet", claude.DefaultModel)

	codex, err := provider.Lookup("codex")
	require.NoError(t, err)
	assert.Empty(t, codex.DefaultModel)
}
