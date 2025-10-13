# helm-git-diff

A Helm plugin that compares rendered Kubernetes manifests between git references. Automatically detects changed charts and shows differences in rendered YAML.

## Features

- Auto-detects changed charts between git references
- Compares rendered manifests (not chart source)
- Includes uncommitted changes when using `HEAD`
- Supports custom values files and inline value overrides

## Installation

Requires Helm 3.18+ and Git.

```bash
helm plugin install https://github.com/ihs7/helm-git-diff
```

## Usage

### Basic

Compare changed charts against `origin/main`:

```bash
helm git-diff
```

### Custom References

Compare between specific git references:

```bash
helm git-diff --base HEAD~1 --current HEAD
helm git-diff --base main --current feature-branch
```

### With Values

```bash
helm git-diff --values custom-values.yaml
helm git-diff --set image.tag=v2.0.0
helm git-diff --values prod.yaml --set replicas=3
```

## Options

| Flag             | Default       | Description                                       |
| ---------------- | ------------- | ------------------------------------------------- |
| `--base`         | `origin/main` | Base git reference                                |
| `--current`      | `HEAD`        | Current git reference (HEAD includes uncommitted) |
| `--chart-dir`    | `.`           | Directory containing charts                       |
| `--values`       | -             | Comma-separated values files                      |
| `--set`          | -             | Inline values (format: `key1=val1,key2=val2`)     |
| `--fail-on-diff` | `false`       | Exit 1 if differences found                       |
| `--no-color`     | `false`       | Disable colored output                            |

## Contributing

### Prerequisites

- Go 1.20+
- Helm 3.18+
- Git
- GNU Make

### Development

```bash
# Build
make build

# Test
make test
go test -v ./...

# Lint
make lint

# Install locally
helm plugin install .
```

## Release

Releases are automated via GitHub Actions when tags are pushed which triggers the release workflow that:

- Builds binaries for Linux, macOS, and Windows (amd64/arm64)
- Creates GitHub Release with changelog
- Uploads archives with checksums

## License

MIT
