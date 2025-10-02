# gic

**Git + Claude** - Generate intelligent commit messages using Claude AI.

`gic` analyzes your git changes and creates meaningful commit messages that explain _why_ you made the changes, not just _what_ changed.

## Features

- 🤖 **AI-powered commits** - Claude analyzes your changes and generates contextual messages
- 🔐 **Seamless auth** - OAuth authentication with automatic token refresh
- ⚡ **Fast analysis** - Parallel git operations for quick processing
- 🎨 **Beautiful UI** - Interactive terminal interface powered by [tap](https://github.com/yarlson/tap)
- 📦 **Smart context** - Automatically handles large changesets within Claude's limits
- 🎯 **Style matching** - Learns from your commit history to match your style

## Installation

```bash
git clone https://github.com/yarlson/gic.git
cd gic
go build
```

Move the binary to your PATH:

```bash
mv gic /usr/local/bin/
```

## Usage

### Interactive Mode

Run `gic` in any git repository:

```bash
gic
```

### Add context

Provide additional context to guide the commit message:

```bash
gic fixed the authentication bug
gic refactored for performance
```

The text after `gic` is passed to Claude as additional context.

### MCP Server Mode

Start an MCP (Model Context Protocol) server to expose git commit functionality to Claude Desktop or other MCP clients:

```bash
gic mcp
```

This starts a stdio-based MCP server that provides:

**Tools:**

- `generate_commit_message` - Analyze git changes and generate a commit message
  - Input: `user_context` (optional) - Additional context about changes
  - Output: Generated commit message
- `create_commit` - Stage all changes and create a commit
  - Input: `user_context` (optional), `message` (optional) - Custom message or context
  - Output: Commit hash and message

**Resources:**

- `git://status` - Current git repository status
- `git://diff` - Current git diff (staged and unstaged changes)
- `git://recent-commits` - Recent commit history (last 10 commits)

#### Using with Claude Desktop

Add to your Claude Desktop MCP settings (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "gic": {
      "command": "/path/to/gic",
      "args": ["mcp"]
    }
  }
}
```

After restarting Claude Desktop, you can ask Claude to generate commit messages or create commits for your git repositories.

### First run

On first run, you'll authenticate with Claude (requires Claude Pro/Max):

1. Visit the authorization URL displayed
2. Paste the code from the callback URL (format: `code#state`)
3. Token is saved to `~/.config/gic/tokens.json`

Subsequent runs use the saved token automatically.

## How it works

1. **Stages changes** - Runs `git add .` to stage all files
2. **Analyzes repo** - Fetches git status, diff, and recent commits in parallel
3. **Excludes noise** - Filters out lock files from diffs
4. **Smart context** - For large changesets, prioritizes smaller files and provides summaries
5. **Generates message** - Sends context to Claude with instructions to focus on "why"
6. **Shows preview** - Displays proposed commit in a formatted box
7. **Confirms** - Asks for confirmation before committing
8. **Creates commit** - Applies the generated message

## Configuration

### Token storage

Tokens are stored at:

- **macOS**: `~/Library/Application Support/gic/tokens.json`
- **Linux**: `~/.config/gic/tokens.json`
- **Windows**: `%APPDATA%\gic\tokens.json`

File permissions: `0600` (owner read/write only)

### Lock files excluded

The following lock files are excluded from diffs but shown in status:

- `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`
- `Gemfile.lock`, `Cargo.lock`, `go.sum`
- `composer.lock`, `Pipfile.lock`, `poetry.lock`
- `mix.lock`, `pubspec.lock`, `Podfile.lock`
- `packages.lock.json`, `paket.lock`

### Claude model

Uses `claude-sonnet-4-5` via the Anthropic API.

## Large changesets

When diffs exceed ~500K characters (~125K tokens):

1. All files are listed with line change counts
2. Smaller files get full diffs included
3. Larger files are excluded from detailed diffs
4. Claude is informed which files were excluded

This ensures the tool works with any size changeset while staying within Claude's context window.

## Project structure

```
gic/
├── main.go                 # Entry point, OAuth flow
├── internal/
│   ├── auth/
│   │   ├── oauth.go        # OAuth PKCE flow
│   │   └── token.go        # Token management
│   ├── client/
│   │   └── client.go       # Claude API client
│   ├── commit/
│   │   └── commit.go       # Commit workflow
│   └── git/
│       └── git.go          # Git operations
└── README.md
```

## License

MIT License - see [LICENSE](LICENSE) file for details.
