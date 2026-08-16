# Repository instructions

`cmt` is a Go CLI that generates commit messages through a local Claude or
Codex CLI and can optionally create the commit. Keep the Git index as the
commit source of truth and keep provider execution non-interactive.

## Boundaries

- `main.go` owns Cobra flags, environment precedence, executable discovery,
  provider preflight, dependency assembly, and plain process output.
- `internal/app` owns the user workflows. Normal mode stages, previews,
  confirms, and commits. Message-only mode must not stage, show interactive UI,
  prompt, or commit.
- `internal/git` owns Git command construction, repository-directory binding,
  context cancellation, and Git error text. Keep Git process details here.
- `internal/provider` owns the supported provider registry, capability and auth
  preflight, provider arguments, read-only constraints, output capture, and
  temporary output cleanup.
- `internal/commit` owns status cleanup and the shared commit-message prompt.
  Staged changes remain the prompt source of truth.
- `.goreleaser.yml`, `install.sh`, and release workflows own distribution.
  Keep supported targets and installation documentation aligned.

## Product contracts

- Provider selection precedence is `--provider`, `CMT_PROVIDER`, then Claude.
  Model selection is `--model`, `CMT_MODEL`, then the provider default.
- Preflight must finish before normal mode stages files or starts interactive
  output. Missing capabilities and authentication must fail closed.
- Normal mode stages `.` from the current directory. Do not silently widen that
  path or change what an existing index contains.
- `--message-only` requires staged changes and writes only the generated message
  to stdout. It must preserve the index, working tree, and HEAD and must reject
  `--auto-approve` before provider preflight.
- Provider CLIs inspect the live repository even though the staged snapshot is
  authoritative. Do not claim complete isolation from unstaged context.
- Propagate cancellation to Git and provider subprocesses. Capture child output,
  preserve useful stderr, and remove owned temporary files.

## Go and developer-experience rules

- Use explicit, idiomatic Go, small functions, `%w` error wrapping, and no
  panics for expected failures. Avoid new dependencies and abstractions unless
  the current behavior needs them.
- Pin the Go toolchain and persistent development tools in `mise.toml`, not the
  main module dependency graph.
- Use Testify in tests: `require` for prerequisites and `assert` for independent
  outcomes. Keep scenario state visible and use real temporary Git repositories
  at the Git boundary.
- Run `make check` for formatting, workflow linting, static analysis, tests,
  builds, module consistency, and release configuration.
- Run `make ci` before delivery. It adds race tests, CRAP, CGO-free tests and
  builds, supported-target cross-builds, vulnerability scanning, and a release
  snapshot.
- New or heavily changed production functions must keep every numeric CRAP
  score below 15. Do not worsen an existing above-threshold score.
- For changed production behavior, run a diff- or package-scoped Gremlins check
  while iterating and `make mutation` before delivery. Surviving or timed-out
  mutants fail the gate. Do not lower `.gremlins.yaml` thresholds or add
  exclusions without evidence that a mutant is equivalent or unsupported.
- Do not apply repository-wide automatic fixes except the configured formatter.
  Review every generated edit.

## Change rules

- Keep changes to one cohesive behavior. Preserve public flags, environment
  variables, defaults, provider semantics, prompt output, and release targets
  unless the task explicitly changes them.
- Test normal, boundary, failure, cancellation, cleanup, and state-preservation
  behavior where the contract permits deterministic evidence.
- Update README usage and troubleshooting text when user-visible behavior or
  development commands change.
- Before finalizing, review the full diff, remove unrelated work, run the
  applicable repository gates, and report any inconclusive evidence.
