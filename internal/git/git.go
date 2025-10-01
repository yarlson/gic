package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FileChange represents statistics for a changed file.
type FileChange struct {
	Path    string
	Added   int
	Removed int
}

// Status returns the output of git status.
func Status() (string, error) {
	return run("status")
}

// Diff returns the output of git diff (staged and unstaged), excluding lock files.
func Diff() (string, error) {
	// Common lock files to exclude from diff
	excludes := []string{
		":(exclude)package-lock.json",
		":(exclude)yarn.lock",
		":(exclude)pnpm-lock.yaml",
		":(exclude)Gemfile.lock",
		":(exclude)Cargo.lock",
		":(exclude)go.sum",
		":(exclude)composer.lock",
		":(exclude)Pipfile.lock",
		":(exclude)poetry.lock",
		":(exclude)mix.lock",
		":(exclude)pubspec.lock",
		":(exclude)Podfile.lock",
		":(exclude)packages.lock.json",
		":(exclude)paket.lock",
	}

	stagedArgs := append([]string{"diff", "--cached"}, excludes...)

	staged, err := run(stagedArgs...)
	if err != nil {
		return "", err
	}

	unstagedArgs := append([]string{"diff"}, excludes...)

	unstaged, err := run(unstagedArgs...)
	if err != nil {
		return "", err
	}

	return staged + "\n" + unstaged, nil
}

// DiffStat returns statistics for all changed files (staged and unstaged).
func DiffStat() ([]FileChange, error) {
	// Get staged file stats
	stagedOutput, err := run("diff", "--numstat", "--cached")
	if err != nil {
		return nil, err
	}

	// Get unstaged file stats
	unstagedOutput, err := run("diff", "--numstat")
	if err != nil {
		return nil, err
	}

	// Parse both outputs
	statsMap := make(map[string]*FileChange)

	parseNumstat := func(output string) {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}

			added, _ := strconv.Atoi(parts[0])
			removed, _ := strconv.Atoi(parts[1])
			path := parts[2]

			if existing, ok := statsMap[path]; ok {
				existing.Added += added
				existing.Removed += removed
			} else {
				statsMap[path] = &FileChange{
					Path:    path,
					Added:   added,
					Removed: removed,
				}
			}
		}
	}

	parseNumstat(stagedOutput)
	parseNumstat(unstagedOutput)

	// Convert map to slice
	var stats []FileChange
	for _, stat := range statsMap {
		stats = append(stats, *stat)
	}

	return stats, nil
}

// DiffFiles returns the diff for specific files only, excluding lock files.
func DiffFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	// Common lock files to exclude
	excludes := []string{
		":(exclude)package-lock.json",
		":(exclude)yarn.lock",
		":(exclude)pnpm-lock.yaml",
		":(exclude)Gemfile.lock",
		":(exclude)Cargo.lock",
		":(exclude)go.sum",
		":(exclude)composer.lock",
		":(exclude)Pipfile.lock",
		":(exclude)poetry.lock",
		":(exclude)mix.lock",
		":(exclude)pubspec.lock",
		":(exclude)Podfile.lock",
		":(exclude)packages.lock.json",
		":(exclude)paket.lock",
	}

	// Build args: diff --cached [excludes...] -- [paths...]
	stagedArgs := append([]string{"diff", "--cached"}, excludes...)
	stagedArgs = append(stagedArgs, "--")
	stagedArgs = append(stagedArgs, paths...)

	staged, err := run(stagedArgs...)
	if err != nil {
		return "", err
	}

	// Build args: diff [excludes...] -- [paths...]
	unstagedArgs := append([]string{"diff"}, excludes...)
	unstagedArgs = append(unstagedArgs, "--")
	unstagedArgs = append(unstagedArgs, paths...)

	unstaged, err := run(unstagedArgs...)
	if err != nil {
		return "", err
	}

	return staged + "\n" + unstaged, nil
}

// Log returns recent commit messages (last 10).
// Returns empty string if no commits exist yet.
func Log() (string, error) {
	output, err := run("log", "-10", "--oneline")
	if err != nil && strings.Contains(err.Error(), "does not have any commits yet") {
		return "", nil
	}

	return output, err
}

// Add stages files for commit.
func Add(files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := run(args...)

	return err
}

// Commit creates a commit with the given message.
func Commit(message string) error {
	_, err := run("commit", "-m", message)
	return err
}

// CommitAmend amends the last commit with a new message.
func CommitAmend(message string) error {
	_, err := run("commit", "--amend", "-m", message)
	return err
}

// LastCommitAuthor returns the author name and email of the last commit.
func LastCommitAuthor() (name, email string, err error) {
	output, err := run("log", "-1", "--format=%an|%ae")
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(strings.TrimSpace(output), "|")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected author format: %s", output)
	}

	return parts[0], parts[1], nil
}

// IsAheadOfRemote checks if the current branch is ahead of remote.
func IsAheadOfRemote() (bool, error) {
	output, err := run("status", "-sb")
	if err != nil {
		return false, err
	}

	return strings.Contains(output, "ahead"), nil
}

// run executes a git command and returns its output.
func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), stderr.String())
		}

		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}
