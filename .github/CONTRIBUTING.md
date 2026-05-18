# Contributing Guide

Thank you for your interest in Mihosh! Here's how to get involved.

## Development Environment

- **Go**: 1.24+
- **OS**: Windows / Linux / macOS
- **Mihomo**: A running Mihomo proxy service is required for debugging

```bash
# Clone the repository
git clone https://github.com/AimAI-Labs/mihosh.git
cd mihosh

# Install dependencies
go mod download

# Run local checks
make check

# Build and run manually if make is unavailable
go build .
./mihosh
```

## Code Standards

- Run `go fmt ./...` to format code
- Run `go vet ./...` to check for potential issues
- Run `go test ./...` to ensure tests pass
- Run `go build .` to verify the binary builds
- Prefer `make check` when `make` is available; it runs the standard local gate
- Follow the architecture and coding conventions in [AGENTS.md](../AGENTS.md)

## Automation

Pull requests run these checks in GitHub Actions:

- CI: gofmt check, module verification, `go vet ./...`, `go test ./...`, and `go build .`
- Lint: golangci-lint with a conservative correctness-focused configuration
- CodeQL: Go security analysis

Dependabot opens weekly pull requests for Go module and GitHub Actions updates.

## Core Principles

1. **KISS** — Solve problems in the simplest way possible; avoid over-engineering
2. **Add state to `*State`** — Do not add loose fields to the main `Model`
3. **Concurrency control** — Batch network operations must use Semaphore (≤20)
4. **Ring Buffer** — Use ring buffers for fixed-length history; never use slice truncation
5. **Message passing** — Use `tea.Msg` for inter-page communication; never modify other page states directly

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types**:

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `refactor` | Refactoring (no functional change) |
| `docs` | Documentation changes |
| `test` | Test-related |
| `chore` | Build / CI / tooling |
| `style` | Code formatting (no logic change) |

**Examples**:

```
feat(nodes): add batch proxy testing with progress bar
fix(connections): fix ring buffer overflow on high traffic
refactor(tui): extract sidebar component to shared module
docs: update installation guide for macOS
```

## Pull Request Workflow

1. **Fork** this repository and create your branch (`git checkout -b feat/amazing-feature`)
2. Make your changes and ensure tests pass
3. Run `make check` or the equivalent Go commands
4. Commit following the convention above
5. Push to your fork (`git push origin feat/amazing-feature`)
6. Open a Pull Request and fill in the PR template

## Reporting Bugs

Please use the [Bug Report](https://github.com/AimAI-Labs/mihosh/issues/new?template=bug_report.yml) template to submit bugs.

## Feature Requests

Please use the [Feature Request](https://github.com/AimAI-Labs/mihosh/issues/new?template=feature_request.yml) template to submit suggestions.

## License

All contributions will be released under the [MIT License](../LICENSE).
