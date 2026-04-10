# Configuration

Cliford uses a layered configuration system with three sources, resolved in
this order (highest priority first):

1. **`cliford.yaml`** operation-level overrides
2. **`x-cliford-*`** OpenAPI extensions (inline in the spec)
3. **`cliford.yaml`** global settings
4. **Built-in defaults**

## cliford.yaml Reference

```yaml
# Schema version
version: "1"

# Path to OpenAPI spec
spec: ./openapi.yaml

# App identity
app:
  name: myapp              # Binary name
  package: github.com/myorg/myapp  # Go module path
  envVarPrefix: MYAPP      # Env var prefix (auto-derived from name if omitted)
  version: "0.1.0"         # SemVer, injected via ldflags at build time
  description: "My API CLI"

# Generation options
generation:
  mode: hybrid             # pure-cli | pure-tui | hybrid
  sdk:
    generator: oapi-codegen
    outputDir: internal/sdk
    package: sdk
  cli:
    outputDir: internal/cli
    removeStutter: true    # "listPets" under "pets" becomes "list"
  tui:
    enabled: false         # Generate Bubbletea TUI
    outputDir: internal/tui

# Authentication
auth:
  interactive: true        # Generate login/logout/status commands
  keychain: true           # Store credentials in OS keychain
  methods:                 # Restrict which methods are offered at login
    - bearer
    - apiKey

# TUI theme
theme:
  colors:
    primary: "#7D56F4"
    secondary: "#FF6B6B"
    accent: "#4ECDC4"
    background: "#1A1A2E"
    text: "#EAEAEA"
    dimmed: "#666666"
    error: "#FF4444"
    success: "#44FF44"
    warning: "#FFAA00"
  borders: rounded         # normal | rounded | thick | double
  spinner: dot
  table:
    headerBold: true
    stripeRows: true
  compact: false

# Feature toggles
features:
  pagination: true
  retries:
    enabled: true
    maxAttempts: 3
  customCodeRegions: false
  documentation:
    markdown: true
    llmsTxt: true
  distribution:
    goreleaser: true
    homebrew: false

# Per-operation overrides (keyed by operationId)
operations:
  listPets:
    cli:
      aliases: [ls, list]
      group: pets          # Override tag-based grouping
    tui:
      displayAs: table
      refreshable: true
  createPet:
    cli:
      confirm: true
      confirmMessage: "Create a new pet?"
  deletePet:
    cli:
      hidden: true         # Hide from --help but still callable
      confirm: true

# Hooks (see hooks.md)
hooks:
  "after:generate":
    - run: "gofmt -w ."
    - run: "go vet ./..."

# Global parameters added to all requests
globalParams:
  - name: X-Request-ID
    in: header
    generate: uuid
```

## Per-Operation Overrides

### CLI Overrides

| Field | Type | Description |
|-------|------|-------------|
| `aliases` | `[]string` | Alternative command names |
| `group` | `string` | Override tag-based grouping |
| `hidden` | `bool` | Hide from help output (still callable) |
| `confirm` | `bool` | Require confirmation before executing |
| `confirmMessage` | `string` | Custom confirmation prompt text |

### TUI Overrides

| Field | Type | Description |
|-------|------|-------------|
| `displayAs` | `string` | `table`, `detail`, `form`, `custom` |
| `refreshable` | `bool` | Enable auto-refresh in TUI |

## Stutter Removal

By default, Cliford removes redundant prefixes from command names. An operation
called `listPets` under the `pets` tag becomes `pets list` instead of
`pets list-pets`.

Disable with:

```yaml
generation:
  cli:
    removeStutter: false
```

## Environment Variables

All cliford.yaml fields can be overridden via environment variables prefixed
with `CLIFORD_`:

```bash
CLIFORD_SPEC=./openapi.yaml
CLIFORD_APP_NAME=myapp
CLIFORD_GENERATION_MODE=hybrid
```

## Generated App Configuration

The apps Cliford generates also use Viper for end-user configuration:

```
~/.config/<app>/config.yaml
```

Resolution order for the generated app:
1. CLI flags (`--server`, `--output-format`, etc.)
2. Environment variables (`<PREFIX>_SERVER_URL`, etc.)
3. Config file
4. Defaults
