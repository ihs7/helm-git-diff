# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Helm plugin written in Go that compares rendered Kubernetes manifests between git commits.

## Build and Development Commands

```bash
# Build
make build                    # Output: bin/helm-git-diff

# Testing
make test                     # Run all tests
go test -v ./...              # Alternative test command
go test -run TestName ./...   # Run specific test

# Linting
make lint                     # Go linter (golangci-lint)
make lint-yaml                # YAML linter
make lint-md                  # Markdown linter (markdownlint)

# Local Installation
helm plugin install .         # Install from source
helm git-diff                 # Test plugin after install

# Cleanup
make clean                    # Remove bin/ directory
```

## Architecture

**Single-package, flat structure** - all code in `main` package within `main.go`:

### Execution Flow

1. `main()` → `parseFlags()` → `checkGitRepo()` → `run()`
2. `run()` → Either uses provided chart names or calls `detectChangedCharts()`
3. For each chart → `diffChart()` → renders at both refs → compares manifests

### Key Functions (in order)

- **Configuration**: `parseFlags()`, `shouldUseColor()`, `isTerminal()`
- **Workflow**: `run()`, `detectChangedCharts()`, `detectChartContext()`
- **Core Operations**: `diffChart()`, `renderChartFromWorkdir()`, `renderChartAtRef()`
- **Utilities**: `colorizeDiff()`, `isLibraryChart()`, `buildDependencies()`, `getChartPathsToExtract()`

### Config Struct

Central configuration object passed through all operations containing flags, chart paths, and state (differences detected, color usage).

### Rendering Strategy

- **Base ref**: Extracts chart files at commit using `git archive`, renders in temp directory
- **Current ref**:
  - If `HEAD`: Uses working directory directly (captures uncommitted changes)
  - Otherwise: Uses `git archive` like base ref
- Both use `helm template` via `exec.Command` to render manifests

### Chart Detection

`detectChangedCharts()` uses `git diff --name-only` to find modified files, then maps them back to chart directories by looking for `Chart.yaml` in parent paths.

## Code Conventions

- **Single package** (`main`), flat structure - no subdirectories
- **PascalCase** for exported identifiers, **camelCase** for unexported
- **Return errors explicitly** - no panics except for unrecoverable failures
- **Print errors to stderr**, normal output to stdout
- **Exit code 1** for all errors
- **Minimal comments** - code should be self-documenting
- **Function ordering**: config → workflow → operations → utilities

## Testing

- Standard `testing` package
- Tests in `main_test.go`
- Use `t.TempDir()` for temporary directories (auto-cleanup)
- Skip tests when prerequisites unavailable (conditional checks)
- Create isolated git repos in tests for integration testing
- Clean up resources with `defer` statements

## Dependencies

- **Runtime**: golang, Helm, Git
- **Build**: GNU Make
- **Linting**: golangci-lint, yamllint, markdownlint-cli

## Plugin Configuration

Defined in `plugin.yaml`:

- Binary location: `${HELM_PLUGIN_DIR}/bin/helm-git-diff`
- Install/update hook: `install-plugin.sh` (builds from source)
- Cross-platform support (Windows uses `.exe` extension)
