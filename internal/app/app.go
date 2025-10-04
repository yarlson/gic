package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"gic/internal/commit"
	"gic/internal/git"

	"github.com/yarlson/tap"
)

// Run executes the commit workflow.
func Run(accessToken, userInput string, autoApprove bool) error {
	ctx := context.Background()

	tap.Intro("🤖 Git Commit Assistant")

	// Step 1: Stage all changes first
	if err := git.Add("."); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

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

	// Check if there are any changes to commit
	if diff == "" || strings.TrimSpace(diff) == "" {
		tap.Outro("No changes to commit")
		return nil
	}

	// Show status in a box (clean up each line)
	tap.Box(commit.CleanStatus(status), "📝 Repository Status", tap.BoxOptions{
		TitleAlign:     tap.BoxAlignLeft,
		ContentAlign:   tap.BoxAlignLeft,
		TitlePadding:   1,
		ContentPadding: 1,
		Rounded:        true,
		IncludePrefix:  true,
		FormatBorder:   tap.GrayBorder,
	})

	// Step 3: Check if we need smart diff selection
	totalSize := len(status) + len(diff) + len(log) + commit.PromptOverhead

	var smartDiff string

	if totalSize > commit.MaxPromptChars {
		tap.Message("⚠️  Large changeset detected, selecting most relevant files...")

		smartDiff = commit.BuildSmartDiff(fileStats, diff, commit.MaxPromptChars-len(status)-len(log)-commit.PromptOverhead)
	} else {
		smartDiff = diff
	}

	// Step 4: Generate commit message with Claude
	sp := tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Generating commit message with Claude")

	commitMsg, err := commit.GenerateMessage(accessToken, status, smartDiff, log, fileStats, userInput)
	if err != nil {
		sp.Stop("Failed to generate commit message", 2)
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	sp.Stop("Commit message generated               ", 0)

	// Show proposed commit message
	tap.Box(commitMsg, "📋 Proposed Commit Message", tap.BoxOptions{
		TitleAlign:     tap.BoxAlignLeft,
		ContentAlign:   tap.BoxAlignLeft,
		TitlePadding:   1,
		ContentPadding: 1,
		Rounded:        true,
		IncludePrefix:  true,
		FormatBorder:   tap.GrayBorder,
	})

	// Step 5: Ask for confirmation unless auto-approval requested
	proceed := true

	if autoApprove {
		tap.Message("Auto-approve enabled; skipping confirmation prompt")
	} else {
		proceed = tap.Confirm(ctx, tap.ConfirmOptions{
			Message:      "Proceed with commit?",
			Active:       "Yes",
			Inactive:     "No",
			InitialValue: true,
		})
	}

	if !proceed {
		tap.Message("Commit cancelled")
		return fmt.Errorf("commit cancelled")
	}

	// Step 6: Create commit
	sp = tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Creating commit")

	if err := git.Commit(commitMsg); err != nil {
		sp.Stop("Failed to create commit", 2)
		return fmt.Errorf("failed to create commit: %w", err)
	}

	sp.Stop("Commit created!", 0)
	tap.Outro("All done!")

	return nil
}
