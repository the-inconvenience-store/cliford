# Configuration Reference

Cliford uses a layered configuration system with four sources, resolved in
this order (highest priority first):

1. **`cliford.yaml`** operation-level overrides
2. **`x-cliford-*`** OpenAPI extensions (inline in the spec)
3. **`cliford.yaml`** global settings
4. **Built-in defaults**

## cliford.yaml reference

```yaml
# Schema version
version: "1"

# Path to OpenAPI spec
spec: ./openapi.yaml

# App identity
app:
  name: myapp                           # Binary name
  package: github.com/myorg/myapp       # Go module path
  envVarPrefix: MYAPP                   # Env var prefix (auto-derived from name if omitted)
  version: "0.1.0"                      # SemVer, injected via ldflags at build time
  description: "My API CLI"

# Generation options
generation:
  mode: hybrid                          # pure-cli | pure-tui | hybrid
  sdk:
    generator: oapi-codegen
    outputDir: internal/sdk
    package: sdk
  cli:
    outputDir: internal/cli
    removeStutter: true                 # "listPets" under "pets" becomes "list"
  tui:
    enabled: false                      # Generate Bubbletea TUI
    outputDir: internal/tui

# Authentication
auth:
  interactive: true                     # Generate login/logout/status commands
  keychain: true                        # Store credentials in OS keychain
  methods:                              # Restrict which methods are offered at login
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
  borders: rounded                      # normal | rounded | thick | double
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
  spinner:
    enabled: true                       # Show loading animation during HTTP requests
    frames:                             # Animation frames (any Unicode/ASCII characters)
      - "⠋"
      - "⠙"
      - "⠹"
      - "⠸"
      - "⠼"
      - "⠴"
      - "⠦"
      - "⠧"
      - "⠇"
      - "⠏"
    intervalMs: 80                      # Milliseconds between frames
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
      group: pets                       # Override tag-based grouping
      defaultJQ: ".pets"               # Always apply this jq filter to output
    tui:
      displayAs: table
      refreshable: true
  createPet:
    cli:
      confirm: true
      confirmMessage: "Create a new pet?"
  deletePet:
    cli:
      hidden: true                      # Hide from --help but still callable
      confirm: true

# Hooks (see hooks.md)
hooks:
  "after:generate":
    - run: "gofmt -w ."
    - run: "go vet ./..."
```

## Per-operation overrides

### CLI overrides

| Field | Type | Description |
|-------|------|-------------|
| `aliases` | `[]string` | Alternative command names |
| `group` | `string` | Override tag-based grouping |
| `hidden` | `bool` | Hide from help output (still callable) |
| `confirm` | `bool` | Require confirmation before executing |
| `confirmMessage` | `string` | Custom confirmation prompt text |
| `defaultJQ` | `string` | Default jq expression applied to output; overridable with `--jq` at runtime |

### TUI overrides

| Field | Type | Description |
|-------|------|-------------|
| `displayAs` | `string` | `table`, `detail`, `form`, `custom` |
| `refreshable` | `bool` | Enable auto-refresh in TUI |

## Stutter removal

By default, Cliford removes redundant prefixes from command names. An
operation called `listPets` under the `pets` tag becomes `pets list` instead
of `pets list-pets`.

Disable with:

```yaml
generation:
  cli:
    removeStutter: false
```

## Environment variables

All `cliford.yaml` fields can be overridden via environment variables
prefixed with `CLIFORD_`:

```bash
CLIFORD_SPEC=./openapi.yaml
CLIFORD_APP_NAME=myapp
CLIFORD_GENERATION_MODE=hybrid
```

## Generated app configuration

The apps Cliford generates use Viper for end-user configuration. The config
file is located at:

```
~/.config/<app>/config.yaml
```

### Resolution order

1. CLI flags (`--server`, `--output-format`, `--verbose`, etc.)
2. Environment variables (`<PREFIX>_SERVER_URL`, `<PREFIX>_REQUEST_TIMEOUT`, etc.)
3. Config file (`~/.config/<app>/config.yaml`)
4. Built-in defaults

### Generated app config keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server_url` | string | (from spec) | Override the API server URL |
| `request_timeout` | duration | `30s` | Global request timeout |
| `active_profile` | string | `default` | Active auth profile name |
| `features.retry.enabled` | bool | `true` | Enable/disable automatic retries |
| `features.retry.max_attempts` | int | `3` | Maximum retry attempts |
| `features.retry.initial_interval` | duration | `1s` | Initial backoff interval |
| `features.hooks.enabled` | bool | `false` | Enable before/after request hooks |
| `global_params.headers` | map | `{}` | Headers added to every request |
| `global_params.query` | map | `{}` | Query parameters added to every request |

### Server URL override

The server URL can be overridden three ways:

```bash
# CLI flag (highest priority)
./myapp pets list --server https://staging.api.example.com

# Environment variable
export MYAPP_SERVER_URL=https://staging.api.example.com

# Config file
echo "server_url: https://staging.api.example.com" >> ~/.config/myapp/config.yaml
```

### Global parameters

Inject headers or query parameters into every request via config:

```yaml
# ~/.config/myapp/config.yaml
global_params:
  headers:
    X-Tenant-ID: "acme-corp"
    X-Request-Source: "cli"
  query:
    api_version: "2024-01"
```

Per-operation values take precedence. If a command explicitly sets a header
that is also in `global_params.headers`, the per-operation value is used.
