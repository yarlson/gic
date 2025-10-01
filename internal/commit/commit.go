package commit

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gic/internal/client"
	"gic/internal/git"

	"github.com/yarlson/tap"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

const (
	// Conservative limit: ~125K tokens (500K chars ≈ 125K tokens at 4 chars/token)
	// Leaves room for system prompt + response
	maxPromptChars = 500000
	// Reserve space for prompt template overhead (~2K chars)
	promptOverhead = 2000
)

// Run executes the commit workflow.
func Run(accessToken string) error {
	ctx := context.Background()

	// Step 1: Stage all changes first
	sp := tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Staging all changes...")

	if err := git.Add("."); err != nil {
		sp.Stop("Failed to stage changes", 2)
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	sp.Stop("Changes staged", 0)

	// Step 2: Gather git information in parallel
	sp = tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Analyzing repository changes...")

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

		s, err := git.Status()
		if err != nil {
			mu.Lock()

			errs = append(errs, fmt.Errorf("git status failed: %w", err))

			mu.Unlock()

			return
		}

		status = s
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
		sp.Stop("Analysis failed", 2)
		return errs[0]
	}

	sp.Stop("Analysis complete", 0)

	// Check if there are any changes to commit
	if diff == "" || strings.TrimSpace(diff) == "" {
		tap.Message("No changes to commit")
		return nil
	}

	// Show status in a box (clean up each line)
	tap.Box(fmt.Sprintf("\n%s", cleanStatus(status)), " 📝 Repository Status ", tap.BoxOptions{
		TitleAlign:     tap.BoxAlignLeft,
		ContentAlign:   tap.BoxAlignLeft,
		ContentPadding: 1,
		Rounded:        true,
		FormatBorder:   tap.CyanBorder,
	})

	// Step 3: Check if we need smart diff selection
	totalSize := len(status) + len(diff) + len(log) + promptOverhead

	var smartDiff string

	if totalSize > maxPromptChars {
		tap.Message("⚠️  Large changeset detected, selecting most relevant files...")

		smartDiff = buildSmartDiff(fileStats, diff, maxPromptChars-len(status)-len(log)-promptOverhead)
	} else {
		smartDiff = diff
	}

	// Step 4: Generate commit message with Claude
	sp = tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Generating commit message with Claude...")

	commitMsg, err := generateCommitMessage(accessToken, status, smartDiff, log, fileStats)
	if err != nil {
		sp.Stop("Failed to generate commit message", 2)
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	sp.Stop("Commit message generated", 0)

	// Show proposed commit message
	tap.Box(fmt.Sprintf("\n%s\n", commitMsg), " 📋 Proposed Commit Message ", tap.BoxOptions{
		TitleAlign:     tap.BoxAlignLeft,
		ContentAlign:   tap.BoxAlignLeft,
		ContentPadding: 1,
		Rounded:        true,
		FormatBorder:   tap.CyanBorder,
	})

	// Step 5: Ask for confirmation
	proceed := tap.Confirm(ctx, tap.ConfirmOptions{
		Message:      "Proceed with commit?",
		Active:       "Yes",
		Inactive:     "No",
		InitialValue: true,
	})

	if !proceed {
		return fmt.Errorf("commit cancelled")
	}

	// Step 6: Create commit
	sp = tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Creating commit...")

	if err := git.Commit(commitMsg); err != nil {
		sp.Stop("Failed to create commit", 2)
		return fmt.Errorf("failed to create commit: %w", err)
	}

	sp.Stop("Commit created!", 0)
	tap.Outro("All done! ✅")

	return nil
}

// cleanStatus strips ANSI codes and trailing whitespace from each line.
func cleanStatus(s string) string {
	var result strings.Builder
	for _, line := range strings.Split(s, "\n") {
		// Strip ANSI codes
		cleaned := ansiRegex.ReplaceAllString(line, "")
		// Trim trailing whitespace
		cleaned = strings.TrimRight(cleaned, " \t\r")
		result.WriteString(cleaned)
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// buildSmartDiff creates an intelligent diff when the full diff is too large.
func buildSmartDiff(fileStats []git.FileChange, fullDiff string, budget int) string {
	if len(fileStats) == 0 {
		return fullDiff
	}

	var result strings.Builder

	// Write summary header with all files
	result.WriteString("Changed Files Summary:\n")

	for _, stat := range fileStats {
		result.WriteString(fmt.Sprintf("  %s: +%d -%d lines\n", stat.Path, stat.Added, stat.Removed))
	}

	result.WriteString("\n")

	summarySize := result.Len()

	// Sort files by total changes (smallest first - more signal, less noise)
	sorted := make([]git.FileChange, len(fileStats))
	copy(sorted, fileStats)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Added+sorted[i].Removed < sorted[j].Added+sorted[j].Removed
	})

	// Select files that fit within budget
	var (
		selectedPaths []string
		excludedPaths []string
	)

	usedBudget := summarySize

	for _, stat := range sorted {
		// Estimate size per file (rough: ~5 chars per line change for context)
		estimatedSize := (stat.Added + stat.Removed) * 5

		if usedBudget+estimatedSize > budget {
			excludedPaths = append(excludedPaths, stat.Path)

			continue
		}

		selectedPaths = append(selectedPaths, stat.Path)
		usedBudget += estimatedSize
	}

	// Get diff for selected files only
	if len(selectedPaths) > 0 {
		result.WriteString("Detailed Diffs (selected files):\n\n")

		selectedDiff, err := git.DiffFiles(selectedPaths)
		if err == nil {
			result.WriteString(selectedDiff)
		}
	}

	// Note excluded files
	if len(excludedPaths) > 0 {
		result.WriteString(fmt.Sprintf("\n[Note: Diffs excluded for %d large files: %s]\n",
			len(excludedPaths), strings.Join(excludedPaths, ", ")))
	}

	return result.String()
}

// generateCommitMessage uses Claude to generate a commit message.
func generateCommitMessage(accessToken, status, diff, log string, fileStats []git.FileChange) (string, error) {
	// Check if we have file stats and diff looks like our smart diff
	hasSmartDiff := len(fileStats) > 0 && strings.Contains(diff, "Changed Files Summary:")

	contextNote := ""
	if hasSmartDiff {
		contextNote = "\n(Note: Due to large changeset, detailed diffs shown for selected files only. Use summary above for full picture.)\n"
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
`+"```"+`

IMPORTANT: Your entire response must be ONLY the commit message text itself.
Do NOT include:
- Any analysis or explanation
- Prefixes like "Claude:", "Here's", "Based on"
- Phrases like "I'll analyze" or "my suggested commit message is"
- Signatures or attributions

Write a commit message that:
1. Summarizes the changes concisely (1-2 sentences)
2. Focuses on WHY rather than WHAT

Start your response directly with the commit message text.`, status, diff, contextNote, log)

	return client.Ask(accessToken, prompt)
}
