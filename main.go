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

	"github.com/yarlson/tap"
)

func main() {
	// Check if first argument is "mcp" subcommand
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := runMCP(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		return
	}

	// Capture additional user input from command line args
	userInput := strings.Join(os.Args[1:], " ")

	if err := run(userInput); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
