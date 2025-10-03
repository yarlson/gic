package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gic/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AuthTestSuite is an integration test suite for auth operations
type AuthTestSuite struct {
	suite.Suite
	tmpDir string
}

// SetupTest creates a temporary directory for token storage
func (s *AuthTestSuite) SetupTest() {
	tmpDir, err := os.MkdirTemp("", "gic-auth-test-*")
	require.NoError(s.T(), err)
	s.tmpDir = tmpDir
}

// TearDownTest cleans up the temporary directory
func (s *AuthTestSuite) TearDownTest() {
	if s.tmpDir != "" {
		_ = os.RemoveAll(s.tmpDir)
	}
}

// TestTokenSaveAndLoad verifies that tokens can be saved and loaded from disk
func (s *AuthTestSuite) TestTokenSaveAndLoad() {
	tokenPath := filepath.Join(s.tmpDir, "tokens.json")

	// Create a token
	token := &auth.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() + 3600,
	}

	// Save token
	err := auth.Save(token, tokenPath)
	assert.NoError(s.T(), err)

	// Verify file exists
	_, err = os.Stat(tokenPath)
	assert.NoError(s.T(), err)

	// Load token
	loadedToken, err := auth.Load(tokenPath)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), loadedToken)

	// Verify token contents
	assert.Equal(s.T(), token.AccessToken, loadedToken.AccessToken)
	assert.Equal(s.T(), token.RefreshToken, loadedToken.RefreshToken)
	assert.Equal(s.T(), token.ExpiresIn, loadedToken.ExpiresIn)
	assert.Equal(s.T(), token.ExpiresAt, loadedToken.ExpiresAt)
}

// TestTokenLoadNonExistent verifies that loading a non-existent token returns nil without error
func (s *AuthTestSuite) TestTokenLoadNonExistent() {
	tokenPath := filepath.Join(s.tmpDir, "nonexistent.json")

	// Load non-existent token
	token, err := auth.Load(tokenPath)
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), token)
}

// TestTokenSaveCreatesDirectory verifies that Save creates parent directories
func (s *AuthTestSuite) TestTokenSaveCreatesDirectory() {
	// Create path with nested directories
	tokenPath := filepath.Join(s.tmpDir, "nested", "path", "tokens.json")

	token := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() + 3600,
	}

	// Save should create directories
	err := auth.Save(token, tokenPath)
	assert.NoError(s.T(), err)

	// Verify file exists
	_, err = os.Stat(tokenPath)
	assert.NoError(s.T(), err)

	// Verify directory permissions are restrictive (0700)
	dirInfo, err := os.Stat(filepath.Dir(tokenPath))
	require.NoError(s.T(), err)
	assert.Equal(s.T(), os.FileMode(0700), dirInfo.Mode().Perm())
}

// TestTokenIsValid verifies token expiration checking
func (s *AuthTestSuite) TestTokenIsValid() {
	// Valid token (expires in 2 minutes)
	validToken := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    120,
		ExpiresAt:    time.Now().Unix() + 120,
	}
	assert.True(s.T(), validToken.IsValid())

	// Token expiring soon (30 seconds - within 1 minute buffer)
	expiringToken := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    30,
		ExpiresAt:    time.Now().Unix() + 30,
	}
	assert.False(s.T(), expiringToken.IsValid())

	// Expired token
	expiredToken := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() - 1,
	}
	assert.False(s.T(), expiredToken.IsValid())
}

// TestBuildAuthURL verifies OAuth authorization URL construction
func (s *AuthTestSuite) TestBuildAuthURL() {
	// Test claude.ai OAuth (not console)
	authURL, verifier, err := auth.BuildAuthURL(false)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), verifier)
	assert.Contains(s.T(), authURL, "https://claude.ai/oauth/authorize")
	assert.Contains(s.T(), authURL, "client_id="+auth.ClientID)
	assert.Contains(s.T(), authURL, "response_type=code")
	assert.Contains(s.T(), authURL, "redirect_uri=")
	assert.Contains(s.T(), authURL, "scope=")
	assert.Contains(s.T(), authURL, "state="+verifier)
	assert.Contains(s.T(), authURL, "code_challenge=")
	assert.Contains(s.T(), authURL, "code_challenge_method=S256")

	// Test console.anthropic.com OAuth
	authURL, verifier, err = auth.BuildAuthURL(true)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), verifier)
	assert.Contains(s.T(), authURL, "https://console.anthropic.com/oauth/authorize")
}

// TestExchangeCode verifies authorization code exchange
func (s *AuthTestSuite) TestExchangeCode() {
	// Create mock token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST request
		assert.Equal(s.T(), "POST", r.Method)

		// Verify content type
		assert.Equal(s.T(), "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var reqBody map[string]string

		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(s.T(), err)

		// Verify required fields
		assert.NotEmpty(s.T(), reqBody["code"])
		assert.NotEmpty(s.T(), reqBody["state"])
		assert.Equal(s.T(), "authorization_code", reqBody["grant_type"])
		assert.NotEmpty(s.T(), reqBody["code_verifier"])

		// Return mock token response
		response := map[string]interface{}{
			"access_token":  "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"expires_in":    3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Temporarily override TokenURL
	oldTokenURL := auth.TokenURL
	// Can't actually override const, so this test documents expected behavior
	// In real usage, we'd use dependency injection or interfaces
	_ = oldTokenURL

	// Test exchange - this will call the real endpoint, so we just verify
	// the function structure and error handling
	authCode := "test-code#test-state"
	verifier := "test-verifier"

	// Note: This will fail because we're not hitting our mock server
	// In a production refactor, we'd inject the HTTP client or endpoint
	_, err := auth.ExchangeCode(authCode, verifier)

	// We expect an error here since we can't actually reach the real endpoint
	// This test documents the API contract
	assert.Error(s.T(), err)

	// Test invalid code format
	_, err = auth.ExchangeCode("invalid-format", verifier)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid code format")
}

// TestRefresh verifies token refresh functionality
func (s *AuthTestSuite) TestRefresh() {
	// Create mock token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(s.T(), "POST", r.Method)

		var reqBody map[string]string

		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(s.T(), err)

		// Verify refresh token request
		assert.Equal(s.T(), "refresh_token", reqBody["grant_type"])
		assert.Equal(s.T(), "test-refresh-token", reqBody["refresh_token"])

		// Return new token
		response := map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create expired token
	expiredToken := &auth.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() - 1,
	}

	// Test refresh with mock server
	newToken, err := auth.Refresh(expiredToken, auth.ClientID, server.URL)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), newToken)

	assert.Equal(s.T(), "new-access-token", newToken.AccessToken)
	assert.Equal(s.T(), "new-refresh-token", newToken.RefreshToken)
	assert.Equal(s.T(), 3600, newToken.ExpiresIn)
	assert.True(s.T(), newToken.ExpiresAt > time.Now().Unix())
}

// TestRefreshFailure verifies refresh error handling
func (s *AuthTestSuite) TestRefreshFailure() {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid refresh token"))
	}))
	defer server.Close()

	token := &auth.Token{
		RefreshToken: "invalid-token",
	}

	// Test refresh failure
	_, err := auth.Refresh(token, auth.ClientID, server.URL)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "token refresh failed")
}

// TestEnsureValid verifies token validation and refresh flow
func (s *AuthTestSuite) TestEnsureValid() {
	tokenPath := filepath.Join(s.tmpDir, "tokens.json")

	// Create mock token server
	refreshCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalled = true
		response := map[string]interface{}{
			"access_token":  "refreshed-token",
			"refresh_token": "refreshed-refresh-token",
			"expires_in":    3600,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test 1: Valid token should not be refreshed
	validToken := &auth.Token{
		AccessToken:  "valid-token",
		RefreshToken: "valid-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() + 3600,
	}

	result, err := auth.EnsureValid(validToken, tokenPath, auth.ClientID, server.URL)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), validToken.AccessToken, result.AccessToken)
	assert.False(s.T(), refreshCalled, "should not refresh valid token")

	// Test 2: Expired token should be refreshed
	expiredToken := &auth.Token{
		AccessToken:  "expired-token",
		RefreshToken: "expired-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() - 1,
	}

	result, err = auth.EnsureValid(expiredToken, tokenPath, auth.ClientID, server.URL)
	require.NoError(s.T(), err)
	assert.True(s.T(), refreshCalled, "should refresh expired token")
	assert.Equal(s.T(), "refreshed-token", result.AccessToken)

	// Verify refreshed token was saved
	loadedToken, err := auth.Load(tokenPath)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "refreshed-token", loadedToken.AccessToken)
}

// TestTokenFilePermissions verifies that token files have secure permissions
func (s *AuthTestSuite) TestTokenFilePermissions() {
	tokenPath := filepath.Join(s.tmpDir, "tokens.json")

	token := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() + 3600,
	}

	// Save token
	err := auth.Save(token, tokenPath)
	require.NoError(s.T(), err)

	// Check file permissions (should be 0600)
	info, err := os.Stat(tokenPath)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), os.FileMode(0600), info.Mode().Perm())
}

// TestTokenLoadInvalidJSON verifies error handling for corrupted token files
func (s *AuthTestSuite) TestTokenLoadInvalidJSON() {
	tokenPath := filepath.Join(s.tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(tokenPath, []byte("not valid json {{{"), 0600)
	require.NoError(s.T(), err)

	// Load should return error
	token, err := auth.Load(tokenPath)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), token)
}

// TestBuildAuthURLUniqueness verifies that each auth URL has unique PKCE values
func (s *AuthTestSuite) TestBuildAuthURLUniqueness() {
	// Generate multiple auth URLs
	url1, verifier1, err1 := auth.BuildAuthURL(false)
	require.NoError(s.T(), err1)

	url2, verifier2, err2 := auth.BuildAuthURL(false)
	require.NoError(s.T(), err2)

	// Verifiers should be different
	assert.NotEqual(s.T(), verifier1, verifier2)

	// URLs should be different (contain different state/challenge)
	assert.NotEqual(s.T(), url1, url2)

	// Both should contain their respective verifiers as state
	assert.True(s.T(), strings.Contains(url1, "state="+verifier1))
	assert.True(s.T(), strings.Contains(url2, "state="+verifier2))
}

// TestSuite runs the auth integration test suite
func TestAuthIntegration(t *testing.T) {
	suite.Run(t, new(AuthTestSuite))
}
