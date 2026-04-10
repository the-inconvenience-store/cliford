# Getting Started

This guide walks you through installing Cliford, generating your first CLI app
from an OpenAPI spec, and running it.

## Prerequisites

- **Go 1.22+** — [install Go](https://go.dev/dl/)
- An **OpenAPI 3.0 or 3.1** specification file for your API

Verify your Go installation:

```bash
go version
# go version go1.22+ ...
```

## Install Cliford

```bash
go install github.com/the-inconvenience-store/cliford/cmd/cliford@latest
```

Verify it's installed:

```bash
cliford --version
cliford doctor
```

The `doctor` command checks that Go, oapi-codegen, and your environment are
ready.

## Generate Your First App

### 1. Initialize

Create a project directory and initialize Cliford with your spec:

```bash
mkdir myapp && cd myapp
cliford init --spec ../path/to/openapi.yaml --name myapp --package github.com/myorg/myapp
```

This creates `cliford.yaml` with sensible defaults derived from your spec.

### 2. Review the configuration

Open `cliford.yaml` and verify:

```yaml
spec: ../path/to/openapi.yaml
app:
  name: myapp
  package: github.com/myorg/myapp
  envVarPrefix: MYAPP
generation:
  mode: hybrid
```

### 3. Generate

```bash
cliford generate --spec ../path/to/openapi.yaml --name myapp --package github.com/myorg/myapp
```

Or if you have `cliford.yaml`, Cliford reads settings from it automatically.

Optional flags:

| Flag               | Purpose                                                |
| ------------------ | ------------------------------------------------------ |
| `--tui`            | Generate TUI mode (Bubbletea explorer, forms, views)   |
| `--release`        | Generate GoReleaser, install scripts, Homebrew formula |
| `--custom-regions` | Add custom code region markers                         |
| `--verbose`        | Show pipeline stage timings                            |
| `--dry-run`        | Preview without writing files                          |

### 4. Build

```bash
go mod tidy
go build -o myapp ./cmd/myapp/
```

### 5. Run

```bash
./myapp --help
```

You'll see all your API operations organized as CLI commands grouped by
OpenAPI tag, with `auth`, `config`, and `completion` commands included.

## Example Session

Given a Petstore OpenAPI spec with `pets` and `users` tags:

```bash
# List pets
./myapp pets list --limit 10 --output-format table

# Create a pet (with JSON body)
./myapp pets create --body '{"name": "Fido", "species": "dog"}'

# Get a specific pet
./myapp pets get --pet-id 42 -o json

# Dry-run to see the HTTP request
./myapp pets list --dry-run

# Authenticate
./myapp auth login --token "sk-..."
./myapp auth status

# Switch servers
./myapp pets list --server https://staging.api.example.com/v1
```

## Verification Checklist

After generation, verify everything works:

```bash
go build ./...           # Compiles
go vet ./...             # No issues
./myapp --help           # Shows all commands
./myapp pets --help      # Shows operations under "pets"
./myapp completion bash  # Generates shell completions
```

## Next Steps

- [Configuration Guide](configuration.md) — customize naming, aliases, theming
- [Authentication](authentication.md) — set up auth for your API
- [TUI Mode](tui-mode.md) — enable the interactive terminal UI
- [Distribution](distribution.md) — ship your app to users
