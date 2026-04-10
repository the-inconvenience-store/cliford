# Quickstart: Cliford

## Prerequisites

- Go 1.22 or later
- An OpenAPI 3.0 or 3.1 specification file

## Install Cliford

```bash
go install github.com/cliford/cliford/cmd/cliford@latest
```

## Generate Your First CLI App

### 1. Initialize a project

```bash
mkdir myapp && cd myapp
cliford init --spec ../openapi.yaml
```

This creates `cliford.yaml` with sensible defaults derived from your spec.
If running in a terminal, an interactive wizard guides you through
configuration.

### 2. Review the configuration

```bash
cat cliford.yaml
```

Key settings to verify:
- `app.name` - your binary name
- `app.package` - Go module path
- `app.envVarPrefix` - environment variable prefix
- `generation.mode` - hybrid (default), pure-cli, or pure-tui

### 3. Generate the app

```bash
cliford generate
```

This runs the full pipeline: parses your spec, generates the SDK, builds
CLI commands, creates TUI views (if enabled), and generates infrastructure
files (GoReleaser, docs, completions).

### 4. Build and run

```bash
go build -o myapp ./cmd/myapp/
./myapp --help
```

You should see your API's operations organized as commands.

### 5. Authenticate

```bash
./myapp auth login
```

Follow the interactive prompt to set up credentials. They are stored
securely in your OS keychain.

### 6. Make your first API call

```bash
./myapp users list --output-format table
```

## Common Workflows

### Customize command names

Edit `cliford.yaml`:

```yaml
operations:
  listUsers:
    cli:
      aliases: [ls, list]
      group: users
```

Then regenerate: `cliford generate`

### Enable TUI mode

```yaml
generation:
  mode: hybrid  # or pure-tui

tui:
  enabled: true
```

Launch TUI: `./myapp --tui` or `./myapp` (opens explorer when no subcommand).

### Add custom code

Enable custom code regions in `cliford.yaml`:

```yaml
features:
  customCodeRegions: true
```

Regenerate, then add your code between the marked regions:

```go
// --- CUSTOM CODE START: listUsers:pre ---
ctx = tracing.InjectSpan(ctx)
// --- CUSTOM CODE END: listUsers:pre ---
```

This code survives future regeneration cycles.

### Preview changes before regenerating

```bash
cliford diff
```

### Configure theme

```yaml
theme:
  colors:
    primary: "#7D56F4"
    accent: "#4ECDC4"
    error: "#FF4444"
  borders: rounded
```

## Verification

After generation, verify everything works:

```bash
# Build succeeds
go build ./...

# Vet passes
go vet ./...

# Tests pass
go test ./...

# Help works
./myapp --help

# Completions generate
./myapp completion bash > /dev/null

# Docs generate
go run ./cmd/docgen -out ./docs/cli -format markdown
```

All six checks should pass with zero manual intervention.
