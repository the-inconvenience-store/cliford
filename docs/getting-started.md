# Tutorial: Your First Generated CLI

This tutorial walks you through generating a working CLI application from an
OpenAPI specification. By the end, you will have a compiled binary that can
authenticate with an API, list resources in a formatted table, and handle
errors with automatic retries.

## What you will learn

- Installing Cliford
- Generating a CLI from an OpenAPI spec
- Running commands against an API
- Setting up authentication
- Using verbose mode to inspect requests

## Prerequisites

- **Go 1.22 or later** installed ([download](https://go.dev/dl/))
- An **OpenAPI 3.0 or 3.1** specification file for your API

Verify Go is installed:

```bash
go version
```

## Step 1: Install Cliford

```bash
go install github.com/the-inconvenience-store/cliford/cmd/cliford@latest
```

Verify the installation:

```bash
cliford --version
cliford doctor
```

The `doctor` command checks that Go, required tools, and your environment are
ready for code generation.

## Step 2: Initialize a project

Create a directory and initialize Cliford with your OpenAPI spec:

```bash
mkdir petstore && cd petstore
cliford init --spec ../path/to/petstore.yaml --name petstore --package github.com/myorg/petstore
```

This creates a `cliford.yaml` configuration file with defaults derived from
your spec.

## Step 3: Generate the CLI

```bash
cliford generate
```

Cliford reads `cliford.yaml` and generates a complete Go project:

```
petstore/
  cmd/petstore/main.go      # Entry point
  internal/cli/              # Cobra commands for every operation
  internal/sdk/              # HTTP client with retry and pagination
  internal/auth/             # Credential storage and resolution
  internal/client/           # Layered HTTP client factory
  go.mod
```

## Step 4: Build the binary

```bash
go mod tidy
go build -o petstore ./cmd/petstore/
```

## Step 5: Explore the commands

```bash
./petstore --help
```

You will see all your API operations organized as CLI commands grouped by
OpenAPI tag. The output also includes `auth`, `config`, `generate-docs`, and
`completion` commands.

```bash
./petstore pets --help
./petstore pets list --help
```

## Step 6: Make your first API call

```bash
# List resources (output defaults to formatted JSON)
./petstore pets list --limit 10

# Use verbose mode to see the full HTTP request and response
./petstore pets list --limit 10 --verbose
```

The `--verbose` (or `-v`) flag prints the request method, URL, headers, and
response to stderr. Sensitive headers like `Authorization` are replaced with
`[REDACTED]`.

## Step 7: Set up authentication

If your API requires authentication, set the credential via an environment
variable. Cliford generates env var names following the pattern
`<APP>_<SCHEME>_<TYPE>`:

```bash
# For a Bearer token scheme named "BearerAuth"
export PETSTORE_BEARERAUTH_TOKEN="your-api-token"

# For an API key scheme named "ApiKeyAuth"
export PETSTORE_APIKEYAUTH_API_KEY="your-api-key"
```

Or use the interactive login command:

```bash
./petstore auth login --method bearer --token "your-api-token"
./petstore auth status
```

Now every request includes the correct authentication header automatically.

## Step 8: Try a dry run

See what HTTP request would be sent without executing it:

```bash
./petstore pets create --body '{"name": "Fido", "species": "dog"}' --dry-run
```

## Step 9: Generate shell completions

```bash
# Bash
./petstore completion bash > /etc/bash_completion.d/petstore

# Zsh
./petstore completion zsh > "${fpath[1]}/_petstore"

# Fish
./petstore completion fish > ~/.config/fish/completions/petstore.fish
```

## Verification checklist

After generation, confirm everything works:

```bash
go build ./...              # Compiles without errors
go vet ./...                # No issues
./petstore --help           # Shows all commands
./petstore pets --help      # Shows operations under "pets"
./petstore completion bash  # Outputs valid completion script
./petstore --version        # Shows version string
```

## Next steps

- [Authentication](authentication.md) for credential storage, profiles, and OAuth
- [Configuration](configuration.md) for the full `cliford.yaml` reference
- [Pagination and Retries](pagination-retries.md) for handling large result sets
- [TUI Mode](tui-mode.md) for generating an interactive terminal interface
- [Architecture](architecture.md) for how the generated app is structured internally
