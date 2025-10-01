# gic - Git + Claude

Generate intelligent git commit messages with Claude.

## Features

- ✅ **Claude-powered commit messages** - Analyzes your changes and generates meaningful commits
- ✅ Automatic OAuth authentication (Claude Pro/Max)
- ✅ Secure token storage with automatic refresh
- ✅ Parallel git analysis (status, diff, log)
- ✅ Commit style learning from history

## Setup

```bash
go mod tidy
go build
```

## Usage

Simply run `gic` in any git repository:

```bash
./gic
```

**First run:** You'll be prompted to authorize with Claude (OAuth flow)
**Subsequent runs:** Automatically uses your saved token

### Workflow:

1. **Stage All Changes** - Runs `git add .` to stage all modified and untracked files
2. **Parallel Analysis** - Concurrently runs:
   - `git status` - Repository state
   - `git diff --numstat` - File change statistics
   - `git diff` - Full diffs (excluding lock files)
   - `git log` - Recent commit history
3. **Smart Context Management** - Automatically handles large changesets:
   - Estimates total prompt size (~500K char limit, ~125K tokens)
   - If too large: provides file summary + selective diffs for smaller files
   - Always includes: status, all file stats, commit history
   - Prioritizes smaller files (more signal, less noise)
4. **Claude Generation** - Analyzes changes and generates commit message
5. **Validation** - Shows proposed message for review
6. **Create Commit** - Creates the commit with generated message

## How it works

1. **Token Check**: Loads existing OAuth token or initiates authentication
2. **Stage Changes**: Runs `git add .` to stage all changes
3. **Git Analysis**: Concurrently runs `git status`, `git diff`, and `git log`
4. **Claude Generation**: Sends git context to Claude for commit message generation
5. **User Review**: Shows proposed message for confirmation
6. **Commit**: Creates the commit with the generated message

**Authentication**: Uses Claude Pro/Max OAuth with PKCE for secure authentication. Tokens are automatically refreshed when expired.

## Token Storage

Tokens are stored securely at:

- macOS/Linux: `~/.config/gic/tokens.json`
- Windows: `%APPDATA%\gic\tokens.json`

File permissions: `0600` (owner read/write only)

## Large Changeset Handling

When dealing with massive diffs that exceed Claude's context window:

1. **File Summary** - All changed files are listed with line counts
2. **Selective Diffs** - Smaller files get full diffs included
3. **Smart Prioritization** - Sorts by size (smallest first = better signal-to-noise)
4. **Budget Management** - Fills context up to ~500K characters (~125K tokens)
5. **Transparency** - Claude is informed which files were excluded

## Project Structure

```
gic/
├── main.go                 # CLI entry point with subcommands
├── internal/
│   ├── auth/              # OAuth & token management
│   │   ├── oauth.go       # PKCE, authorization, token exchange
│   │   └── token.go       # Token storage, refresh, validation
│   ├── client/            # Claude API operations
│   │   └── client.go      # API key creation, Claude requests
│   ├── commit/            # Git commit workflow
│   │   └── commit.go      # Parallel git analysis, Claude generation
│   └── git/               # Git command wrappers
│       └── git.go         # Status, diff, log, commit, amend
└── README.md
```

## License

MIT License - see [LICENSE](LICENSE) file for details
