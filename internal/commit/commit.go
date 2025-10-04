package commit

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gic/internal/client"
	"gic/internal/git"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

const (
	// Conservative limit: ~125K tokens (500K chars ≈ 125K tokens at 4 chars/token)
	// Leaves room for system prompt + response
	MaxPromptChars = 500000
	// Reserve space for prompt template overhead (~2K chars)
	PromptOverhead = 2000
)

// CleanStatus strips ANSI codes and trailing whitespace from each line.
func CleanStatus(s string) string {
	var cleanedLines []string

	for _, line := range strings.Split(s, "\n") {
		// Strip ANSI codes
		cleaned := ansiRegex.ReplaceAllString(line, "")
		// Trim trailing whitespace
		cleaned = strings.TrimRight(cleaned, " \t\r")
		if strings.Trim(cleaned, " \t\r") == "" {
			continue
		}

		cleanedLines = append(cleanedLines, cleaned)
	}

	return strings.Join(cleanedLines, "\n")
}

// BuildSmartDiff creates an intelligent diff when the full diff is too large.
func BuildSmartDiff(fileStats []git.FileChange, fullDiff string, budget int) string {
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

// GenerateMessage uses Claude to generate a commit message.
func GenerateMessage(accessToken, status, diff, log string, fileStats []git.FileChange, userInput string) (string, error) {
	// Check if we have file stats and diff looks like our smart diff
	hasSmartDiff := len(fileStats) > 0 && strings.Contains(diff, "Changed Files Summary:")

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

Start your response directly with the commit message text.`, status, diff, contextNote, log, userInputSection)

	return client.Ask(accessToken, prompt)
}
