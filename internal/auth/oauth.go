package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	ClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	TokenURL    = "https://console.anthropic.com/v1/oauth/token"
	Scope       = "org:create_api_key user:profile user:inference"
	RedirectURI = "https://console.anthropic.com/oauth/code/callback"
)

// pkce holds PKCE verifier and challenge.
type pkce struct {
	verifier  string
	challenge string
}

// generatePKCE creates a PKCE verifier and challenge pair.
func generatePKCE() (*pkce, error) {
	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		return nil, err
	}

	codeVerifier := base64.RawURLEncoding.EncodeToString(verifier)
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &pkce{
		verifier:  codeVerifier,
		challenge: codeChallenge,
	}, nil
}

// BuildAuthURL creates the OAuth authorization URL.
func BuildAuthURL(useConsole bool) (authURL, verifier string, err error) {
	p, err := generatePKCE()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate PKCE: %w", err)
	}

	baseURL := "https://claude.ai/oauth/authorize"
	if useConsole {
		baseURL = "https://console.anthropic.com/oauth/authorize"
	}

	u, _ := url.Parse(baseURL)
	params := url.Values{}
	params.Add("code", "true")
	params.Add("client_id", ClientID)
	params.Add("response_type", "code")
	params.Add("redirect_uri", RedirectURI)
	params.Add("scope", Scope)
	params.Add("state", p.verifier)
	params.Add("code_challenge", p.challenge)
	params.Add("code_challenge_method", "S256")
	u.RawQuery = params.Encode()

	return u.String(), p.verifier, nil
}

// ExchangeCode exchanges an authorization code for a token.
func ExchangeCode(authCode, verifier string) (*Token, error) {
	// Split code by # to get code and state parts
	parts := bytes.Split([]byte(authCode), []byte("#"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid code format, expected: code#state")
	}

	code := string(parts[0])
	state := string(parts[1])

	payload := map[string]string{
		"code":          code,
		"state":         state,
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"redirect_uri":  RedirectURI,
		"code_verifier": verifier,
	}

	resp, err := post(TokenURL, payload)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s - %s", resp.Status, string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	token.ExpiresAt = time.Now().Unix() + int64(token.ExpiresIn)

	return &token, nil
}

// post is a helper for making JSON POST requests.
func post(url string, payload interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	return resp, nil
}
