# Contract: Generated App CLI Interface

**Date**: 2026-04-10

This document defines the CLI interface contract for applications generated
by Cliford. All commands, flags, and behaviors described here are produced
by the generation pipeline.

## Command Structure

```
<app> [global flags] <group> <command> [flags] [args]
```

Commands are organized by OpenAPI tags. Each operation becomes a command
within its tag group. Stutter removal is applied by default.

Example for a Petstore API:
```
petstore pets list [--limit N] [--status active]
petstore pets create [--name "Fido"] [--body '{"name":"Fido"}']
petstore pets get --id 123
petstore pets delete --id 123
petstore users list
petstore users create
```

## Global Flags (all commands)

```
--help, -h                   Show help for any command
--version, -v                Show app version, commit, build date
--output-format, -o <fmt>    Output format: pretty|json|yaml|table (default: pretty)
--server <url>               Override API server URL
--timeout <duration>         Request timeout (default: 30s)
--debug                      Log full request/response to stderr (secrets redacted)
--dry-run                    Display the HTTP request without executing
--no-interactive             Disable interactive prompts
--tui                        Launch full TUI mode
-y, --yes                    Skip all confirmations, use defaults
--no-retries                 Disable retry logic for this request
--jq <expr>                  Apply jq expression to JSON output
--output-file <path>         Write binary response to file
--agent                      Force agent mode (structured JSON, no interactive)
```

## Auth Commands

```
<app> auth login              Interactive credential setup
<app> auth logout             Clear stored credentials
<app> auth status             Show current auth state (values redacted)
<app> auth switch             Switch between profiles
<app> auth refresh            Force OAuth token refresh
```

### `auth login`

**Behavior**: When TTY detected, presents an interactive selector for auth
method (from spec's securitySchemes), then collects credentials via TUI form
or browser redirect (OAuth). Stores in OS keychain. In non-interactive mode,
reads from flags or env vars.

**Flags**: `--method <type>`, `--profile <name>`, `--token <value>`,
`--username <value>`, `--password <value>`, `--api-key <value>`

---

## Config Commands

```
<app> config show              Display current configuration
<app> config set <key> <val>   Set a config value
<app> config get <key>         Get a config value
<app> config reset             Reset to defaults
<app> config edit              Open config in $EDITOR
<app> config use-profile <n>   Switch active profile
<app> config path              Show config file location
<app> config validate          Validate current config
```

---

## Pagination Flags (on paginated operations)

```
--all                        Fetch all pages
--max-pages N                Maximum pages to fetch (with --all)
--page N                     Specific page (page-number pagination)
--limit N                    Items per page
--cursor <value>             Start cursor (cursor-based pagination)
```

---

## Retry Flags (when retries enabled)

```
--no-retries                 Disable retries for this request
--retry-max-attempts N       Override max retry attempts
--retry-max-elapsed <dur>    Override max elapsed retry time
```

---

## Input Precedence

For operation parameters, input is accepted from three sources with this
precedence (highest to lowest):

1. **Individual flags** (e.g., `--name "Alice"`)
2. **`--body` JSON payload** (e.g., `--body '{"name":"Alice"}'`)
3. **stdin** (piped JSON)

When multiple sources provide the same field, higher-precedence sources win.

---

## Output Behavior

| Format | Use Case | Notes |
|--------|----------|-------|
| `pretty` | Human terminal (default for TTY) | Colored, formatted |
| `json` | Scripting, piping | Raw JSON |
| `yaml` | Config-like output | YAML format |
| `table` | Tabular data | Column-aligned |

Binary responses use `--output-file` for disk or `--output-b64` for base64.

In agent mode (auto-detected or `--agent`): defaults to `json`, suppresses
interactive prompts, structures error output.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | API error (4xx/5xx after retries exhausted) |
| 2 | Authentication error (missing/invalid credentials) |
| 3 | Validation error (invalid flags/input) |
| 4 | Network error (connection failed, DNS, timeout) |
| 5 | Configuration error |
| 126 | User cancelled (declined confirmation) |

---

## Shell Completions

```
<app> completion bash
<app> completion zsh
<app> completion fish
<app> completion powershell
```

Output completion scripts to stdout for shell integration.

---

## Documentation

```
<app> docs generate [--format markdown|man|llms] [--output-dir <path>]
```

Generates documentation from the command tree. The `llms` format produces
a flat, LLM-optimized text file.

---

## Environment Variables

All flags can be set via environment variables using the configured prefix:

```
<PREFIX>_SERVER_URL         API server URL
<PREFIX>_OUTPUT_FORMAT      Default output format
<PREFIX>_NO_INTERACTIVE     Disable interactive mode
<PREFIX>_MODE               App mode (cli|tui|headless)
<PREFIX>_API_KEY            API key credential
<PREFIX>_BEARER_TOKEN       Bearer token credential
<PREFIX>_TIMEOUT            Default request timeout
```

---

## Config File

Default location: `~/.config/<app>/config.yaml`

```yaml
server:
  url: https://api.example.com
  timeout: 30s
auth:
  method: bearer
output:
  format: pretty
  color: auto
  pager: true
mode: hybrid
interactive: true
confirm_destructive: true
tui:
  theme: default
  animations: true
  mouse: true
profiles:
  default:
    server:
      url: https://api.example.com
  staging:
    server:
      url: https://staging.api.example.com
active_profile: default
```
