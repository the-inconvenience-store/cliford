# Generated App Reference

This is the technical reference for the commands, flags, files, and packages
produced by `cliford generate`.

## Generated command tree

Every generated app includes these commands:

```
<app>
  <tag> <operation>          # One command per OpenAPI operation, grouped by tag
  auth
    login                    # Store credentials interactively or via flags
    logout                   # Clear stored credentials
    status                   # Show current auth state
    switch <profile>         # Switch active profile
    refresh                  # Force refresh OAuth2 token for the active profile
  config
    show                     # Display current configuration
    set <key> <value>        # Set a config value
    get <key>                # Get a config value
  generate-docs              # Generate man pages or Markdown documentation
  completion                 # Generate shell completions (bash, zsh, fish)
```

## Global flags

These flags are available on every command:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output-format` | `-o` | `pretty` | Output format: `pretty`, `json`, `yaml`, `table`, `toon`, `go-template`, `jsonpath` |
| `--template` | | `""` | Go template or JSONPath expression (used with `-o go-template\|jsonpath`) |
| `--template-file` | | `""` | Path to a Go template or JSONPath file (used with `-o go-template\|jsonpath`) |
| `--jq` | | `""` | Filter JSON output with a jq expression (gojq syntax) |
| `--output-file` | | `""` | Write response body to a file instead of stdout |
| `--include-headers` | | `false` | Print response headers alongside the body |
| `--server` | | (from spec) | Override entire API server URL |
| `--server-<varname>` | | (from spec) | Set a server URL template variable (see below) |
| `--timeout` | | `30s` | Request timeout |
| `--verbose` | `-v` | `false` | Log request/response to stderr (secrets redacted) |
| `--dry-run` | | `false` | Display HTTP request without executing |
| `--yes` | `-y` | `false` | Skip all confirmations |
| `--agent` | | `false` | Force agent mode (structured JSON, no interactive prompts) |
| `--no-interactive` | | `false` | Disable interactive prompts |
| `--tui` | | `false` | Launch full TUI mode |

### Server URL template variables

When the OpenAPI spec uses a URL template (e.g.,
`https://{tenant}.api.example.com/{version}`), Cliford generates one
persistent flag per variable:

```
--server-<varname>   (default: value from servers[0].variables.<varname>.default)
```

For example, a spec with:

```yaml
servers:
  - url: "https://{tenant}.api.example.com/{version}"
    variables:
      tenant:
        default: "acme"
        enum: ["acme", "globex", "initech"]
      version:
        default: "v1"
```

Produces these flags on every command:

```
--server-tenant string   Customer tenant identifier (default "acme")
--server-version string  API version (default "v1")
```

The flags substitute their values into the URL template at request time:

```bash
# Uses default values: https://acme.api.example.com/v1/items
./myapp items list

# Override tenant and version:
./myapp items list --server-tenant globex --server-version v2
# → https://globex.api.example.com/v2/items
```

When `--server` is provided, it overrides the entire URL and variable
substitution is skipped:

```bash
./myapp items list --server https://dev.example.com
```

If a variable defines `enum` values, passing an invalid value returns an
error before the request is made:

```
invalid --server-tenant value "unknown": allowed values are [acme globex initech]
```

## Per-operation flags

Each operation command receives flags derived from its OpenAPI parameters:

| Parameter location | Flag format | Example |
|-------------------|-------------|---------|
| Path parameter | `--<param-name>` | `--pet-id` |
| Query parameter | `--<param-name>` | `--limit`, `--offset` |
| Header parameter | `--<param-name>` | `--x-request-id` |
| Body property | `--<prop-name>` | `--name`, `--email` |
| Body (raw JSON) | `--body` | `--body '{"name":"Fido"}'` |

### Body property flags

When a request body has a JSON schema with properties, each property becomes
a separate flag. The three input sources are merged in this order (highest
priority first):

1. Individual property flags (`--name Fido`)
2. `--body` JSON string
3. stdin (piped JSON)

### Flag name collision

If a body property has the same name as a path or query parameter, the body
property flag is prefixed with `body-`. For example, if both a path parameter
`id` and a body property `id` exist:

- `--id` sets the path parameter
- `--body-id` sets the body property

The `--body` flag itself is also reserved and cannot collide with body
property names.

## Pagination flags

Operations with pagination configuration receive additional flags:

| Flag | Description |
|------|-------------|
| `--all` | Fetch all pages and output combined results |
| `--max-pages` | Maximum pages to fetch with `--all` (default: 1000) |
| `--<cursor-param>` | Cursor or page token for manual pagination |
| `--<limit-param>` | Items per page |

The specific parameter names depend on the `x-cliford-pagination`
configuration.

## Retry flags

All commands include retry flags:

| Flag | Description |
|------|-------------|
| `--no-retries` | Disable retries for this request |
| `--retry-max-attempts` | Override max retry attempts |
| `--retry-max-elapsed` | Override max elapsed time (e.g., `5m`) |

## OAuth2 token lifecycle

When the spec includes an OAuth2 security scheme, the generated app manages
token expiry transparently.

### Automatic refresh on every request

If the following environment variables are set, the auth transport checks
the stored token's expiry before each request and refreshes it when fewer
than 60 seconds remain:

```bash
export <PREFIX>_<SCHEME>_TOKEN_URL="https://auth.example.com/token"
export <PREFIX>_<SCHEME>_CLIENT_ID="your-client-id"
export <PREFIX>_<SCHEME>_CLIENT_SECRET="your-client-secret"   # omit for public clients
```

`<PREFIX>` is the app's env var prefix (from `cliford.yaml` or derived from
the app name). `<SCHEME>` is the security scheme name from the spec,
uppercased with dashes and spaces replaced by underscores.

Example for an app named `petstore` with scheme `OAuth2Auth`:

```bash
export PETSTORE_OAUTH2AUTH_TOKEN_URL="https://auth.example.com/token"
export PETSTORE_OAUTH2AUTH_CLIENT_ID="client-abc"
```

When these variables are absent, the stored token is used as-is until it
expires, at which point the API returns a 401.

### Manual refresh with `auth refresh`

To force a token refresh before it expires:

```bash
./myapp auth refresh
./myapp auth refresh --profile staging
```

`auth refresh`:

1. Reads the stored OAuth2 credential for the active (or `--profile`) profile.
2. Validates that the credential has a refresh token.
3. Reads the token URL and client ID from env vars (same names as above).
4. Calls the token endpoint with `grant_type=refresh_token`.
5. Writes the new `access_token`, `expires_at`, and `refresh_token` (if
   the provider rotates it) back to the credential store.
6. Prints the new expiry time.

If the env vars are not set, the command exits with an error message that
names the exact variables that need to be set.

## Loading spinner

When a command makes an HTTP request, a loading spinner animates on stderr
while the response is in flight. The spinner only appears when stderr is a
terminal. It is suppressed in `--no-interactive`, `--agent`, and piped
contexts.

The spinner frames, speed, and whether it appears at all are configured in
`cliford.yaml` at generation time:

```yaml
features:
  spinner:
    enabled: true
    frames: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
    intervalMs: 80
```

Setting `enabled: false` removes the spinner entirely from the generated
code. The frames array accepts any Unicode or ASCII characters. The
`intervalMs` value controls how fast the animation cycles.

Some alternative frame sets:

```yaml
# Simple dots
frames: [".", "..", "...", ""]

# Arrow
frames: ["←", "↖", "↑", "↗", "→", "↘", "↓", "↙"]

# Block fill
frames: ["▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"]

# Classic ASCII
frames: ["|", "/", "-", "\\"]

# Claude Code 
frames: ["·", "✻", "✽", "✶", "✳", "✢"]
```

## Generated file structure

```
<output-dir>/
  cmd/<app>/main.go                # Entry point, creates layered HTTP client
  go.mod                           # Go module with dependencies
  internal/
    cli/
      root.go                      # Root command, global flags, FormatOutput, table renderer
      <tag>.go                     # One file per OpenAPI tag with operation commands
      auth.go                      # Auth login/logout/status/switch commands
      config_cmd.go                # Config show/set/get commands
      generate_docs.go             # generate-docs subcommand (man, markdown)
      error_output.go              # Error formatting utilities
    sdk/
      sdk.gen.go                   # oapi-codegen generated types and client
      retry.go                     # RetryTransport with exponential backoff
      pagination.go                # PageIterator and pagination helpers
      errors.go                    # Typed error types (APIError, ValidationError, etc.)
      verbose.go                   # VerboseTransport for --verbose logging
    auth/
      middleware.go                # AuthTransport credential injection
      resolver.go                  # 5-tier credential resolution chain
      keychain.go                  # AES-256-GCM encrypted file store
      profiles.go                  # Profile management via Viper
      redact.go                    # Header and value redaction utilities
      oauth.go                     # OAuth 2.0 flows (if spec has OAuth schemes)
    client/
      factory.go                   # HTTP client factory with transport chain
    hooks/
      runner.go                    # Shell and go-plugin hook execution
    config/
      config.go                    # Viper config initialization
    hybrid/
      mode.go                      # Mode detection (CLI, TUI, headless, agent)
      adapter.go                   # Inline Bubbletea prompts
    tui/                           # (only with --tui flag)
      app.go                       # Root Bubbletea model
      explorer.go                  # Filterable operation list
      operation.go                 # Parameter form with field types
      response.go                  # Scrollable response viewer
      theme.go                     # Lipgloss theme engine
  docs/
    cli/index.md                   # Markdown command reference
    cli/<tag>.md                   # Per-tag operation documentation
    llms.txt                       # LLM-optimized documentation
```

## Output formats

The `--output-format` (or `-o`) flag controls how responses are displayed:

| Format | Behavior |
|--------|----------|
| `pretty` | Indented JSON (default) |
| `json` | Indented JSON |
| `yaml` | YAML |
| `table` | ASCII table with column headers from response schema properties |
| `toon` | TOON columnar format — token-efficient for AI agents (see below) |
| `go-template` | Apply a Go `text/template` expression to the response |
| `go-template-file` | Load a Go template from a file: `-o go-template-file=./pod.tmpl` |
| `jsonpath` | Apply a JSONPath expression (kubectl-compatible syntax) |
| `jsonpath-file` | Load a JSONPath expression from a file: `-o jsonpath-file=./expr.txt` |

Table output uses sorted column headers and `text/tabwriter` alignment. For
empty arrays, it prints "No results." instead of an empty table.

## TOON output (`--output-format toon`)

[TOON](https://github.com/toon-format/toon-go) (Token-Oriented Object Notation)
is a compact columnar format that achieves roughly 60% token reduction compared
to JSON. It declares field names once in a header row and then streams values —
ideal for passing API responses to LLMs or AI coding agents.

```bash
# Explicit toon output
./myapp pets list --output-format toon
```

Example output for an array response:

```
pets[3]{id,name,status}:
  1,Fido,available
  2,Rex,pending
  3,Whiskers,available
```

Compare with equivalent JSON (much larger for the same data):

```json
[
  {"id": 1, "name": "Fido", "status": "available"},
  {"id": 2, "name": "Rex", "status": "pending"},
  {"id": 3, "name": "Whiskers", "status": "available"}
]
```

If toon encoding fails for a particular response shape, the output falls back
to indented JSON without error.

### Auto-selecting toon in agent mode

Rather than requiring `--output-format toon` on every invocation, API designers
can bake toon as the default format when `--agent` is active. This is configured
at generation time and has no effect on normal interactive use.

**Global default** — applies to all commands in the generated app:

```yaml
# cliford.yaml
features:
  agentOutputFormat: toon
```

**Per-operation override** — overrides the global for a specific command:

```yaml
# cliford.yaml
operations:
  listPets:
    cli:
      agentFormat: toon   # use toon in agent mode
  getRawConfig:
    cli:
      agentFormat: json   # always use JSON for this op, even in agent mode
```

Or inline in the OpenAPI spec:

```yaml
paths:
  /pets:
    get:
      x-cliford-cli:
        agentFormat: toon
```

**Resolution order** (highest priority first):

1. `--output-format <value>` passed explicitly at runtime — always wins
2. Per-operation `agentFormat` (from `cliford.yaml` or `x-cliford-cli`)
3. Global `features.agentOutputFormat` from `cliford.yaml`
4. No agent format configured — `outputFormat` flag value used unchanged

When an agent format is configured, the generated command emits this logic at
runtime:

```go
if agentMode && --output-format was not explicitly set {
    use configured agent format
} else {
    use --output-format value (default: "pretty")
}
```

A user who explicitly passes `--output-format json` alongside `--agent` always
gets JSON, regardless of the configured agent default.

## jq filtering

The `--jq` flag pipes the JSON response through a
[gojq](https://github.com/itchyny/gojq) expression before display. No
external `jq` binary is required — gojq is embedded in the generated binary.

```bash
# Extract a nested field
./myapp pets list --jq '.pets[] | .name'

# Select a single item
./myapp pets list --jq '.[0]'

# Combine with --output-format
./myapp pets list --jq '.pets' --output-format table

# Count results
./myapp pets list --jq '.pets | length'
```

The jq expression receives the parsed JSON response as input (i.e., a Go
`any` value). The filter runs after any custom code regions, so post-response
transformations in custom code are visible to the jq expression.

When the expression produces a single value, that value is passed to the
formatter. Multiple values are collected into a slice. A filter that matches
nothing (e.g., `select(false)`) produces `null`.

### Per-operation default jq

API designers can bake a default jq expression into a command so it always
shapes the response without requiring the user to specify `--jq`. The user
can still override the default by passing `--jq` explicitly.

Set via `cliford.yaml`:

```yaml
operations:
  listPets:
    cli:
      defaultJQ: ".pets"
```

Or via the `x-cliford-cli` OpenAPI extension:

```yaml
paths:
  /pets:
    get:
      x-cliford-cli:
        defaultJQ: ".pets"
```

Cliford validates the expression at generation time and returns an error
immediately if it cannot be parsed, rather than failing at runtime.

## Go template and JSONPath output

Two additional output formats let you extract and format specific fields from
responses without needing `jq` or external tools — particularly useful for
scripting and for users who are already familiar with kubectl's `-o` flag.

### Go template (`-o go-template`)

Applies a Go [`text/template`](https://pkg.go.dev/text/template) expression to
the parsed JSON response. The template receives the decoded response as its
data object (`.`).

```bash
# Single field — inline expression
./myapp pets list -o 'go-template={{range .pets}}{{.name}}{{"\n"}}{{end}}'

# Separate --template flag (cleaner for complex templates)
./myapp pets list -o go-template --template '{{range .pets}}{{.id}}: {{.name}}{{"\n"}}{{end}}'

# Load template from a file
./myapp pets list -o go-template --template-file ./pets.tmpl

# Inline file path shorthand (no --template-file needed)
./myapp pods list -o 'go-template-file=./pod.tmpl'
```

**Available template functions:**

| Function | Signature | Description |
|----------|-----------|-------------|
| `json` | `json(v any) string` | Marshal a value to a compact JSON string |

Example using the `json` function:

```bash
./myapp pets get --id 1 -o go-template --template '{{json .tags}}'
# → ["indoor","vaccinated"]
```

### JSONPath (`-o jsonpath`)

Applies a JSONPath expression to the response. Supports kubectl-compatible
curly-brace syntax (`{.items[*].name}`), standard dot-notation, and `$`
prefix — all are interchangeable.

The `[*]` wildcard is converted to the gojq `[]` iterator internally;
no new dependencies are required.

```bash
# kubectl-style curly braces
./myapp pods list -o 'jsonpath={.items[*].metadata.name}'

# Standard dot-notation (no braces needed)
./myapp pods list -o 'jsonpath=.items[*].metadata.name'

# Separate --template flag
./myapp pods list -o jsonpath --template '{.items[*].metadata.name}'

# Load expression from a file
./myapp pods list -o jsonpath-file=./expr.txt
./myapp pods list -o jsonpath --template-file ./expr.txt
```

**Output rules (kubectl-compatible):**

| Result type | Output |
|------------|--------|
| String | Printed as-is followed by a newline |
| Array of strings | Space-separated on a single line |
| Array of mixed types | Space-separated; non-strings are JSON-encoded inline |
| Other | JSON-encoded, followed by a newline |
| Null / no match | No output |

### Composing with `--jq`

`--jq` runs first (before `FormatOutput`), so template formats see the
already-filtered data. This lets you pre-shape the data with jq and then
apply a template for final rendering:

```bash
# --jq extracts .pets, then go-template formats each item
./myapp pets list --jq '.pets' -o go-template \
  --template '{{range .}}{{.id}}: {{.name}}{{"\n"}}{{end}}'
```

### Per-operation default template format

Use `defaultOutputFormat` to bake a template format into a command so users
get structured output without specifying `-o` every time:

```yaml
# cliford.yaml
operations:
  listPets:
    cli:
      defaultOutputFormat: "go-template={{range .pets}}{{.name}}{{\"\\n\"}}{{end}}"
```

```yaml
# OpenAPI extension
paths:
  /items:
    get:
      x-cliford-cli:
        defaultOutputFormat: "jsonpath={.items[*].name}"
```

The user can still override the default by passing `-o` explicitly.

## File downloads (`--output-file`)

The `--output-file <path>` flag writes the raw response body directly to a
file instead of printing to stdout. It works for any response — binary
(PDFs, images, archives) or JSON — and shows a progress indicator during
the download. No external tools are required.

```bash
# Download a PDF report
./myapp reports export --id 42 --output-file report.pdf

# Save a ZIP archive
./myapp backups download --output-file backup.zip

# Save a JSON response to disk for offline inspection
./myapp pets list --output-file pets.json
```

### Progress display

The progress indicator adapts to the execution context:

| Context | Behaviour |
|---------|-----------|
| Interactive TTY | Animated progress bar on stderr (percentage when server sends `Content-Length`, byte counter otherwise) |
| `--no-interactive` or piped | Silent download; `"Wrote <path> (<size>)"` printed to stderr on completion |
| `--agent` or AI environment | Silent download; `{"path":"…","bytes":N,"contentType":"…"}` JSON printed to stdout on completion |

The progress bar uses `charmbracelet/bubbles/progress`, already embedded in
the generated binary — no external binary needed.

### Behaviour notes

- The spinner that normally shows during the connect phase still runs; the
  progress bar takes over once response headers are received and the body
  starts streaming.
- Error responses (HTTP 4xx/5xx) are still reported as errors; the file is
  not created.
- `--output-file` can be combined with `--verbose` to log request/response
  headers while downloading. When verbose is active the entire body is
  buffered in memory before writing; for very large files prefer omitting
  `--verbose`.
- Agent-mode JSON output gives AI callers structured metadata for further
  processing:
  ```json
  {"path":"report.pdf","bytes":204800,"contentType":"application/pdf"}
  ```

## Response headers (`--include-headers`)

The `--include-headers` flag includes the HTTP response headers in the output
alongside the response body. It works with all output formats and can be
combined with `--jq`, `--output-file`, and `--output-format`.

### JSON / pretty / yaml output

When the response body is valid JSON, the output is wrapped in an envelope
object:

```json
{
  "headers": {
    "Content-Type": "application/json",
    "X-Request-Id": "abc-123",
    "X-Ratelimit-Remaining": "99"
  },
  "body": { ... }
}
```

The `headers` object uses the canonical HTTP header names as keys. Headers with
multiple values are represented as a JSON array; single-value headers are a
plain string.

```bash
# Inspect rate-limit headers alongside results
./myapp pets list --include-headers

# Extract just the headers with --jq
./myapp pets list --include-headers --jq '.headers'

# Extract a specific header value
./myapp pets list --include-headers --jq '.headers["X-Ratelimit-Remaining"]'

# Use with YAML output
./myapp pets list --include-headers --output-format yaml
```

### Non-JSON responses

When the response body cannot be parsed as JSON, headers are printed as
`Name: Value` lines to stdout, followed by a blank line, then the raw body:

```
Content-Type: text/plain
X-Request-Id: abc-123

plain text response body here
```

### `--output-file` with `--include-headers`

When `--output-file` is set, the response body is written to the file and
headers are printed to stderr instead (one `Name: Value` line each, followed
by a blank line):

```bash
./myapp reports export --id 42 --output-file report.pdf --include-headers
# stderr: Content-Type: application/pdf
# stderr: Content-Length: 204800
# stderr: (blank line)
# file:   report.pdf written with progress bar
```

## Verbose output format

When `--verbose` or `-v` is passed, request and response details are printed
to stderr:

```
> GET https://api.example.com/pets?limit=10
> Authorization: [REDACTED]
> Content-Type: application/json
>
< 200 OK (142ms)
< Content-Type: application/json
<
< [{"id":1,"name":"Fido"}, ...]
```

Binary response bodies are shown as `[binary response, N bytes]`. Response
bodies larger than 2048 bytes are truncated.

## generate-docs subcommand

```bash
# Generate man pages
./myapp generate-docs --format man --output-dir ./docs/man

# Generate Markdown
./myapp generate-docs --format markdown --output-dir ./docs/md
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `markdown` | Output format: `man` or `markdown` |
| `--output-dir` | `./docs` | Directory for generated documentation |

Man pages use Cobra's `doc.GenManTree`. Markdown uses `doc.GenMarkdownTree`.
