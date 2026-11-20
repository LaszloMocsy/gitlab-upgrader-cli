# Contributing to gluper

Thanks for helping. gluper runs as root on other people's production GitLab
servers, so the bar is correctness and predictability rather than features.

## Development setup

You need the Go version in `go.mod` (or newer).

```sh
go build ./...
go test -race ./...
go run ./cmd/gluper version
```

Linting uses [golangci-lint](https://golangci-lint.run) v2:

```sh
golangci-lint run ./...
```

To check the release configuration without publishing anything:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=sign,sbom
```

CI builds and tests on Linux, macOS and Windows and cross-compiles every
release target on each pull request.

## Ground rules

These are enforced in review and, where possible, by CI.

- **Detect, don't assume.** Read paths, install type and backup location
  from the system and GitLab's effective configuration. Never hardcode a
  setup-specific path or behaviour.
- **Fail closed.** If a fact can't be determined, block and explain why.
- **No `os/exec` outside `internal/runner`.** Every external command goes
  through the runner and is tagged read-only or mutating, so a dry run is
  exactly what a real run would do. The `depguard` and `forbidigo` linters
  enforce this.
- **No shell strings.** Pass arguments as a list; never interpolate values
  into a shell command.
- **Locale-proof parsing.** Commands whose output is parsed run with
  `LC_ALL=C`; prefer machine-readable output formats.
- **Capabilities, not distro names.** Ask "is `apt-mark` available?", not
  "is this Ubuntu?".
- **No privilege escalation.** gluper never calls `sudo` itself.
- **Linux-only code behind build tags.** Code that touches the GitLab host
  uses `//go:build linux`, so macOS and Windows builds stay read-only. Host
  commands are registered in `internal/cli/host_linux.go`; their names go in
  `internal/cli/host.go` so other platforms get a clear "Linux only" stub.
- **Exit codes are a contract** (see the README). Don't change their meaning.

## Adding an install adapter

*Arrives with milestone M1.* Each install type (Linux package, later Docker)
implements one adapter interface and ships with recorded command output and
fake filesystem roots under `testdata/`, including non-English locale samples.

## Adding a dataset rule

*Arrives with milestone M2.* Version-specific upgrade rules (OS cut-offs,
PostgreSQL minimums, removed configuration) are data with an ID, a severity
and a source URL, plus a test. Adding one doesn't need Go code.

## Pull requests

- Keep them focused, and add tests for behaviour changes.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
  messages (`feat:`, `fix:`, `docs:`, …). The changelog is built from them.
- CI must pass on all three operating systems.

## Reporting bugs

Use the issue forms. For security problems, follow [SECURITY.md](SECURITY.md)
instead.
