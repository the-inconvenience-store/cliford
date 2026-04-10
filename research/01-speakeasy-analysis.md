# Speakeasy CLI & SDK Generation Analysis

> Research document for Cliford - analyzing Speakeasy's approach to learn from their DX patterns.

## Overview

Speakeasy is the primary prior art for OpenAPI-to-CLI generation. They generate fully functional Go CLIs from OpenAPI specs using Cobra, with an integrated SDK layer. Their approach is OpenAPI-native, embedding configuration via `x-speakeasy-*` extensions directly in the spec.

## Generation Pipeline

Speakeasy's pipeline works as follows:

1. **Input**: OpenAPI 3.0/3.1 spec (or JSON Schema)
2. **SDK Generation**: Produces a Go SDK from the spec (internal to the CLI project)
3. **CLI Generation**: Wraps the SDK with Cobra commands, flag parsing, output formatting
4. **Distribution**: GoReleaser config, install scripts, GitHub Actions workflows

Quickstart: `speakeasy quickstart --target cli`

Three core config params: `packageName` (Go module path), `cliName` (binary name), `envVarPrefix` (env var prefix).

## Project Structure (Generated)

```
cmd/              # Binary entrypoint + doc generator
internal/
  cli/            # Cobra command files (one per operation)
  client/         # SDK wrapper + diagnostics
  config/         # Configuration + keychain management
  flagutil/       # Flag registration + request construction
  output/         # Format handling + agent-mode
  usage/          # Help text + machine-readable schemas
  explorer/       # Interactive command browser (TUI)
  sdk/            # Auto-generated Go SDK
```

**Key takeaway**: Clean separation between CLI layer, SDK layer, and infrastructure concerns.

## API-to-CLI Command Mapping

- Each API operation becomes a CLI command
- Operations group by OpenAPI tags
- **Smart stutter removal**: `users list-users` becomes `users list` (configurable via `removeStutter`)
- Exact matches are promoted to parent group commands

**Lesson for Cliford**: This mapping strategy is elegant. We should adopt tag-based grouping with stutter removal as a default, but make the mapping fully configurable via hooks.

## Request Input Handling

Three input methods with hierarchical precedence (highest to lowest):

1. **Individual flags** - Each operation parameter becomes a flag
2. **`--body` JSON payload** - Full request body as JSON
3. **stdin JSON** - Pipe JSON into the command

Individual flags override conflicting values from other sources. Bytes fields accept: `file:./path`, `b64:encoded`, or raw strings.

**Lesson for Cliford**: This is well-designed. We should replicate this hierarchy. The `file:` prefix for bytes fields is particularly clever.

## Authentication

Credential resolution order:
1. Command flags (highest priority)
2. Environment variables
3. OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
4. Config file (`~/.config/<app>/config.yaml`)

Supported methods:
- HTTP Basic Auth
- API Key (header/query)
- Bearer Token
- OAuth (GA + select beta)
- mTLS (via SDK hooks)
- Custom schemes (via `x-speakeasy-custom-security` extension)

Interactive `auth login`/`auth logout` commands when `interactiveAuth` is enabled.

**Lesson for Cliford**: The keychain integration is a must-have. We should support all these auth methods and add the same resolution hierarchy. The interactive auth flow is a great UX pattern.

## Output Formatting

The `--output-format` / `-o` flag supports:
- `pretty` (default for TTY)
- `json` / `yaml` (structured)
- `table` (tabular)
- `toon` (compact, agent-optimized)
- `--jq` expressions for JSON filtering

Binary responses: `--output-file` for disk, `--output-b64` for base64.

**Lesson for Cliford**: The agent-mode output format (`toon`) is forward-thinking. We should have similar awareness of AI agent contexts.

## Interactive & Agent Modes

Interactive mode (default when TTY detected):
- Auto-prompts for required unresolved fields
- TUI explorer for command browsing
- Interactive config flows

Agent mode (auto-detected for Claude Code, Cursor, Codex, etc.):
- Switches to `toon` output format
- Structures error responses
- Suppresses interactive surfaces

**Lesson for Cliford**: Auto-detecting agent environments is brilliant. We should do the same, plus allow explicit `--agent` flag.

## Configuration (gen.yaml)

Key CLI config fields:
- `version`, `packageName`, `cliName`, `envVarPrefix` - identity
- `removeStutter` - command naming
- `generateRelease` - distribution (GoReleaser + GitHub Actions)
- `interactiveMode` - TUI features
- `interactiveAuth` - auth flow UX
- `interactiveTheme` - colors (accent, dimmed, subtle, error, success)
- `enableCustomCodeRegions` - preserve hand-written code across regeneration
- `additionalDependencies` - extra Go modules
- `baseErrorName` / `defaultErrorName` - error type naming

Distribution channels: Homebrew, WinGet, nFPM (deb/rpm).

**Lesson for Cliford**: The `interactiveTheme` with semantic color names (accent, dimmed, etc.) is a clean abstraction. Custom code regions are essential for real-world use.

## Diagnostics

Built-in flags:
- `--dry-run` - display request without execution
- `--debug` - log request/response to stderr
- Both auto-redact sensitive headers/fields

**Lesson for Cliford**: Must-have. The auto-redaction of sensitive fields shows attention to security.

## Documentation & Discoverability

- `--usage` flag outputs KDL representation of commands (machine-readable)
- Shell completions for bash, zsh, fish, PowerShell
- Cobra's built-in help system

**Lesson for Cliford**: We should go further with Cobra's doc generation for LLMs (llms.txt format).

## Key Strengths to Adopt

1. **OpenAPI-native config** - No separate config language to learn
2. **Smart defaults** - Stutter removal, TTY detection, auto-agent mode
3. **Layered auth** - Flags > env > keychain > config file
4. **Custom code regions** - Essential for real-world extensibility
5. **Distribution pipeline** - GoReleaser + Homebrew + WinGet out of the box
6. **Interactive theme** - Semantic color naming

## Key Gaps / Opportunities for Cliford

1. **No pure TUI mode** - Speakeasy has an "explorer" but no full Bubbletea TUI app mode
2. **Limited hook system** - Extensions are mainly via OpenAPI, not programmatic hooks
3. **Config locked to OpenAPI** - No alternative config formats
4. **No `-y` mode** - No explicit skip-all-confirmations flag
5. **No headless mode** - Agent mode is implicit, not an explicit config choice
6. **No SemVer automation** - Version is manual in gen.yaml
7. **Tied to Speakeasy platform** - Not a standalone OSS tool
