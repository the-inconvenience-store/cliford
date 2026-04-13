# Cliford

**Generate complete CLI & TUI apps from OpenAPI specs.**

Cliford takes an OpenAPI 3.0/3.1 specification and produces a fully functional,
production-ready Go application with CLI commands, an interactive TUI, secure
authentication, pagination, retries, and distribution tooling — all from a
single command.

```bash
cliford init --spec openapi.yaml --name myapp
cliford generate
```

That's it. You get a compilable Go binary with one command per API operation,
`--help` on every command, shell completions, JSON/YAML/table output,
authentication, and more.

## Features

- **OpenAPI-driven** — every command, flag, and parameter derived from your spec
- **Overlay support** — patch any spec with [OAI Overlay](https://github.com/OAI/Overlay-Specification) files; add `x-cliford-*` extensions to specs you don't own without modifying them
- **SDK-first** — generates a typed Go SDK via oapi-codegen, then builds CLI/TUI on top
- **Three modes** — pure CLI, full Bubbletea TUI, or hybrid (CLI + inline prompts)
- **Authentication** — API key, bearer, basic, OAuth 2.0 with OS keychain storage
- **Pagination** — offset, cursor, Link header patterns with `--all` flag
- **Retries** — exponential backoff with jitter, Retry-After header support
- **Custom code regions** — add logic that survives regeneration
- **Theming** — Lipgloss-powered TUI with configurable colors and borders
- **Distribution** — GoReleaser, Homebrew formula, install scripts out of the box
- **Documentation** — auto-generated Markdown docs and LLM-optimized `llms.txt`
- **Hooks** — lifecycle and transform hooks at every pipeline stage
- **Agent-aware** — auto-detects Claude Code, Cursor, Codex and switches to structured output
- **Watch/poll mode** — `--watch` re-runs GET commands on a timer; `--poll-interval 2s` sets the interval; TTY-aware screen clearing like `watch(1)`
- **Waiters** — `--wait` blocks until a jq condition is true (`aws ec2 wait`-style); `--wait-for` overrides at runtime; configurable via `x-cliford-wait` in the spec
- **User aliases** — gh CLI-style shortcuts stored in config; `alias set lp "pets list --limit 10"`
- **jq filtering** — `--jq` flag filters JSON output via embedded gojq; no external binary required
- **Go template & JSONPath output** — `-o go-template` and `-o jsonpath` extract fields kubectl-style; no extra dependencies
- **File downloads** — `--output-file` streams any response to disk with a live progress bar; adapts to agent mode
- **Compact agent output** — `--output-format toon` uses [TOON](https://github.com/toon-format/toon-go) for ~60% token reduction; auto-selected in agent mode via `features.agentOutputFormat`

## Quick Start

### Install

```bash
go install github.com/the-inconvenience-store/cliford/cmd/cliford@latest
```

### Generate your first app

```bash
# Initialize a project
cliford init --spec ./openapi.yaml --name myapp --package github.com/myorg/myapp

# Generate the app
cliford generate

# Build and run
cd output && go mod tidy && go build -o myapp ./cmd/myapp/
./myapp --help
```

### What gets generated

```
myapp/
├── cmd/myapp/main.go           # App entrypoint
├── internal/
│   ├── sdk/                    # Typed Go SDK (oapi-codegen) + pagination/retry/errors
│   ├── cli/                    # Cobra commands (one per operation) + auth + config
│   ├── tui/                    # Bubbletea app (explorer, forms, response views)
│   ├── hybrid/                 # Mode detection + inline prompts
│   ├── auth/                   # Keychain storage, middleware, profiles, redaction
│   └── config/                 # Viper configuration
├── docs/
│   ├── cli/*.md                # Markdown CLI reference
│   └── llms.txt                # LLM-optimized documentation
├── .goreleaser.yaml            # Cross-platform release config
├── .github/workflows/release.yaml
├── install.sh / install.ps1
├── homebrew/<app>.rb
└── cliford.lock                # Generation lockfile
```

## Configuration

Cliford uses a layered configuration system:

1. **`cliford.yaml`** — primary config file (see [Configuration Guide](docs/configuration.md))
2. **`x-cliford-*` OpenAPI extensions** — per-operation inline config
3. **CLI flags** — override any setting at generation time

```yaml
# cliford.yaml
spec: ./openapi.yaml

app:
  name: petstore
  package: github.com/myorg/petstore
  envVarPrefix: PET

generation:
  mode: hybrid
  tui:
    enabled: true

theme:
  colors:
    primary: "#7D56F4"
    accent: "#4ECDC4"
  borders: rounded

# Overlay files patch the spec before generation (no modifications to the original)
overlays:
  - cliford.overlay.yaml

operations:
  listPets:
    cli:
      aliases: [ls, list]
  deletePet:
    cli:
      confirm: true
      confirmMessage: "Are you sure? This cannot be undone."
```

## Cliford Commands

| Command                  | Description                                        |
| ------------------------ | -------------------------------------------------- |
| `cliford init`           | Create `cliford.yaml` with defaults from your spec |
| `cliford generate`       | Run the full generation pipeline                   |
| `cliford validate`       | Check spec and config for errors                   |
| `cliford diff`           | Preview what regeneration would change             |
| `cliford version <type>` | Bump SemVer (auto/patch/minor/major)               |
| `cliford doctor`         | Check environment and dependencies                 |

Both `cliford generate` and `cliford validate` accept `--overlay <path>`
(repeatable) to apply overlay files ad-hoc without editing `cliford.yaml`.

## Generated App Commands

Every generated app includes:

| Command                     | Description                                  |
| --------------------------- | -------------------------------------------- |
| `<app> <group> <operation>` | API operations grouped by tag                |
| `<app> auth login`          | Authenticate (bearer, API key, basic, OAuth) |
| `<app> auth logout`         | Clear stored credentials                     |
| `<app> auth status`         | Show current auth state (redacted)           |
| `<app> config show/set/get` | Manage app configuration                     |
| `<app> completion <shell>`  | Shell completions (bash/zsh/fish/powershell) |

### Global Flags

```
-o, --output-format    pretty|json|yaml|table|toon|go-template|jsonpath (default: pretty)
    --template         Go template or JSONPath expression (use with -o go-template|jsonpath)
    --template-file    Path to a Go template or JSONPath file
    --jq               Filter JSON output with a jq expression (no binary required)
    --output-file      Write response body to a file with progress bar
    --include-headers  Print response headers alongside the body
    --server           Override API server URL
    --dry-run          Show HTTP request without executing
    --debug            Log request/response to stderr (secrets redacted)
-y, --yes              Skip all confirmations
    --tui              Launch full TUI mode
    --agent            Force agent mode
    --no-retries       Disable retries
```

## Documentation

| Guide                                              | Description                                               |
| -------------------------------------------------- | --------------------------------------------------------- |
| [Getting Started](docs/getting-started.md)         | Install, generate, and run your first app                 |
| [Configuration](docs/configuration.md)             | cliford.yaml, OpenAPI extensions, per-operation overrides |
| [Overlays](docs/overlays.md)                       | Patch third-party specs with OAI Overlay files            |
| [Authentication](docs/authentication.md)           | Auth methods, keychain storage, profiles                  |
| [TUI Mode](docs/tui-mode.md)                       | Bubbletea TUI, theming, hybrid mode, mode detection       |
| [Pagination & Retries](docs/pagination-retries.md) | Auto-pagination, retry strategies, error handling         |
| [Custom Code Regions](docs/custom-code-regions.md) | Extend generated code that survives regeneration          |
| [Distribution](docs/distribution.md)               | GoReleaser, Homebrew, install scripts, SemVer             |
| [Hooks](docs/hooks.md)                             | Lifecycle and transform hooks                             |
| [OpenAPI Extensions](docs/openapi-extensions.md)   | All `x-cliford-*` extensions reference                    |
| [Generated App Reference](docs/generated-app-reference.md) | Global flags, output formats, jq filtering, file structure |

## Requirements

- Go 1.22+
- An OpenAPI 3.0 or 3.1 specification

## License

MIT
