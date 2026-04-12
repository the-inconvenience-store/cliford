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
| `--output-format` | `-o` | `pretty` | Output format: `pretty`, `json`, `yaml`, `table` |
| `--server` | | (from spec) | Override API server URL |
| `--timeout` | | `30s` | Request timeout |
| `--verbose` | `-v` | `false` | Log request/response to stderr (secrets redacted) |
| `--dry-run` | | `false` | Display HTTP request without executing |
| `--yes` | `-y` | `false` | Skip all confirmations |
| `--agent` | | `false` | Force agent mode (structured JSON, no interactive prompts) |
| `--no-interactive` | | `false` | Disable interactive prompts |
| `--tui` | | `false` | Launch full TUI mode |

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

Table output uses sorted column headers and `text/tabwriter` alignment. For
empty arrays, it prints "No results." instead of an empty table.

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
