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
    flags:                              # Control which global flags are generated (all enabled by default)
      outputFormat:
        enabled: true
        default: "pretty"              # Default --output-format value
        hidden: false
      jq:
        enabled: true
        hidden: false
      outputFile:
        enabled: true
        hidden: false
      includeHeaders:
        enabled: true
        hidden: false
      server:
        enabled: true
        hidden: false
      timeout:
        enabled: true
        default: "30s"                 # Default --timeout value
        hidden: false
      verbose:
        enabled: true
        hidden: false
      dryRun:
        enabled: true
        hidden: false
      yes:
        enabled: true
        hidden: false
      agent:
        enabled: true
        hidden: false
      noInteractive:
        enabled: true
        hidden: false
      tui:
        enabled: true                  # Only relevant when generation.tui.enabled is true
        hidden: false
      retries:
        enabled: true                  # Controls --no-retries, --retry-max-attempts, --retry-max-elapsed
        hidden: false
      template:
        enabled: true                  # --template expression flag
        hidden: false
      templateFile:
        enabled: true                  # --template-file path flag
        hidden: false
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
  agentOutputFormat: toon                # Default output format when --agent is active ("toon", "json", etc.)
  requestId:
    enabled: false                       # Auto-inject X-Request-ID UUID on every request (default: false)
    header: "X-Request-ID"              # HTTP header name (default: X-Request-ID)
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
      agentFormat: json                 # Override agent format for this operation

# OAI overlay files applied before generation (see overlays.md)
overlays:
  - cliford.overlay.yaml
  - overlays/my-extensions.yaml

# Hooks (see hooks.md)
hooks:
  "after:generate":
    - run: "gofmt -w ."
    - run: "go vet ./..."
```

## Overlays

The `overlays` key lists [OAI Overlay Specification](https://github.com/OAI/Overlay-Specification)
files to apply to the spec before generation. Overlays let you add
`x-cliford-*` extensions, remove internal paths, or patch any field — without
touching the original spec file. This is particularly useful when the spec is
owned by a third party or is machine-generated and re-synced regularly.

```yaml
overlays:
  - cliford.overlay.yaml          # committed alongside cliford.yaml
  - overlays/local.yaml           # developer-only, gitignored
```

Overlays are applied in listed order. All downstream stages — spec validation,
SDK generation, and CLI generation — see the merged result.

See [Overlays](overlays.md) for the full reference.

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
| `agentFormat` | `string` | Output format override when `--agent` is active (e.g. `toon`, `json`); overrides global `features.agentOutputFormat` |
| `defaultOutputFormat` | `string` | Default `--output-format` for this operation (e.g. `table`); overridable explicitly at runtime; `--agent` still takes priority |
| `requestId` | `bool` | Enable request ID injection for this operation even when `features.requestId.enabled` is `false` |

### TUI overrides

| Field | Type | Description |
|-------|------|-------------|
| `displayAs` | `string` | `table`, `detail`, `form`, `custom` |
| `refreshable` | `bool` | Enable auto-refresh in TUI |

## Generated flags

Cliford bakes a fixed set of global persistent flags into the root command of
every generated CLI. `generation.cli.flags` lets you control each one
individually — disable it entirely, hide it from `--help`, or change its
default value.

### Flag reference

| Flag key | Generated flag(s) | Type | Built-in default |
|----------|-------------------|------|-----------------|
| `outputFormat` | `--output-format` / `-o` | string | `pretty` |
| `jq` | `--jq` | string | `""` |
| `outputFile` | `--output-file` | string | `""` |
| `includeHeaders` | `--include-headers` | bool | `false` |
| `server` | `--server` | string | `""` |
| `timeout` | `--timeout` | string | `30s` |
| `verbose` | `--verbose` / `-v` + `--debug` alias | bool | `false` |
| `dryRun` | `--dry-run` | bool | `false` |
| `yes` | `--yes` / `-y` | bool | `false` |
| `agent` | `--agent` | bool | `false` |
| `noInteractive` | `--no-interactive` | bool | `false` |
| `tui` | `--tui` | bool | `false` (only when `generation.tui.enabled: true`) |
| `retries` | `--no-retries`, `--retry-max-attempts`, `--retry-max-elapsed` | — | — |
| `template` | `--template` | string | `""` |
| `templateFile` | `--template-file` | string | `""` |

### `enabled: false`

The flag is not registered. The backing variable is still declared in the
generated code (always holding its zero value), so all generated logic that
reads it continues to compile and runs safely.

### `hidden: true`

The flag is registered normally but hidden via `MarkHidden` so it does not
appear in `--help`. It still works when passed explicitly.

### `default` (string flags only)

Overrides the baked-in default value for `outputFormat` (default `"pretty"`)
and `timeout` (default `"30s"`). Ignored for bool/int flags.

### Example — hide internal flags, change defaults

```yaml
generation:
  cli:
    flags:
      outputFormat:
        default: "table"    # list-heavy APIs default to table view
      jq:
        hidden: true        # power-user flag; keep it working but out of help
      dryRun:
        hidden: true
      agent:
        enabled: false      # not needed for this CLI
```

## Request ID

When `features.requestId.enabled: true`, Cliford generates a UUID (v4) at the
start of every command's `RunE`, attaches it as an HTTP header, and embeds it in
all error messages. This makes it easy to find the matching server-side log entry
when a request fails.

```yaml
features:
  requestId:
    enabled: true             # inject on all operations
    header: "X-Request-ID"    # header name (default: X-Request-ID)
```

**What changes in the generated code:**

- Every `RunE` declares `requestID := generateRequestID()` immediately after the
  request is built.
- `req.Header.Set("X-Request-ID", requestID)` attaches the UUID before execution.
- Error messages become `HTTP 404 (request-id: 550e8400-…): not found` instead of
  the bare `HTTP 404: not found`.
- Dry-run output prints the header (all headers are printed, so the UUID appears
  there for free).
- Verbose (`--verbose`) output logs the header in the `> X-Request-ID: …` line
  (all request headers are logged).

**Per-operation enable:**

Set `requestId: true` on individual operations to opt them in without enabling
globally:

```yaml
operations:
  createPet:
    cli:
      requestId: true
```

**Interaction with `global_params.generate: uuid`:**

If the same header name is configured in both `features.requestId` and
`global_params`, the RunE-generated UUID takes priority: the transport-level
generator skips headers that are already set. Both approaches together for the
same header are harmless but redundant.

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
| `aliases` | `map[string]string` | `{}` | User-defined command aliases (see [Generated App Reference](generated-app-reference.md#aliases)) |

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
