package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gic/internal/auth"
	"gic/internal/commit"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	tokenPath := filepath.Join(configDir, "gic", "tokens.json")

	// Try to load existing token
	token, err := auth.Load(tokenPath)
	if err != nil || token == nil {
		// No token found, run OAuth flow
		fmt.Println("🔐 No authentication token found. Starting OAuth flow...")
		fmt.Println()

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
	return commit.Run(token.AccessToken)
}

func performOAuthFlow(tokenPath string) (*auth.Token, error) {
	// Use claude.ai OAuth (Pro/Max)
	authURL, verifier, err := auth.BuildAuthURL(false)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Please visit this URL to authorize:\n\n%s\n\n", authURL)
	fmt.Print("Paste the full code here (format: code#state): ")

	var authCode string
	if _, err := fmt.Scanln(&authCode); err != nil {
		return nil, fmt.Errorf("failed to read code: %w", err)
	}

	token, err := auth.ExchangeCode(authCode, verifier)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	if err := auth.Save(token, tokenPath); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Authorization successful!")
	fmt.Println()

	return token, nil
}
