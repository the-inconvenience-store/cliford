# Cliford: High-Level Architecture Plan

> An OSS CLI tool & library that generates Go-powered CLI and TUI applications from OpenAPI specifications.

## Vision

Cliford takes an OpenAPI spec and produces a fully functional, beautifully styled, production-ready Go application that can operate as a pure CLI, a full TUI, or a hybrid of both. It is completely configurable, extensible via hooks, and generates apps that are themselves configurable by their end users.

**Think**: "Speakeasy CLI generation, but open source, with first-class TUI support, and a richer extensibility model."

## Core Principles

1. **OpenAPI as the source of truth** - The spec drives everything, but config isn't limited to it
2. **Sensible defaults, full customizability** - Works great out of the box, infinitely tweakable
3. **SDK-first architecture** - Generate the SDK, then build CLI/TUI on top of it
4. **Three modes, one codebase** - Pure CLI, pure TUI, or hybrid from the same generation
5. **Custom code survives regeneration** - Custom code regions let devs extend without forking
6. **Developer experience above all** - The tool should be a joy to use, and the apps it creates should be a joy to use

---

## Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| SDK Generation | **oapi-codegen** | OpenAPI -> Go types + HTTP client |
| CLI Framework | **Cobra** | Command tree, flags, help, completions |
| TUI Framework | **Bubbletea** | Elm architecture TUI applications |
| TUI Components | **Bubbles** | Pre-built UI components (inputs, tables, lists, etc.) |
| TUI Styling | **Lipgloss** | CSS-inspired terminal styling |
| Configuration | **Viper** | Config files, env vars, flags (both Cliford and generated apps) |
| Distribution | **GoReleaser** | Cross-platform builds, Homebrew, WinGet, deb/rpm |
| Auth Storage | **go-keyring** | OS keychain integration |

---

## Generation Pipeline

```
                        ┌─────────────────────────────────────────────────────────────┐
                        │                    CLIFORD PIPELINE                          │
                        │                                                              │
 ┌──────────┐          │  ┌─────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐  │   ┌──────────────┐
 │ OpenAPI  │──────────>│  │ Parse & │──>│ Generate │──>│ Generate │──>│ Generate │  │──>│  Complete    │
 │ Spec     │          │  │ Validate│   │ SDK      │   │ CLI/TUI  │   │ Infra    │  │   │  Go App      │
 └──────────┘          │  └─────────┘   └──────────┘   └──────────┘   └──────────┘  │   └──────────────┘
                        │       │              │              │              │         │
 ┌──────────┐          │  ┌────▼────┐   ┌─────▼────┐   ┌────▼─────┐  ┌────▼─────┐  │
 │cliford   │──────────>│  │Operation│   │oapi-     │   │Cobra +   │  │GoReleaser│  │
 │.yaml     │          │  │Registry │   │codegen + │   │Bubbletea │  │+ Docs +  │  │
 └──────────┘          │  │Builder  │   │Enhance-  │   │+ Lipgloss│  │Scripts   │  │
                        │  └─────────┘   │ments     │   └──────────┘  └──────────┘  │
                        │                └──────────┘                                │
                        │                                                              │
                        │  Hooks run at ──> before/after each stage                    │
                        └─────────────────────────────────────────────────────────────┘
```

### Pipeline Stages

#### Stage 1: Parse & Validate
- Parse OpenAPI spec (3.0/3.1)
- Parse `cliford.yaml` config
- Extract `x-cliford-*` extensions from spec
- Merge configs (cliford.yaml > OpenAPI extensions > defaults)
- Build Operation Registry (central metadata store)
- Validate everything
- **Hooks**: `before:generate`, `after:validate`

#### Stage 2: Generate SDK
- Invoke oapi-codegen (as Go library) for types + client
- Post-process: add retry wrappers, pagination helpers, auth middleware
- Generate enhanced error types
- Write custom code region markers
- **Hooks**: `before:sdk`, `after:sdk`, `transform:operation`

#### Stage 3: Generate CLI/TUI
- Generate Cobra commands from Operation Registry
- Generate Bubbletea models for TUI mode
- Generate Lipgloss theme from config
- Generate hybrid-mode inline prompts
- Wire up SDK calls, auth, pagination, output formatting
- Generate mode-switching logic (CLI/TUI/headless detection)
- **Hooks**: `before:cli`, `after:cli`, `before:tui`, `after:tui`, `transform:command`, `transform:model`

#### Stage 4: Generate Infrastructure
- Generate `main.go` with Viper config init
- Generate config commands (`config show/set/get/edit`)
- Generate auth commands (`auth login/logout/status`)
- Generate documentation (Markdown + llms.txt)
- Generate GoReleaser config
- Generate GitHub Actions workflows
- Generate install scripts
- Generate shell completions
- **Hooks**: `before:docs`, `after:docs`, `after:generate`

---

## Generated App Structure

```
myapp/
├── cmd/
│   ├── main.go                 # Entrypoint
│   └── docgen/
│       └── main.go             # Documentation generator
├── internal/
│   ├── sdk/
│   │   ├── sdk.gen.go          # oapi-codegen output (DO NOT EDIT)
│   │   ├── client.go           # Enhanced client wrapper
│   │   ├── pagination.go       # Pagination helpers
│   │   ├── errors.go           # Error types
│   │   └── middleware.go       # Auth, retry, logging middleware
│   ├── cli/
│   │   ├── root.go             # Root Cobra command
│   │   ├── auth.go             # Auth commands
│   │   ├── config.go           # Config commands
│   │   ├── users.go            # Per-tag command groups
│   │   ├── users_list.go       # Per-operation commands
│   │   ├── users_create.go
│   │   └── ...
│   ├── tui/
│   │   ├── app.go              # Main Bubbletea program
│   │   ├── explorer.go         # Command explorer view
│   │   ├── operation.go        # Operation form view
│   │   ├── response.go         # Response display view
│   │   ├── theme.go            # Lipgloss theme engine
│   │   └── components/
│   │       ├── confirm.go      # Confirmation dialog
│   │       ├── prompt.go       # Inline prompt (for hybrid mode)
│   │       └── status.go       # Status bar
│   ├── config/
│   │   ├── config.go           # Viper setup
│   │   ├── keychain.go         # Credential storage
│   │   └── profiles.go         # Profile management
│   ├── output/
│   │   ├── formatter.go        # Output formatting (json/yaml/table/pretty)
│   │   ├── pager.go            # Pager integration
│   │   └── agent.go            # Agent mode detection
│   └── registry/
│       └── operations.go       # Operation metadata registry
├── docs/
│   ├── cli/                    # Generated Markdown docs
│   └── llms.txt                # LLM-optimized docs
├── .goreleaser.yaml
├── .github/
│   └── workflows/
│       └── release.yaml
├── go.mod
├── go.sum
├── install.sh
├── install.ps1
├── cliford.lock                # Generation lockfile (tracks what was generated)
└── CUSTOM_CODE.md              # Guide for developers on custom code regions
```

---

## App Modes: The Three-Way Split

### Detection & Override Logic

```
Priority (highest to lowest):
1. Explicit flag: --tui, --no-interactive, -y
2. Environment: MYAPP_MODE=tui|cli|headless
3. Config file: mode: tui
4. Auto-detection:
   a. Agent environment detected (Claude Code, Cursor, etc.) -> headless
   b. No TTY (piped stdin/stdout) -> headless
   c. TTY detected -> hybrid (CLI with inline TUI prompts)
```

### Mode Behaviors

| Feature | Pure CLI | Hybrid | Pure TUI | Headless |
|---------|----------|--------|----------|----------|
| Cobra commands | Yes | Yes | No (TUI navigation) | Yes |
| Interactive prompts | No | Yes (missing params) | Yes (all params) | No |
| TUI explorer | No | Yes (default when no subcommand) | Yes (home screen) | No |
| Confirmations | No (fail on destructive) | Yes | Yes | No (fail or `-y`) |
| Output formatting | `--output-format` | `--output-format` | Styled views | JSON only |
| Shell completions | Yes | Yes | No | Yes |
| Agent detection | No | Yes | No | Yes |
| Pager | Yes | Yes | Built-in viewport | No |

### The -y Flag

`-y` / `--yes` is a global persistent flag:
- Skips all confirmation prompts
- Auto-selects defaults for optional interactive inputs
- Required params without values cause error (not prompt)
- Useful for scripting, CI, piping into other commands

---

## Configuration Architecture

### Dual Config System

```
┌───────────────────────────────────────────────────────────┐
│                    DEVELOPER TIME                          │
│                                                            │
│  cliford.yaml ──────> Cliford Tool ──────> Generated App   │
│  + OpenAPI spec           │                                │
│  + cliford.go (optional)  │                                │
│                           │                                │
│  Resolution:              │                                │
│  cliford.yaml op-level    │                                │
│  > x-cliford-* extension  │                                │
│  > cliford.yaml globals   │                                │
│  > Cliford defaults       │                                │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│                    USER TIME                               │
│                                                            │
│  CLI flags ──────> Generated App ──────> API Call           │
│  > Env vars          │                                     │
│  > Config file       │ Viper resolves                      │
│  > Defaults          │                                     │
│                                                            │
│  Auth resolution:                                          │
│  Flags > Env > Keychain > Config file                      │
└───────────────────────────────────────────────────────────┘
```

### Config Approaches (Supported in Parallel)

1. **`cliford.yaml`** - Primary config file (90% of users)
2. **`x-cliford-*` OpenAPI extensions** - Per-operation inline config
3. **`cliford.go`** - Go DSL for power users / complex hooks

---

## Authentication Architecture

### Supported Methods (from OpenAPI `securitySchemes`)

- API Key (header, query, cookie)
- HTTP Basic
- Bearer Token
- OAuth 2.0 (Authorization Code, Client Credentials, Device Code)
- OpenID Connect
- mTLS (via hooks)
- Custom schemes (via hooks)

### Credential Storage

```
OS Keychain (preferred)
  └──> Encrypted file fallback (containers/CI)
         └──> Environment variables
                └──> Config file (with warning)
```

### Auth Commands

```
myapp auth login       # Interactive login
myapp auth logout      # Clear credentials
myapp auth status      # Show auth state
myapp auth switch      # Switch profiles
myapp auth refresh     # Force OAuth token refresh
```

### Multi-Profile Support

Each profile has independent server URL, auth method, and credentials.

---

## Hook System

### Hook Points

```
before:generate ─> before:validate ─> after:validate
    ─> before:sdk ─> transform:operation ─> after:sdk
    ─> before:cli ─> transform:command ─> after:cli
    ─> before:tui ─> transform:model ─> transform:style ─> after:tui
    ─> before:docs ─> after:docs
    ─> after:generate
```

### Hook Definition Methods

```yaml
# cliford.yaml - Shell commands
hooks:
  after:generate:
    - run: "gofmt -w ."
    - run: "go vet ./..."
```

```go
// cliford.go - Go functions
config.Hook("transform:command", func(ctx *hook.Context, cmd *CommandMeta) error {
    if cmd.OperationID == "deleteUser" {
        cmd.Confirm = true
        cmd.ConfirmMessage = "Are you sure? This cannot be undone."
    }
    return nil
})
```

---

## Theme System

### Developer Configuration

```yaml
# cliford.yaml
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
  borders: rounded        # normal | rounded | thick | double
  spinner: dot            # dot | line | minidot | jump | pulse
  table:
    headerBold: true
    stripeRows: true
  compact: false          # Reduce padding/margins
```

### End-User Overrides

```yaml
# ~/.config/myapp/config.yaml
tui:
  theme: dark             # Preset: dark | light | default
  # Or custom overrides:
  colors:
    primary: "#00FF00"
  animations: false
  mouse: true
```

### Theme Engine Implementation

Lipgloss styles generated from theme config:

```go
// Generated in internal/tui/theme.go
type Theme struct {
    Primary    lipgloss.Style
    Secondary  lipgloss.Style
    Accent     lipgloss.Style
    Error      lipgloss.Style
    Success    lipgloss.Style
    Border     lipgloss.Border
    // ... component-specific styles
}

func LoadTheme(cfg *viper.Viper) *Theme {
    return &Theme{
        Primary: lipgloss.NewStyle().
            Foreground(lipgloss.Color(cfg.GetString("theme.colors.primary"))),
        Border: lipgloss.RoundedBorder(), // from cfg
        // ...
    }
}
```

---

## Documentation Generation

### Auto-Generated Docs

1. **Markdown CLI docs** - One file per command, Cobra `doc` package
2. **llms.txt** - LLM-optimized flat documentation
3. **Man pages** - Optional, via Cobra
4. **JSON Schema** - For config file validation + IDE support
5. **`--usage` flag** - Machine-readable command schema (KDL or JSON)
6. **Shell completions** - bash, zsh, fish, PowerShell

### Build Integration

```go
//go:generate go run ./cmd/docgen -out ./docs/cli -format markdown
//go:generate go run ./cmd/docgen -out ./docs/llms.txt -format llms
```

---

## SemVer & Release Management

### Version Strategy

- Version tracked in `cliford.yaml` and injected via ldflags at build time
- GoReleaser handles cross-compilation, checksums, changelogs
- GitHub Actions workflow for automated releases on tag push

### Auto-Version Bumping

```
cliford version bump auto   # Diff OpenAPI specs, determine bump type
cliford version bump patch  # Manual bump
```

Auto-detection:
- Operations added -> minor
- Operations removed/signature changed -> major
- Descriptions/metadata only -> patch

---

## Cliford CLI Interface

```
cliford init                      # Initialize new project from OpenAPI spec
cliford generate                  # Run the generation pipeline
cliford generate --dry-run        # Show what would be generated
cliford diff                      # Show changes since last generation
cliford validate                  # Validate cliford.yaml + OpenAPI spec
cliford version bump [auto|patch|minor|major]
cliford config show               # Show current Cliford config
cliford config init               # Create cliford.yaml interactively
cliford plugin install <name>     # Install a hook plugin
cliford doctor                    # Check dependencies, config, report issues
```

---

## Roadmap (Phased Implementation)

### Phase 1: Foundation (MVP)
- [ ] OpenAPI spec parsing + validation
- [ ] Operation Registry
- [ ] oapi-codegen integration (types + client)
- [ ] Basic Cobra CLI generation (commands, flags, help)
- [ ] Viper config for generated apps
- [ ] JSON/pretty/table output formatting
- [ ] API Key + Bearer Token auth
- [ ] Basic error handling
- [ ] `cliford init` + `cliford generate` commands

### Phase 2: Enhanced CLI
- [ ] All auth methods (OAuth, Basic, OIDC)
- [ ] Keychain integration
- [ ] Retry logic with exponential backoff
- [ ] Pagination (offset, cursor, link-based)
- [ ] Custom code regions
- [ ] Server configuration + profiles
- [ ] Global parameters
- [ ] `--dry-run`, `--debug` flags
- [ ] Shell completions
- [ ] Markdown doc generation

### Phase 3: TUI & Hybrid Mode
- [ ] Bubbletea TUI app generation
- [ ] Lipgloss theme engine
- [ ] Explorer view (List bubble)
- [ ] Operation form view
- [ ] Response viewport
- [ ] Hybrid mode (inline Bubbletea prompts in CLI)
- [ ] Mode detection logic (TTY, agent, flags)
- [ ] `-y` flag implementation
- [ ] Agent mode (output format, suppress interactive)

### Phase 4: Polish & Distribution
- [ ] GoReleaser integration
- [ ] Homebrew / WinGet / nFPM support
- [ ] llms.txt generation
- [ ] SemVer auto-bumping
- [ ] `cliford diff` command
- [ ] `cliford doctor` command
- [ ] Hook system (shell commands)
- [ ] JSON Schema for config files
- [ ] Install scripts generation

### Phase 5: Extensibility
- [ ] Go DSL config (`cliford.go`)
- [ ] Go function hooks
- [ ] Plugin system
- [ ] Alternative SDK generators (ogen)
- [ ] Transform hooks (command, model, style)
- [ ] Community theme marketplace
- [ ] Multi-spec support (generate from multiple OpenAPI specs)

---

## Open Questions

1. **Should Cliford itself be a TUI?** - Should `cliford init` and `cliford generate` have their own TUI mode? Probably yes for `init` (interactive wizard), maybe not for `generate`.

2. **How much of oapi-codegen's config should we expose?** - We wrap oapi-codegen, but should users be able to pass through all oapi-codegen options or just a curated subset?

3. **Plugin distribution** - How should third-party plugins be distributed? Go plugins? Separate binaries? A registry?

4. **Monorepo or multi-repo?** - Should Cliford be a single repo or split (core, plugins, themes)?

5. **Template engine** - Should we use Go templates for code generation, or a more structured AST-based approach? Templates are more accessible; AST is more reliable.

6. **Testing generated apps** - Should Cliford generate test scaffolding for the generated app? Mock servers? Integration test helpers?

7. **Backwards compatibility** - When Cliford updates its templates, how do we handle regenerating existing apps without breaking custom code? The lockfile + custom code regions help, but edge cases will exist.

---

## Research Documents Index

| # | Document | Focus |
|---|----------|-------|
| 01 | [Speakeasy Analysis](./01-speakeasy-analysis.md) | Prior art, DX patterns, gaps |
| 02 | [CLI/TUI Architecture](./02-cli-tui-architecture.md) | Cobra + Bubbletea + mode split |
| 03 | [Authentication Systems](./03-authentication-systems.md) | Auth methods, storage, flows |
| 04 | [Configuration Systems](./04-configuration-systems.md) | Viper, OpenAPI extensions, hooks |
| 05 | [oapi-codegen Integration](./05-oapi-codegen-integration.md) | SDK generation strategy |
| 06 | [Runtime Features](./06-runtime-features.md) | Pagination, retries, errors, SemVer |
