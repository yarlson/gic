package commit

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"gic/internal/client"
	"gic/internal/git"
)

const (
	// Conservative limit: ~125K tokens (500K chars ≈ 125K tokens at 4 chars/token)
	// Leaves room for system prompt + response
	maxPromptChars = 500000
	// Reserve space for prompt template overhead (~2K chars)
	promptOverhead = 2000
)

// Run executes the commit workflow.
func Run(accessToken string) error {
	// Step 1: Stage all changes first
	fmt.Println("📦 Staging all changes...")

	if err := git.Add("."); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	fmt.Println("🔍 Analyzing repository changes...")

	// Step 2: Gather git information in parallel
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
		return errs[0]
	}

	fmt.Println("📝 Status:")
	fmt.Println(status)

	// Step 3: Check if we need smart diff selection
	totalSize := len(status) + len(diff) + len(log) + promptOverhead

	var smartDiff string

	if totalSize > maxPromptChars {
		fmt.Println("\n⚠️  Large changeset detected, selecting most relevant files...")

		smartDiff = buildSmartDiff(fileStats, diff, maxPromptChars-len(status)-len(log)-promptOverhead)
	} else {
		smartDiff = diff
	}

	// Step 4: Generate commit message with Claude
	fmt.Println("\n🤖 Generating commit message...")

	commitMsg, err := generateCommitMessage(accessToken, status, smartDiff, log, fileStats)
	if err != nil {
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	fmt.Printf("\n📋 Proposed commit message:\n%s\n\n", commitMsg)

	// Step 5: Ask for confirmation
	fmt.Print("Proceed with commit? [y/N]: ")

	var proceed string
	if _, err := fmt.Scanln(&proceed); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if strings.ToLower(proceed) != "y" {
		return fmt.Errorf("commit cancelled")
	}

	// Step 6: Create commit
	fmt.Println("\n💾 Creating commit...")

	if err := git.Commit(commitMsg); err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	fmt.Println("✅ Commit created!")

	return nil
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
%s

Git Diff:
%s%s

Recent Commits (for style reference):
%s

Generate a commit message that:
1. Summarizes the changes concisely (1-2 sentences)
2. Focuses on WHY rather than WHAT
3. Follows the style of recent commits
4. Do not include signatures or AI attributions (e.g. "Claude: ", "Generated with…" or "Co-Authored-By")

Return ONLY the commit message, no explanations.`, status, diff, contextNote, log)

	return client.Ask(accessToken, prompt)
}
