package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cmt/internal/app"
	"cmt/internal/git"
	"cmt/internal/provider"

	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

// version metadata is injected via ldflags; defaults cover local builds.
var (
	version   = "dev"
	buildTime = "unknown"
)

var (
	showVersion  bool
	autoApprove  bool
	messageOnly  bool
	modelFlag    string
	providerFlag string

	rootCmd = &cobra.Command{
		Use:           "cmt [commit-message]",
		Short:         "Generate polished git commits with AI assistance",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				printVersion()
				return nil
			}

			if messageOnly && autoApprove {
				return fmt.Errorf("--message-only cannot be combined with --auto-approve")
			}

			userInput := strings.Join(args, " ")

			return run(cmd.Context(), cmd, userInput, messageOnly)
		},
	}

	versionCmd = &cobra.Command{
		Use:           "version",
		Short:         "Show build metadata",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}
)

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show version information")
	rootCmd.Flags().BoolVarP(&autoApprove, "auto-approve", "y", false, "Skip confirmation prompt and create the commit automatically")
	rootCmd.Flags().BoolVar(&messageOnly, "message-only", false, "Generate a commit message from staged changes without creating a commit")
	rootCmd.Flags().StringVar(&providerFlag, "provider", "", "Provider to use (`claude` or `codex`)")
	rootCmd.Flags().StringVar(&modelFlag, "model", "", "Model name to use (defaults depend on the selected provider)")
	rootCmd.AddCommand(versionCmd)
}

func printVersion() {
	tap.Intro("📦 cmt")

	tap.Box(
		fmt.Sprintf("Version:    %s\nBuilt:      %s", version, buildTime),
		"Build Details",
		tap.BoxOptions{
			TitleAlign:     tap.BoxAlignLeft,
			ContentAlign:   tap.BoxAlignLeft,
			TitlePadding:   1,
			ContentPadding: 1,
			Rounded:        true,
			IncludePrefix:  true,
			FormatBorder:   tap.GrayBorder,
		},
	)

	tap.Outro("Run `cmt` without flags to launch the assistant ✨")
}

func run(ctx context.Context, cmd *cobra.Command, userInput string, messageOnly bool) error {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("required executable `git` not found in $PATH: %w", err)
	}

	providerID := resolveOption(cmd, "provider", "CMT_PROVIDER", provider.DefaultProviderID)

	definition, err := provider.Lookup(providerID)
	if err != nil {
		return err
	}

	executablePath, err := exec.LookPath(definition.ExecutableName)
	if err != nil {
		return fmt.Errorf("required executable `%s` not found in $PATH: %w", definition.ExecutableName, err)
	}

	if err := provider.Preflight(ctx, definition, executablePath); err != nil {
		return err
	}

	explicitModel := resolveOption(cmd, "model", "CMT_MODEL", "")

	effectiveModel, err := definition.ResolveModel(ctx, executablePath, explicitModel)
	if err != nil {
		return err
	}

	repoDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine current directory: %w", err)
	}

	adapter := definition.NewAdapter(executablePath, effectiveModel)
	deps := app.Dependencies{
		Git:      git.NewClient(repoDir, gitPath),
		Provider: adapter,
	}

	if messageOnly {
		message, err := app.GenerateMessage(ctx, deps, userInput)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
			return fmt.Errorf("failed to write commit message: %w", err)
		}

		return nil
	}

	return app.Run(ctx, deps, userInput, autoApprove)
}

func resolveOption(cmd *cobra.Command, flagName, envName, fallback string) string {
	if cmd.Flags().Changed(flagName) {
		if trimmed := strings.TrimSpace(flagValue(cmd, flagName)); trimmed != "" {
			return trimmed
		}

		return fallback
	}

	if envValue, ok := os.LookupEnv(envName); ok {
		trimmed := strings.TrimSpace(envValue)
		if trimmed != "" {
			return trimmed
		}
	}

	return fallback
}

func flagValue(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}

	return value
}
