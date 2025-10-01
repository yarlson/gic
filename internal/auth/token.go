package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Token represents an OAuth token with expiration info.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// Load reads a token from disk.
func Load(path string) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// Save writes a token to disk.
func Save(token *Token, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// IsValid checks if the token is still valid (with 1 minute buffer).
func (t *Token) IsValid() bool {
	return time.Now().Unix() < t.ExpiresAt-60
}

// Refresh refreshes an expired token.
func Refresh(token *Token, clientID, tokenURL string) (*Token, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     clientID,
		"refresh_token": token.RefreshToken,
	}

	resp, err := post(tokenURL, payload)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token refresh failed: %s", resp.Status)
	}

	var newToken Token
	if err := json.NewDecoder(resp.Body).Decode(&newToken); err != nil {
		return nil, err
	}

	newToken.ExpiresAt = time.Now().Unix() + int64(newToken.ExpiresIn)

	return &newToken, nil
}

// EnsureValid ensures a token is valid, refreshing if necessary.
func EnsureValid(token *Token, path, clientID, tokenURL string) (*Token, error) {
	if token.IsValid() {
		return token, nil
	}

	fmt.Println("Token expired, refreshing...")

	newToken, err := Refresh(token, clientID, tokenURL)
	if err != nil {
		return nil, err
	}

	if err := Save(newToken, path); err != nil {
		return nil, err
	}

	return newToken, nil
}
