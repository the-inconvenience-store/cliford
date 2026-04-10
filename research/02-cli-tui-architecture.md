# CLI/TUI Architecture Research

> How Cobra, Bubbletea, Bubbles, and Lipgloss work together - and how to architect the CLI/TUI split.

## Technology Stack

### Cobra (CLI Framework)

Cobra is the de facto standard for Go CLI tools. It provides:
- Command tree structure (root command + nested subcommands)
- Automatic flag parsing (persistent flags, local flags)
- Built-in help generation
- Shell completions (bash, zsh, fish, PowerShell)
- Documentation generation (Markdown, Man pages, ReST)
- **LLM documentation**: `doc.GenMarkdownTree` generates one file per command path with usage, synopsis, examples, options, and inherited flags

Key Cobra doc features for LLMs:
- Populate `Example` fields with concrete input/output demos
- Add `Long` descriptions with context and rationale
- Use `GroupID` for logical command organization
- Set `DisableAutoGenTag = true` for reproducible docs (no timestamps)
- Structure with H2/H3 headings for vector index chunking
- Can inject front matter for static site compatibility

### Bubbletea (TUI Framework)

Bubbletea implements **The Elm Architecture** (TEA) in Go:

```
Model -> Update(msg) -> (Model, Cmd) -> View() -> string
```

- **Model**: Application state (any Go struct)
- **Update**: Processes messages (keypresses, timer ticks, HTTP responses), returns new state + optional command
- **View**: Renders UI string from current state (declarative, full re-render each time)
- **Cmd**: Async I/O operations (file reads, HTTP requests, timers)

Program lifecycle: `tea.NewProgram(initialModel())` enters loop: input -> update -> render -> repeat until `tea.Quit`.

Key features:
- Cell-based high-performance renderer
- Native keyboard + mouse support
- Clipboard integration
- Headless debugging via Delve
- File-based logging (`tea.LogToFile()`)

### Bubbles (TUI Components)

Pre-built components following the TEA pattern:

| Component | Purpose | CLI/TUI Relevance |
|-----------|---------|-------------------|
| **Spinner** | Loading indicators | Show during API calls |
| **Text Input** | Single-line input | Parameter prompts |
| **Text Area** | Multi-line input | JSON body editing |
| **Table** | Tabular data display | API response tables |
| **Progress** | Completion meter | Pagination progress, uploads |
| **Paginator** | Page navigation | List pagination UI |
| **Viewport** | Scrollable content | Long response bodies |
| **List** | Filterable item browser | Command explorer, resource lists |
| **File Picker** | Filesystem navigation | File upload parameters |
| **Timer/Stopwatch** | Time tracking | Request timing |
| **Help** | Auto-generated keybinding help | Always-on help bar |
| **Key** | Keybinding management | Customizable shortcuts |

### Lipgloss (Styling)

CSS-inspired declarative styling for terminals:

```go
var style = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#FAFAFA")).
    Background(lipgloss.Color("#7D56F4")).
    PaddingTop(2).
    Width(22)
```

Capabilities:
- **Colors**: ANSI 16, ANSI 256, True Color (auto-downsamples to terminal capability)
- **Text**: Bold, italic, underline (curly/double/dotted/dashed), strikethrough, blink, hyperlinks
- **Layout**: Padding, margins, alignment (left/right/center), width/height constraints
- **Borders**: Normal, rounded, thick, double, custom + gradient borders
- **Utilities**: Color darken/lighten/complementary, alpha/opacity
- **Style inheritance**: Styles can inherit unset rules from parents
- Styles are **immutable value types** (safe to copy)

---

## The CLI/TUI Split: Architecture Design

This is the critical design decision for Cliford. We need to support three modes:

### Mode 1: Pure CLI

Traditional command-line interface. No interactive TUI elements.

```
$ myapp users list --limit 10 --format json
[{"id": 1, "name": "Alice"}, ...]
```

**Implementation**: Cobra commands only. Execute API call, format output, exit.

### Mode 2: Pure TUI

Full-screen terminal application. All interaction happens through Bubbletea.

```
$ myapp
┌─────────────────────────────────┐
│ MyApp Explorer                  │
│                                 │
│ > Users                         │
│   Posts                         │
│   Settings                      │
│                                 │
│ [enter] select  [q] quit        │
└─────────────────────────────────┘
```

**Implementation**: Bubbletea program as the root command. Navigation, forms, and output all within the TUI.

### Mode 3: Hybrid CLI+TUI (Recommended Default)

CLI commands that enhance with TUI elements when running in a TTY.

```
# Non-interactive (piped/scripted)
$ myapp users create --name "Alice" --email "alice@example.com"

# Interactive (TTY detected, missing required params)
$ myapp users create
? Name: █
? Email: █
? Role: [admin/user/viewer]
```

**Implementation**: Cobra commands as the skeleton. When TTY is detected and params are missing, launch inline Bubbletea components for prompts. Full TUI explorer available as a subcommand or default when no subcommand given.

---

## Proposed Architecture: The Adapter Pattern

```
┌─────────────────────────────────────────────────┐
│                  Generated App                   │
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │  Cobra    │    │ Bubbletea│    │ Headless  │  │
│  │  Adapter  │    │ Adapter  │    │ Adapter   │  │
│  └─────┬────┘    └─────┬────┘    └─────┬────┘  │
│        │               │               │        │
│        └───────┬───────┴───────┬───────┘        │
│                │               │                 │
│         ┌──────▼──────┐ ┌─────▼──────┐          │
│         │  Operation  │ │   Theme    │          │
│         │  Registry   │ │   Engine   │          │
│         └──────┬──────┘ └─────┬──────┘          │
│                │              │                   │
│         ┌──────▼──────────────▼──────┐           │
│         │      Generated SDK         │           │
│         │    (from oapi-codegen)     │           │
│         └────────────────────────────┘           │
│                                                  │
└─────────────────────────────────────────────────┘
```

### Core Concepts

**Operation Registry**: A central registry that maps each API operation to its metadata:
- Operation ID, method, path
- Parameters (with types, required/optional)
- Request/response schemas
- Auth requirements
- Pagination config
- Custom hooks

**Adapters**: Each mode (CLI, TUI, Headless) is an adapter that reads from the Operation Registry and presents the interface accordingly:

- **Cobra Adapter**: Generates `cobra.Command` for each operation. Flags from params. Executes SDK call.
- **Bubbletea Adapter**: Generates TUI screens for each operation. Forms from params. Renders responses in styled views.
- **Headless Adapter**: No UI. Reads all input from flags/env/stdin. Used for scripting, CI, agent mode.

### Mode Selection Logic

```
if --tui flag or TUI_MODE=true in config:
    use Bubbletea Adapter (full TUI)
elif --no-interactive flag or -y flag or NO_TTY or piped stdin/stdout:
    use Headless Adapter
elif TTY detected:
    use Cobra Adapter with inline Bubbletea for prompts (hybrid)
else:
    use Headless Adapter
```

Config hierarchy for mode selection:
1. CLI flags (`--tui`, `--no-interactive`, `-y`)
2. Environment variables (`<PREFIX>_MODE=tui|cli|headless`)
3. User config file (`~/.config/<app>/config.yaml` -> `mode: tui`)
4. Auto-detection (TTY check)

### The -y Flag

The `-y` (or `--yes`) flag skips all user confirmations:
- Destructive operations proceed without confirmation
- Required params without defaults cause an error (not a prompt)
- Useful for scripting and CI/CD

### Inline TUI Components (Hybrid Mode)

When in hybrid mode, individual Bubbletea components run inline within Cobra commands:

```go
// Pseudocode for hybrid mode
func runCreateUser(cmd *cobra.Command, args []string) error {
    name, _ := cmd.Flags().GetString("name")
    if name == "" && isTTY() && !skipPrompts() {
        // Launch inline Bubbletea text input
        name = promptTextInput("Name", "Enter user name")
    }
    if name == "" {
        return fmt.Errorf("--name is required")
    }
    // ... call SDK
}
```

This uses Bubbletea's ability to run small, focused programs that return values.

### Full TUI Mode Components

For full TUI mode, the app is a single Bubbletea program with these views:

1. **Explorer View** (List bubble) - Browse available commands/operations
2. **Operation View** (Form) - Fill in parameters for a selected operation
3. **Response View** (Viewport bubble) - Display API response with syntax highlighting
4. **Config View** - Manage auth, server, preferences
5. **Help View** - Keybinding reference + command docs

Navigation: Tab between views, breadcrumb trail, back with Escape.

---

## Theme Engine Design

Developers configure themes via their Cliford config:

```yaml
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
  borders:
    style: rounded  # normal, rounded, thick, double
  components:
    spinner: dot     # dot, line, minidot, jump, pulse, points, globe, moon, monkey
    table:
      header_bold: true
      stripe_rows: true
```

The theme engine translates this config into Lipgloss styles that are injected into all TUI components.

**Key principle**: Developers define semantic colors and component preferences. Cliford translates these into the actual Lipgloss styles. End users of the generated app can also override the theme via their own config file.

---

## Documentation Generation

Cobra's `doc` package generates documentation, enhanced for LLM consumption:

```go
// In cmd/docgen/main.go (generated)
func main() {
    rootCmd := cmd.RootCmd()
    rootCmd.DisableAutoGenTag = true
    
    // Markdown for humans
    doc.GenMarkdownTree(rootCmd, "./docs/cli")
    
    // llms.txt for AI agents
    generateLLMsDocs(rootCmd, "./docs/llms.txt")
}
```

The `llms.txt` format provides:
- Flat list of all commands with full usage
- Structured examples
- Parameter schemas
- Auth requirements
- Optimized for context window consumption

This integrates into the build pipeline via `go generate` or a dedicated `docs` subcommand.

---

## Key Design Decisions for Cliford

1. **Default to hybrid mode** - Best of both worlds, graceful degradation
2. **Adapter pattern** - Clean separation allows adding new modes later
3. **Operation Registry** - Single source of truth, all adapters read from it
4. **Theme at config level** - Semantic colors, not raw Lipgloss calls
5. **Inline Bubbletea** - Small focused TUI components within CLI commands
6. **Full TUI as opt-in** - `--tui` flag or config setting, not the default
7. **Auto-generate docs** - Both human-readable and LLM-optimized
