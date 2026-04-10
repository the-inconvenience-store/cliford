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
-o, --output-format   pretty|json|yaml|table (default: pretty)
    --server          Override API server URL
    --dry-run         Show HTTP request without executing
    --debug           Log request/response to stderr (secrets redacted)
-y, --yes             Skip all confirmations
    --tui             Launch full TUI mode
    --agent           Force agent mode
    --no-retries      Disable retries
```

## Documentation

| Guide                                              | Description                                               |
| -------------------------------------------------- | --------------------------------------------------------- |
| [Getting Started](docs/getting-started.md)         | Install, generate, and run your first app                 |
| [Configuration](docs/configuration.md)             | cliford.yaml, OpenAPI extensions, per-operation overrides |
| [Authentication](docs/authentication.md)           | Auth methods, keychain storage, profiles                  |
| [TUI Mode](docs/tui-mode.md)                       | Bubbletea TUI, theming, hybrid mode, mode detection       |
| [Pagination & Retries](docs/pagination-retries.md) | Auto-pagination, retry strategies, error handling         |
| [Custom Code Regions](docs/custom-code-regions.md) | Extend generated code that survives regeneration          |
| [Distribution](docs/distribution.md)               | GoReleaser, Homebrew, install scripts, SemVer             |
| [Hooks](docs/hooks.md)                             | Lifecycle and transform hooks                             |
| [OpenAPI Extensions](docs/openapi-extensions.md)   | All `x-cliford-*` extensions reference                    |

## Requirements

- Go 1.22+
- An OpenAPI 3.0 or 3.1 specification

## License

MIT
