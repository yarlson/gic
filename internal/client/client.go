package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const apiKeyEndpoint = "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"

// CreateAPIKey creates an API key from an OAuth token.
func CreateAPIKey(accessToken string) (string, error) {
	fmt.Println("\nCreating API key from OAuth token...")

	req, err := http.NewRequest("POST", apiKeyEndpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create API key: %s - %s", resp.Status, string(body))
	}

	var result struct {
		RawKey string `json:"raw_key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	fmt.Println("✓ API key created successfully")

	return result.RawKey, nil
}

// TestWithAPIKey tests the API with an API key.
func TestWithAPIKey(apiKey string) error {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	fmt.Println("\nTesting API call with API key...")

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-3-7-sonnet-20250219",
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Say hello and confirm you're Claude!")),
		},
	})
	if err != nil {
		return fmt.Errorf("API call failed: %w", err)
	}

	fmt.Println("\n✓ API Response:")

	for _, block := range message.Content {
		fmt.Println(block.Text)
	}

	return nil
}

// TestWithOAuth tests the API with an OAuth token.
func TestWithOAuth(accessToken string) error {
	httpClient := &http.Client{
		Transport: &oauthTransport{token: accessToken},
	}

	client := anthropic.NewClient(option.WithHTTPClient(httpClient))

	fmt.Println("\nTesting API call with OAuth token...")

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-3-7-sonnet-20250219",
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: "You are Claude Code, Anthropic's official CLI for Claude.",
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Say hello and confirm you're Claude!")),
		},
	})
	if err != nil {
		return fmt.Errorf("API call failed: %w", err)
	}

	fmt.Println("\n✓ API Response:")

	for _, block := range message.Content {
		fmt.Println(block.Text)
	}

	return nil
}

// Ask sends a prompt to Claude and returns the response text.
func Ask(accessToken, prompt string) (string, error) {
	httpClient := &http.Client{
		Transport: &oauthTransport{token: accessToken},
	}

	client := anthropic.NewClient(option.WithHTTPClient(httpClient))

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-3-7-sonnet-20250219",
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: "You are Claude Code, Anthropic's official CLI for Claude.",
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}

	var response string
	for _, block := range message.Content {
		response += block.Text
	}

	return response, nil
}

// oauthTransport implements http.RoundTripper to add OAuth headers.
type oauthTransport struct {
	token string
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	req.Header.Del("x-api-key")
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	return http.DefaultTransport.RoundTrip(req)
}
