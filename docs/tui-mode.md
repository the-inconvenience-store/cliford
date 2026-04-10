# TUI Mode

Cliford generates interactive terminal user interfaces using
[Bubbletea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles), and
[Lipgloss](https://github.com/charmbracelet/lipgloss).

## Enabling TUI Generation

```bash
cliford generate --tui
```

Or in `cliford.yaml`:

```yaml
generation:
  tui:
    enabled: true
```

This generates a full `internal/tui/` package with explorer, operation forms,
response views, theme engine, and shared components.

## Three Execution Modes

Generated apps support three modes, resolved by this precedence chain:

| Priority | Source | Result |
|----------|--------|--------|
| 1 | `--agent` flag | Headless (JSON, no prompts) |
| 2 | `--tui` flag | Full TUI |
| 3 | `--no-interactive` or `-y` flag | Headless |
| 4 | `<PREFIX>_MODE` env var | tui / cli / headless |
| 5 | Agent environment detected | Headless |
| 6 | No TTY (piped stdin/stdout) | Headless |
| 7 | TTY detected | Hybrid (CLI + inline prompts) |

### Pure CLI

Traditional command-line. No interactive elements.

```bash
myapp pets list --limit 10 -o json
```

### Hybrid Mode (default when TTY)

CLI commands with inline Bubbletea prompts for missing required parameters.
When no subcommand is given, the TUI explorer launches.

```bash
myapp pets create
# ? Name: █           <- inline Bubbletea prompt
# ? Species: [dog/cat/bird/fish/other]
```

### Full TUI Mode

Full-screen Bubbletea application with keyboard navigation.

```bash
myapp --tui
```

### Headless Mode

No interactive elements. Structured JSON output. Used for scripting, CI,
and AI agent contexts.

```bash
myapp pets list -y -o json | jq '.[] | .name'
```

## Agent Detection

The generated app auto-detects these AI agent environments and switches to
headless mode:

- Claude Code (`CLAUDE_CODE`)
- Cursor (`CURSOR_SESSION_ID`)
- OpenAI Codex CLI (`CODEX_SESSION`)
- Aider (`AIDER_MODEL`)
- Cline (`CLINE_TASK`)
- Windsurf (`WINDSURF_SESSION`)
- GitHub Copilot (`COPILOT_AGENT`)
- Amazon Q (`AMAZON_Q_SESSION`)
- Gemini Code Assist (`GEMINI_CODE_ASSIST`)
- Sourcegraph Cody (`CODY_SESSION`)

Override with `--agent` flag for environments not yet in the detection list.

## TUI Views

### Explorer

The home screen. A filterable list of all API operations grouped by tag.
Navigate with arrow keys, filter by typing, select with Enter.

### Operation Form

After selecting an operation, a parameter form appears with appropriate
input controls for each parameter type. Press Enter to submit.

### Response View

API responses display in a scrollable viewport. Lists render as tables,
single objects as detail views. Status codes are color-coded (green for
2xx, red for 4xx/5xx).

## Theming

Configure TUI colors and styles in `cliford.yaml`:

```yaml
theme:
  colors:
    primary: "#7D56F4"     # Titles, highlights
    secondary: "#FF6B6B"   # Secondary elements
    accent: "#4ECDC4"      # Keybinding labels
    dimmed: "#666666"       # Subtle text
    error: "#FF4444"        # Error messages
    success: "#44FF44"      # Success indicators
    warning: "#FFAA00"      # Warnings
  borders: rounded          # normal | rounded | thick | double
  spinner: dot              # Loading animation style
  table:
    headerBold: true
    stripeRows: true
  compact: false            # Reduce padding
```

Cliford translates these semantic colors into Lipgloss styles at generation
time. End users of the generated app can override the theme via their own
config file at `~/.config/<app>/config.yaml`.

## Generated TUI Files

```
internal/tui/
├── app.go          # Root Bubbletea model with view routing
├── explorer.go     # Filterable operation list (Bubbles List)
├── operation.go    # Parameter form (Bubbles TextInput)
├── response.go     # Scrollable response viewer (Bubbles Viewport)
├── theme.go        # Lipgloss theme engine
└── components.go   # StatusBar, ErrorBar, HelpBar
```
