package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gic/internal/auth"
	"gic/internal/commit"
	"gic/internal/mcp"

	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

// version metadata is injected via ldflags; defaults cover local builds.
var (
	version   = "dev"
	buildTime = "unknown"
)

var (
	showVersion bool

	rootCmd = &cobra.Command{
		Use:           "gic [commit-message]",
		Short:         "Generate polished git commits with AI assistance",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				printVersion()
				return nil
			}

			userInput := strings.Join(args, " ")

			return run(userInput)
		},
	}

	mcpCmd = &cobra.Command{
		Use:           "mcp",
		Short:         "Start the MCP server for Claude Desktop integration",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				printVersion()
				return nil
			}

			return runMCP()
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
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show version information")
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(versionCmd)
}

func printVersion() {
	tap.Intro("📦 gic")

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
			FormatBorder:   tap.CyanBorder,
		},
	)

	tap.Outro("Run `gic` without flags to launch the assistant ✨")
}

func run(userInput string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	tokenPath := filepath.Join(configDir, "gic", "tokens.json")

	// Try to load existing token
	token, err := auth.Load(tokenPath)
	if err != nil || token == nil {
		// No token found, run OAuth flow
		tap.Intro("🔐 Authentication Required")

		token, err = performOAuthFlow(tokenPath)
		if err != nil {
			return fmt.Errorf("oauth flow failed: %w", err)
		}
	}

	// Ensure token is valid (refresh if needed)
	token, err = auth.EnsureValid(token, tokenPath, auth.ClientID, auth.TokenURL)
	if err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	// Run commit workflow
	return commit.Run(token.AccessToken, userInput)
}

func performOAuthFlow(tokenPath string) (*auth.Token, error) {
	ctx := context.Background()

	// Use claude.ai OAuth (Pro/Max)
	authURL, verifier, err := auth.BuildAuthURL(false)
	if err != nil {
		return nil, err
	}

	tap.Message("Please visit this URL to authorize:")
	tap.Box(authURL, "Authorization URL", tap.BoxOptions{
		TitleAlign:   tap.BoxAlignLeft,
		ContentAlign: tap.BoxAlignLeft,
		Rounded:      true,
	})

	authCode := tap.Text(ctx, tap.TextOptions{
		Message:     "Paste the authorization code here:",
		Placeholder: "code#state",
		Validate: func(s string) error {
			if !strings.Contains(s, "#") {
				return fmt.Errorf("code should be in format: code#state")
			}

			return nil
		},
	})

	if authCode == "" {
		return nil, fmt.Errorf("authorization cancelled")
	}

	sp := tap.NewSpinner(tap.SpinnerOptions{Indicator: "dots"})
	sp.Start("Exchanging authorization code for token...")

	token, err := auth.ExchangeCode(authCode, verifier)
	if err != nil {
		sp.Stop("Failed to exchange code", 2)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	if err := auth.Save(token, tokenPath); err != nil {
		sp.Stop("Failed to save token", 2)
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	sp.Stop("Authorization successful!", 0)
	tap.Outro("You're all set! 🎉")

	return token, nil
}

func runMCP() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	tokenPath := filepath.Join(configDir, "gic", "tokens.json")

	// Try to load existing token
	token, err := auth.Load(tokenPath)
	if err != nil || token == nil {
		return fmt.Errorf("authentication required: please run 'gic' first to authenticate")
	}

	// Ensure token is valid (refresh if needed)
	token, err = auth.EnsureValid(token, tokenPath, auth.ClientID, auth.TokenURL)
	if err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	// Create and run MCP server
	server := mcp.NewServer(token.AccessToken, tokenPath)

	return server.Run(context.Background())
}
