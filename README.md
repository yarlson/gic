# cmt

`cmt` uses a local Claude Code or Codex CLI to write a Git commit message.
It can create the commit or print only the message.

## Requirements

You need:

- Git
- a Git repository
- Claude Code or Codex on `PATH`
- an authenticated provider CLI

Use `claude auth login` to authenticate Claude Code.
Use `codex login` to authenticate Codex.

## Install

### Homebrew

The Homebrew tap publishes `cmt` as a Cask.

```bash
brew tap yarlson/homebrew-tap
brew install --cask cmt
```

### Install script

This command installs the latest GitHub release in `/usr/local/bin`.
The install step uses `sudo`.

```bash
curl -sSL https://raw.githubusercontent.com/yarlson/cmt/master/install.sh | bash
```

Pass a tag to install one release:

```bash
curl -sSL https://raw.githubusercontent.com/yarlson/cmt/master/install.sh | bash -s v0.11.0
```

You can also set `cmt_VERSION`:

```bash
cmt_VERSION=v0.11.0 curl -sSL https://raw.githubusercontent.com/yarlson/cmt/master/install.sh | bash
```

The release process builds these targets:

- macOS on AMD64 and ARM64
- Linux on AMD64 and ARM64
- Windows on AMD64

### Build from source

The Go module requires Go 1.26.6.

```bash
git clone https://github.com/yarlson/cmt.git
cd cmt
go build -o cmt .
```

## Create a commit

Run `cmt` in a Git repository:

```bash
cmt
```

Normal mode performs these steps:

1. It checks Git and the selected provider.
2. It runs `git add .` from the current directory.
3. It shows the repository status.
4. It asks the provider to write a commit message.
5. It shows the message and asks for confirmation.
6. It creates the commit with `git commit -m`.

Run `cmt` from the directory whose changes you want to stage.
Git stages changed files at and below that directory.

Add a short hint when the staged diff does not show the full intent:

```bash
cmt fix the expired-session redirect
cmt explain why retries stop after the third failure
```

`cmt` sends all positional arguments to the provider as one user hint.

## Generate only a message

Stage the files before you use `--message-only`:

```bash
git add path/to/file
cmt --message-only
```

This mode:

- bases the message on the existing staged snapshot
- prints the generated message to standard output
- does not stage files
- does not show the interactive interface
- does not ask for confirmation
- does not create a commit
- does not change the working tree, Git index, or `HEAD`

The command returns an error if the Git index has no staged changes.

You can pass a hint in this mode:

```bash
cmt --message-only focus on the user-visible behavior
```

You can pass the output to Git:

```bash
git commit -m "$(cmt --message-only)"
```

## Skip confirmation

Use `--auto-approve` or `-y` to create the commit without a confirmation prompt:

```bash
cmt --auto-approve
cmt -y
```

Normal mode still shows status and message output before it creates the commit.
You cannot combine `--auto-approve` with `--message-only`.

## Select a provider and model

`cmt` supports two providers:

| Provider    | Select it           | Default model         |
| ----------- | ------------------- | --------------------- |
| Claude Code | `--provider claude` | `sonnet`              |
| Codex       | `--provider codex`  | The Codex CLI default |

Claude Code is the default provider.

Select Codex for one command:

```bash
cmt --provider codex
```

Set a model for one command:

```bash
cmt --model haiku
cmt --provider codex --model gpt-5
```

Set defaults with environment variables:

```bash
export CMT_PROVIDER=codex
export CMT_MODEL=gpt-5
```

Command flags take priority over environment variables.
Environment variables take priority over provider defaults.
Empty values do not replace provider defaults.

`cmt` passes an explicit model name to the provider CLI.
The provider reports an error if it does not accept that name.

## Provider access

`cmt` checks the provider command, required options, and authentication before it stages files.
Both providers run as local processes in the current repository.

The prompt asks each provider to inspect these Git commands:

- `git status --porcelain`
- `git diff --cached`
- `git log -10 --oneline`

The prompt tells the provider to base the message on the staged snapshot.
It also tells the provider to ignore unstaged changes and avoid write commands.
The provider can still read the live repository, so unstaged data is not fully isolated.

Codex runs with an ephemeral, read-only sandbox.
It ignores Codex user configuration and repository rules for this command.

Claude Code runs without session persistence or slash commands.
`cmt` passes an allowlist for Git inspection commands.
It also uses Claude Code's `bypassPermissions` mode for non-interactive use.

## Version information

Both commands show the release version and build time:

```bash
cmt --version
cmt version
```

## Errors

`cmt` writes errors to standard error and exits with a non-zero status.

- A missing `git`, `claude`, or `codex` command causes the startup checks to fail.
- An unsupported provider value lists the supported providers.
- A missing provider option causes the capability check to fail.
- Failed authentication asks you to run `claude auth login` or `codex login`.
- An empty provider response causes message generation to fail.
- `cmt` reports Git stage and commit failures.

## Development

### Set up the toolchain

Mise pins Go and the development tools in `mise.toml`.

```bash
mise trust
mise install
```

The file pins these tools:

- Go
- golangci-lint
- GoReleaser
- Gremlins
- actionlint

Tests use Testify.
Use `require` for setup that must succeed.
Use `assert` for independent outcomes.

### Run checks

Run the standard checks:

```bash
mise exec -- make check
```

This command checks workflows, formatting, lint, vet, tests, builds, modules,
and the GoReleaser configuration.

Run all delivery checks:

```bash
mise exec -- make ci
```

This command adds:

- race detection
- CRAP scores
- tests and builds without CGO
- builds for supported targets
- a vulnerability scan
- a release snapshot

Run mutation tests separately:

```bash
mise exec -- make mutation
```

The mutation check fails if a code change survives or times out.
A surviving change is a change that the tests did not detect.
The configured mutation efficacy threshold is 99.9 percent.

Every numeric CRAP score must stay below 15.

Use a focused target when you work on one check:

```bash
mise exec -- make test
mise exec -- make race
mise exec -- make coverage
mise exec -- make crap
mise exec -- make vuln
mise exec -- make no-cgo
mise exec -- make cross-build
mise exec -- make snapshot
```

### Release

Push a tag whose name starts with `v` to start the release workflow.
The workflow runs the tests and then runs GoReleaser.
GoReleaser publishes the GitHub release and updates the Homebrew Cask.

## Support

Open an issue at <https://github.com/yarlson/cmt/issues>.

## License

`cmt` uses the [MIT License](LICENSE).
