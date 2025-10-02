package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"gic/internal/auth"
	"gic/internal/client"
	"gic/internal/git"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server represents an MCP server for git commit operations.
type Server struct {
	server      *mcp.Server
	accessToken string
	tokenPath   string
}

// NewServer creates a new MCP server instance.
func NewServer(accessToken, tokenPath string) *Server {
	impl := &mcp.Implementation{
		Name:    "gic",
		Version: "1.0.0",
	}

	server := mcp.NewServer(impl, nil)

	s := &Server{
		server:      server,
		accessToken: accessToken,
		tokenPath:   tokenPath,
	}

	// Register tools
	s.registerTools()

	// Register resources
	s.registerResources()

	return s
}

// Run starts the MCP server with stdio transport.
func (s *Server) Run(ctx context.Context) error {
	log.Println("Starting gic MCP server...")
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

// Tool input/output types

type GenerateCommitMessageInput struct {
	UserContext string `json:"user_context,omitempty" jsonschema:"Additional context about the changes"`
}

type GenerateCommitMessageOutput struct {
	CommitMessage string `json:"commit_message" jsonschema:"The generated commit message"`
}

type CreateCommitInput struct {
	UserContext string `json:"user_context,omitempty" jsonschema:"Additional context about the changes"`
	Message     string `json:"message,omitempty" jsonschema:"Custom commit message (if not provided, one will be generated)"`
}

type CreateCommitOutput struct {
	CommitHash string `json:"commit_hash,omitempty" jsonschema:"The hash of the created commit"`
	Message    string `json:"message" jsonschema:"The commit message used"`
	Success    bool   `json:"success" jsonschema:"Whether the commit was successful"`
	Error      string `json:"error,omitempty" jsonschema:"Error message if commit failed"`
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	// Tool 1: Generate commit message
	mcp.AddTool(
		s.server,
		&mcp.Tool{
			Name: "generate_commit_message",
			Description: "IMPORTANT: Use this tool whenever the user asks to generate a commit message, create a commit, or commit changes. " +
				"This tool analyzes git changes and generates an intelligent, contextual commit message using Claude AI. " +
				"It automatically stages changes, reviews diffs, and creates a commit message that explains WHY changes were made, not just WHAT changed. " +
				"The generated message follows the repository's commit style by analyzing recent commits.",
		},
		s.handleGenerateCommitMessage,
	)

	// Tool 2: Create commit
	mcp.AddTool(
		s.server,
		&mcp.Tool{
			Name: "create_commit",
			Description: "IMPORTANT: Use this tool whenever the user asks to commit changes, create a commit, or save work to git. " +
				"This tool stages all changes and creates a git commit with either a generated or provided message. " +
				"If no message is provided, it will automatically generate an intelligent commit message using Claude AI. " +
				"Use this tool instead of manual git commands when the user wants to commit their work. " +
				"Optionally provide user_context to guide the commit message generation (e.g., 'fixed bug in authentication' or 'added new feature').",
		},
		s.handleCreateCommit,
	)
}

// registerResources registers all MCP resources.
func (s *Server) registerResources() {
	// Resource 1: Git status
	s.server.AddResource(
		&mcp.Resource{
			URI:         "git://status",
			Name:        "Git Status",
			Description: "Current git repository status",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			status, err := git.Status()
			if err != nil {
				return nil, fmt.Errorf("failed to get git status: %w", err)
			}

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "git://status",
						MIMEType: "text/plain",
						Text:     status,
					},
				},
			}, nil
		},
	)

	// Resource 2: Git diff
	s.server.AddResource(
		&mcp.Resource{
			URI:         "git://diff",
			Name:        "Git Diff",
			Description: "Current git diff (staged and unstaged changes)",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			diff, err := git.Diff()
			if err != nil {
				return nil, fmt.Errorf("failed to get git diff: %w", err)
			}

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "git://diff",
						MIMEType: "text/plain",
						Text:     diff,
					},
				},
			}, nil
		},
	)

	// Resource 3: Recent commits
	s.server.AddResource(
		&mcp.Resource{
			URI:         "git://recent-commits",
			Name:        "Recent Commits",
			Description: "Recent commit history (last 10 commits)",
			MIMEType:    "text/plain",
		},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			log, err := git.Log()
			if err != nil {
				return nil, fmt.Errorf("failed to get git log: %w", err)
			}

			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      "git://recent-commits",
						MIMEType: "text/plain",
						Text:     log,
					},
				},
			}, nil
		},
	)
}

// handleGenerateCommitMessage handles the generate_commit_message tool.
func (s *Server) handleGenerateCommitMessage(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GenerateCommitMessageInput,
) (*mcp.CallToolResult, GenerateCommitMessageOutput, error) {
	// Ensure token is valid
	token, err := s.ensureValidToken()
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, GenerateCommitMessageOutput{}, err
	}

	s.accessToken = token

	// Gather git information
	var (
		status, diff, log string
		fileStats         []git.FileChange
		errs              []error
		wg                sync.WaitGroup
		mu                sync.Mutex
	)

	wg.Add(4)

	go func() {
		defer wg.Done()

		st, err := git.Status()
		if err != nil {
			mu.Lock()

			errs = append(errs, fmt.Errorf("git status failed: %w", err))

			mu.Unlock()

			return
		}

		status = st
	}()

	go func() {
		defer wg.Done()

		stats, err := git.DiffStat()
		if err != nil {
			mu.Lock()

			errs = append(errs, fmt.Errorf("git diff stat failed: %w", err))

			mu.Unlock()

			return
		}

		fileStats = stats
	}()

	go func() {
		defer wg.Done()

		d, err := git.Diff()
		if err != nil {
			mu.Lock()

			errs = append(errs, fmt.Errorf("git diff failed: %w", err))

			mu.Unlock()

			return
		}

		diff = d
	}()

	go func() {
		defer wg.Done()

		l, err := git.Log()
		if err != nil {
			mu.Lock()

			errs = append(errs, fmt.Errorf("git log failed: %w", err))

			mu.Unlock()

			return
		}

		log = l
	}()

	wg.Wait()

	if len(errs) > 0 {
		return &mcp.CallToolResult{IsError: true}, GenerateCommitMessageOutput{}, errs[0]
	}

	// Check if there are changes
	if diff == "" || strings.TrimSpace(diff) == "" {
		return &mcp.CallToolResult{IsError: true}, GenerateCommitMessageOutput{},
			fmt.Errorf("no changes to commit")
	}

	// Generate commit message
	commitMsg, err := generateCommitMessage(s.accessToken, status, diff, log, fileStats, input.UserContext)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, GenerateCommitMessageOutput{}, err
	}

	return nil, GenerateCommitMessageOutput{CommitMessage: commitMsg}, nil
}

// handleCreateCommit handles the create_commit tool.
func (s *Server) handleCreateCommit(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateCommitInput,
) (*mcp.CallToolResult, CreateCommitOutput, error) {
	// Stage all changes
	if err := git.Add("."); err != nil {
		return nil, CreateCommitOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to stage changes: %v", err),
		}, nil
	}

	var (
		commitMsg string
		err       error
	)

	if input.Message != "" {
		// Use provided message
		commitMsg = input.Message
	} else {
		// Generate message
		token, err := s.ensureValidToken()
		if err != nil {
			return nil, CreateCommitOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to ensure valid token: %v", err),
			}, nil
		}

		s.accessToken = token

		// Gather git information
		var (
			status, diff, log string
			fileStats         []git.FileChange
			errs              []error
			wg                sync.WaitGroup
			mu                sync.Mutex
		)

		wg.Add(4)

		go func() {
			defer wg.Done()

			st, err := git.Status()
			if err != nil {
				mu.Lock()

				errs = append(errs, fmt.Errorf("git status failed: %w", err))

				mu.Unlock()

				return
			}

			status = st
		}()

		go func() {
			defer wg.Done()

			stats, err := git.DiffStat()
			if err != nil {
				mu.Lock()

				errs = append(errs, fmt.Errorf("git diff stat failed: %w", err))

				mu.Unlock()

				return
			}

			fileStats = stats
		}()

		go func() {
			defer wg.Done()

			d, err := git.Diff()
			if err != nil {
				mu.Lock()

				errs = append(errs, fmt.Errorf("git diff failed: %w", err))

				mu.Unlock()

				return
			}

			diff = d
		}()

		go func() {
			defer wg.Done()

			l, err := git.Log()
			if err != nil {
				mu.Lock()

				errs = append(errs, fmt.Errorf("git log failed: %w", err))

				mu.Unlock()

				return
			}

			log = l
		}()

		wg.Wait()

		if len(errs) > 0 {
			return nil, CreateCommitOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to gather git info: %v", errs[0]),
			}, nil
		}

		// Check if there are changes
		if diff == "" || strings.TrimSpace(diff) == "" {
			return nil, CreateCommitOutput{
				Success: false,
				Error:   "no changes to commit",
			}, nil
		}

		// Generate commit message
		commitMsg, err = generateCommitMessage(s.accessToken, status, diff, log, fileStats, input.UserContext)
		if err != nil {
			return nil, CreateCommitOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to generate commit message: %v", err),
			}, nil
		}
	}

	// Create commit
	if err = git.Commit(commitMsg); err != nil {
		return nil, CreateCommitOutput{
			Success: false,
			Message: commitMsg,
			Error:   fmt.Sprintf("failed to create commit: %v", err),
		}, nil
	}

	// Get commit hash
	output, _ := git.Log()
	lines := strings.Split(output, "\n")
	commitHash := ""

	if len(lines) > 0 && lines[0] != "" {
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			commitHash = parts[0]
		}
	}

	return nil, CreateCommitOutput{
		Success:    true,
		Message:    commitMsg,
		CommitHash: commitHash,
	}, nil
}

// ensureValidToken ensures the access token is valid, refreshing if needed.
func (s *Server) ensureValidToken() (string, error) {
	token, err := auth.Load(s.tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to load token: %w", err)
	}

	token, err = auth.EnsureValid(token, s.tokenPath, auth.ClientID, auth.TokenURL)
	if err != nil {
		return "", fmt.Errorf("failed to ensure valid token: %w", err)
	}

	return token.AccessToken, nil
}

// generateCommitMessage generates a commit message using Claude.
func generateCommitMessage(accessToken, status, diff, log string, fileStats []git.FileChange, userInput string) (string, error) {
	const (
		maxPromptChars = 500000
		promptOverhead = 2000
	)

	totalSize := len(status) + len(diff) + len(log) + promptOverhead

	var smartDiff string
	if totalSize > maxPromptChars {
		smartDiff = buildSmartDiff(fileStats, diff, maxPromptChars-len(status)-len(log)-promptOverhead)
	} else {
		smartDiff = diff
	}

	hasSmartDiff := len(fileStats) > 0 && strings.Contains(smartDiff, "Changed Files Summary:")

	contextNote := ""
	if hasSmartDiff {
		contextNote = "\n(Note: Due to large changeset, detailed diffs shown for selected files only. Use summary above for full picture.)\n"
	}

	userInputSection := ""
	if userInput != "" {
		userInputSection = fmt.Sprintf(`

User Input:
`+"```"+`
%s
`+"```"+`
`, userInput)
	}

	prompt := fmt.Sprintf(`Analyze the following git repository state and generate a concise commit message.

Git Status:
`+"```"+`
%s
`+"```"+`

Git Diff:
`+"```"+`
%s%s
`+"```"+`

Recent Commits (for style reference):
`+"```"+`
%s
`+"```"+`%s

IMPORTANT: Your entire response must be ONLY the commit message text itself.
Do NOT include:
- Any analysis or explanation
- Prefixes like "Claude:", "Here's", "Based on"
- Phrases like "I'll analyze" or "my suggested commit message is"
- Signatures or attributions

Write a commit message that:
1. Summarizes the changes concisely (1-2 sentences)
2. Focuses on WHY rather than WHAT
3. Follows the style of recent commits shown above

Start your response directly with the commit message text.`, status, smartDiff, contextNote, log, userInputSection)

	return client.Ask(accessToken, prompt)
}

// buildSmartDiff creates an intelligent diff when the full diff is too large.
func buildSmartDiff(fileStats []git.FileChange, fullDiff string, budget int) string {
	if len(fileStats) == 0 {
		return fullDiff
	}

	var result strings.Builder

	result.WriteString("Changed Files Summary:\n")

	for _, stat := range fileStats {
		result.WriteString(fmt.Sprintf("  %s: +%d -%d lines\n", stat.Path, stat.Added, stat.Removed))
	}

	result.WriteString("\n")

	summarySize := result.Len()

	// Sort files by size
	sorted := make([]git.FileChange, len(fileStats))
	copy(sorted, fileStats)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Added+sorted[i].Removed > sorted[j].Added+sorted[j].Removed {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var (
		selectedPaths []string
		excludedPaths []string
	)

	usedBudget := summarySize

	for _, stat := range sorted {
		estimatedSize := (stat.Added + stat.Removed) * 5
		if usedBudget+estimatedSize > budget {
			excludedPaths = append(excludedPaths, stat.Path)
			continue
		}

		selectedPaths = append(selectedPaths, stat.Path)
		usedBudget += estimatedSize
	}

	if len(selectedPaths) > 0 {
		result.WriteString("Detailed Diffs (selected files):\n\n")

		selectedDiff, err := git.DiffFiles(selectedPaths)
		if err == nil {
			result.WriteString(selectedDiff)
		}
	}

	if len(excludedPaths) > 0 {
		result.WriteString(fmt.Sprintf("\n[Note: Diffs excluded for %d large files: %s]\n",
			len(excludedPaths), strings.Join(excludedPaths, ", ")))
	}

	return result.String()
}
