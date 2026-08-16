package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoCommitsYet reports that the repository has not recorded its first commit.
var ErrNoCommitsYet = errors.New("git repository has no commits yet")

// Client runs git commands against a specific repository directory.
type Client struct {
	Dir        string
	Executable string
}

// NewClient constructs a git client bound to a repository directory.
func NewClient(dir, executable string) *Client {
	return &Client{
		Dir:        dir,
		Executable: executable,
	}
}

// Status returns the output of git status.
func (c *Client) Status(ctx context.Context) (string, error) {
	return c.run(ctx, "status", "--porcelain")
}

// HasStagedChanges reports whether the index differs from HEAD.
func (c *Client) HasStagedChanges(ctx context.Context) (bool, error) {
	files, err := c.run(ctx, "diff", "--cached", "--name-only", "-z", "--")
	if err != nil {
		return false, err
	}

	return files != "", nil
}

// Log returns recent commit messages (last 10).
func (c *Client) Log(ctx context.Context) (string, error) {
	hasCommits, err := c.hasCommits(ctx)
	if err != nil {
		return "", err
	}

	if !hasCommits {
		return "", ErrNoCommitsYet
	}

	return c.run(ctx, "log", "-10", "--oneline")
}

// Add stages files for commit.
func (c *Client) Add(ctx context.Context, files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := c.run(ctx, args...)

	return err
}

// Commit creates a commit with the given message.
func (c *Client) Commit(ctx context.Context, message string) error {
	_, err := c.run(ctx, "commit", "-m", message)
	return err
}

func (c *Client) hasCommits(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, c.executable(), "show-ref", "--head", "--quiet")
	cmd.Dir = c.Dir

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}

		if stderr.Len() > 0 {
			return false, fmt.Errorf("git show-ref --head --quiet failed: %s", strings.TrimSpace(stderr.String()))
		}

		return false, fmt.Errorf("git show-ref --head --quiet failed: %w", err)
	}

	return true, nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.executable(), args...)
	cmd.Dir = c.Dir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), ctx.Err())
		}

		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}

		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}

func (c *Client) executable() string {
	if c.Executable != "" {
		return c.Executable
	}

	return "git"
}
