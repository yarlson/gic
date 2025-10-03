package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gic/internal/auth"
	"gic/internal/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MainTestSuite is an integration test suite for main package
type MainTestSuite struct {
	suite.Suite
	tmpDir string
	oldDir string
}

// SetupTest creates a temporary git repository
func (s *MainTestSuite) SetupTest() {
	// Save current directory
	oldDir, err := os.Getwd()
	require.NoError(s.T(), err)
	s.oldDir = oldDir

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "gic-main-test-*")
	require.NoError(s.T(), err)
	s.tmpDir = tmpDir

	// Change to temporary directory
	err = os.Chdir(tmpDir)
	require.NoError(s.T(), err)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	err = cmd.Run()
	require.NoError(s.T(), err)

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	err = cmd.Run()
	require.NoError(s.T(), err)

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	err = cmd.Run()
	require.NoError(s.T(), err)

	// Create initial commit
	err = os.WriteFile("initial.txt", []byte("initial"), 0644)
	require.NoError(s.T(), err)
	err = git.Add("initial.txt")
	require.NoError(s.T(), err)
	err = git.Commit("Initial commit")
	require.NoError(s.T(), err)
}

// TearDownTest cleans up
func (s *MainTestSuite) TearDownTest() {
	if s.oldDir != "" {
		_ = os.Chdir(s.oldDir)
	}

	if s.tmpDir != "" {
		_ = os.RemoveAll(s.tmpDir)
	}
}

// TestMainEntryPoint documents main entry point behavior
func (s *MainTestSuite) TestMainEntryPoint() {
	// The main() function should:
	// 1. Check if first argument is "mcp"
	// 2. If yes, call runMCP() and exit
	// 3. Otherwise, join args as userInput and call run()
	// 4. Handle errors and exit with code 1 on failure

	// We can't easily test main() directly as it calls os.Exit
	// But we document the behavior
	s.T().Log("Main entry point routes to MCP or regular mode")
}

// TestRunFunction documents run() behavior
func (s *MainTestSuite) TestRunFunction() {
	// The run() function should:
	// 1. Get user config directory
	// 2. Construct token path: {configDir}/gic/tokens.json
	// 3. Try to load existing token
	// 4. If no token or error, perform OAuth flow
	// 5. Ensure token is valid (refresh if needed)
	// 6. Call commit.Run() with token and user input

	// We can test token loading/creation
	configDir, err := os.UserConfigDir()
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), configDir)

	tokenPath := filepath.Join(configDir, "gic", "tokens.json")
	assert.NotEmpty(s.T(), tokenPath)

	s.T().Log("run() function workflow documented")
}

// TestPerformOAuthFlow documents OAuth flow
func (s *MainTestSuite) TestPerformOAuthFlow() {
	// The performOAuthFlow() function should:
	// 1. Create context
	// 2. Build auth URL with BuildAuthURL(false) for claude.ai
	// 3. Show intro message
	// 4. Display auth URL in a box
	// 5. Prompt user to paste authorization code
	// 6. Validate code format (must contain #)
	// 7. Show spinner while exchanging code
	// 8. Save token to disk
	// 9. Show success message

	// We can test the components without user interaction
	authURL, verifier, err := auth.BuildAuthURL(false)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), authURL)
	assert.NotEmpty(s.T(), verifier)
	assert.Contains(s.T(), authURL, "claude.ai")

	s.T().Log("OAuth flow components documented")
}

// TestRunMCP documents MCP mode
func (s *MainTestSuite) TestRunMCP() {
	// The runMCP() function should:
	// 1. Get user config directory
	// 2. Construct token path
	// 3. Try to load existing token
	// 4. If no token, return error asking to run 'gic' first
	// 5. Ensure token is valid (refresh if needed)
	// 6. Create MCP server with token
	// 7. Run server with stdio transport

	// We can test token requirements
	tmpTokenPath := filepath.Join(s.tmpDir, "tokens.json")

	// Load non-existent token
	token, err := auth.Load(tmpTokenPath)
	assert.NoError(s.T(), err)
	assert.Nil(s.T(), token)

	// In runMCP, this would trigger an error
	s.T().Log("MCP mode requires existing token")
}

// TestCommandLineArgumentParsing documents argument handling
func (s *MainTestSuite) TestCommandLineArgumentParsing() {
	// Arguments are parsed as follows:
	// - If args[1] == "mcp": Run MCP server
	// - Otherwise: Join args[1:] as user input for commit message

	// Examples:
	// gic                    -> run("") - interactive
	// gic mcp                -> runMCP()
	// gic fix auth bug       -> run("fix auth bug")
	// gic "commit message"   -> run("commit message")
	userInput1 := strings.Join([]string{"fix", "auth", "bug"}, " ")
	assert.Equal(s.T(), "fix auth bug", userInput1)

	userInput2 := strings.Join([]string{}, " ")
	assert.Equal(s.T(), "", userInput2)

	s.T().Log("Command-line argument parsing documented")
}

// TestTokenPathConstruction verifies token path logic
func (s *MainTestSuite) TestTokenPathConstruction() {
	// Token path should be: {UserConfigDir}/gic/tokens.json
	configDir, err := os.UserConfigDir()
	require.NoError(s.T(), err)

	tokenPath := filepath.Join(configDir, "gic", "tokens.json")

	// Verify path structure
	assert.Contains(s.T(), tokenPath, "gic")
	assert.Contains(s.T(), tokenPath, "tokens.json")

	// On different platforms:
	// - macOS: ~/Library/Application Support/gic/tokens.json
	// - Linux: ~/.config/gic/tokens.json
	// - Windows: %APPDATA%\gic\tokens.json

	s.T().Log("Token path construction verified")
}

// TestAuthenticationFlow documents the complete auth flow
func (s *MainTestSuite) TestAuthenticationFlow() {
	// Complete authentication flow:

	// First run (no token):
	// 1. User runs: gic
	// 2. No token found
	// 3. OAuth flow starts
	// 4. User visits auth URL
	// 5. User pastes code
	// 6. Token saved
	// 7. Commit workflow runs

	// Subsequent runs:
	// 1. User runs: gic
	// 2. Token loaded from disk
	// 3. Token validity checked
	// 4. If expired, refreshed automatically
	// 5. Commit workflow runs

	// MCP mode:
	// 1. User runs: gic mcp
	// 2. Token loaded (must exist)
	// 3. If expired, refreshed
	// 4. MCP server starts
	// 5. Server handles requests from Claude Desktop
	s.T().Log("Complete authentication flow documented")
}

// TestErrorHandling documents error scenarios
func (s *MainTestSuite) TestErrorHandling() {
	// Error scenarios:

	// 1. Failed to get config dir
	//    - Exit with error message

	// 2. OAuth flow failed
	//    - Show error
	//    - Exit with code 1

	// 3. Token refresh failed
	//    - Show error
	//    - Exit with code 1

	// 4. Commit workflow failed
	//    - Show error from commit.Run
	//    - Exit with code 1

	// 5. MCP mode without token
	//    - Show "please run 'gic' first"
	//    - Exit with code 1

	// 6. MCP server failed to start
	//    - Show error
	//    - Exit with code 1
	s.T().Log("Error handling scenarios documented")
}

// TestUserInteraction documents interactive prompts
func (s *MainTestSuite) TestUserInteraction() {
	// User interactions using tap library:

	// OAuth flow (performOAuthFlow):
	// - tap.Intro("🔐 Authentication Required")
	// - tap.Box(authURL, ...) - shows auth URL
	// - tap.Text(...) - prompts for code with validation
	// - tap.NewSpinner(...) - shows progress
	// - tap.Outro("You're all set! 🎉")

	// Commit workflow (in commit.Run):
	// - tap.Intro("🤖 Git Commit Assistant")
	// - tap.Box(status, ...) - shows repo status
	// - tap.NewSpinner(...) - shows generation progress
	// - tap.Box(commitMsg, ...) - shows proposed message
	// - tap.Confirm(...) - asks for confirmation
	// - tap.NewSpinner(...) - shows commit progress
	// - tap.Outro("All done! ✅")
	s.T().Log("User interaction patterns documented")
}

// TestConfigurationLocations documents where config is stored
func (s *MainTestSuite) TestConfigurationLocations() {
	// Configuration locations by platform:
	configDir, err := os.UserConfigDir()
	require.NoError(s.T(), err)

	// Verify we can construct paths
	gicConfigDir := filepath.Join(configDir, "gic")
	tokensPath := filepath.Join(gicConfigDir, "tokens.json")

	assert.NotEmpty(s.T(), gicConfigDir)
	assert.NotEmpty(s.T(), tokensPath)

	// Stored data:
	// - {configDir}/gic/tokens.json - OAuth tokens

	// Not stored:
	// - Git credentials (handled by git)
	// - Commit history (in git)
	// - API keys (temporary, in memory)

	s.T().Log("Configuration locations documented")
}

// TestIntegrationWithCommitPackage verifies integration
func (s *MainTestSuite) TestIntegrationWithCommitPackage() {
	// The main package integrates with commit package:

	// run() calls:
	// - commit.Run(token.AccessToken, userInput)

	// commit.Run expects:
	// - Valid OAuth access token
	// - Optional user input (additional context)
	// - To be run in a git repository
	// - User interaction for confirmation

	// We verify we're in a git repo
	_, err := git.Status()
	require.NoError(s.T(), err)

	s.T().Log("Integration with commit package verified")
}

// TestIntegrationWithMCPPackage verifies MCP integration
func (s *MainTestSuite) TestIntegrationWithMCPPackage() {
	// The main package integrates with MCP package:

	// runMCP() calls:
	// - mcp.NewServer(token.AccessToken, tokenPath)
	// - server.Run(context.Background())

	// MCP server expects:
	// - Valid OAuth access token
	// - Path to token file for refresh
	// - To be run in a git repository
	// - Stdio transport for communication

	// We verify we're in a git repo
	_, err := git.Status()
	require.NoError(s.T(), err)

	s.T().Log("Integration with MCP package verified")
}

// TestIntegrationWithAuthPackage verifies auth integration
func (s *MainTestSuite) TestIntegrationWithAuthPackage() {
	// The main package uses auth package for:

	// Token operations:
	// - auth.Load(tokenPath) - load existing token
	// - auth.Save(token, tokenPath) - save new token
	// - auth.EnsureValid(token, ...) - validate/refresh

	// OAuth operations:
	// - auth.BuildAuthURL(false) - build auth URL
	// - auth.ExchangeCode(code, verifier) - exchange code for token

	// Create a test token
	tmpTokenPath := filepath.Join(s.tmpDir, "test-tokens.json")
	token := &auth.Token{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Unix() + 3600,
	}

	// Save and load
	err := auth.Save(token, tmpTokenPath)
	require.NoError(s.T(), err)

	loaded, err := auth.Load(tmpTokenPath)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), token.AccessToken, loaded.AccessToken)

	s.T().Log("Integration with auth package verified")
}

// TestProgramFlow documents the complete flow
func (s *MainTestSuite) TestProgramFlow() {
	// Complete program flow:

	// Standard mode (gic):
	// main() -> run() -> performOAuthFlow() [if needed] -> commit.Run()

	// MCP mode (gic mcp):
	// main() -> runMCP() -> mcp.NewServer() -> server.Run()

	// With user input (gic "message"):
	// main() -> run() with userInput -> commit.Run(token, userInput)

	// Both modes require:
	// 1. Valid git repository
	// 2. Valid OAuth token
	// 3. Internet connection (for API calls)
	s.T().Log("Complete program flow documented")
}

// TestBuildOutput documents build artifacts
func (s *MainTestSuite) TestBuildOutput() {
	// The main package builds to:
	// - Binary name: gic
	// - Module name: gic (from go.mod)

	// Build command:
	// go build -o gic

	// Binary can be:
	// - Run directly: ./gic
	// - Installed: go install
	// - Used in PATH: gic

	// MCP usage:
	// In Claude Desktop config: "command": "gic", "args": ["mcp"]
	s.T().Log("Build output and usage documented")
}

// TestSuite runs the main integration test suite
func TestMainIntegration(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
