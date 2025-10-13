# GitHub Copilot Instructions

Helm plugin that compares rendered Kubernetes manifests between git commits for Helm charts.

## Prerequisites

- Go toolchain
- Helm
- GNU Make
- Git repository

## Build

```bash
make build
```

Binary output: `bin/helm-git-diff`

## Tests

```bash
make test
```

Use standard `testing` package. Create isolated git repos in tests using `t.TempDir()`. Skip tests when prerequisites unavailable with conditional checks.

## Linting

```bash
make lint        # Go
make lint-yaml   # YAML files
make lint-md     # Markdown files
```

## Plugin Installation

```bash
helm plugin install .
```

Test locally after installation:

```bash
helm git-diff
```

## Commands Reference

```bash
make build                    # Compile binary
make test                     # Run unit tests
make lint                     # Run Go linter
make lint-yaml                # Run YAML linter
make lint-md                  # Run Markdown linter
make clean                    # Remove build artifacts
helm plugin install .         # Install locally
helm git-diff [flags] [CHART...] # Run plugin
```

## Key Rules

- Single package (main), flat structure
- All logic in `main.go`, tests in `main_test.go`
- PascalCase for exported identifiers, camelCase for unexported
- Return errors explicitly from functions
- Print errors to stderr
- Exit code 1 for errors
- Minimal comments (self-documenting code)
- Use `t.TempDir()` for temporary directories in tests
- Clean up resources with defer statements
- Keep function ordering logical: config → workflow → operations → utilities
